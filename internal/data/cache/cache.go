package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

type CacheMissReason int

const (
	MissReasonNone CacheMissReason = iota
	MissReasonError
	MissReasonInode
	MissReasonSize
	MissReasonModTime
	MissReasonFingerprint
	MissReasonNoFingerprint
	MissReasonNotFound
)

type CacheResult struct {
	Data       *aggregator.AggregatedData
	Found      bool
	MissReason CacheMissReason
}

type Cache interface {
	Get(sessionId string) CacheResult
	Set(sessionId string, data *aggregator.AggregatedData) error
	Clear() error
	Preload() error
	BatchValidate(sessionIds []string) map[string]BatchValidateResult
}

type FileCache struct {
	baseDir     string
	mu          sync.RWMutex
	memoryCache map[string]*aggregator.AggregatedData
}

func NewFileCache(baseDir string) (*FileCache, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	return &FileCache{
		baseDir:     baseDir,
		memoryCache: make(map[string]*aggregator.AggregatedData),
	}, nil
}

// extractSessionId extracts the session ID from a file path
// e.g., "/path/to/00aec530-0614-436f-a53b-faaa0b32f123.jsonl" -> "00aec530-0614-436f-a53b-faaa0b32f123"
func extractSessionId(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func (c *FileCache) Get(sessionId string) CacheResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// First, check memory cache
	if memData, exists := c.memoryCache[sessionId]; exists {
		if ret := c.validateCachedData(memData); ret.cached {
			return CacheResult{Data: memData, Found: true, MissReason: MissReasonNone}
		} else {
			// Remove invalid entry from memory cache
			delete(c.memoryCache, sessionId)
		}
	}

	// Second, check file cache
	return c.getFromFile(sessionId)
}

func (c *FileCache) getFromFile(sessionId string) CacheResult {
	// Use session ID based filename
	cachePath := filepath.Join(c.baseDir, sessionId+".json")

	file, err := os.Open(cachePath)
	if err != nil {
		return CacheResult{Data: nil, Found: false, MissReason: MissReasonNotFound}
	}
	defer file.Close()

	var data aggregator.AggregatedData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return CacheResult{Data: nil, Found: false, MissReason: MissReasonError}
	}

	// Ensure SessionId is set
	if data.SessionId == "" && data.FilePath != "" {
		data.SessionId = extractSessionId(data.FilePath)
	}

	if ret := c.validateCachedData(&data); !ret.cached {
		return CacheResult{Data: nil, Found: false, MissReason: ret.reason}
	}

	// Add valid data to memory cache for future access
	c.memoryCache[sessionId] = &data

	return CacheResult{Data: &data, Found: true, MissReason: MissReasonNone}
}

type ValidateResult struct {
	cached bool
	reason CacheMissReason
}

func (c *FileCache) validateCachedData(data *aggregator.AggregatedData) ValidateResult {
	currentInfo, err := util.GetFileInfo(data.FilePath)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Cache validation failed for %s: unable to get file info: %v", data.FilePath, err))
		return ValidateResult{cached: false, reason: MissReasonError}
	}

	// Step 1: Check inode/modtime/size
	if currentInfo.Inode != data.Inode {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: inode changed (cached: %d, current: %d)",
			data.FilePath, data.Inode, currentInfo.Inode))
		return ValidateResult{cached: false, reason: MissReasonInode}
	}
	if currentInfo.Size != data.FileSize {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: size changed (cached: %d, current: %d)",
			data.FilePath, data.FileSize, currentInfo.Size))
		return ValidateResult{cached: false, reason: MissReasonSize}
	}
	if currentInfo.ModTime != data.LastModified {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: modtime changed (cached: %d, current: %d)",
			data.FilePath, data.LastModified, currentInfo.ModTime))
		return ValidateResult{cached: false, reason: MissReasonModTime}
	}

	// Step 2: If file modification time is two days ago, skip fingerprint check
	modTime := time.Unix(currentInfo.ModTime, 0)
	if time.Since(modTime) > 48*time.Hour {
		return ValidateResult{cached: true, reason: MissReasonNone}
	}

	// Step 3: Check content fingerprint
	if data.ContentFingerprint == "" {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: no fingerprint in cached data", data.FilePath))
		return ValidateResult{cached: false, reason: MissReasonNoFingerprint}
	}

	fingerprint, err := util.CalculateFileFingerprint(data.FilePath)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: unable to calculate fingerprint: %v", data.FilePath, err))
		return ValidateResult{cached: false, reason: MissReasonNoFingerprint}
	}

	if fingerprint == data.ContentFingerprint {
		return ValidateResult{cached: true, reason: MissReasonNone}
	} else {
		util.LogDebug(fmt.Sprintf("Cache invalidated for %s: fingerprint mismatch (cached: %s, current: %s)",
			data.FilePath, data.ContentFingerprint, fingerprint))
		return ValidateResult{cached: false, reason: MissReasonFingerprint}
	}
}

func (c *FileCache) Set(sessionId string, data *aggregator.AggregatedData) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use enhanced file info retrieval
	fileInfo, err := util.GetFileInfo(data.FilePath)
	if err != nil {
		return err
	}

	data.LastModified = fileInfo.ModTime
	data.FileSize = fileInfo.Size
	data.Inode = fileInfo.Inode

	// Calculate content fingerprint
	fingerprint, err := util.CalculateFileFingerprint(data.FilePath)
	if err == nil {
		data.ContentFingerprint = fingerprint
	}

	// Ensure session ID is set in data
	if data.SessionId == "" {
		data.SessionId = sessionId
	}

	// Write to file cache first - use session ID as filename
	cachePath := filepath.Join(c.baseDir, sessionId+".json")
	file, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return err
	}

	// Update memory cache atomically
	c.memoryCache[sessionId] = data

	return nil
}

func (c *FileCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear memory cache
	c.memoryCache = make(map[string]*aggregator.AggregatedData)

	// Clear file cache - removes all .json files (both hash-based and session-id-based)
	return filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			os.Remove(path)
		}

		return nil
	})
}

func (c *FileCache) Preload() error {
	util.LogInfo("Start preloading cache files into memory...")

	// Get all cache files
	var cacheFiles []string
	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".json") {
			cacheFiles = append(cacheFiles, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("Failed to scan cache directory: %w", err)
	}

	if len(cacheFiles) == 0 {
		util.LogInfo("Cache directory is empty, skipping preload")
		return nil
	}

	util.LogInfo(fmt.Sprintf("Found %d cache files, starting concurrent loading...", len(cacheFiles)))

	// Use worker pool for concurrent loading
	numWorkers := runtime.NumCPU()
	if numWorkers > len(cacheFiles) {
		numWorkers = len(cacheFiles)
	}

	util.LogInfo(fmt.Sprintf("Using %d worker threads for concurrent loading", numWorkers))

	filesChan := make(chan string, len(cacheFiles))
	resultsChan := make(chan preloadResult, len(cacheFiles))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go c.preloadWorker(filesChan, resultsChan, &wg)
	}

	// Send files to workers
	for _, file := range cacheFiles {
		filesChan <- file
	}
	close(filesChan)

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results with progress reporting
	loaded := 0
	invalid := 0
	errors := 0
	processed := 0
	total := len(cacheFiles)

	c.mu.Lock()
	for result := range resultsChan {
		processed++

		if result.err != nil {
			errors++
			util.LogWarn(fmt.Sprintf("Failed to preload cache file %s: %v", result.filePath, result.err))
		} else if result.data != nil && c.validateCachedData(result.data).cached {
			c.memoryCache[result.sessionId] = result.data
			loaded++
		} else {
			invalid++
		}

		// Report progress every 100 files or at the end
		if processed%100 == 0 || processed == total {
			util.LogInfo(fmt.Sprintf("Preload progress: %d/%d (%.1f%%) - Success:%d Invalid:%d Errors:%d",
				processed, total, float64(processed)/float64(total)*100, loaded, invalid, errors))
		}
	}
	c.mu.Unlock()

	util.LogInfo(fmt.Sprintf("Cache preload complete: %d loaded, %d invalid, %d errors (total %d)",
		loaded, invalid, errors, total))

	if loaded > 0 {
		util.LogInfo(fmt.Sprintf("Memory cache is ready with %d valid entries", loaded))
	}
	return nil
}

type preloadResult struct {
	filePath  string
	sessionId string
	data      *aggregator.AggregatedData
	err       error
}

func (c *FileCache) preloadWorker(filesChan <-chan string, resultsChan chan<- preloadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range filesChan {
		result := preloadResult{filePath: filePath}

		// Extract session ID from filename
		fileName := filepath.Base(filePath)
		if strings.HasSuffix(fileName, ".json") {
			result.sessionId = strings.TrimSuffix(fileName, ".json")
		} else {
			result.err = fmt.Errorf("Invalid cache file name format")
			resultsChan <- result
			continue
		}

		// Load and parse file
		file, err := os.Open(filePath)
		if err != nil {
			result.err = err
			resultsChan <- result
			continue
		}

		var data aggregator.AggregatedData
		err = json.NewDecoder(file).Decode(&data)
		file.Close()

		if err != nil {
			result.err = err
			resultsChan <- result
			continue
		}

		// If SessionId is empty (old cache files), extract it from FilePath
		if data.SessionId == "" && data.FilePath != "" {
			data.SessionId = extractSessionId(data.FilePath)
		}

		result.data = &data
		resultsChan <- result
	}
}

func (c *FileCache) GetCacheStats() (memoryCount, fileCount int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	memoryCount = len(c.memoryCache)

	// Count file cache entries
	filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".json") {
			fileCount++
		}
		return nil
	})

	return memoryCount, fileCount
}

func (c *FileCache) ValidateCache(files []string) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	valid := make(map[string]bool)

	for _, file := range files {
		sessionId := extractSessionId(file)
		result := c.Get(sessionId)
		valid[file] = result.Found && result.Data != nil
	}

	return valid
}

type BatchValidateResult struct {
	Valid      bool
	MissReason CacheMissReason
}

func (c *FileCache) BatchValidate(sessionIds []string) map[string]BatchValidateResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]BatchValidateResult, len(sessionIds))

	// Initialize all with NotFound reason
	for _, sessionId := range sessionIds {
		result[sessionId] = BatchValidateResult{Valid: false, MissReason: MissReasonNotFound}
	}

	// Batch validate entries
	for _, sessionId := range sessionIds {
		// Check memory cache first
		if memData, exists := c.memoryCache[sessionId]; exists {
			validateResult := c.validateCachedData(memData)
			result[sessionId] = BatchValidateResult{
				Valid:      validateResult.cached,
				MissReason: validateResult.reason,
			}
		} else {
			// Check file cache
			cacheResult := c.getFromFile(sessionId)
			result[sessionId] = BatchValidateResult{
				Valid:      cacheResult.Found,
				MissReason: cacheResult.MissReason,
			}
		}
	}

	validCount := 0
	for _, r := range result {
		if r.Valid {
			validCount++
		}
	}

	util.LogDebug(fmt.Sprintf("Batch validation complete: %d files, %d valid",
		len(sessionIds), validCount))

	return result
}

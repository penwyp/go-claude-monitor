package top

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/cache"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	datacache "github.com/penwyp/go-claude-monitor/internal/data/cache"
	"github.com/penwyp/go-claude-monitor/internal/data/parser"
	"github.com/penwyp/go-claude-monitor/internal/data/scanner"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// DataLoader handles all data loading and caching operations
type DataLoader struct {
	config        *TopConfig
	sessionConfig session.SessionConfig
	fileCache     datacache.Cache
	memoryCache   *cache.MemoryCache
	scanner       *scanner.FileScanner
	parser        *parser.Parser
	aggregator    *aggregator.Aggregator
}

// NewDataLoader creates a new DataLoader instance
func NewDataLoader(config *TopConfig) (*DataLoader, error) {
	// Initialize file cache
	fileCache, err := datacache.NewFileCache(config.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create file cache: %w", err)
	}

	// Create aggregator with pricing configuration
	agg, err := aggregator.NewAggregatorWithConfig(
		config.PricingSource,
		config.PricingOfflineMode,
		config.CacheDir,
		config.Timezone,
	)
	if err != nil {
		util.LogError("Failed to create aggregator with pricing config: " + err.Error())
		// Fallback to default aggregator
		agg = aggregator.NewAggregatorWithTimezone(config.Timezone)
	}

	// Get session configuration
	sessionConfig := session.GetSessionConfig()

	return &DataLoader{
		config:        config,
		sessionConfig: sessionConfig,
		fileCache:     fileCache,
		memoryCache:   cache.NewMemoryCache(),
		scanner:       scanner.NewFileScanner(config.DataDir),
		parser:        parser.NewParser(config.Concurrency),
		aggregator:    agg,
	}, nil
}

// Preload loads cache and recent data
func (dl *DataLoader) Preload() error {
	util.LogInfo("Preloading cache and recent data...")

	// 1. Preload file cache to memory
	if err := dl.fileCache.Preload(); err != nil {
		util.LogWarn(fmt.Sprintf("Cache preload warning: %v", err))
	}

	// 2. Scan recent files
	files, err := dl.ScanRecentFiles()
	if err != nil {
		return err
	}

	util.LogInfo(fmt.Sprintf("Found %d files to process", len(files)))

	// 3. Load data in parallel
	return dl.LoadFiles(files)
}

// ScanRecentFiles scans for recent files based on configuration
func (dl *DataLoader) ScanRecentFiles() ([]string, error) {
	// Get all files
	allFiles, err := dl.scanner.Scan()
	if err != nil {
		return nil, err
	}

	// Apply configuration-based filtering
	switch dl.sessionConfig.TimelineMode {
	case "full":
		// Return ALL files without time filtering
		return allFiles, nil
	case "recent":
		// Legacy behavior: filter by recent modification time
		cutoff := time.Now().Add(-48 * time.Hour).Unix()
		var recentFiles []string
		for _, file := range allFiles {
			info, err := util.GetFileInfo(file)
			if err != nil {
				continue
			}
			if info.ModTime > cutoff {
				recentFiles = append(recentFiles, file)
			}
		}
		return recentFiles, nil
	case "optimized":
		// Balanced approach: load files based on retention config
		if dl.sessionConfig.DataRetentionHours > 0 {
			cutoff := time.Now().Add(-time.Duration(dl.sessionConfig.DataRetentionHours) * time.Hour).Unix()
			var filteredFiles []string
			for _, file := range allFiles {
				info, err := util.GetFileInfo(file)
				if err != nil {
					continue
				}
				if info.ModTime > cutoff {
					filteredFiles = append(filteredFiles, file)
				}
			}
			return filteredFiles, nil
		}
		return allFiles, nil
	default:
		// Default to full mode
		return allFiles, nil
	}
}

// LoadFiles loads and processes the specified files
func (dl *DataLoader) LoadFiles(files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Batch validate cache
	sessionIdMap := make(map[string]string)
	sessionIds := make([]string, 0, len(files))

	for _, file := range files {
		sessionId := extractSessionId(file)
		sessionIdMap[file] = sessionId
		sessionIds = append(sessionIds, sessionId)
	}

	validCache := dl.fileCache.BatchValidate(sessionIds)

	// Separate files to parse and cache hits
	var filesToParse []string

	for _, file := range files {
		sessionId := sessionIdMap[file]
		validateResult := validCache[sessionId]
		if validateResult.Valid {
			// Load from cache
			if result := dl.fileCache.Get(sessionId); result.Found && result.Data != nil {
				dl.memoryCache.Set(sessionId, &cache.MemoryCacheEntry{
					AggregatedData: result.Data,
					LastAccessed:   time.Now().Unix(),
					RawLogs:        nil, // Raw logs not stored in file cache currently
				})
			}
		} else {
			filesToParse = append(filesToParse, file)
		}
	}

	// Parse files that need processing
	if len(filesToParse) > 0 {
		util.LogInfo(fmt.Sprintf("Parsing %d files...", len(filesToParse)))
		dl.parseAndCacheFiles(filesToParse, sessionIdMap)
	}

	return nil
}

// parseAndCacheFiles parses files and updates caches
func (dl *DataLoader) parseAndCacheFiles(files []string, sessionIdMap map[string]string) {
	parseResults := dl.parser.ParseFiles(files)

	for result := range parseResults {
		if result.Error != nil {
			util.LogWarn(fmt.Sprintf("Failed to parse %s: %v", result.File, result.Error))
			continue
		}

		// Filter logs if needed
		recentLogs := dl.filterRecentLogs(result.Logs)
		if len(recentLogs) == 0 {
			continue
		}

		// Aggregate data
		projectName := aggregator.ExtractProjectName(result.File)
		hourlyData := dl.aggregator.AggregateByHourAndModel(recentLogs, projectName)

		// Extract limit messages
		limitParser := session.NewLimitParser()
		limitInfos := limitParser.ParseLogs(recentLogs)
		cachedLimits := make([]aggregator.CachedLimitInfo, 0, len(limitInfos))
		for _, limit := range limitInfos {
			cachedLimits = append(cachedLimits, aggregator.CachedLimitInfo{
				Type:      limit.Type,
				Timestamp: limit.Timestamp,
				ResetTime: limit.ResetTime,
				Content:   limit.Content,
				Model:     limit.Model,
			})
		}

		// Create aggregated data
		sessionId := sessionIdMap[result.File]
		aggregatedData := &aggregator.AggregatedData{
			FileHash:      sessionId,
			FilePath:      result.File,
			ProjectName:   projectName,
			HourlyStats:   hourlyData,
			SessionId:     sessionId,
			LimitMessages: cachedLimits,
		}

		// Save to cache
		if err := dl.fileCache.Set(sessionId, aggregatedData); err != nil {
			util.LogWarn(fmt.Sprintf("Failed to cache %s: %v", result.File, err))
		}

		// Update memory cache with raw logs
		dl.memoryCache.Set(sessionId, &cache.MemoryCacheEntry{
			AggregatedData: aggregatedData,
			LastAccessed:   time.Now().Unix(),
			RawLogs:        recentLogs,
		})
	}
}

// filterRecentLogs filters logs based on retention configuration
func (dl *DataLoader) filterRecentLogs(logs []model.ConversationLog) []model.ConversationLog {
	// Apply configuration-based filtering
	if dl.sessionConfig.DataRetentionHours <= 0 {
		// No filtering - return all logs
		return logs
	}

	// Filter based on retention configuration
	cutoff := time.Now().Add(-time.Duration(dl.sessionConfig.DataRetentionHours) * time.Hour).Unix()
	var filtered []model.ConversationLog

	for _, log := range logs {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}

		if ts.Unix() > cutoff {
			filtered = append(filtered, log)
		}
	}

	return filtered
}

// GetGlobalTimeline returns the global timeline of all logs
func (dl *DataLoader) GetGlobalTimeline(secondsBack int64) []timeline.TimestampedLog {
	return dl.memoryCache.GetGlobalTimeline(secondsBack)
}

// GetCachedWindowInfo returns cached window detection information
func (dl *DataLoader) GetCachedWindowInfo() map[string]*session.WindowDetectionInfo {
	return dl.memoryCache.GetCachedWindowInfo()
}

// UpdateWindowInfo updates window detection information for a session
func (dl *DataLoader) UpdateWindowInfo(sessionID string, info *session.WindowDetectionInfo) {
	dl.memoryCache.UpdateWindowInfo(sessionID, info)
}

// IdentifyChangedFiles returns files that have changed since last load
func (dl *DataLoader) IdentifyChangedFiles(files []string) []string {
	changedFiles := make([]string, 0)

	for _, file := range files {
		sessionId := extractSessionId(file)

		// Check if file is in memory cache
		if entry, exists := dl.memoryCache.Get(sessionId); exists {
			// File exists, check if it's marked as dirty (changed)
			if entry.IsDirty {
				changedFiles = append(changedFiles, sessionId)
			}
		} else {
			// New file
			changedFiles = append(changedFiles, sessionId)
		}
	}

	return changedFiles
}


// PersistDirtyEntries persists dirty cache entries to file cache
func (dl *DataLoader) PersistDirtyEntries() error {
	dirtyEntries := dl.memoryCache.GetDirtyEntries()

	for hash, entry := range dirtyEntries {
		if err := dl.fileCache.Set(hash, entry); err != nil {
			util.LogError(fmt.Sprintf("Failed to persist cache entry %s: %v", hash, err))
		}
	}

	return nil
}

// GetMemoryCache returns the memory cache instance (for session detection)
func (dl *DataLoader) GetMemoryCache() *cache.MemoryCache {
	return dl.memoryCache
}

// extractSessionId extracts the session ID from a file path
func extractSessionId(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}
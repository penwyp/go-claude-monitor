package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileCache(t *testing.T) {
	tempDir := t.TempDir()
	
	cache, err := NewFileCache(tempDir)
	
	require.NoError(t, err)
	assert.NotNil(t, cache)
	assert.Equal(t, tempDir, cache.baseDir)
	assert.NotNil(t, cache.memoryCache)
	assert.Empty(t, cache.memoryCache)
	
	// Verify directory was created
	info, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewFileCacheInvalidDirectory(t *testing.T) {
	// Try to create cache in a location that cannot be created (e.g., under a file)
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)
	
	invalidDir := filepath.Join(filePath, "subdir") // Try to create dir under a file
	
	cache, err := NewFileCache(invalidDir)
	
	assert.Error(t, err)
	assert.Nil(t, cache)
}

func TestExtractSessionId(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "simple JSONL file",
			filePath: "/path/to/session-123.jsonl",
			expected: "session-123",
		},
		{
			name:     "UUID session file",
			filePath: "/home/user/.claude/projects/my-project/12345678-1234-1234-1234-123456789012.jsonl",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "nested path",
			filePath: "/very/deep/nested/path/session-456.jsonl",
			expected: "session-456",
		},
		{
			name:     "file without extension",
			filePath: "/path/to/session-789",
			expected: "session-789",
		},
		{
			name:     "file with different extension",
			filePath: "/path/to/session-abc.json",
			expected: "session-abc",
		},
		{
			name:     "just filename",
			filePath: "session-xyz.jsonl",
			expected: "session-xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSessionId(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileCacheSetAndGet(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file to reference
	testFile := filepath.Join(tempDir, "test.jsonl")
	testContent := `{"test": "data"}`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create test data
	sessionId := "test-session-123"
	testData := &aggregator.AggregatedData{
		FileHash:    "test-hash",
		FilePath:    testFile,
		SessionId:   sessionId,
		ProjectName: "test-project",
		HourlyStats: []aggregator.HourlyData{
			{
				Hour:        1640995200,
				Model:       "claude-3-sonnet",
				ProjectName: "test-project",
				InputTokens: 100,
			},
		},
	}

	// Test Set
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Verify file was created
	cachePath := filepath.Join(tempDir, sessionId+".json")
	_, err = os.Stat(cachePath)
	require.NoError(t, err)

	// Test Get
	result := cache.Get(sessionId)
	assert.True(t, result.Found)
	assert.Equal(t, MissReasonNone, result.MissReason)
	require.NotNil(t, result.Data)
	assert.Equal(t, testData.FileHash, result.Data.FileHash)
	assert.Equal(t, testData.FilePath, result.Data.FilePath)
	assert.Equal(t, testData.SessionId, result.Data.SessionId)
	assert.Equal(t, testData.ProjectName, result.Data.ProjectName)
	assert.Len(t, result.Data.HourlyStats, 1)
}

func TestFileCacheGetNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	result := cache.Get("non-existent-session")
	
	assert.False(t, result.Found)
	assert.Equal(t, MissReasonNotFound, result.MissReason)
	assert.Nil(t, result.Data)
}

func TestFileCacheMemoryCache(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	sessionId := "test-session-456"
	testData := &aggregator.AggregatedData{
		FileHash:    "test-hash",
		FilePath:    testFile,
		SessionId:   sessionId,
		ProjectName: "test-project",
	}

	// Set data
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// First get should read from file and cache in memory
	result1 := cache.Get(sessionId)
	assert.True(t, result1.Found)
	assert.Equal(t, MissReasonNone, result1.MissReason)

	// Verify it's in memory cache
	cache.mu.RLock()
	_, exists := cache.memoryCache[sessionId]
	cache.mu.RUnlock()
	assert.True(t, exists)

	// Second get should use memory cache (would be faster, but we can't test timing here)
	result2 := cache.Get(sessionId)
	assert.True(t, result2.Found)
	assert.Equal(t, MissReasonNone, result2.MissReason)
	assert.Equal(t, result1.Data.FileHash, result2.Data.FileHash)
}

func TestFileCacheInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	sessionId := "invalid-session"
	cachePath := filepath.Join(tempDir, sessionId+".json")
	
	// Write invalid JSON to cache file
	err = os.WriteFile(cachePath, []byte("invalid json content"), 0644)
	require.NoError(t, err)

	result := cache.Get(sessionId)
	
	assert.False(t, result.Found)
	assert.Equal(t, MissReasonError, result.MissReason)
	assert.Nil(t, result.Data)
}

func TestFileCacheValidation(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	testContent := `{"test": "data"}`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Get file info
	fileInfo, err := util.GetFileInfo(testFile)
	require.NoError(t, err)

	sessionId := "validation-test"
	testData := &aggregator.AggregatedData{
		FilePath:     testFile,
		SessionId:    sessionId,
		LastModified: fileInfo.ModTime,
		FileSize:     fileInfo.Size,
		Inode:        fileInfo.Inode,
	}

	// Set fingerprint
	fingerprint, err := util.CalculateFileFingerprint(testFile)
	require.NoError(t, err)
	testData.ContentFingerprint = fingerprint

	// Store in cache
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Should be valid
	result := cache.Get(sessionId)
	assert.True(t, result.Found)
	assert.Equal(t, MissReasonNone, result.MissReason)

	// Modify the file to make cache invalid
	time.Sleep(time.Millisecond * 10) // Ensure different modification time
	err = os.WriteFile(testFile, []byte(`{"test": "modified data"}`), 0644)
	require.NoError(t, err)

	// Cache should be invalid now
	result = cache.Get(sessionId)
	assert.False(t, result.Found)
	assert.NotEqual(t, MissReasonNone, result.MissReason)
}

func TestFileCacheValidationOldFile(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file with old modification time
	testFile := filepath.Join(tempDir, "old-test.jsonl")
	testContent := `{"test": "data"}`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Set modification time to 3 days ago
	oldTime := time.Now().Add(-72 * time.Hour)
	err = os.Chtimes(testFile, oldTime, oldTime)
	require.NoError(t, err)

	// Get file info
	fileInfo, err := util.GetFileInfo(testFile)
	require.NoError(t, err)

	sessionId := "old-file-test"
	testData := &aggregator.AggregatedData{
		FilePath:           testFile,
		SessionId:          sessionId,
		LastModified:       fileInfo.ModTime,
		FileSize:           fileInfo.Size,
		Inode:              fileInfo.Inode,
		ContentFingerprint: "", // No fingerprint
	}

	// Store in cache
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Should be valid because file is old (>48 hours), fingerprint check is skipped
	result := cache.Get(sessionId)
	assert.True(t, result.Found)
	assert.Equal(t, MissReasonNone, result.MissReason)
}

func TestFileCacheValidationMissingFingerprint(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	testContent := `{"test": "data"}`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Get file info
	fileInfo, err := util.GetFileInfo(testFile)
	require.NoError(t, err)

	sessionId := "no-fingerprint-test"
	testData := &aggregator.AggregatedData{
		FilePath:           testFile,
		SessionId:          sessionId,
		LastModified:       fileInfo.ModTime,
		FileSize:           fileInfo.Size,
		Inode:              fileInfo.Inode,
		ContentFingerprint: "", // No fingerprint
	}

	// Store in cache
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Should be valid even without fingerprint if the cache.Set() call calculated one
	result := cache.Get(sessionId)
	// The Set operation would have calculated the fingerprint, so this should be valid
	assert.True(t, result.Found)
	assert.Equal(t, MissReasonNone, result.MissReason)
}

func TestFileCacheClear(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Add some data to cache
	sessionIds := []string{"session-1", "session-2", "session-3"}
	for _, sessionId := range sessionIds {
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "test-project",
		}
		err = cache.Set(sessionId, testData)
		require.NoError(t, err)
	}

	// Verify cache files exist
	for _, sessionId := range sessionIds {
		cachePath := filepath.Join(tempDir, sessionId+".json")
		_, err = os.Stat(cachePath)
		require.NoError(t, err)
	}

	// Verify memory cache is populated
	cache.mu.RLock()
	assert.Len(t, cache.memoryCache, len(sessionIds))
	cache.mu.RUnlock()

	// Clear cache
	err = cache.Clear()
	require.NoError(t, err)

	// Verify memory cache is empty
	cache.mu.RLock()
	assert.Empty(t, cache.memoryCache)
	cache.mu.RUnlock()

	// Verify cache files are deleted
	for _, sessionId := range sessionIds {
		cachePath := filepath.Join(tempDir, sessionId+".json")
		_, err = os.Stat(cachePath)
		assert.True(t, os.IsNotExist(err), "Cache file should be deleted: %s", cachePath)
	}
}

func TestFileCachePreload(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Create cache entries
	sessionIds := []string{"preload-1", "preload-2", "preload-3"}
	for _, sessionId := range sessionIds {
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "test-project",
		}
		err = cache.Set(sessionId, testData)
		require.NoError(t, err)
	}

	// Clear memory cache to simulate fresh start
	cache.mu.Lock()
	cache.memoryCache = make(map[string]*aggregator.AggregatedData)
	cache.mu.Unlock()

	// Preload cache
	err = cache.Preload()
	require.NoError(t, err)

	// Verify all entries are loaded into memory
	cache.mu.RLock()
	assert.Len(t, cache.memoryCache, len(sessionIds))
	for _, sessionId := range sessionIds {
		_, exists := cache.memoryCache[sessionId]
		assert.True(t, exists, "Session should be in memory cache: %s", sessionId)
	}
	cache.mu.RUnlock()
}

func TestFileCachePreloadEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	err = cache.Preload()
	require.NoError(t, err)

	// Memory cache should remain empty
	cache.mu.RLock()
	assert.Empty(t, cache.memoryCache)
	cache.mu.RUnlock()
}

func TestFileCachePreloadInvalidFiles(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create invalid cache files
	invalidFiles := []struct {
		name    string
		content string
	}{
		{"invalid-1.json", "invalid json content"},
		{"invalid-2.json", `{"incomplete": `},
		{"valid.json", `{"filePath": "/nonexistent/file.jsonl", "sessionId": "valid"}`},
	}

	for _, file := range invalidFiles {
		filePath := filepath.Join(tempDir, file.name)
		err = os.WriteFile(filePath, []byte(file.content), 0644)
		require.NoError(t, err)
	}

	err = cache.Preload()
	require.NoError(t, err)

	// Only valid entries with existing files should be loaded
	cache.mu.RLock()
	assert.Empty(t, cache.memoryCache) // No valid files exist, so memory cache should be empty
	cache.mu.RUnlock()
}

func TestFileCacheGetCacheStats(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Initially empty
	memCount, fileCount := cache.GetCacheStats()
	assert.Equal(t, 0, memCount)
	assert.Equal(t, 0, fileCount)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Add some cache entries
	sessionIds := []string{"stats-1", "stats-2"}
	for _, sessionId := range sessionIds {
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "test-project",
		}
		err = cache.Set(sessionId, testData)
		require.NoError(t, err)
	}

	memCount, fileCount = cache.GetCacheStats()
	assert.Equal(t, len(sessionIds), memCount)
	assert.Equal(t, len(sessionIds), fileCount)
}

func TestFileCacheValidateCache(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test files
	validFile := filepath.Join(tempDir, "valid.jsonl")
	invalidFile := filepath.Join(tempDir, "invalid.jsonl")
	
	err = os.WriteFile(validFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Add valid session to cache
	validSessionId := "valid"
	testData := &aggregator.AggregatedData{
		FilePath:    validFile,
		SessionId:   validSessionId,
		ProjectName: "test-project",
	}
	err = cache.Set(validSessionId, testData)
	require.NoError(t, err)

	files := []string{validFile, invalidFile}
	validationResults := cache.ValidateCache(files)

	assert.Len(t, validationResults, 2)
	assert.True(t, validationResults[validFile])
	assert.False(t, validationResults[invalidFile])
}

func TestFileCacheBatchValidate(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Add valid session to cache
	validSessionId := "batch-valid"
	testData := &aggregator.AggregatedData{
		FilePath:    testFile,
		SessionId:   validSessionId,
		ProjectName: "test-project",
	}
	err = cache.Set(validSessionId, testData)
	require.NoError(t, err)

	sessionIds := []string{validSessionId, "batch-invalid"}
	results := cache.BatchValidate(sessionIds)

	assert.Len(t, results, 2)
	
	validResult := results[validSessionId]
	assert.True(t, validResult.Valid)
	assert.Equal(t, MissReasonNone, validResult.MissReason)

	invalidResult := results["batch-invalid"]
	assert.False(t, invalidResult.Valid)
	assert.Equal(t, MissReasonNotFound, invalidResult.MissReason)
}

func TestFileCacheSetWithoutSessionId(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	sessionId := "auto-session"
	testData := &aggregator.AggregatedData{
		FilePath:    testFile,
		// SessionId intentionally empty
		ProjectName: "test-project",
	}

	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Verify SessionId was set
	result := cache.Get(sessionId)
	assert.True(t, result.Found)
	assert.Equal(t, sessionId, result.Data.SessionId)
}

func TestFileCacheConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Number of concurrent operations
	numGoroutines := 10
	numOperations := 100

	var wg sync.WaitGroup
	
	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sessionId := fmt.Sprintf("concurrent-%d-%d", id, j)
				testData := &aggregator.AggregatedData{
					FilePath:    testFile,
					SessionId:   sessionId,
					ProjectName: "test-project",
				}
				err := cache.Set(sessionId, testData)
				assert.NoError(t, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sessionId := fmt.Sprintf("concurrent-%d-%d", id, j)
				result := cache.Get(sessionId)
				assert.True(t, result.Found)
			}
		}(i)
	}
	wg.Wait()

	// Verify final state
	memCount, fileCount := cache.GetCacheStats()
	expectedCount := numGoroutines * numOperations
	assert.Equal(t, expectedCount, memCount)
	assert.Equal(t, expectedCount, fileCount)
}

func TestFileCacheMemoryCacheInvalidation(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	sessionId := "invalidation-test"
	testData := &aggregator.AggregatedData{
		FilePath:    testFile,
		SessionId:   sessionId,
		ProjectName: "test-project",
	}

	// Set data
	err = cache.Set(sessionId, testData)
	require.NoError(t, err)

	// Get data (should be cached in memory)
	result1 := cache.Get(sessionId)
	assert.True(t, result1.Found)

	// Verify it's in memory cache
	cache.mu.RLock()
	_, exists := cache.memoryCache[sessionId]
	cache.mu.RUnlock()
	assert.True(t, exists)

	// Modify the file to invalidate cache
	time.Sleep(time.Millisecond * 10) // Ensure different modification time
	err = os.WriteFile(testFile, []byte(`{"test": "modified data"}`), 0644)
	require.NoError(t, err)

	// Get data again (should remove invalid entry from memory cache)
	result2 := cache.Get(sessionId)
	assert.False(t, result2.Found)

	// Verify it's removed from memory cache
	cache.mu.RLock()
	_, exists = cache.memoryCache[sessionId]
	cache.mu.RUnlock()
	assert.False(t, exists)
}

func TestCacheMissReasonConstants(t *testing.T) {
	// Test that all cache miss reason constants are defined
	reasons := []CacheMissReason{
		MissReasonNone,
		MissReasonError,
		MissReasonInode,
		MissReasonSize,
		MissReasonModTime,
		MissReasonFingerprint,
		MissReasonNoFingerprint,
		MissReasonNotFound,
	}

	// Verify they have different values
	reasonSet := make(map[CacheMissReason]bool)
	for _, reason := range reasons {
		assert.False(t, reasonSet[reason], "Duplicate cache miss reason value: %d", reason)
		reasonSet[reason] = true
	}

	assert.Len(t, reasonSet, len(reasons))
}

func TestCacheResultStructure(t *testing.T) {
	// Test CacheResult structure
	testData := &aggregator.AggregatedData{
		SessionId: "test",
	}

	result := CacheResult{
		Data:       testData,
		Found:      true,
		MissReason: MissReasonNone,
	}

	assert.Equal(t, testData, result.Data)
	assert.True(t, result.Found)
	assert.Equal(t, MissReasonNone, result.MissReason)
}

func TestBatchValidateResultStructure(t *testing.T) {
	// Test BatchValidateResult structure
	result := BatchValidateResult{
		Valid:      true,
		MissReason: MissReasonNone,
	}

	assert.True(t, result.Valid)
	assert.Equal(t, MissReasonNone, result.MissReason)
}

func TestFileCacheInterface(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create a test file for the interface test
	testFile := filepath.Join(tempDir, "interface-test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Verify that FileCache implements the Cache interface
	var _ Cache = cache

	// Test all interface methods are callable
	result := cache.Get("non-existent")
	assert.False(t, result.Found)

	testData := &aggregator.AggregatedData{
		FilePath:    testFile,
		SessionId:   "test",
		ProjectName: "interface-test",
	}
	err = cache.Set("test", testData)
	assert.NoError(t, err)

	err = cache.Clear()
	assert.NoError(t, err)

	err = cache.Preload()
	assert.NoError(t, err)

	batchResults := cache.BatchValidate([]string{"test"})
	assert.NotNil(t, batchResults)
}

// Benchmark tests for performance critical operations
func BenchmarkFileCacheSet(b *testing.B) {
	tempDir := b.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(b, err)

	// Create test file
	testFile := filepath.Join(tempDir, "bench.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(b, err)

	testData := &aggregator.AggregatedData{
		FilePath:    testFile,
		ProjectName: "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionId := fmt.Sprintf("bench-set-%d", i)
		err := cache.Set(sessionId, testData)
		require.NoError(b, err)
	}
}

func BenchmarkFileCacheGet(b *testing.B) {
	tempDir := b.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(b, err)

	// Create test file
	testFile := filepath.Join(tempDir, "bench.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(b, err)

	// Pre-populate cache
	numEntries := 1000
	sessionIds := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		sessionId := fmt.Sprintf("bench-get-%d", i)
		sessionIds[i] = sessionId
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "benchmark",
		}
		err := cache.Set(sessionId, testData)
		require.NoError(b, err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionId := sessionIds[i%numEntries]
		result := cache.Get(sessionId)
		require.True(b, result.Found)
	}
}

func BenchmarkFileCachePreload(b *testing.B) {
	tempDir := b.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "bench.jsonl")
	err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(b, err)

	// Pre-create cache files
	numFiles := 100
	for i := 0; i < numFiles; i++ {
		sessionId := fmt.Sprintf("bench-preload-%d", i)
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "benchmark",
		}

		cachePath := filepath.Join(tempDir, sessionId+".json")
		file, err := os.Create(cachePath)
		require.NoError(b, err)
		encoder := json.NewEncoder(file)
		err = encoder.Encode(testData)
		require.NoError(b, err)
		file.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache, err := NewFileCache(tempDir)
		require.NoError(b, err)
		
		err = cache.Preload()
		require.NoError(b, err)
	}
}

func TestFileCachePreloadWorkerCount(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFileCache(tempDir)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	// Create more cache files than CPU cores to test worker pool behavior
	numFiles := runtime.NumCPU() * 2
	for i := 0; i < numFiles; i++ {
		sessionId := fmt.Sprintf("worker-test-%d", i)
		testData := &aggregator.AggregatedData{
			FilePath:    testFile,
			SessionId:   sessionId,
			ProjectName: "test-project",
		}
		err = cache.Set(sessionId, testData)
		require.NoError(t, err)
	}

	// Clear memory cache
	cache.mu.Lock()
	cache.memoryCache = make(map[string]*aggregator.AggregatedData)
	cache.mu.Unlock()

	// Preload should handle all files with worker pool
	err = cache.Preload()
	require.NoError(t, err)

	// All files should be loaded
	cache.mu.RLock()
	assert.Len(t, cache.memoryCache, numFiles)
	cache.mu.RUnlock()
}
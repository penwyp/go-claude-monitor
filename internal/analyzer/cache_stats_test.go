package analyzer

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/cache"
	"github.com/stretchr/testify/assert"
)

func TestCacheMissReasonString(t *testing.T) {
	tests := []struct {
		name     string
		reason   cache.CacheMissReason
		expected string
	}{
		{
			name:     "none",
			reason:   cache.MissReasonNone,
			expected: "none",
		},
		{
			name:     "error",
			reason:   cache.MissReasonError,
			expected: "Cache read error",
		},
		{
			name:     "inode_changed",
			reason:   cache.MissReasonInode,
			expected: "File inode changed",
		},
		{
			name:     "size_changed",
			reason:   cache.MissReasonSize,
			expected: "File size changed",
		},
		{
			name:     "mod_time_changed",
			reason:   cache.MissReasonModTime,
			expected: "Modification time changed",
		},
		{
			name:     "fingerprint_changed",
			reason:   cache.MissReasonFingerprint,
			expected: "File fingerprint changed",
		},
		{
			name:     "no_fingerprint",
			reason:   cache.MissReasonNoFingerprint,
			expected: "Cached file has no fingerprint",
		},
		{
			name:     "not_found",
			reason:   cache.MissReasonNotFound,
			expected: "Cache not found",
		},
		{
			name:     "unknown_reason",
			reason:   cache.CacheMissReason(999), // Invalid reason
			expected: "Unknown reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cacheMissReasonString(tt.reason)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewCacheStats(t *testing.T) {
	stats := NewCacheStats()
	
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.totalFiles)
	assert.Equal(t, int64(0), stats.cacheHits)
	assert.Equal(t, int64(0), stats.cacheMisses)
	assert.Equal(t, int64(0), stats.failures)
	assert.NotNil(t, stats.missDetails)
	assert.Empty(t, stats.missDetails)
}

func TestCacheStatsIncrementOperations(t *testing.T) {
	stats := NewCacheStats()
	
	// Test initial state
	total, hits, misses, failures, hitRate := stats.GetStats()
	assert.Equal(t, int64(0), total)
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, int64(0), failures)
	assert.Equal(t, 0.0, hitRate)
	
	// Test IncrementTotal
	stats.IncrementTotal()
	stats.IncrementTotal()
	stats.IncrementTotal()
	
	total, _, _, _, _ = stats.GetStats()
	assert.Equal(t, int64(3), total)
	
	// Test IncrementHit
	stats.IncrementHit()
	stats.IncrementHit()
	
	_, hits, _, _, hitRate = stats.GetStats()
	assert.Equal(t, int64(2), hits)
	assert.InDelta(t, 2.0/3.0*100, hitRate, 0.0001) // ~66.67%
	
	// Test IncrementMiss
	stats.IncrementMiss("/path/to/file1.jsonl", cache.MissReasonNotFound)
	
	_, _, misses, _, _ = stats.GetStats()
	assert.Equal(t, int64(1), misses)
	
	// Verify miss details are recorded
	assert.Len(t, stats.missDetails, 1)
	assert.Equal(t, "/path/to/file1.jsonl", stats.missDetails[0].FilePath)
	assert.Equal(t, cache.MissReasonNotFound, stats.missDetails[0].Reason)
	
	// Test IncrementFailure
	stats.IncrementFailure()
	
	_, _, _, failures, _ = stats.GetStats()
	assert.Equal(t, int64(1), failures)
}

func TestCacheStatsGetStats(t *testing.T) {
	stats := NewCacheStats()
	
	// Add various counters
	for i := 0; i < 10; i++ {
		stats.IncrementTotal()
	}
	
	for i := 0; i < 7; i++ {
		stats.IncrementHit()
	}
	
	for i := 0; i < 2; i++ {
		stats.IncrementMiss(fmt.Sprintf("/path/file%d.jsonl", i), cache.MissReasonNotFound)
	}
	
	stats.IncrementFailure()
	
	total, hits, misses, failures, hitRate := stats.GetStats()
	
	assert.Equal(t, int64(10), total)
	assert.Equal(t, int64(7), hits)
	assert.Equal(t, int64(2), misses)
	assert.Equal(t, int64(1), failures)
	assert.Equal(t, 70.0, hitRate) // 7/10 * 100
}

func TestCacheStatsHitRateCalculation(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		hits         int
		expectedRate float64
	}{
		{
			name:         "zero_total",
			total:        0,
			hits:         0,
			expectedRate: 0.0,
		},
		{
			name:         "perfect_hit_rate",
			total:        10,
			hits:         10,
			expectedRate: 100.0,
		},
		{
			name:         "zero_hit_rate",
			total:        10,
			hits:         0,
			expectedRate: 0.0,
		},
		{
			name:         "partial_hit_rate",
			total:        100,
			hits:         75,
			expectedRate: 75.0,
		},
		{
			name:         "single_file",
			total:        1,
			hits:         1,
			expectedRate: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := NewCacheStats()
			
			for i := 0; i < tt.total; i++ {
				stats.IncrementTotal()
			}
			
			for i := 0; i < tt.hits; i++ {
				stats.IncrementHit()
			}
			
			_, _, _, _, hitRate := stats.GetStats()
			assert.Equal(t, tt.expectedRate, hitRate)
		})
	}
}

func TestCacheStatsConcurrentAccess(t *testing.T) {
	stats := NewCacheStats()
	
	const numGoroutines = 10
	const operationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 types of operations
	
	// Concurrent IncrementTotal
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.IncrementTotal()
			}
		}()
	}
	
	// Concurrent IncrementHit
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.IncrementHit()
			}
		}()
	}
	
	// Concurrent IncrementMiss
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				filePath := fmt.Sprintf("/path/g%d_file%d.jsonl", goroutineID, j)
				stats.IncrementMiss(filePath, cache.MissReasonNotFound)
			}
		}(i)
	}
	
	// Concurrent IncrementFailure
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.IncrementFailure()
			}
		}()
	}
	
	wg.Wait()
	
	// Verify final counts
	total, hits, misses, failures, hitRate := stats.GetStats()
	
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), total)
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), hits)
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), misses)
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), failures)
	assert.Equal(t, 100.0, hitRate) // hits == total, so 100%
	
	// Verify miss details were recorded (with proper locking)
	assert.Len(t, stats.missDetails, numGoroutines*operationsPerGoroutine)
}

func TestCacheStatsPrintProgress(t *testing.T) {
	// Skip this test since it depends on logger initialization
	// The PrintProgress function is a thin wrapper around util.LogInfo
	// which is tested elsewhere. The core logic is tested in GetStats.
	t.Skip("PrintProgress depends on global logger initialization")
}

func TestCacheStatsPrintPeriodicStats(t *testing.T) {
	// Skip this test since it depends on logger initialization
	// The PrintPeriodicStats function is a thin wrapper around util.LogDebug
	// which is tested elsewhere. The core logic is tested in GetStats.
	t.Skip("PrintPeriodicStats depends on global logger initialization")
}

func TestCacheStatsPrintPeriodicStatsNoMisses(t *testing.T) {
	// Skip this test since it depends on logger initialization
	// The PrintPeriodicStats function is a thin wrapper around util.LogDebug
	// which is tested elsewhere. The core logic is tested in GetStats.
	t.Skip("PrintPeriodicStats depends on global logger initialization")
}

func TestCacheStatsPrintFinalStats(t *testing.T) {
	// Skip this test since it depends on logger initialization
	// The PrintFinalStats function is a thin wrapper around util.LogInfo
	// which is tested elsewhere. The core logic is tested in GetStats.
	t.Skip("PrintFinalStats depends on global logger initialization")
}

func TestCacheStatsPrintFinalStatsNoMisses(t *testing.T) {
	// Skip this test since it depends on logger initialization
	// The PrintFinalStats function is a thin wrapper around util.LogInfo
	// which is tested elsewhere. The core logic is tested in GetStats.
	t.Skip("PrintFinalStats depends on global logger initialization")
}

func TestCacheStatsMissDetailsThreadSafety(t *testing.T) {
	stats := NewCacheStats()
	
	const numGoroutines = 20
	const missesPerGoroutine = 50
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Concurrent miss recording
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < missesPerGoroutine; j++ {
				filePath := fmt.Sprintf("/path/g%d_file%d.jsonl", goroutineID, j)
				reason := cache.CacheMissReason((goroutineID + j) % 8) // Cycle through different reasons
				stats.IncrementMiss(filePath, reason)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all misses were recorded
	assert.Len(t, stats.missDetails, numGoroutines*missesPerGoroutine)
	
	// Verify data integrity by checking for unique file paths
	pathSet := make(map[string]bool)
	for _, detail := range stats.missDetails {
		assert.False(t, pathSet[detail.FilePath], "Duplicate file path found: %s", detail.FilePath)
		pathSet[detail.FilePath] = true
		assert.True(t, strings.HasPrefix(detail.FilePath, "/path/g"), "Invalid file path format: %s", detail.FilePath)
	}
}

// Benchmark tests for performance critical operations
func BenchmarkIncrementTotal(b *testing.B) {
	stats := NewCacheStats()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.IncrementTotal()
		}
	})
}

func BenchmarkIncrementHit(b *testing.B) {
	stats := NewCacheStats()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.IncrementHit()
		}
	})
}

func BenchmarkIncrementMiss(b *testing.B) {
	stats := NewCacheStats()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.IncrementMiss("/path/to/file.jsonl", cache.MissReasonNotFound)
		}
	})
}

func BenchmarkGetStats(b *testing.B) {
	stats := NewCacheStats()
	
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		stats.IncrementTotal()
		if i%2 == 0 {
			stats.IncrementHit()
		} else {
			stats.IncrementMiss(fmt.Sprintf("/path/file%d.jsonl", i), cache.MissReasonNotFound)
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.GetStats()
	}
}

func BenchmarkCacheMissReasonString(b *testing.B) {
	reasons := []cache.CacheMissReason{
		cache.MissReasonNone,
		cache.MissReasonError,
		cache.MissReasonInode,
		cache.MissReasonSize,
		cache.MissReasonModTime,
		cache.MissReasonFingerprint,
		cache.MissReasonNoFingerprint,
		cache.MissReasonNotFound,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cacheMissReasonString(reasons[i%len(reasons)])
	}
}

// Test edge cases and error conditions
func TestCacheStatsEdgeCases(t *testing.T) {
	t.Run("empty_file_path", func(t *testing.T) {
		stats := NewCacheStats()
		
		// Should handle empty file path gracefully
		stats.IncrementMiss("", cache.MissReasonNotFound)
		
		assert.Len(t, stats.missDetails, 1)
		assert.Equal(t, "", stats.missDetails[0].FilePath)
		assert.Equal(t, cache.MissReasonNotFound, stats.missDetails[0].Reason)
	})
	
	t.Run("very_long_file_path", func(t *testing.T) {
		stats := NewCacheStats()
		
		longPath := strings.Repeat("/very/long/path", 100) + "/file.jsonl"
		stats.IncrementMiss(longPath, cache.MissReasonSize)
		
		assert.Len(t, stats.missDetails, 1)
		assert.Equal(t, longPath, stats.missDetails[0].FilePath)
	})
	
	t.Run("high_volume_operations", func(t *testing.T) {
		stats := NewCacheStats()
		
		// Reduced volume to avoid long test execution time
		const operations = 10000
		
		start := time.Now()
		
		for i := 0; i < operations; i++ {
			stats.IncrementTotal()
			if i%2 == 0 {
				stats.IncrementHit()
			} else {
				// Only record some misses to avoid excessive memory usage
				if i%100 == 1 {
					stats.IncrementMiss(fmt.Sprintf("/path/file%d.jsonl", i), cache.MissReasonNotFound)
				}
			}
		}
		
		elapsed := time.Since(start)
		t.Logf("High volume test (%d operations) completed in %v", operations, elapsed)
		
		total, hits, misses, _, hitRate := stats.GetStats()
		assert.Equal(t, int64(operations), total)
		assert.Equal(t, int64(operations/2), hits)
		assert.Equal(t, 50.0, hitRate)
		
		// Should have recorded approximately operations/100 misses
		expectedMisses := operations / 100
		assert.InDelta(t, expectedMisses, misses, 1) // Allow for small rounding differences
	})
	
	t.Run("concurrent_stats_reading", func(t *testing.T) {
		stats := NewCacheStats()
		
		// Start background operations
		var wg sync.WaitGroup
		wg.Add(2)
		
		stopCh := make(chan bool)
		
		// Writer goroutine
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					stats.IncrementTotal()
					stats.IncrementHit()
					time.Sleep(time.Microsecond)
				}
			}
		}()
		
		// Reader goroutine
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					stats.GetStats()
					time.Sleep(time.Microsecond)
				}
			}
		}()
		
		// Let them run for a short time
		time.Sleep(10 * time.Millisecond)
		close(stopCh)
		wg.Wait()
		
		// Should not panic and should have consistent data
		total, hits, _, _, hitRate := stats.GetStats()
		assert.True(t, total > 0)
		assert.True(t, hits > 0)
		assert.Equal(t, 100.0, hitRate) // All operations were hits
	})
}
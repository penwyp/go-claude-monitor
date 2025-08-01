package analyzer

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/penwyp/go-claude-monitor/internal/data/cache"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// Translate cache miss reason to English string for logging
func cacheMissReasonString(r cache.CacheMissReason) string {
	switch r {
	case cache.MissReasonNone:
		return "none"
	case cache.MissReasonError:
		return "Cache read error"
	case cache.MissReasonInode:
		return "File inode changed"
	case cache.MissReasonSize:
		return "File size changed"
	case cache.MissReasonModTime:
		return "Modification time changed"
	case cache.MissReasonFingerprint:
		return "File fingerprint changed"
	case cache.MissReasonNoFingerprint:
		return "Cached file has no fingerprint"
	case cache.MissReasonNotFound:
		return "Cache not found"
	default:
		return "Unknown reason"
	}
}

// CacheStats holds statistics for cache usage
type CacheStats struct {
	totalFiles  int64
	cacheHits   int64
	cacheMisses int64
	failures    int64
	mu          sync.Mutex
	missDetails []MissDetail
}

// MissDetail records details of a cache miss
type MissDetail struct {
	FilePath string
	Reason   cache.CacheMissReason
}

// NewCacheStats creates a new CacheStats instance
func NewCacheStats() *CacheStats {
	return &CacheStats{
		missDetails: make([]MissDetail, 0),
	}
}

// IncrementTotal increases the total file count
func (cs *CacheStats) IncrementTotal() {
	atomic.AddInt64(&cs.totalFiles, 1)
}

// IncrementHit increases the cache hit count
func (cs *CacheStats) IncrementHit() {
	atomic.AddInt64(&cs.cacheHits, 1)
}

// IncrementMiss increases the cache miss count and records the miss detail
func (cs *CacheStats) IncrementMiss(filePath string, reason cache.CacheMissReason) {
	atomic.AddInt64(&cs.cacheMisses, 1)

	cs.mu.Lock()
	cs.missDetails = append(cs.missDetails, MissDetail{
		FilePath: filePath,
		Reason:   reason,
	})
	cs.mu.Unlock()
}

// IncrementFailure increases the failure count
func (cs *CacheStats) IncrementFailure() {
	atomic.AddInt64(&cs.failures, 1)
}

// GetStats returns the current statistics and hit rate
func (cs *CacheStats) GetStats() (total, hits, misses, failures int64, hitRate float64) {
	total = atomic.LoadInt64(&cs.totalFiles)
	hits = atomic.LoadInt64(&cs.cacheHits)
	misses = atomic.LoadInt64(&cs.cacheMisses)
	failures = atomic.LoadInt64(&cs.failures)

	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return
}

// PrintProgress logs the current file scan progress and cache hit rate
func (cs *CacheStats) PrintProgress(processed int64) {
	total, hits, misses, failures, hitRate := cs.GetStats()

	util.LogInfo(fmt.Sprintf("File scan progress: processed %d/%d files, cache hit rate: %.1f%% (%d hits/%d misses/%d failures)",
		processed, total, hitRate, hits, misses, failures))
}

// PrintPeriodicStats logs periodic cache statistics and details of recent cache misses
func (cs *CacheStats) PrintPeriodicStats() {
	total, hits, misses, failures, hitRate := cs.GetStats()

	util.LogDebug(fmt.Sprintf("Cache stats: total files %d, hits %d, misses %d, failures %d, hit rate %.1f%%",
		total, hits, misses, failures, hitRate))

	// Print details of files that missed the cache
	if misses > 0 {
		cs.mu.Lock()
		recentMisses := make([]MissDetail, len(cs.missDetails))
		copy(recentMisses, cs.missDetails)
		cs.mu.Unlock()

		util.LogDebug("Files missed in cache:")
		for _, detail := range recentMisses {
			util.LogDebug(fmt.Sprintf("  %s (%s)", detail.FilePath, cacheMissReasonString(detail.Reason)))
		}
	}
}

// PrintFinalStats logs the final cache statistics and a summary of cache miss reasons
func (cs *CacheStats) PrintFinalStats() {
	total, hits, misses, failures, hitRate := cs.GetStats()

	util.LogInfo(fmt.Sprintf("Cache statistics complete: total files %d, hit rate %.1f%% (%d hits/%d misses/%d failures)",
		total, hitRate, hits, misses, failures))

	if misses > 0 {
		cs.mu.Lock()
		reasonCounts := make(map[cache.CacheMissReason]int)
		for _, detail := range cs.missDetails {
			reasonCounts[detail.Reason]++
		}
		cs.mu.Unlock()

		util.LogInfo("Cache miss reason summary:")
		for reason, count := range reasonCounts {
			util.LogInfo(fmt.Sprintf("  %s: %d files", cacheMissReasonString(reason), count))
		}
	}
}

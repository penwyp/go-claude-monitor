package session

import (
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
)

// WindowDetectionInfo stores sliding window detection state
type WindowDetectionInfo struct {
	WindowStartTime  *int64 // Actual window start time (non-rounded)
	IsWindowDetected bool   // Whether window was explicitly detected
	WindowSource     string // Detection source: "limit_message", "gap", "first_message", "rounded_hour"
	DetectedAt       int64  // When this detection occurred
}

// MemoryCacheEntry extends AggregatedData with access time tracking
type MemoryCacheEntry struct {
	*aggregator.AggregatedData
	LastAccessed int64
	IsDirty      bool                    // Marks if needs persistence
	RawLogs      []model.ConversationLog // Raw logs for limit detection
	WindowInfo   *WindowDetectionInfo    // Sliding window detection state
}

type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*MemoryCacheEntry
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]*MemoryCacheEntry),
	}
}

func (mc *MemoryCache) Set(sessionId string, entry *MemoryCacheEntry) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry.LastAccessed = time.Now().Unix()
	entry.IsDirty = true
	mc.entries[sessionId] = entry
}

func (mc *MemoryCache) Get(sessionId string) (*MemoryCacheEntry, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.entries[sessionId]
	if ok {
		entry.LastAccessed = time.Now().Unix()
	}
	return entry, ok
}

func (mc *MemoryCache) GetDirtyEntries() map[string]*aggregator.AggregatedData {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	dirty := make(map[string]*aggregator.AggregatedData)

	for hash, entry := range mc.entries {
		if entry.IsDirty {
			dirty[hash] = entry.AggregatedData
			entry.IsDirty = false
		}
	}

	return dirty
}

func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.entries = make(map[string]*MemoryCacheEntry)
}

// GetCachedWindowInfo returns cached window detection info for all sessions
func (mc *MemoryCache) GetCachedWindowInfo() map[string]*WindowDetectionInfo {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	windowInfo := make(map[string]*WindowDetectionInfo)
	for sessionId, entry := range mc.entries {
		if entry.WindowInfo != nil {
			windowInfo[sessionId] = entry.WindowInfo
		}
	}
	return windowInfo
}

// GetRecentDataWithLogs returns both hourly data and raw logs for session detection
func (mc *MemoryCache) GetRecentDataWithLogs(duration int64) ([]aggregator.HourlyData, []model.ConversationLog) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	cutoff := time.Now().Unix() - duration
	var hourlyData []aggregator.HourlyData
	var rawLogs []model.ConversationLog

	for _, entry := range mc.entries {
		// Collect hourly data
		for _, hourly := range entry.HourlyStats {
			if hourly.Hour > cutoff {
				hourlyData = append(hourlyData, hourly)
			}
		}

		// Collect raw logs
		if entry.RawLogs != nil {
			for _, log := range entry.RawLogs {
				ts, err := time.Parse(time.RFC3339, log.Timestamp)
				if err == nil && ts.Unix() > cutoff {
					rawLogs = append(rawLogs, log)
				}
			}
		}
	}

	return hourlyData, rawLogs
}

// UpdateWindowInfo updates the window detection info for a specific session
func (mc *MemoryCache) UpdateWindowInfo(sessionId string, windowInfo *WindowDetectionInfo) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, ok := mc.entries[sessionId]; ok {
		entry.WindowInfo = windowInfo
		entry.IsDirty = true
	}
}

package cache

import (
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// WindowDetectionInfo stores sliding window detection state
type WindowDetectionInfo struct {
	WindowStartTime  *int64 // Actual window start time (non-rounded)
	IsWindowDetected bool   // Whether window was explicitly detected
	WindowSource     string // Detection source: "limit_message", "gap", "first_message", "rounded_hour"
	DetectedAt       int64  // When this detection occurred
	FirstEntryTime   int64  // Stable first message time for burn rate calculation
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

	if entry != nil {
		entry.LastAccessed = time.Now().Unix()
		entry.IsDirty = true
	}
	mc.entries[sessionId] = entry
}

func (mc *MemoryCache) Get(sessionId string) (*MemoryCacheEntry, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.entries[sessionId]
	if ok && entry != nil {
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
// TEST-ONLY: This method is used exclusively in tests for verifying cache behavior.
// Production code accesses cache data through different methods.
func (mc *MemoryCache) GetRecentDataWithLogs(duration int64) ([]aggregator.HourlyData, []model.ConversationLog) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var hourlyData []aggregator.HourlyData
	var rawLogs []model.ConversationLog

	// If duration is 0 or negative, return all data
	if duration <= 0 {
		for _, entry := range mc.entries {
			// Collect all hourly data
			hourlyData = append(hourlyData, entry.HourlyStats...)

			// Collect all raw logs
			if entry.RawLogs != nil {
				rawLogs = append(rawLogs, entry.RawLogs...)
			}
		}
		return hourlyData, rawLogs
	}

	// Otherwise, apply time filter
	cutoff := time.Now().Unix() - duration

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

// GetHistoricalLogs retrieves raw logs for a specified duration
// This is used to find historical limit messages
func (mc *MemoryCache) GetHistoricalLogs(duration int64) []model.ConversationLog {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var rawLogs []model.ConversationLog

	// If duration is 0 or negative, return all logs
	if duration <= 0 {
		for _, entry := range mc.entries {
			if entry.RawLogs != nil {
				rawLogs = append(rawLogs, entry.RawLogs...)
			}
		}
		return rawLogs
	}

	// Otherwise, apply time filter
	cutoff := time.Now().Unix() - duration

	for _, entry := range mc.entries {
		if entry.RawLogs != nil {
			for _, log := range entry.RawLogs {
				ts, err := time.Parse(time.RFC3339, log.Timestamp)
				if err == nil && ts.Unix() > cutoff {
					rawLogs = append(rawLogs, log)
				}
			}
		}
	}

	return rawLogs
}

// GetGlobalTimeline returns all logs from all projects sorted by timestamp
// GetLogsForFile returns all logs for a specific file/session
func (mc *MemoryCache) GetLogsForFile(sessionId string) []model.ConversationLog {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	entry, exists := mc.entries[sessionId]
	if !exists || entry.RawLogs == nil {
		return []model.ConversationLog{}
	}
	
	return entry.RawLogs
}

func (mc *MemoryCache) GetGlobalTimeline(duration int64) []timeline.TimestampedLog {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create timeline builder
	tb := timeline.NewTimelineBuilder("Local")
	var allEntries []timeline.TimelineEntry

	// UNIFIED DATA SOURCE: Collect from BOTH raw logs AND aggregated data
	for _, entry := range mc.entries {
		// Primary source: Raw logs if available
		if entry.RawLogs != nil && len(entry.RawLogs) > 0 {
			entries := tb.BuildFromRawLogs(entry.RawLogs, entry.ProjectName)
			allEntries = append(allEntries, entries...)
		}
		
		// Supplementary source: ALWAYS include aggregated data as backup
		// This ensures we have complete historical coverage
		if entry.AggregatedData != nil {
			entries := tb.BuildFromCachedData([]aggregator.AggregatedData{*entry.AggregatedData})
			// Mark as supplementary to handle deduplication later
			for i := range entries {
				entries[i].IsSupplementary = true
			}
			allEntries = append(allEntries, entries...)
		}
	}

	// Filter by duration
	if duration > 0 {
		allEntries = tb.FilterByDuration(allEntries, time.Duration(duration)*time.Second)
	}

	// Deduplicate entries (prefer raw logs over aggregated data)
	allEntries = tb.DeduplicateEntries(allEntries)
	
	// Sort and convert to TimestampedLog format
	sorted := tb.MergeTimelines(allEntries)
	timeline := tb.ConvertToTimestampedLogs(sorted)

	util.LogDebug(fmt.Sprintf("GetGlobalTimeline: %d entries, %d timeline entries, %d logs after conversion",
		len(mc.entries), len(allEntries), len(timeline)))

	return timeline
}

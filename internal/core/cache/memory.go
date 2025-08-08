package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	timelinepkg "github.com/penwyp/go-claude-monitor/internal/core/timeline"
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

	// Double buffering support
	pendingClear      bool                         // Flag indicating cache is pending clear
	shadowEntries     map[string]*MemoryCacheEntry // Shadow buffer for atomic swap
	lastValidTimeline []timelinepkg.TimestampedLog // Last valid timeline for fallback
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries:           make(map[string]*MemoryCacheEntry),
		shadowEntries:     nil,
		pendingClear:      false,
		lastValidTimeline: []timelinepkg.TimestampedLog{},
	}
}

func (mc *MemoryCache) Set(sessionId string, entry *MemoryCacheEntry) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry != nil {
		entry.LastAccessed = time.Now().Unix()
		entry.IsDirty = true
	}

	// If pending clear, add to shadow buffer instead
	if mc.pendingClear && mc.shadowEntries != nil {
		mc.shadowEntries[sessionId] = entry
	} else {
		mc.entries[sessionId] = entry
	}
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

	// Mark cache as pending clear instead of immediately clearing
	mc.pendingClear = true
	// Create shadow buffer for new data
	mc.shadowEntries = make(map[string]*MemoryCacheEntry)

	util.LogInfo("MemoryCache: Marked for pending clear, maintaining data until new data is ready")
}

// CommitClear performs the actual cache clear after new data is loaded
func (mc *MemoryCache) CommitClear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.pendingClear && mc.shadowEntries != nil {
		// Atomic swap: replace entries with shadow entries
		mc.entries = mc.shadowEntries
		mc.shadowEntries = nil
		mc.pendingClear = false
		util.LogInfo("MemoryCache: Committed clear with new data")
	}
}

// CancelClear cancels a pending clear operation
func (mc *MemoryCache) CancelClear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.pendingClear = false
	mc.shadowEntries = nil
	util.LogInfo("MemoryCache: Cancelled pending clear")
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

func (mc *MemoryCache) GetGlobalTimeline(duration int64) []timelinepkg.TimestampedLog {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Create timeline builder
	tb := timelinepkg.NewTimelineBuilder("Local")
	var allEntries []timelinepkg.TimelineEntry
	rawLogsCount := 0
	aggregatedCount := 0

	// CONSISTENT DATA SOURCE: Always use aggregated data for consistency
	// Raw logs are only kept for real-time updates, but aggregated data is the source of truth
	// This ensures consistent token counts regardless of cache state
	for _, entry := range mc.entries {
		if entry.AggregatedData != nil {
			// Always use aggregated data for consistency
			entries := tb.BuildFromCachedData([]aggregator.AggregatedData{*entry.AggregatedData})
			allEntries = append(allEntries, entries...)
			aggregatedCount += len(entries)
		}
		// Count raw logs if present (for debugging)
		if entry.RawLogs != nil && len(entry.RawLogs) > 0 {
			rawLogsCount += len(entry.RawLogs)
		}
	}

	// Filter by duration
	if duration > 0 {
		allEntries = tb.FilterByDuration(allEntries, time.Duration(duration)*time.Second)
	}

	// Deduplicate entries (prefer raw logs over aggregated data)
	beforeDedup := len(allEntries)
	allEntries = tb.DeduplicateEntries(allEntries)
	afterDedup := len(allEntries)
	
	if beforeDedup != afterDedup {
		util.LogInfo(fmt.Sprintf("GetGlobalTimeline: Deduplication removed %d entries (before=%d, after=%d)",
			beforeDedup-afterDedup, beforeDedup, afterDedup))
	}

	// Sort and convert to TimestampedLog format
	sorted := tb.MergeTimelines(allEntries)
	timeline := tb.ConvertToTimestampedLogs(sorted)

	// Save valid timeline for fallback
	if len(timeline) > 0 {
		// Create a copy to avoid issues with concurrent access
		copyTimeline := make([]timelinepkg.TimestampedLog, len(timeline))
		copy(copyTimeline, timeline)
		mc.lastValidTimeline = copyTimeline
		util.LogDebug(fmt.Sprintf("GetGlobalTimeline: Saved %d entries as last valid timeline", len(timeline)))
	} else if len(mc.lastValidTimeline) > 0 {
		// Use last valid timeline as fallback
		util.LogWarn(fmt.Sprintf("GetGlobalTimeline: No data available, using last valid timeline with %d entries", len(mc.lastValidTimeline)))
		timeline = mc.lastValidTimeline
	}

	util.LogDebug(fmt.Sprintf("GetGlobalTimeline: Cache entries=%d | Raw logs available=%d, Aggregated used=%d | Before dedup=%d, After dedup=%d | Final=%d",
		len(mc.entries), rawLogsCount, aggregatedCount, beforeDedup, afterDedup, len(timeline)))

	return timeline
}

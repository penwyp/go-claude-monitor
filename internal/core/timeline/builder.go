package timeline

import (
	"fmt"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
)


// TimelineBuilder builds a unified timeline from various data sources
type TimelineBuilder struct {
	timezone *time.Location
}

// NewTimelineBuilder creates a new timeline builder
func NewTimelineBuilder(timezone string) *TimelineBuilder {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}
	return &TimelineBuilder{
		timezone: loc,
	}
}

// BuildFromRawLogs builds timeline from raw conversation logs
func (tb *TimelineBuilder) BuildFromRawLogs(logs []model.ConversationLog, projectName string) []TimelineEntry {
	entries := make([]TimelineEntry, 0, len(logs))
	
	for _, log := range logs {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}
		
		entries = append(entries, TimelineEntry{
			Timestamp:   ts.Unix(),
			ProjectName: projectName,
			Type:        "message",
			Data:        log,
		})
	}
	
	return entries
}

// BuildFromHourlyData builds timeline from aggregated hourly data
func (tb *TimelineBuilder) BuildFromHourlyData(hourlyData []aggregator.HourlyData) []TimelineEntry {
	entries := make([]TimelineEntry, 0, len(hourlyData))
	
	for _, data := range hourlyData {
		// Use the first entry time as the representative timestamp for the hour
		// This prevents double-counting tokens when converting to logs
		if data.FirstEntryTime > 0 {
			entries = append(entries, TimelineEntry{
				Timestamp:   data.FirstEntryTime,
				ProjectName: data.ProjectName,
				Type:        "hourly",
				Data:        data,
			})
		}
	}
	
	return entries
}

// BuildFromCachedData builds timeline from cached aggregated data
func (tb *TimelineBuilder) BuildFromCachedData(cachedData []aggregator.AggregatedData) []TimelineEntry {
	var entries []TimelineEntry
	
	for _, data := range cachedData {
		// Add entries from hourly data
		hourlyEntries := tb.BuildFromHourlyData(data.HourlyStats)
		entries = append(entries, hourlyEntries...)
		
		// Add entries from cached limit messages
		for _, limit := range data.LimitMessages {
			entries = append(entries, TimelineEntry{
				Timestamp:   limit.Timestamp,
				ProjectName: data.ProjectName,
				Type:        "limit",
				Data:        limit,
			})
		}
	}
	
	return entries
}

// MergeTimelines merges multiple timelines and sorts by timestamp
func (tb *TimelineBuilder) MergeTimelines(timelines ...[]TimelineEntry) []TimelineEntry {
	var totalSize int
	for _, tl := range timelines {
		totalSize += len(tl)
	}
	
	merged := make([]TimelineEntry, 0, totalSize)
	for _, tl := range timelines {
		merged = append(merged, tl...)
	}
	
	// Sort by timestamp
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp < merged[j].Timestamp
	})
	
	return merged
}

// ConvertToTimestampedLogs converts timeline entries to TimestampedLog format
// This is for compatibility with existing session detection logic
func (tb *TimelineBuilder) ConvertToTimestampedLogs(entries []TimelineEntry) []TimestampedLog {
	var logs []TimestampedLog
	
	for _, entry := range entries {
		switch entry.Type {
		case "message":
			if log, ok := entry.Data.(model.ConversationLog); ok {
				logs = append(logs, TimestampedLog{
					Log:         log,
					Timestamp:   entry.Timestamp,
					ProjectName: entry.ProjectName,
				})
			}
		case "hourly":
			// For hourly data, create a synthetic log entry
			if data, ok := entry.Data.(aggregator.HourlyData); ok {
				// Create a minimal log entry that represents activity
				log := model.ConversationLog{
					Timestamp: time.Unix(entry.Timestamp, 0).Format(time.RFC3339),
					Type:      "synthetic",
					Message: model.Message{
						Model: data.Model,
						Usage: model.Usage{
							InputTokens:              data.InputTokens,
							OutputTokens:             data.OutputTokens,
							CacheCreationInputTokens: data.CacheCreation,
							CacheReadInputTokens:     data.CacheRead,
						},
					},
				}
				logs = append(logs, TimestampedLog{
					Log:         log,
					Timestamp:   entry.Timestamp,
					ProjectName: entry.ProjectName,
				})
			}
		}
	}
	
	return logs
}

// FilterByDuration filters timeline entries by duration from now
func (tb *TimelineBuilder) FilterByDuration(entries []TimelineEntry, duration time.Duration) []TimelineEntry {
	if duration <= 0 {
		return entries
	}
	
	cutoff := time.Now().Unix() - int64(duration.Seconds())
	var filtered []TimelineEntry
	
	for _, entry := range entries {
		if entry.Timestamp > cutoff {
			filtered = append(filtered, entry)
		}
	}
	
	return filtered
}

// DeduplicateEntries removes duplicate entries, preferring primary data over supplementary
func (tb *TimelineBuilder) DeduplicateEntries(entries []TimelineEntry) []TimelineEntry {
	if len(entries) == 0 {
		return entries
	}
	
	// Create maps to track seen entries and time windows
	// Use both exact and window-based deduplication
	seen := make(map[string]bool)
	timeWindows := make(map[string][]TimelineEntry) // Track entries by time window
	result := make([]TimelineEntry, 0, len(entries))
	
	primaryCount := 0
	supplementaryCount := 0
	skippedSupplementary := 0
	
	// First pass: Add all non-supplementary entries and track time windows
	for _, entry := range entries {
		if !entry.IsSupplementary {
			key := tb.getEntryKey(entry)
			seen[key] = true
			result = append(result, entry)
			primaryCount++
			
			// Track in time window map for overlap detection
			windowKey := tb.getTimeWindowKey(entry)
			timeWindows[windowKey] = append(timeWindows[windowKey], entry)
		} else {
			supplementaryCount++
		}
	}
	
	// Second pass: Add supplementary entries that don't overlap with primary data
	for _, entry := range entries {
		if entry.IsSupplementary {
			// Check both exact key and time window overlap
			exactKey := tb.getEntryKey(entry)
			windowKey := tb.getTimeWindowKey(entry)
			
			// Skip if exact duplicate or if there's data in the same time window
			if seen[exactKey] || len(timeWindows[windowKey]) > 0 {
				skippedSupplementary++
				continue
			}
			
			seen[exactKey] = true
			timeWindows[windowKey] = append(timeWindows[windowKey], entry)
			result = append(result, entry)
		}
	}
	
	// Log deduplication stats if significant duplicates were found
	if skippedSupplementary > supplementaryCount/2 {
		util.LogInfo(fmt.Sprintf("Deduplication: %d primary, %d supplementary (%d skipped), %d final",
			primaryCount, supplementaryCount, skippedSupplementary, len(result)))
	}
	
	return result
}

// getEntryKey generates a unique key for deduplication
func (tb *TimelineBuilder) getEntryKey(entry TimelineEntry) string {
	// Round timestamp to nearest minute for hourly data comparison
	timestamp := entry.Timestamp
	if entry.Type == "hourly" {
		timestamp = (timestamp / 60) * 60 // Round to minute
	}
	return fmt.Sprintf("%d-%s-%s", timestamp, entry.ProjectName, entry.Type)
}

// getTimeWindowKey generates a key for time window overlap detection
// This helps identify entries that represent the same time period even with different exact timestamps
func (tb *TimelineBuilder) getTimeWindowKey(entry TimelineEntry) string {
	// Round to 5-minute window for better overlap detection
	windowTimestamp := (entry.Timestamp / 300) * 300 // 5 minutes = 300 seconds
	
	// For hourly data, use hour boundaries
	if entry.Type == "hourly" {
		windowTimestamp = (entry.Timestamp / 3600) * 3600 // Round to hour
	}
	
	// Include project name to allow different projects in same time window
	return fmt.Sprintf("%d-%s", windowTimestamp, entry.ProjectName)
}
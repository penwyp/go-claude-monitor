package timeline

import (
	"fmt"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
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
	
	// Create a map to track seen entries
	// Key format: "timestamp-project-type" for uniqueness
	seen := make(map[string]bool)
	result := make([]TimelineEntry, 0, len(entries))
	
	// First pass: Add all non-supplementary entries
	for _, entry := range entries {
		if !entry.IsSupplementary {
			key := tb.getEntryKey(entry)
			seen[key] = true
			result = append(result, entry)
		}
	}
	
	// Second pass: Add supplementary entries that don't have primary data
	for _, entry := range entries {
		if entry.IsSupplementary {
			key := tb.getEntryKey(entry)
			if !seen[key] {
				seen[key] = true
				result = append(result, entry)
			}
		}
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
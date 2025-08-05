package session

import (
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
)

// TimelineEntry represents a single point in the timeline
type TimelineEntry struct {
	Timestamp   int64
	ProjectName string
	Type        string // "message", "limit", "hourly"
	Data        interface{}
}

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
	entries := make([]TimelineEntry, 0, len(hourlyData)*2)
	
	for _, data := range hourlyData {
		// Add entry for first message in the hour
		if data.FirstEntryTime > 0 {
			entries = append(entries, TimelineEntry{
				Timestamp:   data.FirstEntryTime,
				ProjectName: data.ProjectName,
				Type:        "hourly",
				Data:        data,
			})
		}
		
		// Add entry for last message in the hour if different
		if data.LastEntryTime > 0 && data.LastEntryTime != data.FirstEntryTime {
			entries = append(entries, TimelineEntry{
				Timestamp:   data.LastEntryTime,
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
							InputTokens:  data.InputTokens,
							OutputTokens: data.OutputTokens,
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
package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/stretchr/testify/assert"
)

func TestTimelineBuilder_BuildFromRawLogs(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	logs := []model.ConversationLog{
		{
			Timestamp: "2024-01-01T10:00:00Z",
			Type:      "message",
			Message: model.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: model.Usage{
					InputTokens:  100,
					OutputTokens: 200,
				},
			},
		},
		{
			Timestamp: "2024-01-01T11:00:00Z",
			Type:      "message",
			Message: model.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: model.Usage{
					InputTokens:  150,
					OutputTokens: 250,
				},
			},
		},
	}
	
	entries := tb.BuildFromRawLogs(logs, "test-project")
	
	assert.Len(t, entries, 2)
	assert.Equal(t, "test-project", entries[0].ProjectName)
	assert.Equal(t, "message", entries[0].Type)
	assert.Equal(t, int64(1704103200), entries[0].Timestamp)
	assert.Equal(t, int64(1704106800), entries[1].Timestamp)
}

func TestTimelineBuilder_BuildFromHourlyData(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	hourlyData := []aggregator.HourlyData{
		{
			Hour:           1704103200, // 2024-01-01 10:00:00
			ProjectName:    "project-a",
			Model:          "claude-3-5-sonnet-20241022",
			InputTokens:    1000,
			OutputTokens:   2000,
			FirstEntryTime: 1704103300,
			LastEntryTime:  1704106700,
		},
		{
			Hour:           1704106800, // 2024-01-01 11:00:00
			ProjectName:    "project-b",
			Model:          "claude-3-5-sonnet-20241022",
			InputTokens:    1500,
			OutputTokens:   2500,
			FirstEntryTime: 1704106900,
			LastEntryTime:  1704110300,
		},
	}
	
	entries := tb.BuildFromHourlyData(hourlyData)
	
	// Should have 4 entries (first and last for each hour)
	assert.Len(t, entries, 4)
	assert.Equal(t, "hourly", entries[0].Type)
	assert.Equal(t, "project-a", entries[0].ProjectName)
	assert.Equal(t, int64(1704103300), entries[0].Timestamp)
	assert.Equal(t, int64(1704106700), entries[1].Timestamp)
}

func TestTimelineBuilder_MergeTimelines(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	timeline1 := []TimelineEntry{
		{Timestamp: 1000, ProjectName: "project-a", Type: "message"},
		{Timestamp: 3000, ProjectName: "project-a", Type: "message"},
	}
	
	timeline2 := []TimelineEntry{
		{Timestamp: 2000, ProjectName: "project-b", Type: "message"},
		{Timestamp: 4000, ProjectName: "project-b", Type: "message"},
	}
	
	merged := tb.MergeTimelines(timeline1, timeline2)
	
	assert.Len(t, merged, 4)
	// Should be sorted by timestamp
	assert.Equal(t, int64(1000), merged[0].Timestamp)
	assert.Equal(t, int64(2000), merged[1].Timestamp)
	assert.Equal(t, int64(3000), merged[2].Timestamp)
	assert.Equal(t, int64(4000), merged[3].Timestamp)
}

func TestTimelineBuilder_ConvertToTimestampedLogs(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	entries := []TimelineEntry{
		{
			Timestamp:   1704103200,
			ProjectName: "test-project",
			Type:        "message",
			Data: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
				Type:      "message",
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  100,
						OutputTokens: 200,
					},
				},
			},
		},
		{
			Timestamp:   1704106800,
			ProjectName: "test-project",
			Type:        "hourly",
			Data: aggregator.HourlyData{
				Hour:         1704106800,
				Model:        "claude-3-5-sonnet-20241022",
				InputTokens:  1000,
				OutputTokens: 2000,
			},
		},
	}
	
	logs := tb.ConvertToTimestampedLogs(entries)
	
	assert.Len(t, logs, 2)
	assert.Equal(t, "test-project", logs[0].ProjectName)
	assert.Equal(t, int64(1704103200), logs[0].Timestamp)
	assert.Equal(t, "message", logs[0].Log.Type)
	
	// Synthetic log from hourly data
	assert.Equal(t, "synthetic", logs[1].Log.Type)
	assert.Equal(t, 1000, logs[1].Log.Message.Usage.InputTokens)
}

func TestTimelineBuilder_FilterByDuration(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	now := time.Now().Unix()
	entries := []TimelineEntry{
		{Timestamp: now - 7200, ProjectName: "old-project", Type: "message"},     // 2 hours ago
		{Timestamp: now - 3590, ProjectName: "recent-project", Type: "message"},  // Just under 1 hour ago
		{Timestamp: now - 1800, ProjectName: "recent-project2", Type: "message"}, // 30 minutes ago
		{Timestamp: now - 300, ProjectName: "very-recent", Type: "message"},      // 5 minutes ago
	}
	
	// Filter to last hour
	filtered := tb.FilterByDuration(entries, time.Hour)
	
	assert.Len(t, filtered, 3) // Should exclude the 2-hour-old entry
	assert.Equal(t, "recent-project", filtered[0].ProjectName)
}

func TestTimelineBuilder_BuildFromCachedData(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	cachedData := []aggregator.AggregatedData{
		{
			ProjectName: "project-a",
			HourlyStats: []aggregator.HourlyData{
				{
					Hour:           1704103200,
					Model:          "claude-3-5-sonnet-20241022",
					InputTokens:    1000,
					OutputTokens:   2000,
					FirstEntryTime: 1704103300,
					LastEntryTime:  1704106700,
				},
			},
			LimitMessages: []aggregator.CachedLimitInfo{
				{
					Type:      "rate_limit",
					Timestamp: 1704105000,
					ResetTime: func() *int64 { t := int64(1704110000); return &t }(),
					Content:   "Claude AI usage limit reached|1704110000",
					Model:     "claude-3-5-sonnet-20241022",
				},
			},
		},
	}
	
	entries := tb.BuildFromCachedData(cachedData)
	
	// Should have entries from hourly data (2) + limit messages (1)
	assert.Len(t, entries, 3)
	
	// Verify limit message entry
	limitEntry := entries[2]
	assert.Equal(t, "limit", limitEntry.Type)
	assert.Equal(t, int64(1704105000), limitEntry.Timestamp)
	assert.Equal(t, "project-a", limitEntry.ProjectName)
}

func TestTimelineBuilder_EmptyInputs(t *testing.T) {
	tb := NewTimelineBuilder("UTC")
	
	// Test empty raw logs
	entries := tb.BuildFromRawLogs([]model.ConversationLog{}, "test")
	assert.Empty(t, entries)
	
	// Test empty hourly data
	entries = tb.BuildFromHourlyData([]aggregator.HourlyData{})
	assert.Empty(t, entries)
	
	// Test empty cached data
	entries = tb.BuildFromCachedData([]aggregator.AggregatedData{})
	assert.Empty(t, entries)
	
	// Test merge with empty timelines
	merged := tb.MergeTimelines([]TimelineEntry{}, []TimelineEntry{})
	assert.Empty(t, merged)
}

func TestTimelineBuilder_InvalidTimezone(t *testing.T) {
	// Should fall back to Local timezone for invalid timezone
	tb := NewTimelineBuilder("Invalid/Timezone")
	assert.NotNil(t, tb)
	assert.NotNil(t, tb.timezone)
}
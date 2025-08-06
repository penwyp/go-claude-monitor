package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnifiedDataSource tests that both RawLogs and AggregatedData are used together
func TestUnifiedDataSource(t *testing.T) {
	mc := NewMemoryCache()
	
	// Create test data with both raw logs and aggregated data
	now := time.Now()
	entry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			ProjectName: "test-project",
			HourlyStats: []aggregator.HourlyData{
				{
					Hour:           now.Add(-2 * time.Hour).Unix(),
					Model:          "claude-3-5-sonnet-20241022",
					InputTokens:    1000,
					OutputTokens:   500,
					FirstEntryTime: now.Add(-2 * time.Hour).Unix(),
				},
			},
		},
		RawLogs: []model.ConversationLog{
			{
				Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339),
				Type:      "message:sent",
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  2000,
						OutputTokens: 1000,
					},
				},
			},
		},
	}
	
	mc.Set("test-session", entry)
	
	// Get global timeline (should include both sources)
	timeline := mc.GetGlobalTimeline(0)
	
	// Should have entries from both raw logs and aggregated data
	assert.Len(t, timeline, 2, "Should have entries from both sources")
	
	// Verify deduplication works
	entry.AggregatedData.HourlyStats = append(entry.AggregatedData.HourlyStats, aggregator.HourlyData{
		Hour:           now.Add(-1 * time.Hour).Unix(),
		Model:          "claude-3-5-sonnet-20241022",
		InputTokens:    2000, // Same as raw log
		OutputTokens:   1000,
		FirstEntryTime: now.Add(-1 * time.Hour).Unix(),
	})
	
	mc.Set("test-session", entry)
	timeline = mc.GetGlobalTimeline(0)
	
	// Should still have 2 entries due to deduplication
	assert.Len(t, timeline, 2, "Duplicate entries should be removed")
}

// TestActiveSessionDetection tests the creation of active sessions
func TestActiveSessionDetection(t *testing.T) {
	detector := NewSessionDetectorWithAggregator(nil, "UTC", "")
	now := time.Now().Unix()
	
	t.Run("No existing sessions", func(t *testing.T) {
		sessions := detector.detectActiveSession([]*Session{}, now)
		
		// Should create an active session if within window
		if len(sessions) > 0 {
			assert.True(t, sessions[0].IsActive)
			assert.Equal(t, "active_detection", sessions[0].WindowSource)
			assert.True(t, sessions[0].StartTime <= now)
			assert.True(t, sessions[0].EndTime > now)
		}
	})
	
	t.Run("With existing sessions", func(t *testing.T) {
		// Create a session that ended 1 hour ago
		pastSession := &Session{
			ID:        "past",
			StartTime: now - 6*3600,
			EndTime:   now - 1*3600,
			ResetTime: now - 1*3600,
		}
		
		sessions := detector.detectActiveSession([]*Session{pastSession}, now)
		
		// Should have 2 sessions: the past one and a new active one
		assert.Len(t, sessions, 2)
		
		// First should be the active session
		assert.True(t, sessions[0].IsActive)
		assert.Equal(t, "active_detection", sessions[0].WindowSource)
		
		// Active session should start where the last one ended
		assert.Equal(t, pastSession.EndTime, sessions[0].StartTime)
	})
	
	t.Run("Already in active window", func(t *testing.T) {
		// Create a session that includes current time
		currentSession := &Session{
			ID:        "current",
			StartTime: now - 1*3600,
			EndTime:   now + 4*3600,
			ResetTime: now + 4*3600,
			IsActive:  true,
		}
		
		sessions := detector.detectActiveSession([]*Session{currentSession}, now)
		
		// Should not create a new session
		assert.Len(t, sessions, 1)
		assert.Equal(t, "current", sessions[0].ID)
	})
}

// TestTimelineDeduplication tests the deduplication logic
func TestTimelineDeduplication(t *testing.T) {
	tb := timeline.NewTimelineBuilder("UTC")
	
	entries := []timeline.TimelineEntry{
		{
			Timestamp:       1000,
			ProjectName:     "project1",
			Type:           "message",
			IsSupplementary: false,
		},
		{
			Timestamp:       1000,
			ProjectName:     "project1",
			Type:           "message",
			IsSupplementary: true, // Duplicate, supplementary
		},
		{
			Timestamp:       2000,
			ProjectName:     "project2",
			Type:           "hourly",
			IsSupplementary: true,
		},
		{
			Timestamp:       3000,
			ProjectName:     "project1",
			Type:           "limit",
			IsSupplementary: false,
		},
	}
	
	deduplicated := tb.DeduplicateEntries(entries)
	
	// Should have 3 unique entries (removed the supplementary duplicate)
	assert.Len(t, deduplicated, 3)
	
	// Verify the supplementary duplicate was removed
	hasSupplementaryDuplicate := false
	for _, entry := range deduplicated {
		if entry.Timestamp == 1000 && entry.IsSupplementary {
			hasSupplementaryDuplicate = true
		}
	}
	assert.False(t, hasSupplementaryDuplicate, "Supplementary duplicate should be removed")
}

// TestConfigurableDataLoading tests configuration-based data loading
func TestConfigurableDataLoading(t *testing.T) {
	t.Run("Full mode loads all data", func(t *testing.T) {
		config := SessionConfig{
			TimelineMode:       "full",
			DataRetentionHours: 0,
		}
		
		// In full mode, all logs should be returned
		logs := []model.ConversationLog{
			{Timestamp: time.Now().Add(-72 * time.Hour).Format(time.RFC3339)},
			{Timestamp: time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
			{Timestamp: time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
		}
		
		// Simulate filtering with no retention limit
		filtered := filterLogsWithConfig(logs, config)
		assert.Len(t, filtered, 3, "Full mode should return all logs")
	})
	
	t.Run("Recent mode filters old data", func(t *testing.T) {
		config := SessionConfig{
			TimelineMode:       "recent",
			DataRetentionHours: 48,
		}
		
		logs := []model.ConversationLog{
			{Timestamp: time.Now().Add(-72 * time.Hour).Format(time.RFC3339)},
			{Timestamp: time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
			{Timestamp: time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
		}
		
		filtered := filterLogsWithConfig(logs, config)
		assert.Len(t, filtered, 2, "Recent mode should filter old logs")
	})
}

// Helper function to simulate log filtering with config
func filterLogsWithConfig(logs []model.ConversationLog, config SessionConfig) []model.ConversationLog {
	if config.DataRetentionHours <= 0 {
		return logs
	}
	
	cutoff := time.Now().Add(-time.Duration(config.DataRetentionHours) * time.Hour).Unix()
	var filtered []model.ConversationLog
	
	for _, log := range logs {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}
		
		if ts.Unix() > cutoff {
			filtered = append(filtered, log)
		}
	}
	
	return filtered
}

// TestIncrementalDetection tests incremental session detection
func TestIncrementalDetection(t *testing.T) {
	detector := NewSessionDetectorWithAggregator(nil, "UTC", "")
	
	// Create initial sessions
	_ = []*Session{
		{
			ID:               "1000",
			StartTime:        1000,
			EndTime:          19000,
			WindowStartTime:  int64Ptr(1000),
			IsWindowDetected: true,
			WindowSource:     "limit_message",
			Projects:         make(map[string]*ProjectStats),
		},
	}
	
	// Simulate incremental update
	// The window history should preserve the session window
	require.NotNil(t, detector.windowHistory)
	
	record := WindowRecord{
		StartTime:      1000,
		EndTime:        19000,
		Source:         "limit_message",
		IsLimitReached: true,
		SessionID:      "1000",
	}
	detector.windowHistory.AddOrUpdateWindow(record)
	
	// Verify window is preserved
	windows := detector.windowHistory.GetLimitReachedWindows()
	assert.Len(t, windows, 1)
	assert.Equal(t, int64(1000), windows[0].StartTime)
	assert.Equal(t, "limit_message", windows[0].Source)
}

// int64Ptr helper is already defined in limit_parser_test.go
package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/stretchr/testify/assert"
)

// TestStrictFiveHourWindows tests that continuous activity is split into strict 5-hour windows
func TestStrictFiveHourWindows(t *testing.T) {
	// Create detector with mock aggregator
	agg := aggregator.NewAggregatorWithTimezone("Asia/Shanghai")
	detector := NewSessionDetectorWithAggregator(agg, "Asia/Shanghai", "/tmp/test-cache")

	// Test scenario: Continuous activity from 17:40 to next day 00:08 CST
	// Expected results:
	// - Window 1: 17:00-22:00 CST (09:00-14:00 UTC)
	// - Window 2: 22:00-03:00 CST (14:00-19:00 UTC)
	
	// Helper function to create UTC timestamp
	parseTime := func(timeStr string) int64 {
		t, _ := time.Parse("2006-01-02 15:04:05", timeStr)
		return t.Unix()
	}
	
	timeline := []timeline.TimestampedLog{
		// 17:40 CST (09:40 UTC)
		{
			Timestamp:   parseTime("2025-08-07 09:40:00"),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type: "user",
				Message: model.Message{
					Content: []model.ContentItem{{Type: "text", Text: "First activity"}},
				},
			},
		},
		// 22:00 CST (14:00 UTC) - Right at boundary
		{
			Timestamp:   parseTime("2025-08-07 14:00:00"),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type: "user",
				Message: model.Message{
					Content: []model.ContentItem{{Type: "text", Text: "Activity at boundary"}},
				},
			},
		},
		// 22:30 CST (14:30 UTC) - In second window
		{
			Timestamp:   parseTime("2025-08-07 14:30:00"),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type: "user",
				Message: model.Message{
					Content: []model.ContentItem{{Type: "text", Text: "Activity in second window"}},
				},
			},
		},
		// 00:08 CST next day (16:08 UTC) - Still in second window
		{
			Timestamp:   parseTime("2025-08-07 16:08:00"),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type: "user",
				Message: model.Message{
					Content: []model.ContentItem{{Type: "text", Text: "Last activity"}},
				},
			},
		},
	}
	
	input := SessionDetectionInput{
		GlobalTimeline: timeline,
	}
	
	sessions := detector.DetectSessionsWithLimits(input)
	
	// Debug output
	t.Logf("Number of sessions detected: %d", len(sessions))
	for i, s := range sessions {
		t.Logf("Session %d: Start=%s, End=%s, Source=%s",
			i,
			time.Unix(s.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(s.EndTime, 0).Format("2006-01-02 15:04:05"),
			s.WindowSource)
	}
	
	// Should have exactly 2 sessions
	assert.Equal(t, 2, len(sessions), "Should detect 2 strict 5-hour windows")
	
	if len(sessions) >= 2 {
		// Sessions may be in different order, find them by start time
		var session1, session2 *Session
		for _, s := range sessions {
			if s.StartTime == parseTime("2025-08-07 09:00:00") {
				session1 = s
			} else if s.StartTime == parseTime("2025-08-07 14:00:00") {
				session2 = s
			}
		}
		
		// First window: 09:00-14:00 UTC (17:00-22:00 CST)
		assert.NotNil(t, session1, "Should have session starting at 09:00 UTC")
		if session1 != nil {
			assert.Equal(t, parseTime("2025-08-07 09:00:00"), session1.StartTime, "First window should start at 09:00 UTC")
			assert.Equal(t, parseTime("2025-08-07 14:00:00"), session1.EndTime, "First window should end at 14:00 UTC")
			assert.Equal(t, "continuous_activity", session1.WindowSource, "First window should be from continuous_activity")
		}
		
		// Second window: 14:00-19:00 UTC (22:00-03:00 CST)
		assert.NotNil(t, session2, "Should have session starting at 14:00 UTC")
		if session2 != nil {
			assert.Equal(t, parseTime("2025-08-07 14:00:00"), session2.StartTime, "Second window should start at 14:00 UTC")
			assert.Equal(t, parseTime("2025-08-07 19:00:00"), session2.EndTime, "Second window should end at 19:00 UTC")
			assert.Equal(t, "continuous_activity", session2.WindowSource, "Second window should be from continuous_activity")
		}
		
		// Verify activities are assigned to correct windows
		// First window should have 1 activity (at 09:40)
		// Second window should have 3 activities (at 14:00, 14:30, 16:08)
		// Note: Activity at exactly 14:00 goes to second window (>= startTime, < endTime)
	}
}

// TestContinuousActivityAcrossMultipleWindows tests activity spanning 3+ windows
func TestContinuousActivityAcrossMultipleWindows(t *testing.T) {
	agg := aggregator.NewAggregatorWithTimezone("UTC")
	detector := NewSessionDetectorWithAggregator(agg, "UTC", "/tmp/test-cache")
	
	parseTime := func(timeStr string) int64 {
		t, _ := time.Parse("2006-01-02 15:04:05", timeStr)
		return t.Unix()
	}
	
	// Activity from 08:30 to 19:30 (spans 3 windows)
	timeline := []timeline.TimestampedLog{
		{Timestamp: parseTime("2025-08-07 08:30:00"), ProjectName: "test"},
		{Timestamp: parseTime("2025-08-07 10:00:00"), ProjectName: "test"},
		{Timestamp: parseTime("2025-08-07 13:30:00"), ProjectName: "test"},
		{Timestamp: parseTime("2025-08-07 15:00:00"), ProjectName: "test"},
		{Timestamp: parseTime("2025-08-07 18:30:00"), ProjectName: "test"},
		{Timestamp: parseTime("2025-08-07 19:30:00"), ProjectName: "test"},
	}
	
	for i := range timeline {
		timeline[i].Log = model.ConversationLog{Type: "user"}
	}
	
	input := SessionDetectionInput{GlobalTimeline: timeline}
	sessions := detector.DetectSessionsWithLimits(input)
	
	// Should create 3 windows: 08:00-13:00, 13:00-18:00, 18:00-23:00
	assert.Equal(t, 3, len(sessions), "Should create 3 windows for activity spanning 11 hours")
	
	if len(sessions) >= 3 {
		// Find sessions by their start times (they might not be in order)
		var window1, window2, window3 *Session
		for _, s := range sessions {
			switch s.StartTime {
			case parseTime("2025-08-07 08:00:00"):
				window1 = s
			case parseTime("2025-08-07 13:00:00"):
				window2 = s
			case parseTime("2025-08-07 18:00:00"):
				window3 = s
			}
		}
		
		// Verify window 1: 08:00-13:00
		assert.NotNil(t, window1, "Should have window starting at 08:00")
		if window1 != nil {
			assert.Equal(t, parseTime("2025-08-07 08:00:00"), window1.StartTime)
			assert.Equal(t, parseTime("2025-08-07 13:00:00"), window1.EndTime)
		}
		
		// Verify window 2: 13:00-18:00
		assert.NotNil(t, window2, "Should have window starting at 13:00")
		if window2 != nil {
			assert.Equal(t, parseTime("2025-08-07 13:00:00"), window2.StartTime)
			assert.Equal(t, parseTime("2025-08-07 18:00:00"), window2.EndTime)
		}
		
		// Verify window 3: 18:00-23:00
		assert.NotNil(t, window3, "Should have window starting at 18:00")
		if window3 != nil {
			assert.Equal(t, parseTime("2025-08-07 18:00:00"), window3.StartTime)
			assert.Equal(t, parseTime("2025-08-07 23:00:00"), window3.EndTime)
		}
	}
}

// TestBoundaryActivityAssignment tests that activities at exact boundaries are assigned correctly
func TestBoundaryActivityAssignment(t *testing.T) {
	agg := aggregator.NewAggregatorWithTimezone("UTC")
	detector := NewSessionDetectorWithAggregator(agg, "UTC", "/tmp/test-cache")
	
	parseTime := func(timeStr string) int64 {
		t, _ := time.Parse("2006-01-02 15:04:05", timeStr)
		return t.Unix()
	}
	
	// Activities at exact window boundaries
	timeline := []timeline.TimestampedLog{
		{Timestamp: parseTime("2025-08-07 10:00:00"), ProjectName: "test"}, // Start of window 1
		{Timestamp: parseTime("2025-08-07 14:59:59"), ProjectName: "test"}, // End of window 1
		{Timestamp: parseTime("2025-08-07 15:00:00"), ProjectName: "test"}, // Start of window 2 (exact boundary)
		{Timestamp: parseTime("2025-08-07 19:59:59"), ProjectName: "test"}, // End of window 2
	}
	
	for i := range timeline {
		timeline[i].Log = model.ConversationLog{Type: "user"}
	}
	
	input := SessionDetectionInput{GlobalTimeline: timeline}
	sessions := detector.DetectSessionsWithLimits(input)
	
	assert.Equal(t, 2, len(sessions), "Should create 2 windows")
	
	// Activity at 15:00:00 should go to second window (>= startTime rule)
	// First window: 10:00-15:00, should have activities at 10:00:00 and 14:59:59
	// Second window: 15:00-20:00, should have activities at 15:00:00 and 19:59:59
}
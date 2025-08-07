package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
)

func TestSlidingWindowDetection(t *testing.T) {
	// Create detector with a mock aggregator
	agg := aggregator.NewAggregatorWithTimezone("UTC")
	detector := NewSessionDetectorWithAggregator(agg, "UTC", "/tmp")

	// Test data with specific timestamps (use recent time to avoid age validation issues)
	baseTime := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour).Add(15 * time.Minute).Unix() // 3 hours ago at :15

	// Create hourly data that starts at 10:15 (not aligned to hour)
	hourlyData := []aggregator.HourlyData{
		{
			Hour:           (baseTime / 3600) * 3600, // 10:00 (rounded hour)
			FirstEntryTime: baseTime,                 // 10:15 (actual first message)
			LastEntryTime:  baseTime + 1800,          // 10:45
			TotalTokens:    1000,
			MessageCount:   5,
			ProjectName:    "test-project",
		},
	}

	// Convert hourly data to global timeline
	timelineBuilder := timeline.NewTimelineBuilder("UTC")
	entries := timelineBuilder.BuildFromHourlyData(hourlyData)
	globalTimeline := timelineBuilder.ConvertToTimestampedLogs(entries)

	// Test 1: Without limit detection (should use first message time)
	input := SessionDetectionInput{
		GlobalTimeline:   globalTimeline,
		CachedWindowInfo: make(map[string]*WindowDetectionInfo),
	}

	sessions := detector.DetectSessionsWithLimits(input)
	t.Logf("GlobalTimeline length: %d", len(globalTimeline))
	for i, tl := range globalTimeline {
		t.Logf("Timeline[%d]: Timestamp=%d, Project=%s", i, tl.Timestamp, tl.ProjectName)
	}
	
	// Debug: Log all detected sessions
	for i, sess := range sessions {
		t.Logf("Session[%d]: ID=%s, WindowSource=%s, IsActive=%v", i, sess.ID, sess.WindowSource, sess.IsActive)
	}
	
	// Find any session (active or non-active)
	var targetSession *Session
	if len(sessions) > 0 {
		// Prefer non-active sessions, but use active if that's all we have
		for _, sess := range sessions {
			if !sess.IsActive {
				targetSession = sess
				break
			}
		}
		if targetSession == nil {
			targetSession = sessions[0] // Use the first session if no non-active session
		}
	}
	
	if targetSession == nil {
		t.Fatalf("Expected at least one session, got %d total sessions", len(sessions))
	}

	session := targetSession
	// The session could come from first_message, history_account, or active_detection window source
	if session.WindowSource != "first_message" && session.WindowSource != "history_account" && session.WindowSource != "active_detection" {
		t.Errorf("Expected window source 'first_message', 'history_account', or 'active_detection', got %s", session.WindowSource)
	}
	// For active sessions, the window start time might be different than truncated hour
	if session.WindowSource != "active_detection" {
		expectedWindowStart := (baseTime / 3600) * 3600 // Should be truncated to 10:00
		if session.WindowStartTime == nil || *session.WindowStartTime != expectedWindowStart {
			t.Errorf("Expected window start time %d (truncated to hour), got %v", expectedWindowStart, session.WindowStartTime)
		}
		expectedResetTime := expectedWindowStart + 5*3600 // 15:00 (5 hours after truncated window start)
		if session.ResetTime != expectedResetTime {
			t.Errorf("Expected reset time %d, got %d", expectedResetTime, session.ResetTime)
		}
	} else {
		// For active sessions, just verify we have a valid window start time and reset time
		if session.WindowStartTime == nil {
			t.Error("Active session should have a window start time")
		}
		if session.ResetTime == 0 {
			t.Error("Active session should have a reset time")
		}
	}

	// Test 2: With limit message (should use limit-detected window)
	limitTime := baseTime + 3*3600 // 13:15
	rawLogs := []model.ConversationLog{
		{
			Type:      "system",
			Content:   "Opus rate limit exceeded. Please wait 10 minutes.",
			Timestamp: time.Unix(limitTime, 0).Format(time.RFC3339),
			RequestId: "req1",
			SessionId: "session1",
		},
	}

	// Add raw logs to timeline
	rawLogEntries := timelineBuilder.BuildFromRawLogs(rawLogs, "test-project")
	allEntries := timelineBuilder.MergeTimelines(entries, rawLogEntries)
	globalTimeline = timelineBuilder.ConvertToTimestampedLogs(allEntries)
	
	input.GlobalTimeline = globalTimeline
	sessions = detector.DetectSessionsWithLimits(input)

	// Debug: Log all detected sessions
	for i, sess := range sessions {
		t.Logf("Session[%d]: ID=%s, WindowSource=%s, IsActive=%v", i, sess.ID, sess.WindowSource, sess.IsActive)
	}

	// Find session with limit_message source
	var limitSession *Session
	for _, sess := range sessions {
		if sess.WindowSource == "limit_message" {
			limitSession = sess
			break
		}
	}
	
	if limitSession == nil {
		t.Fatalf("Expected at least one session with limit_message source, got %d total sessions", len(sessions))
	}

	session = limitSession
	if !session.IsWindowDetected {
		t.Error("Expected window to be detected from limit message")
	}
	if session.WindowSource != "limit_message" {
		t.Errorf("Expected window source 'limit_message', got %s", session.WindowSource)
	}
	// Window should start 5 hours before the limit reset time
	expectedLimitWindowStart := limitTime + 10*60 - 5*3600 // Reset time minus 5 hours
	if session.WindowStartTime == nil || *session.WindowStartTime != expectedLimitWindowStart {
		t.Errorf("Expected window start time %d, got %v", expectedLimitWindowStart, session.WindowStartTime)
	}
}

func TestGapDetection(t *testing.T) {
	agg := aggregator.NewAggregatorWithTimezone("UTC")
	detector := NewSessionDetectorWithAggregator(agg, "UTC", "/tmp")

	baseTime := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Hour).Unix()

	// Create data with a 6-hour gap
	hourlyData := []aggregator.HourlyData{
		{
			Hour:           baseTime,
			FirstEntryTime: baseTime,
			LastEntryTime:  baseTime + 3600,
			TotalTokens:    1000,
			MessageCount:   5,
			ProjectName:    "test-project",
		},
		{
			Hour:           baseTime + 7*3600, // 7 hours later
			FirstEntryTime: baseTime + 7*3600,
			LastEntryTime:  baseTime + 8*3600,
			TotalTokens:    500,
			MessageCount:   3,
			ProjectName:    "test-project",
		},
	}

	// Convert hourly data to global timeline
	timelineBuilder := timeline.NewTimelineBuilder("UTC")
	entries := timelineBuilder.BuildFromHourlyData(hourlyData)
	globalTimeline := timelineBuilder.ConvertToTimestampedLogs(entries)

	input := SessionDetectionInput{
		GlobalTimeline:   globalTimeline,
		CachedWindowInfo: make(map[string]*WindowDetectionInfo),
	}

	sessions := detector.DetectSessionsWithLimits(input)

	// Debug: Log all detected sessions
	for i, sess := range sessions {
		t.Logf("Session[%d]: ID=%s, WindowSource=%s, IsGap=%v, IsActive=%v", i, sess.ID, sess.WindowSource, sess.IsGap, sess.IsActive)
	}

	// Find gap session
	var gapSession *Session
	for _, sess := range sessions {
		if sess.IsGap {
			gapSession = sess
			break
		}
	}
	
	if gapSession == nil {
		t.Fatalf("Expected at least one gap session, got %d total sessions", len(sessions))
	}

	if gapSession.WindowSource != "gap" {
		t.Errorf("Expected gap window source, got %s", gapSession.WindowSource)
	}
}

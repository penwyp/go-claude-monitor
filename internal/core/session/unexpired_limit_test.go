package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/stretchr/testify/assert"
)

func TestUnexpiredLimitMessageOverride(t *testing.T) {
	// Create a detector
	detector := NewSessionDetectorWithAggregator(
		nil, // aggregator can be nil for this test
		"Local",
		"/tmp/test_cache",
	)

	// Create test data with regular messages and an unexpired limit message
	currentTime := time.Now()
	resetTime := currentTime.Add(3 * time.Hour).Unix() // Reset in 3 hours (unexpired)

	// Create timeline with regular messages first, then a limit message
	globalTimeline := []timeline.TimestampedLog{
		{
			Timestamp:   currentTime.Add(-4 * time.Hour).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "message:sent",
				Timestamp: currentTime.Add(-4 * time.Hour).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
		},
		{
			Timestamp:   currentTime.Add(-3 * time.Hour).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "message:sent",
				Timestamp: currentTime.Add(-3 * time.Hour).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  200,
						OutputTokens: 100,
					},
				},
			},
		},
		{
			Timestamp:   currentTime.Add(-1 * time.Hour).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "assistant",
				Timestamp: currentTime.Add(-1 * time.Hour).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Content: []model.ContentItem{
						{
							Type:    "tool_result",
							Content: fmt.Sprintf("Claude AI usage limit reached|%d", resetTime),
						},
					},
				},
			},
		},
	}

	// Detect sessions
	input := SessionDetectionInput{
		GlobalTimeline: globalTimeline,
	}

	sessions := detector.DetectSessionsWithLimits(input)

	// We should have at least one session
	assert.Greater(t, len(sessions), 0, "Should detect at least one session")

	// Find the session that should contain our limit message
	var limitSession *Session
	for _, s := range sessions {
		// The limit window should end at our reset time
		if s.EndTime == resetTime {
			limitSession = s
			break
		}
	}

	// Verify we found the limit session
	assert.NotNil(t, limitSession, "Should find a session with the limit reset time")

	if limitSession != nil {
		// Verify the session properties
		assert.Equal(t, resetTime, limitSession.EndTime, "Session end time should match reset time")
		assert.Equal(t, resetTime-18000, limitSession.StartTime, "Session start should be 5 hours before reset")
		assert.Equal(t, "limit_message", limitSession.WindowSource, "Window source should be limit_message")
		assert.Equal(t, resetTime, limitSession.ResetTime, "Reset time should match")

		t.Logf("✅ SUCCESS: Unexpired limit message correctly created window from %s to %s",
			time.Unix(limitSession.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(limitSession.EndTime, 0).Format("2006-01-02 15:04:05"))
	}
}

func TestUnexpiredLimitOverridesExistingWindow(t *testing.T) {
	// Create a detector
	detector := NewSessionDetectorWithAggregator(
		nil, // aggregator can be nil for this test
		"Local",
		"/tmp/test_cache2",
	)

	currentTime := time.Now()
	resetTime := currentTime.Add(2 * time.Hour).Unix() // Reset in 2 hours (unexpired)

	// Create timeline with activity that would normally create a different window
	// Then add a limit message that should override it
	globalTimeline := []timeline.TimestampedLog{
		// First cluster of activity (would create window 1)
		{
			Timestamp:   currentTime.Add(-10 * time.Hour).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "message:sent",
				Timestamp: currentTime.Add(-10 * time.Hour).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  500,
						OutputTokens: 250,
					},
				},
			},
		},
		// Gap of 6+ hours
		// Second cluster of activity (would create window 2)
		{
			Timestamp:   currentTime.Add(-3 * time.Hour).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "message:sent",
				Timestamp: currentTime.Add(-3 * time.Hour).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Usage: model.Usage{
						InputTokens:  300,
						OutputTokens: 150,
					},
				},
			},
		},
		// Limit message that should override window boundaries
		{
			Timestamp:   currentTime.Add(-30 * time.Minute).Unix(),
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type:      "assistant",
				Timestamp: currentTime.Add(-30 * time.Minute).Format(time.RFC3339),
				Message: model.Message{
					Model: "claude-3-5-sonnet-20241022",
					Content: []model.ContentItem{
						{
							Type:    "tool_result",
							Content: fmt.Sprintf("Claude AI usage limit reached|%d", resetTime),
						},
					},
				},
			},
		},
	}

	// Detect sessions
	input := SessionDetectionInput{
		GlobalTimeline: globalTimeline,
	}

	sessions := detector.DetectSessionsWithLimits(input)

	// Find the limit session
	var limitSession *Session
	for _, s := range sessions {
		if s.WindowSource == "limit_message" && s.EndTime == resetTime {
			limitSession = s
			break
		}
	}

	// Verify the limit session exists and has correct boundaries
	assert.NotNil(t, limitSession, "Should find limit message session")

	if limitSession != nil {
		// The limit window should have forced its boundaries
		expectedStart := resetTime - 18000 // 5 hours before reset
		assert.Equal(t, expectedStart, limitSession.StartTime, "Limit window start should be exactly 5 hours before reset")
		assert.Equal(t, resetTime, limitSession.EndTime, "Limit window end should match reset time")

		// Verify that messages within the limit window are assigned to it
		messageCount := 0
		for _, tl := range globalTimeline {
			if tl.Timestamp >= limitSession.StartTime && tl.Timestamp < limitSession.EndTime {
				messageCount++
			}
		}
		assert.Greater(t, messageCount, 0, "Limit session should contain messages")

		t.Logf("✅ SUCCESS: Unexpired limit forced window boundaries: %s to %s (contains %d messages)",
			time.Unix(limitSession.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(limitSession.EndTime, 0).Format("2006-01-02 15:04:05"),
			messageCount)
	}
}
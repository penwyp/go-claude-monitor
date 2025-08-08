package strategies

import (
	"testing"
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/stretchr/testify/assert"
)

// Mock implementations for testing

type mockWindowHistory struct {
	accountWindows []HistoricalWindow
	recentWindows  []HistoricalWindow
	limitWindows   []HistoricalWindow
}

func (m *mockWindowHistory) GetAccountLevelWindows() []HistoricalWindow {
	return m.accountWindows
}

func (m *mockWindowHistory) GetRecentWindows(duration time.Duration) []HistoricalWindow {
	return m.recentWindows
}

func (m *mockWindowHistory) GetLimitReachedWindows() []HistoricalWindow {
	return m.limitWindows
}

type mockLimitParser struct {
	limits []LimitInfo
}

func (m *mockLimitParser) ParseLogs(logs []interface{}) []LimitInfo {
	return m.limits
}

// Helper function to create test timeline
func createTestTimeline(timestamps ...int64) []timeline.TimestampedLog {
	timelineLogs := make([]timeline.TimestampedLog, 0, len(timestamps))
	for _, ts := range timestamps {
		timelineLogs = append(timelineLogs, timeline.TimestampedLog{
			Timestamp:   ts,
			ProjectName: "test-project",
			Log: model.ConversationLog{
				Type: "user",
				Message: model.Message{
					Content: []model.ContentItem{{Type: "text", Text: "test"}},
				},
			},
		})
	}
	return timelineLogs
}

// TestStrategyRegistry tests the strategy registry functionality
func TestStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()
	
	// Test registering strategies
	strategy1 := NewFirstMessageStrategy()
	strategy2 := NewGapDetectionStrategy()
	strategy3 := NewContinuousActivityStrategy()
	
	registry.Register(strategy1)
	registry.Register(strategy2)
	registry.Register(strategy3)
	
	// Verify strategies are registered and sorted by priority
	strategies := registry.GetStrategies()
	assert.Equal(t, 3, len(strategies))
	
	// Should be sorted by priority: 8 > 5 > 3
	assert.Equal(t, "continuous_activity", strategies[0].Name())
	assert.Equal(t, 8, strategies[0].Priority())
	assert.Equal(t, "gap", strategies[1].Name())
	assert.Equal(t, 5, strategies[1].Priority())
	assert.Equal(t, "first_message", strategies[2].Name())
	assert.Equal(t, 3, strategies[2].Priority())
	
	// Test getting strategy by name
	strategy, found := registry.GetStrategyByName("gap")
	assert.True(t, found)
	assert.Equal(t, "gap", strategy.Name())
	
	_, notFound := registry.GetStrategyByName("nonexistent")
	assert.False(t, notFound)
}

// TestFirstMessageStrategy tests the first message strategy
func TestFirstMessageStrategy(t *testing.T) {
	strategy := NewFirstMessageStrategy()
	
	// Test with empty timeline
	input := DetectionInput{
		GlobalTimeline:  []timeline.TimestampedLog{},
		SessionDuration: 5 * time.Hour,
	}
	candidates := strategy.Detect(input)
	assert.Empty(t, candidates)
	
	// Test with timeline
	input.GlobalTimeline = createTestTimeline(
		time.Date(2025, 8, 7, 9, 40, 0, 0, time.UTC).Unix(),
	)
	candidates = strategy.Detect(input)
	
	assert.Equal(t, 1, len(candidates))
	assert.Equal(t, "first_message", candidates[0].Source)
	assert.Equal(t, 3, candidates[0].Priority)
	assert.False(t, candidates[0].IsLimit)
	
	// Should start at hour boundary (9:00)
	expectedStart := time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix()
	assert.Equal(t, expectedStart, candidates[0].StartTime)
	assert.Equal(t, expectedStart+18000, candidates[0].EndTime) // +5 hours
}

// TestGapDetectionStrategy tests the gap detection strategy
func TestGapDetectionStrategy(t *testing.T) {
	strategy := NewGapDetectionStrategy()
	
	// Test with continuous activity (no gaps)
	input := DetectionInput{
		GlobalTimeline: createTestTimeline(
			time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(),
			time.Date(2025, 8, 7, 10, 0, 0, 0, time.UTC).Unix(),
			time.Date(2025, 8, 7, 11, 0, 0, 0, time.UTC).Unix(),
		),
		SessionDuration: 5 * time.Hour,
	}
	candidates := strategy.Detect(input)
	assert.Empty(t, candidates, "Should not detect gaps in continuous activity")
	
	// Test with a 6-hour gap
	input.GlobalTimeline = createTestTimeline(
		time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(),
		time.Date(2025, 8, 7, 15, 30, 0, 0, time.UTC).Unix(), // 6.5 hour gap
	)
	candidates = strategy.Detect(input)
	
	assert.Equal(t, 1, len(candidates))
	assert.Equal(t, "gap", candidates[0].Source)
	assert.Equal(t, 5, candidates[0].Priority)
	
	// New window should start at hour boundary of second message (15:00)
	expectedStart := time.Date(2025, 8, 7, 15, 0, 0, 0, time.UTC).Unix()
	assert.Equal(t, expectedStart, candidates[0].StartTime)
}

// TestContinuousActivityStrategy tests the continuous activity strategy
func TestContinuousActivityStrategy(t *testing.T) {
	strategy := NewContinuousActivityStrategy()
	
	// Test with activity spanning multiple windows
	input := DetectionInput{
		GlobalTimeline: createTestTimeline(
			time.Date(2025, 8, 7, 9, 40, 0, 0, time.UTC).Unix(),  // 09:40
			time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix(),  // 14:00 (boundary)
			time.Date(2025, 8, 7, 14, 30, 0, 0, time.UTC).Unix(), // 14:30
			time.Date(2025, 8, 7, 16, 8, 0, 0, time.UTC).Unix(),  // 16:08
		),
		SessionDuration: 5 * time.Hour,
	}
	candidates := strategy.Detect(input)
	
	// Should create 2 windows: 09:00-14:00 and 14:00-19:00
	assert.Equal(t, 2, len(candidates))
	
	// First window
	assert.Equal(t, "continuous_activity", candidates[0].Source)
	assert.Equal(t, 8, candidates[0].Priority)
	assert.Equal(t, time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(), candidates[0].StartTime)
	assert.Equal(t, time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix(), candidates[0].EndTime)
	
	// Second window
	assert.Equal(t, "continuous_activity", candidates[1].Source)
	assert.Equal(t, time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix(), candidates[1].StartTime)
	assert.Equal(t, time.Date(2025, 8, 7, 19, 0, 0, 0, time.UTC).Unix(), candidates[1].EndTime)
}

// TestHistoryLimitStrategy tests the history limit strategy
func TestHistoryLimitStrategy(t *testing.T) {
	strategy := NewHistoryLimitStrategy()
	
	// Test with mock window history
	mockHistory := &mockWindowHistory{
		accountWindows: []HistoricalWindow{
			{
				SessionID:      "12345",
				Source:         "limit_message",
				StartTime:      time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(),
				EndTime:        time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix(),
				IsLimitReached: true,
				IsAccountLevel: true,
				LimitMessage:   "Rate limit reached",
			},
			{
				SessionID:      "67890",
				Source:         "gap",
				StartTime:      time.Date(2025, 8, 7, 15, 0, 0, 0, time.UTC).Unix(),
				EndTime:        time.Date(2025, 8, 7, 20, 0, 0, 0, time.UTC).Unix(),
				IsLimitReached: false,
				IsAccountLevel: true,
			},
		},
	}
	
	input := DetectionInput{
		WindowHistory:   mockHistory,
		SessionDuration: 5 * time.Hour,
	}
	candidates := strategy.Detect(input)
	
	// Should only detect the limit_message window
	assert.Equal(t, 1, len(candidates))
	assert.Equal(t, "history_limit", candidates[0].Source)
	assert.Equal(t, 10, candidates[0].Priority) // Highest priority
	assert.True(t, candidates[0].IsLimit)
	assert.Equal(t, "Rate limit reached", candidates[0].LimitMessage)
}

// TestStrategyCollectCandidates tests collecting candidates from multiple strategies
func TestStrategyCollectCandidates(t *testing.T) {
	registry := NewStrategyRegistry()
	
	// Register multiple strategies
	registry.Register(NewFirstMessageStrategy())
	registry.Register(NewGapDetectionStrategy())
	registry.Register(NewContinuousActivityStrategy())
	
	// Create test input with activity and gaps
	input := DetectionInput{
		GlobalTimeline: createTestTimeline(
			time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(),
			time.Date(2025, 8, 7, 10, 0, 0, 0, time.UTC).Unix(),
			time.Date(2025, 8, 7, 16, 0, 0, 0, time.UTC).Unix(), // 6-hour gap
		),
		SessionDuration: 5 * time.Hour,
	}
	
	candidates := registry.CollectCandidates(input)
	
	// Should collect candidates from all strategies
	// - FirstMessage: 1 candidate
	// - Gap: 1 candidate (after the gap)
	// - ContinuousActivity: 2 candidates (09:00-14:00, 14:00-19:00)
	assert.GreaterOrEqual(t, len(candidates), 3)
	
	// Verify different sources are present
	sources := make(map[string]bool)
	for _, c := range candidates {
		sources[c.Source] = true
	}
	assert.True(t, sources["first_message"])
	assert.True(t, sources["gap"])
	assert.True(t, sources["continuous_activity"])
}

// TestStrategyEnableDisable tests enabling and disabling strategies
func TestStrategyEnableDisable(t *testing.T) {
	registry := NewStrategyRegistry()
	
	// Register strategies
	registry.Register(NewFirstMessageStrategy())
	registry.Register(NewGapDetectionStrategy())
	
	// All strategies should be enabled by default
	assert.True(t, registry.IsEnabled("first_message"))
	assert.True(t, registry.IsEnabled("gap"))
	
	// Disable gap detection
	registry.DisableStrategy("gap")
	assert.False(t, registry.IsEnabled("gap"))
	
	// Create test input
	input := DetectionInput{
		GlobalTimeline: createTestTimeline(
			time.Date(2025, 8, 7, 9, 0, 0, 0, time.UTC).Unix(),
			time.Date(2025, 8, 7, 16, 0, 0, 0, time.UTC).Unix(), // 7-hour gap
		),
		SessionDuration: 5 * time.Hour,
	}
	
	candidates := registry.CollectCandidates(input)
	
	// Should only have candidates from first_message (gap is disabled)
	for _, c := range candidates {
		assert.NotEqual(t, "gap", c.Source)
	}
	
	// Re-enable gap detection
	registry.EnableStrategy("gap")
	assert.True(t, registry.IsEnabled("gap"))
	
	candidates = registry.CollectCandidates(input)
	
	// Now should have candidates from both strategies
	hasGap := false
	for _, c := range candidates {
		if c.Source == "gap" {
			hasGap = true
			break
		}
	}
	assert.True(t, hasGap)
}
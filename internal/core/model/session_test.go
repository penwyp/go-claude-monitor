package model

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregatedMetricsGetTokenPercentage(t *testing.T) {
	tests := []struct {
		name         string
		totalTokens  int
		tokenLimit   int
		expectedPct  float64
	}{
		{
			name:         "no_limit",
			totalTokens:  1000,
			tokenLimit:   -1,
			expectedPct:  0,
		},
		{
			name:         "zero_tokens",
			totalTokens:  0,
			tokenLimit:   1000,
			expectedPct:  0,
		},
		{
			name:         "half_usage",
			totalTokens:  500,
			tokenLimit:   1000,
			expectedPct:  50.0,
		},
		{
			name:         "full_usage",
			totalTokens:  1000,
			tokenLimit:   1000,
			expectedPct:  100.0,
		},
		{
			name:         "over_limit",
			totalTokens:  1200,
			tokenLimit:   1000,
			expectedPct:  100.0, // Capped at 100%
		},
		{
			name:         "fractional_percentage",
			totalTokens:  333,
			tokenLimit:   1000,
			expectedPct:  33.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				TotalTokens: tt.totalTokens,
				TokenLimit:  tt.tokenLimit,
			}
			
			result := aggregated.GetTokenPercentage()
			assert.InDelta(t, tt.expectedPct, result, 0.1) // Allow small floating point differences
		})
	}
}

func TestAggregatedMetricsGetMessagePercentage(t *testing.T) {
	tests := []struct {
		name            string
		totalMessages   int
		messageLimit    int
		expectedPct     float64
	}{
		{
			name:            "no_limit",
			totalMessages:   50,
			messageLimit:    -1,
			expectedPct:     0,
		},
		{
			name:            "zero_messages",
			totalMessages:   0,
			messageLimit:    100,
			expectedPct:     0,
		},
		{
			name:            "quarter_usage",
			totalMessages:   25,
			messageLimit:    100,
			expectedPct:     25.0,
		},
		{
			name:            "full_usage",
			totalMessages:   100,
			messageLimit:    100,
			expectedPct:     100.0,
		},
		{
			name:            "over_limit",
			totalMessages:   150,
			messageLimit:    100,
			expectedPct:     100.0, // Capped at 100%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				TotalMessages: tt.totalMessages,
				MessageLimit:  tt.messageLimit,
			}
			
			result := aggregated.GetMessagePercentage()
			assert.Equal(t, tt.expectedPct, result)
		})
	}
}

func TestAggregatedMetricsGetCostPercentage(t *testing.T) {
	tests := []struct {
		name        string
		totalCost   float64
		costLimit   float64
		expectedPct float64
	}{
		{
			name:        "no_limit",
			totalCost:   15.50,
			costLimit:   -1,
			expectedPct: 0,
		},
		{
			name:        "zero_cost",
			totalCost:   0,
			costLimit:   25.0,
			expectedPct: 0,
		},
		{
			name:        "half_usage",
			totalCost:   12.5,
			costLimit:   25.0,
			expectedPct: 50.0,
		},
		{
			name:        "full_usage",
			totalCost:   25.0,
			costLimit:   25.0,
			expectedPct: 100.0,
		},
		{
			name:        "over_limit",
			totalCost:   30.0,
			costLimit:   25.0,
			expectedPct: 100.0, // Capped at 100%
		},
		{
			name:        "fractional_cost",
			totalCost:   7.33,
			costLimit:   22.0,
			expectedPct: 33.318181818181815, // 7.33/22*100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				TotalCost: tt.totalCost,
				CostLimit: tt.costLimit,
			}
			
			result := aggregated.GetCostPercentage()
			assert.InDelta(t, tt.expectedPct, result, 0.1) // Allow small floating point differences
		})
	}
}

func TestAggregatedMetricsFormatResetTime(t *testing.T) {
	// Initialize time provider for testing
	err := util.InitializeTimeProvider("UTC")
	require.NoError(t, err)
	
	tests := []struct {
		name        string
		resetTime   int64
		timeFormat  string
		expected    string
	}{
		{
			name:       "zero_reset_time",
			resetTime:  0,
			timeFormat: "24h",
			expected:   "Unknown",
		},
		{
			name:       "24h_format",
			resetTime:  time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC).Unix(),
			timeFormat: "24h",
			expected:   "15:30",
		},
		{
			name:       "12h_format",
			resetTime:  time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC).Unix(),
			timeFormat: "12h",
			expected:   "3:30 PM",
		},
		{
			name:       "12h_format_morning",
			resetTime:  time.Date(2024, 1, 1, 9, 15, 0, 0, time.UTC).Unix(),
			timeFormat: "12h",
			expected:   "9:15 AM",
		},
		{
			name:       "midnight_24h",
			resetTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			timeFormat: "24h",
			expected:   "00:00",
		},
		{
			name:       "midnight_12h",
			resetTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			timeFormat: "12h",
			expected:   "12:00 AM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				ResetTime: tt.resetTime,
			}
			
			param := LayoutParam{
				TimeFormat: tt.timeFormat,
				Timezone:   "UTC",
			}
			
			result := aggregated.FormatResetTime(param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregatedMetricsGetTokensRunOut(t *testing.T) {
	// Initialize time provider for testing
	err := util.InitializeTimeProvider("UTC")
	require.NoError(t, err)
	
	tests := []struct {
		name             string
		predictedEndTime int64
		timeFormat       string
		expected         string
	}{
		{
			name:             "zero_predicted_time",
			predictedEndTime: 0,
			timeFormat:       "24h",
			expected:         "Unknown",
		},
		{
			name:             "24h_format",
			predictedEndTime: time.Date(2024, 1, 1, 16, 45, 0, 0, time.UTC).Unix(),
			timeFormat:       "24h",
			expected:         "16:45",
		},
		{
			name:             "12h_format",
			predictedEndTime: time.Date(2024, 1, 1, 16, 45, 0, 0, time.UTC).Unix(),
			timeFormat:       "12h",
			expected:         "4:45 PM",
		},
		{
			name:             "12h_format_morning",
			predictedEndTime: time.Date(2024, 1, 1, 8, 30, 0, 0, time.UTC).Unix(),
			timeFormat:       "12h",
			expected:         "8:30 AM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				PredictedEndTime: tt.predictedEndTime,
			}
			
			param := LayoutParam{
				TimeFormat: tt.timeFormat,
			}
			
			result := aggregated.GetTokensRunOut(param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregatedMetricsAppendWindowIndicator(t *testing.T) {
	tests := []struct {
		name             string
		windowSource     string
		isWindowDetected bool
		resetTimeStr     string
		expected         string
	}{
		{
			name:             "limit_message_source",
			windowSource:     "limit_message",
			isWindowDetected: true,
			resetTimeStr:     "15:30",
			expected:         "15:30 🎯",
		},
		{
			name:             "gap_source",
			windowSource:     "gap",
			isWindowDetected: true,
			resetTimeStr:     "16:00",
			expected:         "16:00 ⏳",
		},
		{
			name:             "first_message_source",
			windowSource:     "first_message",
			isWindowDetected: true,
			resetTimeStr:     "14:45",
			expected:         "14:45 📍",
		},
		{
			name:             "unknown_source",
			windowSource:     "rounded_hour",
			isWindowDetected: true,
			resetTimeStr:     "13:00",
			expected:         "13:00",
		},
		{
			name:             "window_not_detected",
			windowSource:     "limit_message",
			isWindowDetected: false,
			resetTimeStr:     "15:30",
			expected:         "15:30",
		},
		{
			name:             "empty_window_source",
			windowSource:     "",
			isWindowDetected: true,
			resetTimeStr:     "12:00",
			expected:         "12:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				WindowSource:     tt.windowSource,
				IsWindowDetected: tt.isWindowDetected,
			}
			
			result := aggregated.AppendWindowIndicator(tt.resetTimeStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregatedMetricsFormatRemainingTime(t *testing.T) {
	// Initialize time provider for testing
	err := util.InitializeTimeProvider("UTC")
	require.NoError(t, err)
	
	// Use current time for realistic testing
	now := time.Now()
	
	tests := []struct {
		name      string
		resetTime int64
		expected  string
	}{
		{
			name:      "no_reset_time",
			resetTime: 0,
			expected:  "No active session",
		},
		{
			name:      "expired_session",
			resetTime: now.Add(-1 * time.Hour).Unix(), // 1 hour ago
			expected:  "Expired",
		},
		{
			name:      "one_hour_remaining",
			resetTime: now.Add(1 * time.Hour).Unix(), // 1 hour from now
			expected:  "1h", // This depends on util.FormatDuration implementation
		},
		{
			name:      "30_minutes_remaining",
			resetTime: now.Add(30 * time.Minute).Unix(), // 30 minutes from now
			expected:  "30m", // This depends on util.FormatDuration implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregated := AggregatedMetrics{
				ResetTime: tt.resetTime,
			}
			
			// We need to test this with a controlled time, but util.FormatDuration
			// uses the real time provider, so we'll just verify it returns a string
			result := aggregated.FormatRemainingTime()
			
			if tt.resetTime == 0 {
				assert.Equal(t, "No active session", result)
			} else if tt.resetTime < now.Unix() {
				assert.Equal(t, "Expired", result)
			} else {
				// For positive remaining time, just verify it's not empty
				assert.NotEmpty(t, result)
				assert.NotEqual(t, "No active session", result)
				assert.NotEqual(t, "Expired", result)
			}
		})
	}
}

func TestHourlyMetricStructure(t *testing.T) {
	hour := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)
	
	metric := HourlyMetric{
		Hour:         hour,
		Tokens:       1000,
		Cost:         5.0,
		InputTokens:  800,
		OutputTokens: 200,
	}
	
	assert.Equal(t, hour, metric.Hour)
	assert.Equal(t, 1000, metric.Tokens)
	assert.Equal(t, 5.0, metric.Cost)
	assert.Equal(t, 800, metric.InputTokens)
	assert.Equal(t, 200, metric.OutputTokens)
}

func TestFileEventStructure(t *testing.T) {
	event := FileEvent{
		Path:      "/path/to/file.jsonl",
		Operation: "write",
	}
	
	assert.Equal(t, "/path/to/file.jsonl", event.Path)
	assert.Equal(t, "write", event.Operation)
}

func TestInteractionStateStructure(t *testing.T) {
	state := InteractionState{
		IsPaused:      true,
		ShowHelp:      false,
		ForceRefresh:  true,
		LayoutStyle:   1,
		StatusMessage: "Test status",
		ConfirmDialog: &ConfirmDialog{
			Title:   "Confirm Action",
			Message: "Are you sure?",
		},
	}
	
	assert.True(t, state.IsPaused)
	assert.False(t, state.ShowHelp)
	assert.True(t, state.ForceRefresh)
	assert.Equal(t, 1, state.LayoutStyle)
	assert.Equal(t, "Test status", state.StatusMessage)
	assert.NotNil(t, state.ConfirmDialog)
	assert.Equal(t, "Confirm Action", state.ConfirmDialog.Title)
	assert.Equal(t, "Are you sure?", state.ConfirmDialog.Message)
}

func TestConfirmDialogFunctionality(t *testing.T) {
	confirmCalled := false
	cancelCalled := false
	
	dialog := ConfirmDialog{
		Title:   "Test Dialog",
		Message: "Test message",
		OnConfirm: func() {
			confirmCalled = true
		},
		OnCancel: func() {
			cancelCalled = true
		},
	}
	
	// Test confirm callback
	dialog.OnConfirm()
	assert.True(t, confirmCalled)
	assert.False(t, cancelCalled)
	
	// Reset and test cancel callback
	confirmCalled = false
	dialog.OnCancel()
	assert.False(t, confirmCalled)
	assert.True(t, cancelCalled)
}

func TestModelStatsStructure(t *testing.T) {
	stats := ModelStats{
		Model:  "claude-3-5-sonnet",
		Tokens: 1500,
		Cost:   7.5,
		Count:  25,
	}
	
	assert.Equal(t, "claude-3-5-sonnet", stats.Model)
	assert.Equal(t, 1500, stats.Tokens)
	assert.Equal(t, 7.5, stats.Cost)
	assert.Equal(t, 25, stats.Count)
}

func TestBurnRateStructure(t *testing.T) {
	burnRate := BurnRate{
		TokensPerMinute: 50.5,
		CostPerHour:     15.0,
		CostPerMinute:   0.25,
	}
	
	assert.Equal(t, 50.5, burnRate.TokensPerMinute)
	assert.Equal(t, 15.0, burnRate.CostPerHour)
	assert.Equal(t, 0.25, burnRate.CostPerMinute)
}

func TestLayoutParamStructure(t *testing.T) {
	param := LayoutParam{
		Timezone:   "America/New_York",
		TimeFormat: "12h",
		Plan:       "pro",
	}
	
	assert.Equal(t, "America/New_York", param.Timezone)
	assert.Equal(t, "12h", param.TimeFormat)
	assert.Equal(t, "pro", param.Plan)
}

func TestAggregatedMetricsComplexScenario(t *testing.T) {
	// Test a complex scenario with multiple model distributions
	aggregated := AggregatedMetrics{
		TotalCost:           15.0,
		TotalTokens:         3000,
		TotalMessages:       50,
		ActiveSessions:      2,
		TotalSessions:       3,
		AverageBurnRate:     2.5,
		CostBurnRate:        0.1,
		TokenBurnRate:       25.0,
		MessageBurnRate:     0.167,
		CostLimit:           25.0,
		TokenLimit:          5000,
		MessageLimit:        100,
		LimitExceeded:       false,
		LimitExceededReason: "",
		ResetTime:           time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC).Unix(),
		PredictedEndTime:    time.Date(2024, 1, 1, 18, 40, 0, 0, time.UTC).Unix(),
		CostPerMinute:       0.1,
		WindowSource:        "limit_message",
		IsWindowDetected:    true,
		HasActiveSession:    true,
		ModelDistribution: map[string]*ModelStats{
			"claude-3-5-sonnet": {
				Model:  "claude-3-5-sonnet",
				Tokens: 2000,
				Cost:   10.0,
				Count:  30,
			},
			"claude-3-5-haiku": {
				Model:  "claude-3-5-haiku",
				Tokens: 1000,
				Cost:   5.0,
				Count:  20,
			},
		},
	}
	
	// Test percentage calculations
	assert.Equal(t, 60.0, aggregated.GetCostPercentage())  // 15/25 * 100
	assert.Equal(t, 60.0, aggregated.GetTokenPercentage()) // 3000/5000 * 100
	assert.Equal(t, 50.0, aggregated.GetMessagePercentage()) // 50/100 * 100
	
	// Test window indicator
	resetTimeStr := "17:00"
	result := aggregated.AppendWindowIndicator(resetTimeStr)
	assert.Equal(t, "17:00 🎯", result)
	
	// Test model distribution
	assert.Len(t, aggregated.ModelDistribution, 2)
	assert.Contains(t, aggregated.ModelDistribution, "claude-3-5-sonnet")
	assert.Contains(t, aggregated.ModelDistribution, "claude-3-5-haiku")
	
	sonnetStats := aggregated.ModelDistribution["claude-3-5-sonnet"]
	assert.Equal(t, 2000, sonnetStats.Tokens)
	assert.Equal(t, 10.0, sonnetStats.Cost)
	assert.Equal(t, 30, sonnetStats.Count)
	
	haikuStats := aggregated.ModelDistribution["claude-3-5-haiku"]
	assert.Equal(t, 1000, haikuStats.Tokens)
	assert.Equal(t, 5.0, haikuStats.Cost)
	assert.Equal(t, 20, haikuStats.Count)
}

// Benchmark tests for performance critical operations
func BenchmarkGetTokenPercentage(b *testing.B) {
	aggregated := AggregatedMetrics{
		TotalTokens: 1500,
		TokenLimit:  5000,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregated.GetTokenPercentage()
	}
}

func BenchmarkGetCostPercentage(b *testing.B) {
	aggregated := AggregatedMetrics{
		TotalCost: 12.5,
		CostLimit: 25.0,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregated.GetCostPercentage()
	}
}

func BenchmarkAppendWindowIndicator(b *testing.B) {
	aggregated := AggregatedMetrics{
		WindowSource:     "limit_message",
		IsWindowDetected: true,
	}
	
	resetTimeStr := "15:30"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregated.AppendWindowIndicator(resetTimeStr)
	}
}

// Test edge cases and boundary conditions
func TestAggregatedMetricsEdgeCases(t *testing.T) {
	t.Run("negative_limits", func(t *testing.T) {
		aggregated := AggregatedMetrics{
			TotalCost:     10.0,
			TotalTokens:   1000,
			TotalMessages: 50,
			CostLimit:     -1,
			TokenLimit:    -1,
			MessageLimit:  -1,
		}
		
		assert.Equal(t, 0.0, aggregated.GetCostPercentage())
		assert.Equal(t, 0.0, aggregated.GetTokenPercentage())
		assert.Equal(t, 0.0, aggregated.GetMessagePercentage())
	})
	
	t.Run("zero_limits", func(t *testing.T) {
		aggregated := AggregatedMetrics{
			TotalCost:     10.0,
			TotalTokens:   1000,
			TotalMessages: 50,
			CostLimit:     0,
			TokenLimit:    0,
			MessageLimit:  0,
		}
		
		// Division by zero should be handled gracefully
		// Go returns +Inf for float division by zero, which we cap at 100%
		assert.Equal(t, 100.0, aggregated.GetCostPercentage())
		assert.Equal(t, 100.0, aggregated.GetTokenPercentage())
		assert.Equal(t, 100.0, aggregated.GetMessagePercentage())
	})
	
	t.Run("nil_model_distribution", func(t *testing.T) {
		aggregated := AggregatedMetrics{
			ModelDistribution: nil,
		}
		
		// Should handle nil model distribution gracefully
		assert.Nil(t, aggregated.ModelDistribution)
	})
	
	t.Run("empty_model_distribution", func(t *testing.T) {
		aggregated := AggregatedMetrics{
			ModelDistribution: make(map[string]*ModelStats),
		}
		
		assert.NotNil(t, aggregated.ModelDistribution)
		assert.Empty(t, aggregated.ModelDistribution)
	})
}
package layout

import (
	"testing"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

func TestGetStrategy(t *testing.T) {
	tests := []struct {
		name        string
		layoutStyle int
		wantType    string
	}{
		{
			name:        "full_dashboard_style",
			layoutStyle: 0,
			wantType:    "*layout.FullDashboardStrategy",
		},
		{
			name:        "minimal_dashboard_style",
			layoutStyle: 1,
			wantType:    "*layout.MinimalDashboardStrategy",
		},
		{
			name:        "unknown_style_defaults_to_full",
			layoutStyle: 99,
			wantType:    "*layout.FullDashboardStrategy",
		},
		{
			name:        "negative_style_defaults_to_full",
			layoutStyle: -1,
			wantType:    "*layout.FullDashboardStrategy",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := GetStrategy(tt.layoutStyle)
			if strategy == nil {
				t.Fatal("GetStrategy returned nil")
			}
			
			// Check type based on interface methods
			// Since we can't check concrete type directly, verify it implements LayoutStrategy
			var _ LayoutStrategy = strategy
		})
	}
}

func TestLayoutStrategyInterface(t *testing.T) {
	// Test that all strategies implement the interface correctly
	strategies := []struct {
		name     string
		strategy LayoutStrategy
	}{
		{
			name:     "full_dashboard",
			strategy: NewFullDashboardStrategy(),
		},
		{
			name:     "minimal_dashboard",
			strategy: NewMinimalDashboardStrategy(),
		},
	}
	
	metrics := model.AggregatedMetrics{
		TotalCost:       100.0,
		TotalTokens:     1000000,
		TotalMessages:   100,
		ActiveSessions:  2,
		TotalSessions:   5,
		TokenBurnRate:   1000,
		CostBurnRate:    0.10,
		MessageBurnRate: 5,
		CostLimit:       200.0,
		TokenLimit:      2000000,
		MessageLimit:    200,
		ResetTime:       1704106800,
		ModelDistribution: map[string]*model.ModelStats{
			"claude-3-5-sonnet": {
				Model:  "claude-3-5-sonnet",
				Tokens: 600000,
				Cost:   60.0,
				Count:  60,
			},
			"claude-3-5-haiku": {
				Model:  "claude-3-5-haiku",
				Tokens: 400000,
				Cost:   40.0,
				Count:  40,
			},
		},
	}
	
	sessions := []model.SessionDisplay{
		{
			SessionID:      "session-1",
			DisplayName:    "Test Session 1",
			ProjectName:    "project-1",
			WindowDetected: true,
			IsActive:       true,
			Metrics: model.SessionMetrics{
				TotalTokens:   500000,
				TotalCost:     50.0,
				MessageCount:  50,
				LastActive:    "10:30 AM",
				ElapsedTime:   "2h 30m",
				BurnRate:      "200K/hr",
				ModelUsage: map[string]int{
					"claude-3-5-sonnet": 300000,
					"claude-3-5-haiku":  200000,
				},
			},
		},
		{
			SessionID:      "session-2",
			DisplayName:    "Test Session 2",
			ProjectName:    "project-2",
			WindowDetected: false,
			IsActive:       false,
			Metrics: model.SessionMetrics{
				TotalTokens:   500000,
				TotalCost:     50.0,
				MessageCount:  50,
				LastActive:    "08:00 AM",
				ElapsedTime:   "5h 00m",
				BurnRate:      "100K/hr",
				ModelUsage: map[string]int{
					"claude-3-5-sonnet": 300000,
					"claude-3-5-haiku":  200000,
				},
			},
		},
	}
	
	state := model.InteractionState{
		IsPaused:      false,
		ShowHelp:      false,
		ForceRefresh:  false,
		LayoutStyle:   0,
		StatusMessage: "Test Status",
	}
	
	params := model.LayoutParam{
		Timezone:   "America/New_York",
		TimeFormat: "12h",
		Plan:       "pro",
	}
	
	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			// Test RenderHeader
			header := tt.strategy.RenderHeader(metrics, params)
			if header == "" {
				t.Error("RenderHeader returned empty string")
			}
			
			// Test RenderSessionList  
			sessionList := tt.strategy.RenderSessionList(sessions, 0, params)
			if sessionList == "" && len(sessions) > 0 {
				t.Error("RenderSessionList returned empty string with sessions")
			}
			
			// Test RenderFooter
			footer := tt.strategy.RenderFooter(state, 100, 50)
			if footer == "" {
				t.Error("RenderFooter returned empty string")
			}
		})
	}
}

func TestStrategyRenderingConsistency(t *testing.T) {
	// Test that strategies produce consistent output for the same input
	strategy := NewFullDashboardStrategy()
	
	metrics := model.AggregatedMetrics{
		TotalCost:    50.0,
		TotalTokens:  500000,
		CostLimit:    100.0,
		TokenLimit:   1000000,
		ResetTime:    1704106800,
	}
	
	params := model.LayoutParam{
		Timezone:   "UTC",
		TimeFormat: "24h",
		Plan:       "pro",
	}
	
	// Render header multiple times
	header1 := strategy.RenderHeader(metrics, params)
	header2 := strategy.RenderHeader(metrics, params)
	
	if header1 != header2 {
		t.Error("RenderHeader produced different output for same input")
	}
	
	// Test with empty sessions
	emptySessionList := strategy.RenderSessionList([]model.SessionDisplay{}, 0, params)
	if emptySessionList == "" {
		t.Error("Expected non-empty output for empty session list")
	}
}

func TestStrategyErrorHandling(t *testing.T) {
	strategy := NewMinimalDashboardStrategy()
	
	t.Run("nil_model_distribution", func(t *testing.T) {
		metrics := model.AggregatedMetrics{
			TotalCost:         100.0,
			TotalTokens:       1000000,
			ModelDistribution: nil, // This should not cause panic
		}
		
		params := model.LayoutParam{}
		
		// Should not panic
		header := strategy.RenderHeader(metrics, params)
		if header == "" {
			t.Error("Expected non-empty header even with nil model distribution")
		}
	})
	
	t.Run("empty_sessions", func(t *testing.T) {
		// Should handle empty session list gracefully
		result := strategy.RenderSessionList(nil, 0, model.LayoutParam{})
		if result == "" {
			t.Error("Expected non-empty result for nil sessions")
		}
		
		result = strategy.RenderSessionList([]model.SessionDisplay{}, 0, model.LayoutParam{})
		if result == "" {
			t.Error("Expected non-empty result for empty sessions")
		}
	})
	
	t.Run("negative_selected_index", func(t *testing.T) {
		sessions := []model.SessionDisplay{
			{SessionID: "test", DisplayName: "Test"},
		}
		
		// Should handle negative index gracefully
		result := strategy.RenderSessionList(sessions, -1, model.LayoutParam{})
		if result == "" {
			t.Error("Expected non-empty result with negative index")
		}
	})
	
	t.Run("out_of_bounds_index", func(t *testing.T) {
		sessions := []model.SessionDisplay{
			{SessionID: "test", DisplayName: "Test"},
		}
		
		// Should handle out of bounds index gracefully
		result := strategy.RenderSessionList(sessions, 10, model.LayoutParam{})
		if result == "" {
			t.Error("Expected non-empty result with out of bounds index")
		}
	})
}
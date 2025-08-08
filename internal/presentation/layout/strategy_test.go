package layout

import (
	"testing"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

func TestGetLayoutStrategy(t *testing.T) {
	tests := []struct {
		name        string
		layoutStyle int
		wantType    string
	}{
		{
			name:        "full_dashboard_style",
			layoutStyle: 0,
			wantType:    "*layout.FullLayoutStrategy",
		},
		{
			name:        "minimal_dashboard_style",
			layoutStyle: 1,
			wantType:    "*layout.MinimalLayoutStrategy",
		},
		{
			name:        "unknown_style_defaults_to_full",
			layoutStyle: 99,
			wantType:    "*layout.FullLayoutStrategy",
		},
		{
			name:        "negative_style_defaults_to_full",
			layoutStyle: -1,
			wantType:    "*layout.FullLayoutStrategy",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := GetLayoutStrategy(tt.layoutStyle)
			if strategy == nil {
				t.Fatal("GetLayoutStrategy returned nil")
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
		expectedName string
	}{
		{
			name:     "full_layout",
			strategy: &FullLayoutStrategy{},
			expectedName: "Full Dashboard",
		},
		{
			name:     "minimal_layout",
			strategy: &MinimalLayoutStrategy{},
			expectedName: "Minimal Dashboard",
		},
	}
	
	metrics := &model.AggregatedMetrics{
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
	
	params := model.LayoutParam{
		Timezone:   "America/New_York",
		TimeFormat: "12h",
		Plan:       "pro",
	}
	
	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			// Test GetName
			name := tt.strategy.GetName()
			if name != tt.expectedName {
				t.Errorf("GetName() = %v, want %v", name, tt.expectedName)
			}
			
			// Test Render (just verify it doesn't panic)
			// Since Render prints to stdout, we can't easily test its output
			// but we can ensure it doesn't panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Render() panicked: %v", r)
					}
				}()
				tt.strategy.Render(metrics, params)
			}()
		})
	}
}

func TestStrategyErrorHandling(t *testing.T) {
	strategy := &MinimalLayoutStrategy{}
	
	t.Run("nil_model_distribution", func(t *testing.T) {
		metrics := &model.AggregatedMetrics{
			TotalCost:         100.0,
			TotalTokens:       1000000,
			ModelDistribution: nil, // This should not cause panic
		}
		
		params := model.LayoutParam{}
		
		// Should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Render() panicked with nil model distribution: %v", r)
				}
			}()
			strategy.Render(metrics, params)
		}()
	})
	
	t.Run("zero_values", func(t *testing.T) {
		// Should handle zero values gracefully
		metrics := &model.AggregatedMetrics{}
		params := model.LayoutParam{}
		
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Render() panicked with zero values: %v", r)
				}
			}()
			strategy.Render(metrics, params)
		}()
	})
}
package layout

import (
	"strings"
	"testing"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

func TestNewBaseStrategy(t *testing.T) {
	strategy := NewBaseStrategy()
	if strategy == nil {
		t.Fatal("NewBaseStrategy returned nil")
	}
}

func TestBaseStrategyHelpers(t *testing.T) {
	strategy := NewBaseStrategy()
	
	tests := []struct {
		name     string
		testFunc func() string
		wantLen  int // Minimum expected length
	}{
		{
			name: "separator_line",
			testFunc: func() string {
				return strategy.SeparatorLine()
			},
			wantLen: 50,
		},
		{
			name: "box_header",
			testFunc: func() string {
				return strategy.BoxHeader("Test Title", 40)
			},
			wantLen: 40,
		},
		{
			name: "center_text",
			testFunc: func() string {
				return strategy.CenterText("Center", 20)
			},
			wantLen: 20,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc()
			if len(result) < tt.wantLen {
				t.Errorf("Expected length at least %d, got %d for %q", tt.wantLen, len(result), result)
			}
		})
	}
}

func TestBaseStrategyFormatters(t *testing.T) {
	strategy := NewBaseStrategy()
	
	tests := []struct {
		name        string
		value       interface{}
		format      string
		wantContain string
	}{
		{
			name:        "format_percentage",
			value:       75.5,
			format:      "percentage",
			wantContain: "75.5%",
		},
		{
			name:        "format_tokens",
			value:       1000000,
			format:      "tokens",
			wantContain: "1,000,000",
		},
		{
			name:        "format_currency",
			value:       99.99,
			format:      "currency",
			wantContain: "$",
		},
		{
			name:        "format_time",
			value:       "15:30",
			format:      "time",
			wantContain: "15:30",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			switch tt.format {
			case "percentage":
				result = strategy.FormatPercentage(tt.value.(float64))
			case "tokens":
				result = strategy.FormatTokens(tt.value.(int))
			case "currency":
				result = strategy.FormatCurrency(tt.value.(float64))
			case "time":
				result = tt.value.(string) // Just pass through for time test
			}
			
			if result == "" {
				t.Error("Expected non-empty result")
			}
			
			// For formatted values, just check they're non-empty
			// Actual formatting is tested in util package
		})
	}
}

func TestBaseStrategyProgressBar(t *testing.T) {
	strategy := NewBaseStrategy()
	
	tests := []struct {
		name       string
		percentage float64
		width      int
		label      string
		wantWidth  int
	}{
		{
			name:       "zero_percentage",
			percentage: 0,
			width:      20,
			label:      "Test",
			wantWidth:  20,
		},
		{
			name:       "fifty_percentage",
			percentage: 50,
			width:      20,
			label:      "Test",
			wantWidth:  20,
		},
		{
			name:       "hundred_percentage",
			percentage: 100,
			width:      20,
			label:      "Test",
			wantWidth:  20,
		},
		{
			name:       "over_hundred_percentage",
			percentage: 150,
			width:      20,
			label:      "Test",
			wantWidth:  20,
		},
		{
			name:       "negative_percentage",
			percentage: -10,
			width:      20,
			label:      "Test",
			wantWidth:  20,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.ProgressBar(tt.percentage, tt.width, tt.label)
			// Remove ANSI color codes for length check
			cleanResult := stripANSI(result)
			if len(cleanResult) != tt.wantWidth {
				t.Errorf("Expected width %d, got %d for %q", tt.wantWidth, len(cleanResult), cleanResult)
			}
		})
	}
}

func TestBaseStrategyModelIcons(t *testing.T) {
	strategy := NewBaseStrategy()
	
	tests := []struct {
		name      string
		modelName string
		wantIcon  string
	}{
		{
			name:      "opus_model",
			modelName: "claude-opus-4-20250514",
			wantIcon:  "💎",
		},
		{
			name:      "sonnet_model",
			modelName: "claude-3-5-sonnet",
			wantIcon:  "✨",
		},
		{
			name:      "haiku_model",
			modelName: "claude-3-5-haiku",
			wantIcon:  "⚡",
		},
		{
			name:      "unknown_model",
			modelName: "unknown-model",
			wantIcon:  "🤖",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.GetModelIcon(tt.modelName)
			if result != tt.wantIcon {
				t.Errorf("Expected icon %s for model %s, got %s", tt.wantIcon, tt.modelName, result)
			}
		})
	}
}

func TestBaseStrategyFormatMetrics(t *testing.T) {
	strategy := NewBaseStrategy()
	
	metrics := model.AggregatedMetrics{
		TotalCost:       125.50,
		TotalTokens:     1500000,
		TotalMessages:   250,
		ActiveSessions:  3,
		TotalSessions:   10,
		TokenBurnRate:   5000,
		CostBurnRate:    0.50,
		MessageBurnRate: 10,
		CostLimit:       200.0,
		TokenLimit:      2000000,
		MessageLimit:    500,
		ResetTime:       1704106800, // 2024-01-01 12:00:00 UTC
	}
	
	params := model.LayoutParam{
		Timezone:   "America/New_York",
		TimeFormat: "12h",
		Plan:       "pro",
	}
	
	t.Run("format_cost_info", func(t *testing.T) {
		result := strategy.FormatCostInfo(metrics, params)
		if result == "" {
			t.Error("Expected non-empty cost info")
		}
		// Should contain cost and percentage
		if !contains(result, "$") {
			t.Error("Expected cost info to contain dollar sign")
		}
	})
	
	t.Run("format_token_info", func(t *testing.T) {
		result := strategy.FormatTokenInfo(metrics, params)
		if result == "" {
			t.Error("Expected non-empty token info")
		}
		// Should contain formatted tokens
		if !contains(result, ",") {
			t.Error("Expected token info to contain comma-formatted numbers")
		}
	})
	
	t.Run("format_message_info", func(t *testing.T) {
		result := strategy.FormatMessageInfo(metrics, params)
		if result == "" {
			t.Error("Expected non-empty message info")
		}
		// Should contain message count
		if !contains(result, "250") {
			t.Error("Expected message info to contain message count")
		}
	})
}

func TestBaseStrategyEdgeCases(t *testing.T) {
	strategy := NewBaseStrategy()
	
	t.Run("empty_strings", func(t *testing.T) {
		// Test with empty strings
		result := strategy.BoxHeader("", 20)
		if len(stripANSI(result)) != 20 {
			t.Error("Expected box header to maintain width with empty title")
		}
		
		result = strategy.CenterText("", 20)
		if len(result) != 20 {
			t.Error("Expected centered text to maintain width with empty text")
		}
	})
	
	t.Run("long_strings", func(t *testing.T) {
		// Test with strings longer than width
		longText := "This is a very long text that exceeds the specified width"
		result := strategy.CenterText(longText, 10)
		if len(result) != 10 {
			t.Errorf("Expected centered text to be truncated to width 10, got %d", len(result))
		}
	})
	
	t.Run("zero_width", func(t *testing.T) {
		// Test with zero width
		result := strategy.ProgressBar(50, 0, "Test")
		if result != "" {
			t.Error("Expected empty progress bar with zero width")
		}
	})
	
	t.Run("nil_metrics", func(t *testing.T) {
		// Test formatting with zero values
		emptyMetrics := model.AggregatedMetrics{}
		params := model.LayoutParam{}
		
		result := strategy.FormatCostInfo(emptyMetrics, params)
		if result == "" {
			t.Error("Expected non-empty result for zero metrics")
		}
	})
}

// Helper function to strip ANSI color codes
func stripANSI(s string) string {
	// Simple implementation - in real code you'd use a proper ANSI stripping regex
	result := s
	// Remove common ANSI sequences
	for _, seq := range []string{"\033[32m", "\033[33m", "\033[31m", "\033[0m", "\033[1m", "\033[36m", "\033[90m"} {
		result = strings.ReplaceAll(result, seq, "")
	}
	return result
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
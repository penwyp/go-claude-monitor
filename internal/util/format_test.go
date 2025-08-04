package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "small number",
			input:    42,
			expected: "42",
		},
		{
			name:     "hundreds",
			input:    999,
			expected: "999",
		},
		{
			name:     "exactly 1000",
			input:    1000,
			expected: "1.0K",
		},
		{
			name:     "thousands",
			input:    1500,
			expected: "1.5K",
		},
		{
			name:     "tens of thousands",
			input:    25000,
			expected: "25.0K",
		},
		{
			name:     "hundreds of thousands",
			input:    999999,
			expected: "1000.0K",
		},
		{
			name:     "exactly 1 million",
			input:    1000000,
			expected: "1.0M",
		},
		{
			name:     "millions",
			input:    2500000,
			expected: "2.5M",
		},
		{
			name:     "tens of millions",
			input:    50000000,
			expected: "50.0M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			input:    0 * time.Minute,
			expected: "0m",
		},
		{
			name:     "minutes only",
			input:    5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "30 minutes",
			input:    30 * time.Minute,
			expected: "30m",
		},
		{
			name:     "59 minutes",
			input:    59 * time.Minute,
			expected: "59m",
		},
		{
			name:     "exactly 1 hour",
			input:    60 * time.Minute,
			expected: "1h 0m",
		},
		{
			name:     "1 hour 30 minutes",
			input:    90 * time.Minute,
			expected: "1h 30m",
		},
		{
			name:     "2 hours 15 minutes",
			input:    135 * time.Minute,
			expected: "2h 15m",
		},
		{
			name:     "24 hours",
			input:    24 * time.Hour,
			expected: "24h 0m",
		},
		{
			name:     "25 hours 45 minutes",
			input:    25*time.Hour + 45*time.Minute,
			expected: "25h 45m",
		},
		{
			name:     "seconds get rounded down",
			input:    1*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "1h 30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBurnRate(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "zero",
			input:    0.0,
			expected: "0.0 tokens/min",
		},
		{
			name:     "small rate",
			input:    42.5,
			expected: "42.5 tokens/min",
		},
		{
			name:     "hundreds",
			input:    999.9,
			expected: "999.9 tokens/min",
		},
		{
			name:     "exactly 1000",
			input:    1000.0,
			expected: "1.0K tokens/min",
		},
		{
			name:     "thousands",
			input:    1500.0,
			expected: "1.5K tokens/min",
		},
		{
			name:     "tens of thousands",
			input:    25000.0,
			expected: "25.0K tokens/min",
		},
		{
			name:     "hundreds of thousands",
			input:    999999.0,
			expected: "1000.0K tokens/min",
		},
		{
			name:     "exactly 1 million",
			input:    1000000.0,
			expected: "1.0M tokens/min",
		},
		{
			name:     "millions",
			input:    2500000.0,
			expected: "2.5M tokens/min",
		},
		{
			name:     "decimal values",
			input:    1234.567,
			expected: "1.2K tokens/min",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBurnRate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "zero",
			input:    0.0,
			expected: "$0.00",
		},
		{
			name:     "cents only",
			input:    0.99,
			expected: "$0.99",
		},
		{
			name:     "one dollar",
			input:    1.00,
			expected: "$1.00",
		},
		{
			name:     "dollars and cents",
			input:    42.50,
			expected: "$42.50",
		},
		{
			name:     "hundreds",
			input:    999.99,
			expected: "$999.99",
		},
		{
			name:     "exactly 1000",
			input:    1000.00,
			expected: "$1,000.00",
		},
		{
			name:     "thousands with cents",
			input:    1234.56,
			expected: "$1,234.56",
		},
		{
			name:     "tens of thousands",
			input:    25000.00,
			expected: "$25,000.00",
		},
		{
			name:     "hundreds of thousands",
			input:    999999.99,
			expected: "$999,999.99",
		},
		{
			name:     "exactly 1 million",
			input:    1000000.00,
			expected: "$1,000,000.00",
		},
		{
			name:     "millions with cents",
			input:    2500000.50,
			expected: "$2,500,000.50",
		},
		{
			name:     "rounding to 2 decimal places",
			input:    123.456,
			expected: "$123.46",
		},
		{
			name:     "negative number",
			input:    -1234.56,
			expected: "$-1,234.56",
		},
		{
			name:     "very small positive",
			input:    0.01,
			expected: "$0.01",
		},
		{
			name:     "round up cents",
			input:    9.995,
			expected: "$9.99",
		},
		{
			name:     "large number with commas",
			input:    123456789.12,
			expected: "$123,456,789.12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCurrency(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrencyEdgeCases(t *testing.T) {
	// Test edge cases for currency formatting
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "very small decimal",
			input:    0.001,
			expected: "$0.00",
		},
		{
			name:     "rounding 0.005",
			input:    0.005,
			expected: "$0.01",
		},
		{
			name:     "rounding 0.004",
			input:    0.004,
			expected: "$0.00",
		},
		{
			name:     "negative zero",
			input:    -0.0,
			expected: "$0.00",
		},
		{
			name:     "negative rounding",
			input:    -0.004,
			expected: "$-0.00",
		},
		{
			name:     "billion",
			input:    1000000000.00,
			expected: "$1,000,000,000.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCurrency(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatNumberBoundaries(t *testing.T) {
	// Test boundary conditions
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "999 (just below K threshold)",
			input:    999,
			expected: "999",
		},
		{
			name:     "1000 (at K threshold)",
			input:    1000,
			expected: "1.0K",
		},
		{
			name:     "999999 (just below M threshold)",
			input:    999999,
			expected: "1000.0K",
		},
		{
			name:     "1000000 (at M threshold)",
			input:    1000000,
			expected: "1.0M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDurationEdgeCases(t *testing.T) {
	// Test edge cases for duration formatting
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "negative duration",
			input:    -30 * time.Minute,
			expected: "-30m",
		},
		{
			name:     "very long duration",
			input:    100 * time.Hour,
			expected: "100h 0m",
		},
		{
			name:     "nanoseconds only",
			input:    500 * time.Nanosecond,
			expected: "0m",
		},
		{
			name:     "59 seconds",
			input:    59 * time.Second,
			expected: "0m",
		},
		{
			name:     "60 seconds",
			input:    60 * time.Second,
			expected: "1m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
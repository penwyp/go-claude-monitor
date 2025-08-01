package analyzer

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	loc := time.UTC
	now := time.Now().In(loc)

	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		// Hour tests
		{
			name:     "single hour",
			input:    "1h",
			expected: 1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "multiple hours",
			input:    "12h",
			expected: 12 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "24 hours",
			input:    "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		// Day tests
		{
			name:     "single day",
			input:    "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "multiple days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		// Week tests
		{
			name:     "single week",
			input:    "1w",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "multiple weeks",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
			wantErr:  false,
		},
		// Month tests
		{
			name:     "single month",
			input:    "1m",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "multiple months",
			input:    "3m",
			expected: 3 * 30 * 24 * time.Hour,
			wantErr:  false,
		},
		// Year tests
		{
			name:     "single year",
			input:    "1y",
			expected: 365 * 24 * time.Hour,
			wantErr:  false,
		},
		// Combined tests
		{
			name:     "days and hours",
			input:    "1d12h",
			expected: 36 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "weeks and days",
			input:    "2w3d",
			expected: (2*7 + 3) * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "months, weeks, and days",
			input:    "1m2w3d",
			expected: (30 + 2*7 + 3) * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex combination",
			input:    "1y2m3w4d5h",
			expected: 365*24*time.Hour + 2*30*24*time.Hour + 3*7*24*time.Hour + 4*24*time.Hour + 5*time.Hour,
			wantErr:  false,
		},
		// Error cases
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid unit",
			input:    "5x",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  false, // Empty string returns zero time, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input, loc)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%s) expected error but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseDuration(%s) unexpected error: %v", tt.input, err)
				return
			}

			// For empty string, check if result is zero time
			if tt.input == "" {
				if !result.IsZero() {
					t.Errorf("parseDuration('') expected zero time but got %v", result)
				}
				return
			}

			// Calculate expected time
			expectedTime := now.Add(-tt.expected)

			// Allow for small time differences due to execution time
			diff := result.Sub(expectedTime)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("parseDuration(%s) = %v, expected approximately %v (diff: %v)",
					tt.input, result, expectedTime, diff)
			}
		})
	}
}

package analyzer

import (
	"runtime"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/presentation/formatter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAnalyzerConfig(t *testing.T) {
	config := &Config{
		DataDir:            "/tmp/data",
		CacheDir:           "/tmp/cache",
		OutputFormat:       "json",
		Timezone:           "UTC",
		Duration:           "7d",
		GroupBy:            "hour",
		Limit:              10,
		Breakdown:          true,
		Concurrency:        4,
		PricingSource:      "default",
		PricingOfflineMode: false,
	}

	analyzer := New(config)
	require.NotNil(t, analyzer)
	assert.NotNil(t, analyzer.scanner)
	assert.NotNil(t, analyzer.parser)
	assert.NotNil(t, analyzer.aggregator)
	assert.NotNil(t, analyzer.cache)
	assert.Equal(t, config, analyzer.config)
}

func TestExtractSessionId(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "standard session UUID file",
			filePath: "/home/user/.claude/projects/my-project/e1ed93d7-3427-4862-a1da-83ecded9f037.jsonl",
			expected: "e1ed93d7-3427-4862-a1da-83ecded9f037",
		},
		{
			name:     "windows path",
			filePath: `C:\Users\user\.claude\projects\my-project\d6ad9db3-e48c-4130-813a-66d081a79aa8.jsonl`,
			expected: "d6ad9db3-e48c-4130-813a-66d081a79aa8",
		},
		{
			name:     "nested project path",
			filePath: "/home/user/.claude/projects/org/my-project/data/bb5cffa8-a996-4d61-a304-6eb9e4e98e87.jsonl",
			expected: "bb5cffa8-a996-4d61-a304-6eb9e4e98e87",
		},
		{
			name:     "UUID project name",
			filePath: "/home/user/.claude/projects/123e4567-e89b-12d3-a456-426614174000/c2978b0d-d84d-4538-95bb-ca208ac01fe6.jsonl",
			expected: "c2978b0d-d84d-4538-95bb-ca208ac01fe6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSessionId(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "orange",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "apple",
			expected: false,
		},
		{
			name:     "empty string in slice",
			slice:    []string{"", "apple", "banana"},
			item:     "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzerFilterByDateRange(t *testing.T) {
	config := &Config{
		DataDir:      "/tmp/data",
		CacheDir:     "/tmp/cache",
		OutputFormat: "json",
		Timezone:     "UTC",
		Duration:     "7d",
		GroupBy:      "hour",
	}
	analyzer := New(config)

	// Create test data with different timestamps
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	oneDayAgo := now.Add(-24 * time.Hour)
	sixDaysAgo := now.Add(-6 * 24 * time.Hour) // Well within 7 days
	twoWeeksAgo := now.Add(-14 * 24 * time.Hour)

	testData := []aggregator.HourlyData{
		{Hour: oneHourAgo.Unix(), Model: "claude-3-sonnet", InputTokens: 100},
		{Hour: oneDayAgo.Unix(), Model: "claude-3-sonnet", InputTokens: 200},
		{Hour: sixDaysAgo.Unix(), Model: "claude-3-sonnet", InputTokens: 300},
		{Hour: twoWeeksAgo.Unix(), Model: "claude-3-sonnet", InputTokens: 400},
	}

	filtered := analyzer.filterByDateRange(testData)

	// Should include data from the last 7 days (excluding the two weeks ago entry)
	assert.Len(t, filtered, 3, "Should filter out data older than 7 days")
	
	// Verify the oldest entry is filtered out
	for _, item := range filtered {
		assert.Greater(t, item.Hour, twoWeeksAgo.Unix(), "Should not include data older than 7 days")
	}
}

func TestAnalyzerFilterByDateRangeEmptyDuration(t *testing.T) {
	config := &Config{
		Timezone: "UTC",
		Duration: "", // Empty duration should return all data
	}
	analyzer := New(config)

	testData := []aggregator.HourlyData{
		{Hour: time.Now().Unix(), Model: "claude-3-sonnet", InputTokens: 100},
		{Hour: time.Now().Add(-24 * time.Hour).Unix(), Model: "claude-3-sonnet", InputTokens: 200},
	}

	filtered := analyzer.filterByDateRange(testData)
	assert.Equal(t, testData, filtered, "Empty duration should return all data")
}

func TestAnalyzerGetGroupKey(t *testing.T) {
	tests := []struct {
		name     string
		groupBy  string
		item     aggregator.HourlyData
		expected string
	}{
		{
			name:    "group by model",
			groupBy: "model",
			item:    aggregator.HourlyData{Model: "claude-3-sonnet"},
			expected: "claude-3-sonnet",
		},
		{
			name:    "group by project",
			groupBy: "project",
			item:    aggregator.HourlyData{ProjectName: "my-project"},
			expected: "my-project",
		},
		{
			name:    "group by hour",
			groupBy: "hour",
			item:    aggregator.HourlyData{Hour: time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix()},
			expected: time.Unix(time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix(), 0).Format("2006-01-02 15:00"), // Use actual timezone conversion
		},
		{
			name:    "group by day",
			groupBy: "day",
			item:    aggregator.HourlyData{Hour: time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix()},
			expected: "2023-10-15",
		},
		{
			name:    "group by week",
			groupBy: "week",
			item:    aggregator.HourlyData{Hour: time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix()},
			expected: "2023-W41",
		},
		{
			name:    "group by month",
			groupBy: "month",
			item:    aggregator.HourlyData{Hour: time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix()},
			expected: "2023-10",
		},
		{
			name:    "default grouping (day)",
			groupBy: "invalid",
			item:    aggregator.HourlyData{Hour: time.Date(2023, 10, 15, 14, 0, 0, 0, time.UTC).Unix()},
			expected: "2023-10-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				GroupBy:  tt.groupBy,
				Timezone: "UTC", // Set timezone to UTC for consistent testing
				DataDir:  "/tmp/data",
				CacheDir: "/tmp/cache",
			}
			analyzer := New(config)
			result := analyzer.getGroupKey(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzerGroupData(t *testing.T) {
	config := &Config{
		GroupBy:   "model",
		Breakdown: true,
	}
	analyzer := New(config)

	testData := []aggregator.HourlyData{
		{
			Model:        "claude-3-sonnet",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
		{
			Model:        "claude-3-sonnet",
			InputTokens:  200,
			OutputTokens: 100,
			TotalTokens:  300,
		},
		{
			Model:        "claude-3-haiku",
			InputTokens:  150,
			OutputTokens: 75,
			TotalTokens:  225,
		},
	}

	grouped := analyzer.groupData(testData)

	require.Len(t, grouped, 2, "Should have 2 groups for 2 different models")

	// Find sonnet group
	var sonnetGroup *formatter.GroupedData
	for i := range grouped {
		if grouped[i].Date == "claude-3-sonnet" {
			sonnetGroup = &grouped[i]
			break
		}
	}
	require.NotNil(t, sonnetGroup, "Should have claude-3-sonnet group")

	// Verify aggregated values for sonnet
	assert.Equal(t, 300, sonnetGroup.InputTokens, "Should sum input tokens")
	assert.Equal(t, 150, sonnetGroup.OutputTokens, "Should sum output tokens")
	assert.Equal(t, 450, sonnetGroup.TotalTokens, "Should sum total tokens")
	assert.Contains(t, sonnetGroup.Models, "claude-3-sonnet", "Should include model in models list")

	// Verify breakdown is included
	assert.True(t, sonnetGroup.ShowBreakdown, "Should show breakdown when requested")
	assert.Len(t, sonnetGroup.ModelDetails, 1, "Should have model details for breakdown")
}

func TestAnalyzerSortData(t *testing.T) {
	config := &Config{}
	analyzer := New(config)

	testData := []formatter.GroupedData{
		{Date: "2023-10-15"},
		{Date: "2023-10-13"},
		{Date: "2023-10-14"},
	}

	sorted := analyzer.sortData(testData)

	expected := []string{"2023-10-13", "2023-10-14", "2023-10-15"}
	for i, group := range sorted {
		assert.Equal(t, expected[i], group.Date, "Should be sorted by date")
	}
}

func TestAnalyzerFormatAndOutput(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"json format", "json"},
		{"csv format", "csv"},
		{"summary format", "summary"},
		{"table format (default)", "table"},
		{"invalid format defaults to table", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{OutputFormat: tt.format}
			analyzer := New(config)

			testData := []formatter.GroupedData{
				{
					Date:         "2023-10-15",
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
					Models:       []string{"claude-3-sonnet"},
				},
			}

			// Note: This will fail because the formatters try to print to stdout
			// In a real implementation, we'd want to inject io.Writer for testing
			// For now, we just test that the function doesn't panic
			assert.NotPanics(t, func() {
				analyzer.formatAndOutput(testData)
			})
		})
	}
}

func TestAnalyzerConfigDefaults(t *testing.T) {
	config := &Config{
		DataDir:  "/tmp/data",
		CacheDir: "/tmp/cache",
	}

	analyzer := New(config)

	// Test that concurrency defaults to CPU count when 0
	assert.Equal(t, runtime.NumCPU(), config.Concurrency)
	
	// Test that all components are initialized
	assert.NotNil(t, analyzer.config)
	assert.NotNil(t, analyzer.cache)
	assert.NotNil(t, analyzer.scanner)
	assert.NotNil(t, analyzer.parser)
	assert.NotNil(t, analyzer.aggregator)
}

func TestAnalyzerConcurrencyConfig(t *testing.T) {
	config := &Config{
		DataDir:     "/tmp/data",
		CacheDir:    "/tmp/cache",
		Concurrency: 8,
	}

	_ = New(config)

	// Test that explicit concurrency value is preserved
	assert.Equal(t, 8, config.Concurrency)
}

func TestAnalyzerTimezoneHandling(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
	}{
		{"UTC timezone", "UTC"},
		{"Local timezone", "Local"},
		{"Asia/Shanghai timezone", "Asia/Shanghai"},
		{"America/New_York timezone", "America/New_York"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DataDir:  "/tmp/data",
				CacheDir: "/tmp/cache",
				Timezone: tt.timezone,
			}

			analyzer := New(config)
			
			// Test that analyzer is created successfully with different timezones
			assert.NotNil(t, analyzer)
			assert.Equal(t, tt.timezone, analyzer.config.Timezone)
		})
	}
}

func TestParseDurationEdgeCases(t *testing.T) {
	loc := time.UTC
	
	tests := []struct {
		name     string
		input    string
		wantErr  bool
	}{
		{"invalid unit z", "5z", true},
		{"missing number", "h", true},
		{"very large number that causes overflow", "999999999999999999999999h", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDuration(tt.input, loc)
			if tt.wantErr {
				assert.Error(t, err, "Expected error for input: %s", tt.input)
			} else {
				assert.NoError(t, err, "Expected no error for input: %s", tt.input)
			}
		})
	}
}

func TestExtractSessionIdEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "filename without extension",
			filePath: "/path/to/session-id",
			expected: "session-id",
		},
		{
			name:     "multiple extensions",
			filePath: "/path/to/session.backup.jsonl",
			expected: "session.backup",
		},
		{
			name:     "empty filename",
			filePath: "/path/to/.jsonl",
			expected: "",
		},
		{
			name:     "just filename",
			filePath: "session-id.jsonl",
			expected: "session-id",
		},
		{
			name:     "mixed separators",
			filePath: `/path\to/session-id.jsonl`,
			expected: "session-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSessionId(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

package aggregator

import (
	"fmt"
	"testing"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAggregatorWithTimezone(t *testing.T) {
	timezone := "America/New_York"
	aggregator := NewAggregatorWithTimezone(timezone)

	assert.NotNil(t, aggregator)
	assert.Equal(t, timezone, aggregator.timezone)
	assert.NotNil(t, aggregator.pricing)
}

func TestNewAggregatorWithConfig(t *testing.T) {
	tests := []struct {
		name               string
		pricingSource      string
		pricingOfflineMode bool
		cacheDir           string
		timezone           string
		expectError        bool
	}{
		{
			name:               "valid default config",
			pricingSource:      "default",
			pricingOfflineMode: false,
			cacheDir:           "",
			timezone:           "UTC",
			expectError:        false,
		},
		{
			name:               "valid offline mode",
			pricingSource:      "default",
			pricingOfflineMode: true,
			cacheDir:           "",
			timezone:           "America/New_York",
			expectError:        false,
		},
		{
			name:               "invalid pricing source",
			pricingSource:      "invalid",
			pricingOfflineMode: false,
			cacheDir:           "",
			timezone:           "UTC",
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator, err := NewAggregatorWithConfig(tt.pricingSource, tt.pricingOfflineMode, tt.cacheDir, tt.timezone)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, aggregator)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, aggregator)
				assert.Equal(t, tt.timezone, aggregator.timezone)
				assert.NotNil(t, aggregator.pricing)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	aggregator := NewAggregatorWithTimezone("UTC")

	tests := []struct {
		name     string
		data     *HourlyData
		expected float64
	}{
		{
			name: "basic tokens",
			data: &HourlyData{
				Model:        "claude-3-sonnet",
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expected: 0.0105, // Using default pricing fallback
		},
		{
			name: "with cache tokens",
			data: &HourlyData{
				Model:         "claude-3-sonnet",
				InputTokens:   1000,
				OutputTokens:  500,
				CacheCreation: 250,
				CacheRead:     100,
			},
			expected: 0.01146750, // Using default pricing fallback: (1000/1M * 3.0) + (500/1M * 15.0) + (250/1M * 3.75) + (100/1M * 0.3)
		},
		{
			name: "zero tokens",
			data: &HourlyData{
				Model: "claude-3-sonnet",
			},
			expected: 0.0,
		},
		{
			name: "unknown model uses default pricing",
			data: &HourlyData{
				Model:        "unknown-model",
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expected: 0.0105, // (1000/1M * 3.0) + (500/1M * 15.0) = 0.003 + 0.0075 = 0.0105 (using default pricing)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := aggregator.CalculateCost(tt.data)
			assert.NoError(t, err)
			assert.InDelta(t, tt.expected, cost, 0.0000001, "Cost calculation mismatch")
		})
	}
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "simple project name",
			filePath: "/home/user/.claude/projects/my-project/session.jsonl",
			expected: "my-project",
		},
		{
			name:     "UUID project name",
			filePath: "/home/user/.claude/projects/12345678-1234-1234-1234-123456789012/session.jsonl",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "nested UUID project",
			filePath: "/home/user/.claude/projects/parent/12345678-1234-1234-1234-123456789012/session.jsonl",
			expected: "parent/12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "nested non-UUID project",
			filePath: "/home/user/.claude/projects/parent/child/session.jsonl",
			expected: "child",
		},
		{
			name:     "root level file",
			filePath: "/session.jsonl",
			expected: "/",
		},
		{
			name:     "current directory",
			filePath: "./session.jsonl",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractProjectName(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid UUID",
			input:    "12345678-1234-1234-1234-123456789012",
			expected: true,
		},
		{
			name:     "valid UUID with different chars",
			input:    "abcdefab-cdef-abcd-efab-cdefabcdefab",
			expected: true,
		},
		{
			name:     "invalid UUID - too short",
			input:    "12345678-1234-1234-1234-12345678901",
			expected: false,
		},
		{
			name:     "invalid UUID - too long",
			input:    "12345678-1234-1234-1234-1234567890123",
			expected: false,
		},
		{
			name:     "invalid UUID - wrong segments",
			input:    "12345678-1234-1234-12345-123456789012",
			expected: false,
		},
		{
			name:     "invalid UUID - missing dashes",
			input:    "123456781234123412341234567890123",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "regular string",
			input:    "my-project",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUUID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTokens(t *testing.T) {
	tests := []struct {
		name     string
		log      model.ConversationLog
		expected TokenCounts
	}{
		{
			name: "message type with tokens",
			log: model.ConversationLog{
				Type: model.EntryMessage,
				Message: model.Message{
					Usage: model.Usage{
						InputTokens:              100,
						OutputTokens:             50,
						CacheCreationInputTokens: 25,
						CacheReadInputTokens:     10,
					},
				},
			},
			expected: TokenCounts{
				InputTokens:   100,
				OutputTokens:  50,
				CacheCreation: 25,
				CacheRead:     10,
				TotalTokens:   185,
			},
		},
		{
			name: "assistant type with tokens",
			log: model.ConversationLog{
				Type: model.EntryAssistant,
				Message: model.Message{
					Usage: model.Usage{
						InputTokens:  200,
						OutputTokens: 100,
					},
				},
			},
			expected: TokenCounts{
				InputTokens:   200,
				OutputTokens:  100,
				CacheCreation: 0,
				CacheRead:     0,
				TotalTokens:   300,
			},
		},
		{
			name: "non-message type",
			log: model.ConversationLog{
				Type: "other",
				Message: model.Message{
					Usage: model.Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
			expected: TokenCounts{
				InputTokens:   0,
				OutputTokens:  0,
				CacheCreation: 0,
				CacheRead:     0,
				TotalTokens:   0,
			},
		},
		{
			name: "zero tokens",
			log: model.ConversationLog{
				Type: model.EntryMessage,
				Message: model.Message{
					Usage: model.Usage{},
				},
			},
			expected: TokenCounts{
				InputTokens:   0,
				OutputTokens:  0,
				CacheCreation: 0,
				CacheRead:     0,
				TotalTokens:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTokens(tt.log)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateToHourUTC(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		expected  int64
	}{
		{
			name:      "exact hour",
			timestamp: 1640995200, // 2022-01-01 00:00:00 UTC
			expected:  1640995200,
		},
		{
			name:      "mid hour",
			timestamp: 1640995200 + 1800, // 2022-01-01 00:30:00 UTC
			expected:  1640995200,         // Should truncate to 00:00:00
		},
		{
			name:      "end of hour",
			timestamp: 1640995200 + 3599, // 2022-01-01 00:59:59 UTC
			expected:  1640995200,         // Should truncate to 00:00:00
		},
		{
			name:      "next hour",
			timestamp: 1640995200 + 3600, // 2022-01-01 01:00:00 UTC
			expected:  1640995200 + 3600,  // Should be 01:00:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToHourUTC(tt.timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseToUnixTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		timestampStr string
		expectError bool
		expected    int64
	}{
		{
			name:         "valid RFC3339",
			timestampStr: "2022-01-01T00:00:00Z",
			expectError:  false,
			expected:     1640995200,
		},
		{
			name:         "valid RFC3339 with timezone",
			timestampStr: "2022-01-01T05:00:00-05:00",
			expectError:  false,
			expected:     1641031200, // 2022-01-01T10:00:00Z (corrected)
		},
		{
			name:         "invalid format",
			timestampStr: "2022-01-01 00:00:00",
			expectError:  true,
		},
		{
			name:         "empty string",
			timestampStr: "",
			expectError:  true,
		},
		{
			name:         "invalid date",
			timestampStr: "invalid-date",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseToUnixTimestamp(tt.timestampStr)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal model name",
			input:    "claude-3-sonnet",
			expected: "claude-3-sonnet",
		},
		{
			name:     "empty model name",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "whitespace model name",
			input:    "   ",
			expected: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeModelName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregateByHourAndModel(t *testing.T) {
	aggregator := NewAggregatorWithTimezone("UTC")

	tests := []struct {
		name        string
		logs        []model.ConversationLog
		projectName string
		expected    []HourlyData
	}{
		{
			name: "single message",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:30:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    100,
					OutputTokens:   50,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    150,
					MessageCount:   1,
					FirstEntryTime: 1640997000, // 2022-01-01T00:30:00Z
					LastEntryTime:  1640997000,
				},
			},
		},
		{
			name: "multiple messages same hour",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:15:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					},
				},
				{
					Type:      model.EntryAssistant,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:45:00Z",
					Message: model.Message{
						Id:    "msg-2",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  150, // Higher value should be used
							OutputTokens: 75,  // Higher value should be used
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    150, // Max of 100, 150
					OutputTokens:   75,  // Max of 50, 75
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    225,
					MessageCount:   1, // One request ID
					FirstEntryTime: 1640996100, // 2022-01-01T00:15:00Z
					LastEntryTime:  1640997900, // 2022-01-01T00:45:00Z
				},
			},
		},
		{
			name: "different hours",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:30:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					},
				},
				{
					Type:      model.EntryMessage,
					RequestId: "req-2",
					Timestamp: "2022-01-01T01:30:00Z",
					Message: model.Message{
						Id:    "msg-2",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  200,
							OutputTokens: 100,
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    100,
					OutputTokens:   50,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    150,
					MessageCount:   1,
					FirstEntryTime: 1640997000, // 2022-01-01T00:30:00Z
					LastEntryTime:  1640997000,
				},
				{
					Hour:           1640998800, // 2022-01-01T01:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    200,
					OutputTokens:   100,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    300,
					MessageCount:   1,
					FirstEntryTime: 1641000600, // 2022-01-01T01:30:00Z
					LastEntryTime:  1641000600,
				},
			},
		},
		{
			name: "different models",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:30:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					},
				},
				{
					Type:      model.EntryMessage,
					RequestId: "req-2",
					Timestamp: "2022-01-01T00:45:00Z",
					Message: model.Message{
						Id:    "msg-2",
						Model: "claude-3-haiku",
						Usage: model.Usage{
							InputTokens:  200,
							OutputTokens: 100,
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-haiku",
					ProjectName:    "test-project",
					InputTokens:    200,
					OutputTokens:   100,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    300,
					MessageCount:   1,
					FirstEntryTime: 1640997900, // 2022-01-01T00:45:00Z (corrected)
					LastEntryTime:  1640997900,
				},
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    100,
					OutputTokens:   50,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    150,
					MessageCount:   1,
					FirstEntryTime: 1640997000, // 2022-01-01T00:30:00Z
					LastEntryTime:  1640997000,
				},
			},
		},
		{
			name: "with cache tokens",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:30:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "claude-3-sonnet",
						Usage: model.Usage{
							InputTokens:              100,
							OutputTokens:             50,
							CacheCreationInputTokens: 25,
							CacheReadInputTokens:     10,
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "claude-3-sonnet",
					ProjectName:    "test-project",
					InputTokens:    100,
					OutputTokens:   50,
					CacheCreation:  25,
					CacheRead:      10,
					TotalTokens:    185,
					MessageCount:   1,
					FirstEntryTime: 1640997000, // 2022-01-01T00:30:00Z
					LastEntryTime:  1640997000,
				},
			},
		},
		{
			name:        "empty logs",
			logs:        []model.ConversationLog{},
			projectName: "test-project",
			expected:    []HourlyData{},
		},
		{
			name: "invalid logs (missing required fields)",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					Timestamp: "2022-01-01T00:30:00Z",
					// Missing RequestId and Message.Id
				},
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "invalid-timestamp",
					Message: model.Message{
						Id: "msg-1",
					},
				},
			},
			projectName: "test-project",
			expected:    []HourlyData{},
		},
		{
			name: "unknown model name",
			logs: []model.ConversationLog{
				{
					Type:      model.EntryMessage,
					RequestId: "req-1",
					Timestamp: "2022-01-01T00:30:00Z",
					Message: model.Message{
						Id:    "msg-1",
						Model: "", // Empty model name
						Usage: model.Usage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					},
				},
			},
			projectName: "test-project",
			expected: []HourlyData{
				{
					Hour:           1640995200, // 2022-01-01T00:00:00Z
					Model:          "unknown",
					ProjectName:    "test-project",
					InputTokens:    100,
					OutputTokens:   50,
					CacheCreation:  0,
					CacheRead:      0,
					TotalTokens:    150,
					MessageCount:   1,
					FirstEntryTime: 1640997000, // 2022-01-01T00:30:00Z
					LastEntryTime:  1640997000,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregator.AggregateByHourAndModel(tt.logs, tt.projectName)
			
			// Sort both slices by hour and model for consistent comparison
			require.Len(t, result, len(tt.expected))
			
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual.Hour == expected.Hour && actual.Model == expected.Model {
						assert.Equal(t, expected.ProjectName, actual.ProjectName, "ProjectName mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.InputTokens, actual.InputTokens, "InputTokens mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.OutputTokens, actual.OutputTokens, "OutputTokens mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.CacheCreation, actual.CacheCreation, "CacheCreation mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.CacheRead, actual.CacheRead, "CacheRead mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.TotalTokens, actual.TotalTokens, "TotalTokens mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.MessageCount, actual.MessageCount, "MessageCount mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.FirstEntryTime, actual.FirstEntryTime, "FirstEntryTime mismatch for hour %d, model %s", expected.Hour, expected.Model)
						assert.Equal(t, expected.LastEntryTime, actual.LastEntryTime, "LastEntryTime mismatch for hour %d, model %s", expected.Hour, expected.Model)
						found = true
						break
					}
				}
				assert.True(t, found, "Expected entry not found: hour %d, model %s", expected.Hour, expected.Model)
			}
		})
	}
}

func TestHourlyDataAndAggregatedDataStructures(t *testing.T) {
	// Test HourlyData structure
	hourlyData := HourlyData{
		Hour:           1640995200,
		Model:          "claude-3-sonnet",
		ProjectName:    "test-project",
		InputTokens:    100,
		OutputTokens:   50,
		CacheCreation:  25,
		CacheRead:      10,
		TotalTokens:    185,
		MessageCount:   1,
		FirstEntryTime: 1640997000,
		LastEntryTime:  1640997000,
	}

	assert.Equal(t, int64(1640995200), hourlyData.Hour)
	assert.Equal(t, "claude-3-sonnet", hourlyData.Model)
	assert.Equal(t, "test-project", hourlyData.ProjectName)
	assert.Equal(t, 100, hourlyData.InputTokens)
	assert.Equal(t, 50, hourlyData.OutputTokens)
	assert.Equal(t, 25, hourlyData.CacheCreation)
	assert.Equal(t, 10, hourlyData.CacheRead)
	assert.Equal(t, 185, hourlyData.TotalTokens)
	assert.Equal(t, 1, hourlyData.MessageCount)

	// Test AggregatedData structure
	aggregatedData := AggregatedData{
		FileHash:           "test-hash",
		FilePath:           "/path/to/file.jsonl",
		SessionId:          "session-123",
		ProjectName:        "test-project",
		HourlyStats:        []HourlyData{hourlyData},
		LastModified:       1640995200,
		FileSize:           1024,
		Inode:              12345,
		ContentFingerprint: "fingerprint-123",
	}

	assert.Equal(t, "test-hash", aggregatedData.FileHash)
	assert.Equal(t, "/path/to/file.jsonl", aggregatedData.FilePath)
	assert.Equal(t, "session-123", aggregatedData.SessionId)
	assert.Equal(t, "test-project", aggregatedData.ProjectName)
	assert.Len(t, aggregatedData.HourlyStats, 1)
	assert.Equal(t, hourlyData, aggregatedData.HourlyStats[0])
	assert.Equal(t, int64(1640995200), aggregatedData.LastModified)
	assert.Equal(t, int64(1024), aggregatedData.FileSize)
	assert.Equal(t, uint64(12345), aggregatedData.Inode)
	assert.Equal(t, "fingerprint-123", aggregatedData.ContentFingerprint)
}

func TestTokenCountsStructure(t *testing.T) {
	tokens := TokenCounts{
		InputTokens:   100,
		OutputTokens:  50,
		CacheCreation: 25,
		CacheRead:     10,
		TotalTokens:   185,
	}

	assert.Equal(t, 100, tokens.InputTokens)
	assert.Equal(t, 50, tokens.OutputTokens)
	assert.Equal(t, 25, tokens.CacheCreation)
	assert.Equal(t, 10, tokens.CacheRead)
	assert.Equal(t, 185, tokens.TotalTokens)
}

// Benchmark tests for performance critical functions
func BenchmarkAggregateByHourAndModel(b *testing.B) {
	aggregator := NewAggregatorWithTimezone("UTC")
	
	// Create a large set of test logs
	logs := make([]model.ConversationLog, 1000)
	for i := 0; i < 1000; i++ {
		logs[i] = model.ConversationLog{
			Type:      model.EntryMessage,
			RequestId: fmt.Sprintf("req-%d", i),
			Timestamp: "2022-01-01T00:30:00Z",
			Message: model.Message{
				Id:    fmt.Sprintf("msg-%d", i),
				Model: "claude-3-sonnet",
				Usage: model.Usage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregator.AggregateByHourAndModel(logs, "test-project")
	}
}

func BenchmarkExtractTokens(b *testing.B) {
	log := model.ConversationLog{
		Type: model.EntryMessage,
		Message: model.Message{
			Usage: model.Usage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 25,
				CacheReadInputTokens:     10,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractTokens(log)
	}
}

func BenchmarkExtractProjectName(b *testing.B) {
	filePath := "/home/user/.claude/projects/my-project/session.jsonl"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractProjectName(filePath)
	}
}
package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

func TestNewLimitParser(t *testing.T) {
	parser := NewLimitParser()
	if parser == nil {
		t.Fatal("NewLimitParser returned nil")
	}
	if parser.opusPattern == nil {
		t.Error("Expected opusPattern to be initialized")
	}
	if parser.waitPattern == nil {
		t.Error("Expected waitPattern to be initialized")
	}
	if parser.resetPattern == nil {
		t.Error("Expected resetPattern to be initialized")
	}
	if parser.generalPattern == nil {
		t.Error("Expected generalPattern to be initialized")
	}
}

func TestParseLogs(t *testing.T) {
	tests := []struct {
		name          string
		logs          []model.ConversationLog
		expectedCount int
		expectedTypes []string
	}{
		{
			name:          "empty_logs",
			logs:          []model.ConversationLog{},
			expectedCount: 0,
			expectedTypes: []string{},
		},
		{
			name: "opus_limit_system_message",
			logs: []model.ConversationLog{
				{
					Type:      "system",
					Content:   "Opus rate limit exceeded. Please wait 10 minutes.",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req1",
					SessionId: "session1",
				},
			},
			expectedCount: 1,
			expectedTypes: []string{"opus_limit"},
		},
		{
			name: "general_limit_system_message",
			logs: []model.ConversationLog{
				{
					Type:      "system",
					Content:   "Rate limit reached. Please try again later.",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req2",
					SessionId: "session1",
				},
			},
			expectedCount: 1,
			expectedTypes: []string{"system_limit"},
		},
		{
			name: "tool_result_limit_message",
			logs: []model.ConversationLog{
				{
					Type:      "assistant",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req3",
					SessionId: "session1",
					Message: model.Message{
						Id:    "msg1",
						Model: "claude-3-sonnet",
						Content: []model.ContentItem{
							{
								Type:    "tool_result",
								Content: "Claude AI usage limit reached|1704106800",
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedTypes: []string{"general_limit"},
		},
		{
			name: "text_content_limit_message",
			logs: []model.ConversationLog{
				{
					Type:      "user",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req4",
					SessionId: "session1",
					Message: model.Message{
						Id:    "msg2",
						Model: "claude-3-haiku",
						Content: []model.ContentItem{
							{
								Type: "text",
								Text: "Claude AI usage limit reached|1704106800000",
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedTypes: []string{"api_error_limit"},
		},
		{
			name: "multiple_limit_messages",
			logs: []model.ConversationLog{
				{
					Type:      "system",
					Content:   "Opus rate limit exceeded. Please wait 5 minutes.",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req5",
					SessionId: "session1",
				},
				{
					Type:      "system",
					Content:   "General rate limit reached.",
					Timestamp: "2024-01-01T10:05:00Z",
					RequestId: "req6",
					SessionId: "session1",
				},
			},
			expectedCount: 2,
			expectedTypes: []string{"opus_limit", "system_limit"},
		},
		{
			name: "non_limit_messages",
			logs: []model.ConversationLog{
				{
					Type:      "system",
					Content:   "Welcome to Claude!",
					Timestamp: "2024-01-01T10:00:00Z",
					RequestId: "req7",
					SessionId: "session1",
				},
				{
					Type:      "user",
					Timestamp: "2024-01-01T10:01:00Z",
					RequestId: "req8",
					SessionId: "session1",
					Message: model.Message{
						Content: []model.ContentItem{
							{
								Type: "text",
								Text: "Hello, how are you?",
							},
						},
					},
				},
			},
			expectedCount: 0,
			expectedTypes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			limits := parser.ParseLogs(tt.logs)
			
			if len(limits) != tt.expectedCount {
				t.Errorf("Expected %d limits, got %d", tt.expectedCount, len(limits))
			}
			
			for i, expectedType := range tt.expectedTypes {
				if i >= len(limits) {
					t.Errorf("Expected limit %d with type %s, but only got %d limits", i, expectedType, len(limits))
					continue
				}
				if limits[i].Type != expectedType {
					t.Errorf("Expected limit %d to have type %s, got %s", i, expectedType, limits[i].Type)
				}
			}
		})
	}
}

func TestParseSystemMessage(t *testing.T) {
	tests := []struct {
		name           string
		log            model.ConversationLog
		expectedType   string
		expectedWait   *int
		expectedReset  bool
		expectNil      bool
	}{
		{
			name: "opus_limit_with_wait_time",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "Opus rate limit exceeded. Please wait 10 minutes.",
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req1",
				SessionId: "session1",
			},
			expectedType:  "opus_limit",
			expectedWait:  intPtr(10),
			expectedReset: true,
			expectNil:     false,
		},
		{
			name: "opus_limit_without_wait_time",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "Opus rate limit hit",
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req2",
				SessionId: "session1",
			},
			expectedType:  "opus_limit",
			expectedWait:  nil,
			expectedReset: false,
			expectNil:     false,
		},
		{
			name: "general_rate_limit",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "Rate limit exceeded. Please try again later.",
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req3",
				SessionId: "session1",
			},
			expectedType:  "system_limit",
			expectedWait:  nil,
			expectedReset: false,
			expectNil:     false,
		},
		{
			name: "non_limit_message",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "Welcome to Claude!",
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req4",
				SessionId: "session1",
			},
			expectNil: true,
		},
		{
			name: "invalid_timestamp",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "Rate limit exceeded",
				Timestamp: "invalid-timestamp",
				RequestId: "req5",
				SessionId: "session1",
			},
			expectNil: true,
		},
		{
			name: "case_insensitive_matching",
			log: model.ConversationLog{
				Type:      "system",
				Content:   "OPUS RATE LIMIT EXCEEDED. Please wait 15 minutes.",
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req6",
				SessionId: "session1",
			},
			expectedType:  "opus_limit",
			expectedWait:  intPtr(15),
			expectedReset: true,
			expectNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			limit := parser.parseSystemMessage(tt.log)
			
			if tt.expectNil {
				if limit != nil {
					t.Errorf("Expected nil limit, got %+v", limit)
				}
				return
			}
			
			if limit == nil {
				t.Fatal("Expected non-nil limit")
			}
			
			if limit.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, limit.Type)
			}
			
			if tt.expectedWait != nil {
				if limit.WaitMinutes == nil {
					t.Errorf("Expected wait minutes %d, got nil", *tt.expectedWait)
				} else if *limit.WaitMinutes != *tt.expectedWait {
					t.Errorf("Expected wait minutes %d, got %d", *tt.expectedWait, *limit.WaitMinutes)
				}
			} else {
				if limit.WaitMinutes != nil {
					t.Errorf("Expected no wait minutes, got %d", *limit.WaitMinutes)
				}
			}
			
			if tt.expectedReset {
				if limit.ResetTime == nil {
					t.Error("Expected reset time to be set")
				}
			} else {
				if limit.ResetTime != nil {
					t.Errorf("Expected no reset time, got %d", *limit.ResetTime)
				}
			}
			
			// Verify basic fields
			if limit.Content != tt.log.Content {
				t.Errorf("Expected content %s, got %s", tt.log.Content, limit.Content)
			}
			if limit.RequestID != tt.log.RequestId {
				t.Errorf("Expected request ID %s, got %s", tt.log.RequestId, limit.RequestID)
			}
			if limit.SessionID != tt.log.SessionId {
				t.Errorf("Expected session ID %s, got %s", tt.log.SessionId, limit.SessionID)
			}
		})
	}
}

func TestParseUserAssistantMessage(t *testing.T) {
	tests := []struct {
		name         string
		log          model.ConversationLog
		expectedType string
		expectNil    bool
	}{
		{
			name: "assistant_with_tool_result_limit",
			log: model.ConversationLog{
				Type:      "assistant",
				Timestamp: "2024-01-01T10:00:00Z",
				Message: model.Message{
					Id:    "msg1",
					Model: "claude-3-sonnet",
					Content: []model.ContentItem{
						{
							Type:    "tool_result",
							Content: "Claude AI usage limit reached|1704106800",
						},
					},
				},
			},
			expectedType: "general_limit",
			expectNil:    false,
		},
		{
			name: "user_with_text_limit",
			log: model.ConversationLog{
				Type:      "user",
				Timestamp: "2024-01-01T10:00:00Z",
				Message: model.Message{
					Id:    "msg2",
					Model: "claude-3-haiku",
					Content: []model.ContentItem{
						{
							Type: "text",
							Text: "Claude AI usage limit reached|1704106800000",
						},
					},
				},
			},
			expectedType: "api_error_limit",
			expectNil:    false,
		},
		{
			name: "no_limit_content",
			log: model.ConversationLog{
				Type:      "user",
				Timestamp: "2024-01-01T10:00:00Z",
				Message: model.Message{
					Content: []model.ContentItem{
						{
							Type: "text",
							Text: "Hello, how are you?",
						},
					},
				},
			},
			expectNil: true,
		},
		{
			name: "empty_content",
			log: model.ConversationLog{
				Type:      "assistant",
				Timestamp: "2024-01-01T10:00:00Z",
				Message: model.Message{
					Content: []model.ContentItem{},
				},
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			limit := parser.parseUserAssistantMessage(tt.log)
			
			if tt.expectNil {
				if limit != nil {
					t.Errorf("Expected nil limit, got %+v", limit)
				}
				return
			}
			
			if limit == nil {
				t.Fatal("Expected non-nil limit")
			}
			
			if limit.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, limit.Type)
			}
			
			if limit.Model != tt.log.Message.Model {
				t.Errorf("Expected model %s, got %s", tt.log.Message.Model, limit.Model)
			}
		})
	}
}

func TestParseToolResult(t *testing.T) {
	tests := []struct {
		name         string
		item         model.ContentItem
		log          model.ConversationLog
		modelName    string
		expectedType string
		expectReset  bool
		expectNil    bool
	}{
		{
			name: "string_content_with_reset_timestamp",
			item: model.ContentItem{
				Type:    "tool_result",
				Content: "Claude AI usage limit reached|1704106800",
			},
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req1",
				SessionId: "session1",
				Message:   model.Message{Id: "msg1"},
			},
			modelName:    "claude-3-sonnet",
			expectedType: "general_limit",
			expectReset:  true,
			expectNil:    false,
		},
		{
			name: "array_content_with_limit_message",
			item: model.ContentItem{
				Type: "tool_result",
				Content: []interface{}{
					map[string]interface{}{
						"text": "Claude AI usage limit reached|1704106800000",
					},
				},
			},
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req2",
				SessionId: "session1",
			},
			modelName:    "claude-3-haiku",
			expectedType: "general_limit",
			expectReset:  true,
			expectNil:    false,
		},
		{
			name: "no_limit_content",
			item: model.ContentItem{
				Type:    "tool_result",
				Content: "Operation completed successfully",
			},
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
			},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
		{
			name: "empty_content",
			item: model.ContentItem{
				Type:    "tool_result",
				Content: "",
			},
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
			},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
		{
			name: "invalid_timestamp_format",
			item: model.ContentItem{
				Type:    "tool_result",
				Content: "Claude AI usage limit reached|1704106800",
			},
			log: model.ConversationLog{
				Timestamp: "invalid-timestamp",
				RequestId: "req3",
				SessionId: "session1",
			},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			limit := parser.parseToolResult(tt.item, tt.log, tt.modelName)
			
			if tt.expectNil {
				if limit != nil {
					t.Errorf("Expected nil limit, got %+v", limit)
				}
				return
			}
			
			if limit == nil {
				t.Fatal("Expected non-nil limit")
			}
			
			if limit.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, limit.Type)
			}
			
			if limit.Model != tt.modelName {
				t.Errorf("Expected model %s, got %s", tt.modelName, limit.Model)
			}
			
			if tt.expectReset {
				if limit.ResetTime == nil {
					t.Error("Expected reset time to be set")
				} else {
					// Verify reset time is reasonable (should be a Unix timestamp)
					if *limit.ResetTime < 1000000000 || *limit.ResetTime > 2000000000 {
						t.Errorf("Reset time %d seems invalid", *limit.ResetTime)
					}
				}
			} else {
				if limit.ResetTime != nil {
					t.Errorf("Expected no reset time, got %d", *limit.ResetTime)
				}
			}
		})
	}
}

func TestParseTextContent(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		log          model.ConversationLog
		modelName    string
		expectedType string
		expectReset  bool
		expectNil    bool
	}{
		{
			name: "text_with_limit_and_reset_timestamp",
			text: "Claude AI usage limit reached|1704106800000",
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req1",
				SessionId: "session1",
				Message:   model.Message{Id: "msg1"},
			},
			modelName:    "claude-3-sonnet",
			expectedType: "api_error_limit",
			expectReset:  true,
			expectNil:    false,
		},
		{
			name: "text_with_limit_no_timestamp",
			text: "Claude AI usage limit reached without timestamp",
			log: model.ConversationLog{
				Timestamp: "2024-01-01T10:00:00Z",
				RequestId: "req2",
				SessionId: "session1",
			},
			modelName:    "claude-3-haiku",
			expectedType: "api_error_limit",
			expectReset:  false,
			expectNil:    false,
		},
		{
			name:      "text_without_limit",
			text:      "This is a normal message",
			log:       model.ConversationLog{Timestamp: "2024-01-01T10:00:00Z"},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
		{
			name:      "empty_text",
			text:      "",
			log:       model.ConversationLog{Timestamp: "2024-01-01T10:00:00Z"},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
		{
			name: "invalid_log_timestamp",
			text: "Claude AI usage limit reached|1704106800",
			log: model.ConversationLog{
				Timestamp: "invalid-timestamp",
				RequestId: "req3",
			},
			modelName: "claude-3-sonnet",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			limit := parser.parseTextContent(tt.text, tt.log, tt.modelName)
			
			if tt.expectNil {
				if limit != nil {
					t.Errorf("Expected nil limit, got %+v", limit)
				}
				return
			}
			
			if limit == nil {
				t.Fatal("Expected non-nil limit")
			}
			
			if limit.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, limit.Type)
			}
			
			if limit.Model != tt.modelName {
				t.Errorf("Expected model %s, got %s", tt.modelName, limit.Model)
			}
			
			if limit.Content != tt.text {
				t.Errorf("Expected content %s, got %s", tt.text, limit.Content)
			}
			
			if tt.expectReset {
				if limit.ResetTime == nil {
					t.Error("Expected reset time to be set")
				} else {
					// Verify reset time is reasonable
					if *limit.ResetTime < 1000000000 || *limit.ResetTime > 2000000000 {
						t.Errorf("Reset time %d seems invalid", *limit.ResetTime)
					}
				}
			} else {
				if limit.ResetTime != nil {
					t.Errorf("Expected no reset time, got %d", *limit.ResetTime)
				}
			}
		})
	}
}

func TestDetectWindowFromLimits(t *testing.T) {
	// Use timestamps from 2 hours ago to ensure they're recent enough
	now := time.Now()
	recentTimestamp := now.Add(-2 * time.Hour).Unix()
	recentResetTime := recentTimestamp + (5 * 60 * 60) // 5 hours after timestamp
	
	tests := []struct {
		name                string
		limits              []LimitInfo
		expectWindowStart   bool
		expectedSource      string
		expectedWindowStart int64
	}{
		{
			name:              "empty_limits",
			limits:            []LimitInfo{},
			expectWindowStart: false,
			expectedSource:    "",
		},
		{
			name: "single_limit_with_reset_time",
			limits: []LimitInfo{
				{
					Type:      "opus_limit",
					Timestamp: recentTimestamp,
					ResetTime: int64Ptr(recentResetTime),
				},
			},
			expectWindowStart:   true,
			expectedSource:      "limit_message",
			expectedWindowStart: recentTimestamp, // Window starts 5 hours before reset
		},
		{
			name: "multiple_limits_picks_most_recent_with_reset",
			limits: []LimitInfo{
				{
					Type:      "system_limit",
					Timestamp: recentTimestamp - 3600, // 1 hour earlier
					ResetTime: int64Ptr(recentResetTime - 3600),
				},
				{
					Type:      "opus_limit",
					Timestamp: recentTimestamp, // More recent
					ResetTime: int64Ptr(recentResetTime),
				},
			},
			expectWindowStart:   true,
			expectedSource:      "limit_message",
			expectedWindowStart: recentTimestamp, // Based on more recent limit
		},
		{
			name: "limits_without_reset_time",
			limits: []LimitInfo{
				{
					Type:      "system_limit",
					Timestamp: recentTimestamp,
					ResetTime: nil,
				},
				{
					Type:      "general_limit",
					Timestamp: recentTimestamp + 3600,
					ResetTime: nil,
				},
			},
			expectWindowStart: false,
			expectedSource:    "",
		},
		{
			name: "mixed_limits_with_and_without_reset",
			limits: []LimitInfo{
				{
					Type:      "system_limit",
					Timestamp: recentTimestamp + 3600,
					ResetTime: nil, // No reset time
				},
				{
					Type:      "opus_limit",
					Timestamp: recentTimestamp,
					ResetTime: int64Ptr(recentResetTime), // Has reset time
				},
			},
			expectWindowStart:   true,
			expectedSource:      "limit_message",
			expectedWindowStart: recentTimestamp, // Based on limit with reset time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLimitParser()
			windowStart, source := parser.DetectWindowFromLimits(tt.limits)
			
			if tt.expectWindowStart {
				if windowStart == nil {
					t.Error("Expected window start to be set")
				} else {
					if *windowStart != tt.expectedWindowStart {
						t.Errorf("Expected window start %d, got %d", tt.expectedWindowStart, *windowStart)
					}
				}
				if source != tt.expectedSource {
					t.Errorf("Expected source %s, got %s", tt.expectedSource, source)
				}
			} else {
				if windowStart != nil {
					t.Errorf("Expected no window start, got %d", *windowStart)
				}
				if source != tt.expectedSource {
					t.Errorf("Expected empty source, got %s", source)
				}
			}
		})
	}
}

// Test edge cases and error conditions
func TestLimitParserEdgeCases(t *testing.T) {
	t.Run("nil_logs", func(t *testing.T) {
		parser := NewLimitParser()
		limits := parser.ParseLogs(nil)
		if len(limits) != 0 {
			t.Errorf("Expected 0 limits for nil logs, got %d", len(limits))
		}
	})

	t.Run("logs_with_nil_content", func(t *testing.T) {
		parser := NewLimitParser()
		logs := []model.ConversationLog{
			{
				Type:      "assistant",
				Timestamp: "2024-01-01T10:00:00Z",
				Message: model.Message{
					Content: []model.ContentItem{
						{
							Type:    "tool_result",
							Content: nil,
						},
					},
				},
			},
		}
		limits := parser.ParseLogs(logs)
		if len(limits) != 0 {
			t.Errorf("Expected 0 limits for nil content, got %d", len(limits))
		}
	})

	t.Run("complex_array_content", func(t *testing.T) {
		parser := NewLimitParser()
		item := model.ContentItem{
			Type: "tool_result",
			Content: []interface{}{
				map[string]interface{}{
					"text": "Some other text",
				},
				map[string]interface{}{
					"text": "Claude AI usage limit reached|1704106800",
				},
				"invalid_item", // This should be ignored
			},
		}
		log := model.ConversationLog{
			Timestamp: "2024-01-01T10:00:00Z",
			RequestId: "req1",
			SessionId: "session1",
		}
		limit := parser.parseToolResult(item, log, "claude-3-sonnet")
		
		if limit == nil {
			t.Error("Expected to find limit in complex array content")
		} else if limit.Type != "general_limit" {
			t.Errorf("Expected general_limit, got %s", limit.Type)
		}
	})

	t.Run("millisecond_vs_second_timestamps", func(t *testing.T) {
		parser := NewLimitParser()
		
		// Test with millisecond timestamp (should be converted to seconds)
		text1 := "Claude AI usage limit reached|1704106800000"
		log := model.ConversationLog{
			Timestamp: "2024-01-01T10:00:00Z",
		}
		limit1 := parser.parseTextContent(text1, log, "claude-3-sonnet")
		
		if limit1 == nil || limit1.ResetTime == nil {
			t.Fatal("Expected limit with reset time")
		}
		
		// Test with second timestamp (should remain as is)
		text2 := "Claude AI usage limit reached|1704106800"
		limit2 := parser.parseTextContent(text2, log, "claude-3-sonnet")
		
		if limit2 == nil || limit2.ResetTime == nil {
			t.Fatal("Expected limit with reset time")
		}
		
		// Both should result in the same reset time (converted to seconds)
		if *limit1.ResetTime != *limit2.ResetTime {
			t.Errorf("Expected same reset time after conversion: %d vs %d", *limit1.ResetTime, *limit2.ResetTime)
		}
		
		// Verify the reset time is reasonable (should be 1704106800)
		if *limit1.ResetTime != 1704106800 {
			t.Errorf("Expected reset time 1704106800, got %d", *limit1.ResetTime)
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

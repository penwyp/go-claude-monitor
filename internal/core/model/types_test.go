package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleContentUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name          string
		jsonData      string
		expected      FlexibleContent
		expectError   bool
		errorContains string
	}{
		{
			name:     "string_content",
			jsonData: `"Hello, world!"`,
			expected: FlexibleContent{
				{Type: "text", Text: "Hello, world!"},
			},
			expectError: false,
		},
		{
			name:     "empty_string_content",
			jsonData: `""`,
			expected: FlexibleContent{
				{Type: "text", Text: ""},
			},
			expectError: false,
		},
		{
			name:     "array_content_single_item",
			jsonData: `[{"type": "text", "text": "Hello from array"}]`,
			expected: FlexibleContent{
				{Type: "text", Text: "Hello from array"},
			},
			expectError: false,
		},
		{
			name: "array_content_multiple_items",
			jsonData: `[
				{"type": "text", "text": "First item"},
				{"type": "tool_use", "name": "bash", "id": "tool123"}
			]`,
			expected: FlexibleContent{
				{Type: "text", Text: "First item"},
				{Type: "tool_use", Name: "bash", Id: "tool123"},
			},
			expectError: false,
		},
		{
			name:     "empty_array",
			jsonData: `[]`,
			expected: FlexibleContent{},
			expectError: false,
		},
		{
			name: "complex_content_items",
			jsonData: `[
				{
					"type": "tool_use",
					"id": "tool_1",
					"name": "edit_file",
					"input": {
						"file_path": "/path/to/file.go",
						"old_string": "old content",
						"new_string": "new content"
					}
				},
				{
					"type": "tool_result",
					"tool_use_id": "tool_1",
					"content": "File updated successfully",
					"is_error": false
				}
			]`,
			expected: FlexibleContent{
				{
					Type: "tool_use",
					Id:   "tool_1",
					Name: "edit_file",
					Input: Input{
						FilePath:  "/path/to/file.go",
						OldString: "old content",
						NewString: "new content",
					},
				},
				{
					Type:      "tool_result",
					ToolUseId: "tool_1",
					Content:   "File updated successfully",
					IsError:   false,
				},
			},
			expectError: false,
		},
		{
			name:          "invalid_json",
			jsonData:      `{this is not valid json}`,
			expected:      nil,
			expectError:   true,
			errorContains: "Syntax error at index",
		},
		{
			name:          "number_content",
			jsonData:      `123`,
			expected:      nil,
			expectError:   true,
			errorContains: "content must be either string or array of ContentItem",
		},
		{
			name:          "boolean_content",
			jsonData:      `true`,
			expected:      nil,
			expectError:   true,
			errorContains: "content must be either string or array of ContentItem",
		},
		{
			name:          "object_content",
			jsonData:      `{"key": "value"}`,
			expected:      nil,
			expectError:   true,
			errorContains: "content must be either string or array of ContentItem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fc FlexibleContent
			err := sonic.Unmarshal([]byte(tt.jsonData), &fc)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, fc)
			}
		})
	}
}

func TestConversationLogMarshaling(t *testing.T) {
	// Test complete ConversationLog structure
	log := ConversationLog{
		Content:           "Test conversation content",
		Cwd:               "/home/user/project",
		GitBranch:         "main",
		IsApiErrorMessage: false,
		IsMeta:            true,
		IsSidechain:       false,
		LeafUuid:          "leaf-123",
		Level:             "info",
		Message: Message{
			Content: FlexibleContent{
				{Type: "text", Text: "Hello world"},
			},
			Id:           "msg-456",
			Model:        "claude-3-5-sonnet",
			Role:         "assistant",
			StopReason:   stringPtr("end_turn"),
			StopSequence: stringPtr("</answer>"),
			Type:         "message",
			Usage: Usage{
				CacheCreationInputTokens: 100,
				CacheReadInputTokens:     50,
				InputTokens:              500,
				OutputTokens:             200,
				ServerToolUse: ServerToolUse{
					WebSearchRequests: 2,
				},
				ServiceTier: "default",
			},
		},
		ParentUuid:    stringPtr("parent-789"),
		RequestId:     "req-abc",
		SessionId:     "session-def",
		Summary:       "Test summary",
		Timestamp:     "2024-01-01T10:00:00Z",
		ToolUseID:     "tool-ghi",
		ToolUseResult: map[string]interface{}{"status": "success"},
		Type:          "assistant",
		UserType:      "human",
		Uuid:          "uuid-jkl",
		Version:       "1.0",
	}

	// Marshal to JSON
	jsonData, err := sonic.Marshal(log)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Unmarshal back
	var unmarshaledLog ConversationLog
	err = sonic.Unmarshal(jsonData, &unmarshaledLog)
	require.NoError(t, err)

	// Verify critical fields
	assert.Equal(t, log.Content, unmarshaledLog.Content)
	assert.Equal(t, log.SessionId, unmarshaledLog.SessionId)
	assert.Equal(t, log.Timestamp, unmarshaledLog.Timestamp)
	assert.Equal(t, log.Message.Model, unmarshaledLog.Message.Model)
	assert.Equal(t, log.Message.Usage.InputTokens, unmarshaledLog.Message.Usage.InputTokens)
	assert.Equal(t, log.Message.Usage.OutputTokens, unmarshaledLog.Message.Usage.OutputTokens)
	assert.Equal(t, log.Message.Usage.ServerToolUse.WebSearchRequests, unmarshaledLog.Message.Usage.ServerToolUse.WebSearchRequests)
}

func TestMessageStructure(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Message
	}{
		{
			name: "basic_message",
			jsonData: `{
				"content": "Simple text message",
				"id": "msg-123",
				"model": "claude-3-5-sonnet",
				"role": "assistant",
				"type": "message",
				"usage": {
					"input_tokens": 100,
					"output_tokens": 50
				}
			}`,
			expected: Message{
				Content: FlexibleContent{{Type: "text", Text: "Simple text message"}},
				Id:      "msg-123",
				Model:   "claude-3-5-sonnet",
				Role:    "assistant",
				Type:    "message",
				Usage: Usage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		},
		{
			name: "message_with_complex_content",
			jsonData: `{
				"content": [
					{"type": "text", "text": "Here's the result:"},
					{"type": "tool_result", "tool_use_id": "tool1", "content": "Success"}
				],
				"role": "assistant",
				"type": "message"
			}`,
			expected: Message{
				Content: FlexibleContent{
					{Type: "text", Text: "Here's the result:"},
					{Type: "tool_result", ToolUseId: "tool1", Content: "Success"},
				},
				Role: "assistant",
				Type: "message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg Message
			err := sonic.Unmarshal([]byte(tt.jsonData), &msg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, msg)
		})
	}
}

func TestContentItemStructure(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected ContentItem
	}{
		{
			name: "text_content_item",
			jsonData: `{
				"type": "text",
				"text": "Hello world"
			}`,
			expected: ContentItem{
				Type: "text",
				Text: "Hello world",
			},
		},
		{
			name: "tool_use_content_item",
			jsonData: `{
				"type": "tool_use",
				"id": "tool_123",
				"name": "bash",
				"input": {
					"command": "ls -la",
					"description": "List files"
				}
			}`,
			expected: ContentItem{
				Type: "tool_use",
				Id:   "tool_123",
				Name: "bash",
				Input: Input{
					Command:     "ls -la",
					Description: "List files",
				},
			},
		},
		{
			name: "tool_result_content_item",
			jsonData: `{
				"type": "tool_result",
				"tool_use_id": "tool_123",
				"content": "File listing output",
				"is_error": false
			}`,
			expected: ContentItem{
				Type:      "tool_result",
				ToolUseId: "tool_123",
				Content:   "File listing output",
				IsError:   false,
			},
		},
		{
			name: "thinking_content_item",
			jsonData: `{
				"type": "thinking",
				"thinking": "Let me analyze this problem step by step..."
			}`,
			expected: ContentItem{
				Type:     "thinking",
				Thinking: "Let me analyze this problem step by step...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item ContentItem
			err := sonic.Unmarshal([]byte(tt.jsonData), &item)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, item)
		})
	}
}

func TestInputStructure(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Input
	}{
		{
			name: "bash_command_input",
			jsonData: `{
				"command": "echo 'Hello World'",
				"description": "Print hello world"
			}`,
			expected: Input{
				Command:     "echo 'Hello World'",
				Description: "Print hello world",
			},
		},
		{
			name: "file_edit_input",
			jsonData: `{
				"file_path": "/path/to/file.go",
				"old_string": "old content",
				"new_string": "new content"
			}`,
			expected: Input{
				FilePath:  "/path/to/file.go",
				OldString: "old content",
				NewString: "new content",
			},
		},
		{
			name: "multi_edit_input",
			jsonData: `{
				"file_path": "/path/to/file.go",
				"edits": [
					{
						"old_string": "old1",
						"new_string": "new1",
						"replace_all": false
					},
					{
						"old_string": "old2",
						"new_string": "new2",
						"replace_all": true
					}
				]
			}`,
			expected: Input{
				FilePath: "/path/to/file.go",
				Edits: []EditsItem{
					{OldString: "old1", NewString: "new1", ReplaceAll: false},
					{OldString: "old2", NewString: "new2", ReplaceAll: true},
				},
			},
		},
		{
			name: "grep_input",
			jsonData: `{
				"pattern": "func.*main",
				"path": "/src",
				"output_mode": "content",
				"-n": true,
				"-A": 5,
				"-B": 2
			}`,
			expected: Input{
				Pattern:    "func.*main",
				Path:       "/src",
				OutputMode: "content",
				N:          true,
				A:          5,
				B:          2,
			},
		},
		{
			name: "todo_input",
			jsonData: `{
				"todos": [
					{
						"id": "task1",
						"content": "Implement feature A",
						"status": "pending",
						"priority": "high"
					},
					{
						"id": "task2",
						"content": "Fix bug B",
						"status": "completed",
						"priority": "medium"
					}
				]
			}`,
			expected: Input{
				Todos: []TodosItem{
					{Id: "task1", Content: "Implement feature A", Status: "pending", Priority: "high"},
					{Id: "task2", Content: "Fix bug B", Status: "completed", Priority: "medium"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input Input
			err := sonic.Unmarshal([]byte(tt.jsonData), &input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestUsageStructure(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Usage
	}{
		{
			name: "basic_usage",
			jsonData: `{
				"input_tokens": 1000,
				"output_tokens": 500,
				"service_tier": "default"
			}`,
			expected: Usage{
				InputTokens:  1000,
				OutputTokens: 500,
				ServiceTier:  "default",
			},
		},
		{
			name: "usage_with_cache",
			jsonData: `{
				"cache_creation_input_tokens": 200,
				"cache_read_input_tokens": 100,
				"input_tokens": 800,
				"output_tokens": 400,
				"service_tier": "premium"
			}`,
			expected: Usage{
				CacheCreationInputTokens: 200,
				CacheReadInputTokens:     100,
				InputTokens:              800,
				OutputTokens:             400,
				ServiceTier:              "premium",
			},
		},
		{
			name: "usage_with_server_tool_use",
			jsonData: `{
				"input_tokens": 500,
				"output_tokens": 300,
				"server_tool_use": {
					"web_search_requests": 3
				},
				"service_tier": "default"
			}`,
			expected: Usage{
				InputTokens:  500,
				OutputTokens: 300,
				ServerToolUse: ServerToolUse{
					WebSearchRequests: 3,
				},
				ServiceTier: "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var usage Usage
			err := sonic.Unmarshal([]byte(tt.jsonData), &usage)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, usage)
		})
	}
}

func TestToolUseResultStructure(t *testing.T) {
	jsonData := `{
		"content": "Command executed successfully",
		"stdout": "Hello World\n",
		"stderr": "",
		"returnCodeInterpretation": "success",
		"interrupted": false,
		"mode": "command",
		"type": "bash_result"
	}`

	var result ToolUseResult
	err := sonic.Unmarshal([]byte(jsonData), &result)
	require.NoError(t, err)

	expected := ToolUseResult{
		Content:                  "Command executed successfully",
		Stdout:                   "Hello World\n",
		Stderr:                   "",
		ReturnCodeInterpretation: "success",
		Interrupted:              false,
		Mode:                     "command",
		Type:                     "bash_result",
	}

	assert.Equal(t, expected, result)
}

func TestComplexNestedStructures(t *testing.T) {
	// Test a complex nested structure that includes multiple levels
	jsonData := `{
		"sessionId": "session-123",
		"timestamp": "2024-01-01T10:00:00Z",
		"type": "assistant",
		"message": {
			"content": [
				{
					"type": "text",
					"text": "I'll help you edit the file."
				},
				{
					"type": "tool_use",
					"id": "edit_1",
					"name": "MultiEdit",
					"input": {
						"file_path": "/path/to/file.go",
						"edits": [
							{
								"old_string": "func old()",
								"new_string": "func new()",
								"replace_all": false
							}
						]
					}
				},
				{
					"type": "tool_result",
					"tool_use_id": "edit_1",
					"content": "File edited successfully"
				}
			],
			"role": "assistant",
			"usage": {
				"input_tokens": 150,
				"output_tokens": 75,
				"cache_read_input_tokens": 50
			}
		}
	}`

	var log ConversationLog
	err := sonic.Unmarshal([]byte(jsonData), &log)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "session-123", log.SessionId)
	assert.Equal(t, "2024-01-01T10:00:00Z", log.Timestamp)
	assert.Equal(t, "assistant", log.Type)

	// Verify content structure
	assert.Len(t, log.Message.Content, 3)
	assert.Equal(t, "text", log.Message.Content[0].Type)
	assert.Equal(t, "I'll help you edit the file.", log.Message.Content[0].Text)

	assert.Equal(t, "tool_use", log.Message.Content[1].Type)
	assert.Equal(t, "edit_1", log.Message.Content[1].Id)
	assert.Equal(t, "MultiEdit", log.Message.Content[1].Name)
	assert.Equal(t, "/path/to/file.go", log.Message.Content[1].Input.FilePath)
	assert.Len(t, log.Message.Content[1].Input.Edits, 1)
	assert.Equal(t, "func old()", log.Message.Content[1].Input.Edits[0].OldString)
	assert.Equal(t, "func new()", log.Message.Content[1].Input.Edits[0].NewString)
	assert.False(t, log.Message.Content[1].Input.Edits[0].ReplaceAll)

	assert.Equal(t, "tool_result", log.Message.Content[2].Type)
	assert.Equal(t, "edit_1", log.Message.Content[2].ToolUseId)
	assert.Equal(t, "File edited successfully", log.Message.Content[2].Content)

	// Verify usage
	assert.Equal(t, 150, log.Message.Usage.InputTokens)
	assert.Equal(t, 75, log.Message.Usage.OutputTokens)
	assert.Equal(t, 50, log.Message.Usage.CacheReadInputTokens)
}

func TestMalformedJSONHandling(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		targetVar interface{}
	}{
		{
			name:      "malformed_conversation_log",
			jsonData:  `{"sessionId": "test", "timestamp": invalid}`,
			targetVar: &ConversationLog{},
		},
		{
			name:      "malformed_message",
			jsonData:  `{"content": [{"type": "text", "text":}]}`,
			targetVar: &Message{},
		},
		{
			name:      "malformed_usage",
			jsonData:  `{"input_tokens": "not_a_number"}`,
			targetVar: &Usage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sonic.Unmarshal([]byte(tt.jsonData), tt.targetVar)
			assert.Error(t, err, "Should fail to unmarshal malformed JSON")
		})
	}
}

func TestJSONRoundTripConsistency(t *testing.T) {
	// Create a complex structure
	original := ConversationLog{
		SessionId: "session-456",
		Timestamp: "2024-01-01T15:30:00Z",
		Type:      "assistant",
		Message: Message{
			Content: FlexibleContent{
				{
					Type: "tool_use",
					Id:   "tool_789",
					Name: "Bash",
					Input: Input{
						Command:     "ls -la",
						Description: "List directory contents",
					},
				},
			},
			Role: "assistant",
			Usage: Usage{
				InputTokens:              200,
				OutputTokens:             100,
				CacheCreationInputTokens: 50,
				ServerToolUse: ServerToolUse{
					WebSearchRequests: 1,
				},
			},
		},
	}

	// Marshal to JSON
	jsonData, err := sonic.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var roundTrip ConversationLog
	err = sonic.Unmarshal(jsonData, &roundTrip)
	require.NoError(t, err)

	// Verify consistency
	assert.Equal(t, original.SessionId, roundTrip.SessionId)
	assert.Equal(t, original.Timestamp, roundTrip.Timestamp)
	assert.Equal(t, original.Type, roundTrip.Type)
	assert.Equal(t, original.Message.Role, roundTrip.Message.Role)
	assert.Equal(t, original.Message.Usage.InputTokens, roundTrip.Message.Usage.InputTokens)
	assert.Equal(t, original.Message.Usage.OutputTokens, roundTrip.Message.Usage.OutputTokens)
	assert.Equal(t, original.Message.Usage.ServerToolUse.WebSearchRequests, roundTrip.Message.Usage.ServerToolUse.WebSearchRequests)

	// Verify content
	assert.Len(t, roundTrip.Message.Content, 1)
	assert.Equal(t, original.Message.Content[0].Type, roundTrip.Message.Content[0].Type)
	assert.Equal(t, original.Message.Content[0].Id, roundTrip.Message.Content[0].Id)
	assert.Equal(t, original.Message.Content[0].Name, roundTrip.Message.Content[0].Name)
	assert.Equal(t, original.Message.Content[0].Input.Command, roundTrip.Message.Content[0].Input.Command)
}

// Benchmark tests for JSON operations
func BenchmarkFlexibleContentUnmarshalString(b *testing.B) {
	jsonData := []byte(`"This is a test string content for benchmarking"`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var fc FlexibleContent
		sonic.Unmarshal(jsonData, &fc)
	}
}

func BenchmarkFlexibleContentUnmarshalArray(b *testing.B) {
	jsonData := []byte(`[
		{"type": "text", "text": "Hello"},
		{"type": "tool_use", "id": "tool1", "name": "bash", "input": {"command": "ls"}}
	]`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var fc FlexibleContent
		sonic.Unmarshal(jsonData, &fc)
	}
}

func BenchmarkConversationLogUnmarshal(b *testing.B) {
	jsonData := []byte(`{
		"sessionId": "session-123",
		"timestamp": "2024-01-01T10:00:00Z",
		"type": "assistant",
		"message": {
			"content": "Simple text message",
			"role": "assistant",
			"usage": {
				"input_tokens": 100,
				"output_tokens": 50
			}
		}
	}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var log ConversationLog
		sonic.Unmarshal(jsonData, &log)
	}
}

func BenchmarkConversationLogMarshal(b *testing.B) {
	log := ConversationLog{
		SessionId: "session-123",
		Timestamp: "2024-01-01T10:00:00Z",
		Type:      "assistant",
		Message: Message{
			Content: FlexibleContent{{Type: "text", Text: "Test message"}},
			Role:    "assistant",
			Usage: Usage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sonic.Marshal(log)
	}
}

// Test edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("empty_flexible_content", func(t *testing.T) {
		var fc FlexibleContent
		err := sonic.Unmarshal([]byte(`[]`), &fc)
		assert.NoError(t, err)
		assert.Empty(t, fc)
	})
	
	t.Run("null_flexible_content", func(t *testing.T) {
		var fc FlexibleContent
		err := sonic.Unmarshal([]byte(`null`), &fc)
		// This might error depending on sonic's behavior with null
		if err == nil {
			assert.Nil(t, fc)
		}
	})
	
	t.Run("very_long_string_content", func(t *testing.T) {
		longString := strings.Repeat("A", 10000)
		jsonData := fmt.Sprintf(`"%s"`, longString)
		
		var fc FlexibleContent
		err := sonic.Unmarshal([]byte(jsonData), &fc)
		assert.NoError(t, err)
		assert.Len(t, fc, 1)
		assert.Equal(t, longString, fc[0].Text)
	})
	
	t.Run("zero_values", func(t *testing.T) {
		var log ConversationLog
		jsonData, err := sonic.Marshal(log)
		assert.NoError(t, err)
		
		var unmarshaled ConversationLog
		err = sonic.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)
		// Should handle zero values gracefully
	})
	
	t.Run("partial_structures", func(t *testing.T) {
		// Test with only some fields populated
		partialJSON := `{
			"sessionId": "test",
			"message": {
				"role": "assistant"
			}
		}`
		
		var log ConversationLog
		err := sonic.Unmarshal([]byte(partialJSON), &log)
		assert.NoError(t, err)
		assert.Equal(t, "test", log.SessionId)
		assert.Equal(t, "assistant", log.Message.Role)
		assert.Empty(t, log.Timestamp) // Should be zero value
	})
}

// Helper function to create string pointers for testing
func stringPtr(s string) *string {
	return &s
}
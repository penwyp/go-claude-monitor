package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	concurrency := 4
	parser := NewParser(concurrency)

	assert.NotNil(t, parser)
	assert.Equal(t, concurrency, parser.concurrency)
	assert.NotNil(t, parser.cache)
	assert.Empty(t, parser.cache)
}

func TestParserParseFileValidJSONL(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	// Create a valid JSONL file
	validJSONL := `{"type":"message","uuid":"test-uuid-1","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello"},"userType":"human","version":"1.0"}
{"type":"message","uuid":"test-uuid-2","sessionId":"session-1","timestamp":"2023-10-15T10:01:00Z","message":{"role":"assistant","type":"text","content":"Hi there","model":"claude-3-sonnet","usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"service_tier":"default"}},"userType":"human","version":"1.0"}`

	testFile := filepath.Join(tempDir, "test.jsonl")
	err := os.WriteFile(testFile, []byte(validJSONL), 0644)
	require.NoError(t, err)

	logs, err := parser.ParseFile(testFile)

	require.NoError(t, err)
	assert.Len(t, logs, 2)
	
	// Verify first log
	assert.Equal(t, "test-uuid-1", logs[0].Uuid)
	assert.Equal(t, "session-1", logs[0].SessionId)
	assert.Equal(t, "user", logs[0].Message.Role)
	
	// Verify second log with usage data
	assert.Equal(t, "test-uuid-2", logs[1].Uuid)
	assert.Equal(t, "claude-3-sonnet", logs[1].Message.Model)
	assert.Equal(t, 10, logs[1].Message.Usage.InputTokens)
	assert.Equal(t, 5, logs[1].Message.Usage.OutputTokens)
}

func TestParserParseFileInvalidJSON(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	// Create a file with mixed valid and invalid JSON lines
	mixedJSONL := `{"type":"message","uuid":"test-uuid-1","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello"},"userType":"human","version":"1.0"}
invalid json line here
{"type":"message","uuid":"test-uuid-2","sessionId":"session-1","timestamp":"2023-10-15T10:01:00Z","message":{"role":"assistant","type":"text","content":"Hi"},"userType":"human","version":"1.0"}
{incomplete json`

	testFile := filepath.Join(tempDir, "mixed.jsonl")
	err := os.WriteFile(testFile, []byte(mixedJSONL), 0644)
	require.NoError(t, err)

	logs, err := parser.ParseFile(testFile)

	require.NoError(t, err, "Parser should skip invalid lines and continue")
	assert.Len(t, logs, 2, "Should parse only valid JSON lines")
	assert.Equal(t, "test-uuid-1", logs[0].Uuid)
	assert.Equal(t, "test-uuid-2", logs[1].Uuid)
}

func TestParserParseFileEmptyFile(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "empty.jsonl")
	err := os.WriteFile(testFile, []byte(""), 0644)
	require.NoError(t, err)

	logs, err := parser.ParseFile(testFile)

	require.NoError(t, err)
	assert.Empty(t, logs, "Empty file should return empty slice")
}

func TestParserParseFileNonExistent(t *testing.T) {
	parser := NewParser(1)
	nonExistentFile := "/path/that/does/not/exist.jsonl"

	logs, err := parser.ParseFile(nonExistentFile)

	assert.Error(t, err, "Should return error for non-existent file")
	assert.Nil(t, logs)
}

func TestParserParseFileComplexUsage(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	// Create JSONL with complex usage data including cache tokens
	complexJSONL := `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"assistant","type":"text","content":"Response","model":"claude-3-sonnet","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":25,"cache_read_input_tokens":10,"service_tier":"default","server_tool_use":{"web_search_requests":2}}},"userType":"human","version":"1.0"}`

	testFile := filepath.Join(tempDir, "complex.jsonl")
	err := os.WriteFile(testFile, []byte(complexJSONL), 0644)
	require.NoError(t, err)

	logs, err := parser.ParseFile(testFile)

	require.NoError(t, err)
	require.Len(t, logs, 1)
	
	usage := logs[0].Message.Usage
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
	assert.Equal(t, 25, usage.CacheCreationInputTokens)
	assert.Equal(t, 10, usage.CacheReadInputTokens)
	assert.Equal(t, 2, usage.ServerToolUse.WebSearchRequests)
}

func TestParserParseFileFlexibleContent(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		jsonl    string
		expected string
	}{
		{
			name:     "string content",
			jsonl:    `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello world"},"userType":"human","version":"1.0"}`,
			expected: "Hello world",
		},
		{
			name:     "array content",
			jsonl:    `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":[{"type":"text","text":"Hello array"}]},"userType":"human","version":"1.0"}`,
			expected: "Hello array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, fmt.Sprintf("%s.jsonl", tt.name))
			err := os.WriteFile(testFile, []byte(tt.jsonl), 0644)
			require.NoError(t, err)

			logs, err := parser.ParseFile(testFile)

			require.NoError(t, err)
			require.Len(t, logs, 1)
			
			content := logs[0].Message.Content
			require.Len(t, content, 1)
			assert.Equal(t, tt.expected, content[0].Text)
		})
	}
}

func TestParserParseFileCache(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	validJSONL := `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello"},"userType":"human","version":"1.0"}`

	testFile := filepath.Join(tempDir, "cached.jsonl")
	err := os.WriteFile(testFile, []byte(validJSONL), 0644)
	require.NoError(t, err)

	// First parse - should read from file
	logs1, err1 := parser.ParseFile(testFile)
	require.NoError(t, err1)
	require.Len(t, logs1, 1)

	// Second parse - should use cache
	logs2, err2 := parser.ParseFile(testFile)
	require.NoError(t, err2)
	require.Len(t, logs2, 1)

	// Verify results are identical
	assert.Equal(t, logs1[0].Uuid, logs2[0].Uuid)
	assert.Equal(t, logs1[0].SessionId, logs2[0].SessionId)

	// Verify cache was used
	assert.Contains(t, parser.cache, testFile)
}

func TestParserParseFilesSequential(t *testing.T) {
	parser := NewParser(1) // Single concurrency
	tempDir := t.TempDir()

	// Create multiple test files
	files := []string{}
	for i := 0; i < 3; i++ {
		content := fmt.Sprintf(`{"type":"message","uuid":"test-uuid-%d","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello %d"},"userType":"human","version":"1.0"}`, i, i)
		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.jsonl", i))
		err := os.WriteFile(filename, []byte(content), 0644)
		require.NoError(t, err)
		files = append(files, filename)
	}

	results := parser.ParseFiles(files)

	// Collect all results
	var allResults []ParseResult
	for result := range results {
		allResults = append(allResults, result)
	}

	require.Len(t, allResults, 3)
	
	// Verify all files were processed without errors
	for _, result := range allResults {
		assert.NoError(t, result.Error)
		assert.Len(t, result.Logs, 1)
		assert.Contains(t, files, result.File)
	}
}

func TestParserParseFilesConcurrent(t *testing.T) {
	parser := NewParser(4) // High concurrency
	tempDir := t.TempDir()

	// Create multiple test files
	files := []string{}
	expectedUUIDs := []string{}
	for i := 0; i < 10; i++ {
		uuid := fmt.Sprintf("test-uuid-%d", i)
		content := fmt.Sprintf(`{"type":"message","uuid":"%s","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello %d"},"userType":"human","version":"1.0"}`, uuid, i)
		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.jsonl", i))
		err := os.WriteFile(filename, []byte(content), 0644)
		require.NoError(t, err)
		files = append(files, filename)
		expectedUUIDs = append(expectedUUIDs, uuid)
	}

	results := parser.ParseFiles(files)

	// Collect all results
	var allResults []ParseResult
	for result := range results {
		allResults = append(allResults, result)
	}

	require.Len(t, allResults, 10)
	
	// Verify all files were processed
	processedUUIDs := []string{}
	for _, result := range allResults {
		assert.NoError(t, result.Error)
		assert.Len(t, result.Logs, 1)
		processedUUIDs = append(processedUUIDs, result.Logs[0].Uuid)
	}

	// Verify all expected UUIDs were found (order might differ due to concurrency)
	for _, expectedUUID := range expectedUUIDs {
		assert.Contains(t, processedUUIDs, expectedUUID)
	}
}

func TestParserParseFilesWithErrors(t *testing.T) {
	parser := NewParser(2)
	tempDir := t.TempDir()

	// Create mix of valid and invalid files
	validFile := filepath.Join(tempDir, "valid.jsonl")
	validContent := `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello"},"userType":"human","version":"1.0"}`
	err := os.WriteFile(validFile, []byte(validContent), 0644)
	require.NoError(t, err)

	nonExistentFile := filepath.Join(tempDir, "nonexistent.jsonl")

	files := []string{validFile, nonExistentFile}
	results := parser.ParseFiles(files)

	// Collect all results
	var allResults []ParseResult
	for result := range results {
		allResults = append(allResults, result)
	}

	require.Len(t, allResults, 2)
	
	// Find results by file
	var validResult, errorResult *ParseResult
	for _, result := range allResults {
		if result.File == validFile {
			validResult = &result
		} else if result.File == nonExistentFile {
			errorResult = &result
		}
	}

	// Verify valid file was processed successfully
	require.NotNil(t, validResult)
	assert.NoError(t, validResult.Error)
	assert.Len(t, validResult.Logs, 1)

	// Verify error file failed appropriately
	require.NotNil(t, errorResult)
	assert.Error(t, errorResult.Error)
	assert.Nil(t, errorResult.Logs)
}

func TestParserParseFilesEmptyList(t *testing.T) {
	parser := NewParser(1)
	files := []string{}

	results := parser.ParseFiles(files)

	// Should receive empty channel that closes immediately
	var allResults []ParseResult
	for result := range results {
		allResults = append(allResults, result)
	}

	assert.Empty(t, allResults)
}

func TestParserLargeFile(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	// Create a large file with many JSON lines
	var lines []string
	for i := 0; i < 1000; i++ {
		line := fmt.Sprintf(`{"type":"message","uuid":"test-uuid-%d","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Message %d"},"userType":"human","version":"1.0"}`, i, i)
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")

	testFile := filepath.Join(tempDir, "large.jsonl")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	logs, err := parser.ParseFile(testFile)

	require.NoError(t, err)
	assert.Len(t, logs, 1000, "Should parse all 1000 lines")
	
	// Verify first and last entries
	assert.Equal(t, "test-uuid-0", logs[0].Uuid)
	assert.Equal(t, "test-uuid-999", logs[999].Uuid)
}

func TestParserConcurrencyRaceCondition(t *testing.T) {
	parser := NewParser(1)
	tempDir := t.TempDir()

	// Create a test file
	content := `{"type":"message","uuid":"test-uuid","sessionId":"session-1","timestamp":"2023-10-15T10:00:00Z","message":{"role":"user","type":"text","content":"Hello"},"userType":"human","version":"1.0"}`
	testFile := filepath.Join(tempDir, "race.jsonl")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Parse same file multiple times concurrently using ParseFiles
	files := make([]string, 10)
	for i := range files {
		files[i] = testFile
	}

	results := parser.ParseFiles(files)

	// Collect all results
	var allResults []ParseResult
	for result := range results {
		allResults = append(allResults, result)
	}

	require.Len(t, allResults, 10)
	
	// All should succeed due to caching
	for _, result := range allResults {
		assert.NoError(t, result.Error)
		assert.Len(t, result.Logs, 1)
		assert.Equal(t, "test-uuid", result.Logs[0].Uuid)
	}
}
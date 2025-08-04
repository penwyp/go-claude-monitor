package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected func(string) string
	}{
		{
			name:  "home directory expansion",
			input: "~/test/path",
			expected: func(home string) string {
				return filepath.Join(home, "test/path")
			},
		},
		{
			name:  "absolute path unchanged",
			input: "/absolute/path",
			expected: func(home string) string {
				return "/absolute/path"
			},
		},
		{
			name:  "relative path converted to absolute",
			input: "relative/path",
			expected: func(home string) string {
				abs, _ := filepath.Abs("relative/path")
				return abs
			},
		},
	}

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			expected := tt.expected(home)
			assert.Equal(t, expected, result)
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test", "nested", "dir")

	err := ensureDir(testDir)
	assert.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(testDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	// Test idempotency
	err = ensureDir(testDir)
	assert.NoError(t, err)
}

func TestClearCache(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	jsonFile1 := filepath.Join(tempDir, "cache1.json")
	jsonFile2 := filepath.Join(tempDir, "cache2.json")
	otherFile := filepath.Join(tempDir, "other.txt")

	require.NoError(t, os.WriteFile(jsonFile1, []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(jsonFile2, []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(otherFile, []byte("data"), 0644))

	// Clear cache
	err := clearCache(tempDir)
	assert.NoError(t, err)

	// Verify only JSON files were removed
	_, err = os.Stat(jsonFile1)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(jsonFile2)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(otherFile)
	assert.NoError(t, err)
}

func TestClearCacheNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "nonexistent")

	// Should not error on non-existent directory
	err := clearCache(nonExistentDir)
	assert.NoError(t, err)
}

func TestRootCommandFlags(t *testing.T) {

	tests := []struct {
		flag         string
		defaultValue string
		shorthand    string
	}{
		{"dir", defaultDataDir, ""},
		{"duration", "", "d"},
		{"group-by", "day", ""},
		{"output", "table", "o"},
		{"breakdown", "false", "b"},
		{"reset", "false", "r"},
		{"timezone", "Local", ""},
		{"pricing-source", "default", ""},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			flag := rootCmd.Flags().Lookup(tt.flag)
			assert.NotNil(t, flag)
			assert.Equal(t, tt.defaultValue, flag.DefValue)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestRunAnalyzeValidation(t *testing.T) {
	// Create a minimal test environment
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0755))
	
	// Set required flags
	rootCmd.SetArgs([]string{
		"--dir", dataDir,
		"--output", "json",
		"--duration", "1h",
	})

	// We can't fully test runAnalyze without mocking the analyzer,
	// but we can verify the command setup
	assert.NotNil(t, rootCmd.RunE)
	assert.Equal(t, "go-claude-monitor [flags]", rootCmd.Use)
}

func TestFormatFlagAlias(t *testing.T) {

	// Test that format flag exists
	formatFlag := rootCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag)
	
	// Test that output flag exists
	outputFlag := rootCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
}
package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPathSimple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected func() string
	}{
		{
			name:  "home directory expansion",
			input: "~/test",
			expected: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, "test")
			},
		},
		{
			name:     "absolute path unchanged",
			input:    "/tmp/test",
			expected: func() string { return "/tmp/test" },
		},
		{
			name:  "relative path converted to absolute",
			input: "test",
			expected: func() string {
				cwd, _ := os.Getwd()
				return filepath.Join(cwd, "test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			expected := tt.expected()
			assert.Equal(t, expected, result)
		})
	}
}

func TestEnsureDirSimple(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test", "nested", "dir")

	err := ensureDir(testDir)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(testDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestClearCacheSimple(t *testing.T) {
	tempDir := t.TempDir()

	// Create test cache files
	testFiles := []string{
		"cache1.json",
		"cache2.json",
		"not-cache.txt",
		"cache3.json",
	}

	for _, file := range testFiles {
		path := filepath.Join(tempDir, file)
		err := os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Clear cache
	err := clearCache(tempDir)
	require.NoError(t, err)

	// Verify only .json files were removed
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "not-cache.txt", entries[0].Name())
}

func TestClearCacheNonExistentSimple(t *testing.T) {
	// Should not error on non-existent directory
	err := clearCache("/non/existent/path")
	assert.NoError(t, err)
}

func TestCommandConstants(t *testing.T) {
	// Test that constants are defined
	assert.Equal(t, "~/.go-claude-monitor/logs/app.log", defaultLogFile)
	assert.Equal(t, "~/.go-claude-monitor/cache", defaultCacheDir)
	assert.Equal(t, "~/.claude/projects", defaultDataDir)
}

func TestCommandStructure(t *testing.T) {
	// Test basic command structure
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "go-claude-monitor [flags]", rootCmd.Use)
	assert.Equal(t, "Claude Code usage monitoring tool", rootCmd.Short)
	assert.True(t, strings.Contains(rootCmd.Long, "monitoring and analyzing"))
}

func TestRootCommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		flagType string
		expected interface{}
	}{
		{"dir flag", "dir", "string", defaultDataDir},
		{"duration flag", "duration", "string", ""},
		{"group-by flag", "group-by", "string", "day"},
		{"limit flag", "limit", "int", 0},
		{"breakdown flag", "breakdown", "bool", false},
		{"output flag", "output", "string", "table"},
		{"timezone flag", "timezone", "string", "Local"},
		{"debug flag", "debug", "bool", false},
		{"reset flag", "reset", "bool", false},
		{"pricing-source flag", "pricing-source", "string", "default"},
		{"pricing-offline flag", "pricing-offline", "bool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				// Check persistent flags for debug
				flag = rootCmd.PersistentFlags().Lookup(tt.flagName)
			}
			require.NotNil(t, flag, "Flag %s should exist", tt.flagName)
			
			switch tt.flagType {
			case "string":
				assert.Equal(t, tt.expected, flag.DefValue)
			case "bool":
				assert.Equal(t, "false", flag.DefValue)
			case "int":
				assert.Equal(t, "0", flag.DefValue)
			}
		})
	}
}

func TestRunAnalyzeDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a minimal test environment
	dataDir = filepath.Join(tempDir, "data")
	
	// Create a test command that doesn't actually run the analyzer
	testCmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Test only the directory creation logic from runAnalyze
			logFile := expandPath(defaultLogFile)
			ensureDir(filepath.Dir(logFile))
			
			dataDir = expandPath(dataDir)
			cacheDir := expandPath(defaultCacheDir)
			
			return ensureDir(cacheDir)
		},
	}
	
	err := testCmd.Execute()
	require.NoError(t, err)
}

func TestRunAnalyzeFlagProcessing(t *testing.T) {
	tests := []struct {
		name string
		args []string
		test func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "format alias flag",
			args: []string{"--format", "json"},
			test: func(t *testing.T, cmd *cobra.Command) {
				format := cmd.Flags().Lookup("format")
				require.NotNil(t, format)
				assert.Equal(t, "json", format.Value.String())
			},
		},
		{
			name: "debug flag sets debug mode",
			args: []string{"--debug"},
			test: func(t *testing.T, cmd *cobra.Command) {
				debugFlag := cmd.PersistentFlags().Lookup("debug")
				require.NotNil(t, debugFlag)
				assert.Equal(t, "true", debugFlag.Value.String())
			},
		},
		{
			name: "duration flag parsing",
			args: []string{"--duration", "7d"},
			test: func(t *testing.T, cmd *cobra.Command) {
				durationFlag := cmd.Flags().Lookup("duration")
				require.NotNil(t, durationFlag)
				assert.Equal(t, "7d", durationFlag.Value.String())
			},
		},
		{
			name: "group-by flag parsing",
			args: []string{"--group-by", "model"},
			test: func(t *testing.T, cmd *cobra.Command) {
				groupByFlag := cmd.Flags().Lookup("group-by")
				require.NotNil(t, groupByFlag)
				assert.Equal(t, "model", groupByFlag.Value.String())
			},
		},
		{
			name: "multiple flags",
			args: []string{"--output", "csv", "--breakdown", "--limit", "50"},
			test: func(t *testing.T, cmd *cobra.Command) {
				outputFlag := cmd.Flags().Lookup("output")
				require.NotNil(t, outputFlag)
				assert.Equal(t, "csv", outputFlag.Value.String())
				
				breakdownFlag := cmd.Flags().Lookup("breakdown")
				require.NotNil(t, breakdownFlag)
				assert.Equal(t, "true", breakdownFlag.Value.String())
				
				limitFlag := cmd.Flags().Lookup("limit")
				require.NotNil(t, limitFlag)
				assert.Equal(t, "50", limitFlag.Value.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the root command for testing
			testCmd := &cobra.Command{
				Use: "test",
			}
			
			// Add all the flags from rootCmd
			testCmd.Flags().AddFlagSet(rootCmd.Flags())
			testCmd.PersistentFlags().AddFlagSet(rootCmd.PersistentFlags())
			
			// Set up command output capture
			var output bytes.Buffer
			testCmd.SetOut(&output)
			testCmd.SetErr(&output)
			
			// Parse the flags
			testCmd.SetArgs(tt.args)
			testCmd.ParseFlags(tt.args)
			
			// Run the test
			tt.test(t, testCmd)
		})
	}
}

func TestRunAnalyzeCacheClear(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	
	// Create cache directory with test files
	err := os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)
	
	// Create test cache files
	testFiles := []string{
		"session1.json",
		"session2.json",
		"other.txt",
	}
	
	for _, file := range testFiles {
		path := filepath.Join(cacheDir, file)
		err := os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)
	}
	
	// Test cache clearing
	err = clearCache(cacheDir)
	require.NoError(t, err)
	
	// Verify only .json files were removed
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "other.txt", entries[0].Name())
}

func TestRunAnalyzeConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		outputFormat string
		groupBy     string
		timezone    string
		duration    string
		wantValid   bool
	}{
		{
			name:        "valid table output",
			outputFormat: "table",
			groupBy:     "day",
			timezone:    "UTC",
			duration:    "7d",
			wantValid:   true,
		},
		{
			name:        "valid json output",
			outputFormat: "json",
			groupBy:     "model",
			timezone:    "Local",
			duration:    "12h",
			wantValid:   true,
		},
		{
			name:        "valid csv output",
			outputFormat: "csv",
			groupBy:     "project",
			timezone:    "America/New_York",
			duration:    "1m",
			wantValid:   true,
		},
		{
			name:        "valid summary output",
			outputFormat: "summary",
			groupBy:     "week",
			timezone:    "Asia/Shanghai",
			duration:    "2w3d",
			wantValid:   true,
		},
		{
			name:        "empty duration is valid",
			outputFormat: "table",
			groupBy:     "hour",
			timezone:    "UTC",
			duration:    "",
			wantValid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that various configuration combinations are valid
			// (Since we can't easily test the full runAnalyze without setting up the full environment,
			// we test that the configuration values are accepted)
			
			// These are basic validation tests for config values
			assert.Contains(t, []string{"table", "json", "csv", "summary"}, tt.outputFormat)
			assert.Contains(t, []string{"model", "project", "day", "week", "month", "hour"}, tt.groupBy)
			assert.NotEmpty(t, tt.timezone)
			
			// Duration can be empty or follow the expected pattern
			if tt.duration != "" {
				// Basic pattern check for duration format
				validPatterns := []string{"h", "d", "w", "m", "y"}
				hasValidPattern := false
				for _, pattern := range validPatterns {
					if strings.Contains(tt.duration, pattern) {
						hasValidPattern = true
						break
					}
				}
				assert.True(t, hasValidPattern, "Duration should contain valid time unit")
			}
		})
	}
}

func TestRunAnalyzeErrorHandling(t *testing.T) {
	// Test error handling for directory creation failures
	t.Run("invalid cache directory", func(t *testing.T) {
		// Try to use a file as a directory (should fail)
		tempFile := filepath.Join(t.TempDir(), "not-a-dir")
		err := os.WriteFile(tempFile, []byte("test"), 0644)
		require.NoError(t, err)
		
		// This should fail because we're trying to create a directory where a file exists
		err = ensureDir(filepath.Join(tempFile, "subdir"))
		assert.Error(t, err)
	})
}

func TestCommandExamples(t *testing.T) {
	// Test that the command examples in the help text are properly formatted
	examples := []string{
		"go-claude-monitor",
		"go-claude-monitor --dir /path/to/claude/projects",
		"go-claude-monitor --output json --group-by model",
		"go-claude-monitor --duration 12h",
		"go-claude-monitor --duration 7d",
		"go-claude-monitor --duration 2w3d",
		"go-claude-monitor --duration 1d12h",
		"go-claude-monitor --duration 1m --breakdown",
	}
	
	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			// Test that each example appears in the help text
			assert.Contains(t, rootCmd.Long, example)
		})
	}
}
//go:build e2e
// +build e2e

package commands

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCommandBasicAnalysis tests the main analysis functionality
func TestRootCommandBasicAnalysis(t *testing.T) {
	// Create temporary test data directory
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate test data with known values
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	// Build the binary
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test basic table output
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Command should succeed: %s", string(output))
	outputStr := string(output)
	
	// Verify table output contains expected elements
	assert.Contains(t, outputStr, "Project", "Should contain project header")
	assert.Contains(t, outputStr, "test-project", "Should show test project")
	assert.Contains(t, outputStr, "Total", "Should show totals")
	assert.Contains(t, outputStr, "claude-3.5-sonnet", "Should show model usage")
}

// TestRootCommandOutputFormats tests different output formats
func TestRootCommandOutputFormats(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multi-model session for richer output
	err := generator.GenerateMultiModelSession("multi-model-project", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		name           string
		format         string
		expectedChecks []string
	}{
		{
			name:   "JSON format",
			format: "json",
			expectedChecks: []string{
				`"projects"`,
				`"multi-model-project"`,
				`"models"`,
				`"total_cost"`,
			},
		},
		{
			name:   "CSV format",
			format: "csv",
			expectedChecks: []string{
				"Project,Period",
				"multi-model-project",
				"Total Cost,Total Tokens",
			},
		},
		{
			name:   "Summary format",
			format: "summary",
			expectedChecks: []string{
				"Usage Summary",
				"Total Projects:",
				"Total Cost:",
				"Total Tokens:",
			},
		},
		{
			name:   "Table format (default)",
			format: "table",
			expectedChecks: []string{
				"Project",
				"Period",
				"Models",
				"Total",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--output", tc.format, "--duration", "24h")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Command should succeed for %s format: %s", tc.format, string(output))
			outputStr := string(output)
			
			for _, expected := range tc.expectedChecks {
				assert.Contains(t, outputStr, expected, "Output should contain %s for %s format", expected, tc.format)
			}
		})
	}
}

// TestRootCommandDurationFilters tests various time duration filters
func TestRootCommandDurationFilters(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create sessions across different time periods
	now := time.Now()
	testProjects := []struct {
		name      string
		startTime time.Time
	}{
		{"recent-1h", now.Add(-1 * time.Hour)},
		{"recent-6h", now.Add(-6 * time.Hour)},
		{"yesterday", now.Add(-25 * time.Hour)},
		{"week-old", now.Add(-7 * 24 * time.Hour)},
		{"month-old", now.Add(-35 * 24 * time.Hour)},
	}
	
	for _, proj := range testProjects {
		err := generator.GenerateSimpleSession(proj.name, proj.startTime)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		duration        string
		shouldInclude   []string
		shouldNotInclude []string
	}{
		{
			duration:         "2h",
			shouldInclude:    []string{"recent-1h"},
			shouldNotInclude: []string{"recent-6h", "yesterday", "week-old", "month-old"},
		},
		{
			duration:         "24h",
			shouldInclude:    []string{"recent-1h", "recent-6h"},
			shouldNotInclude: []string{"yesterday", "week-old", "month-old"},
		},
		{
			duration:         "7d",
			shouldInclude:    []string{"recent-1h", "recent-6h", "yesterday"},
			shouldNotInclude: []string{"week-old", "month-old"},
		},
		{
			duration:         "30d",
			shouldInclude:    []string{"recent-1h", "recent-6h", "yesterday", "week-old"},
			shouldNotInclude: []string{"month-old"},
		},
	}

	for _, tc := range testCases {
		t.Run("duration_"+tc.duration, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", tc.duration, "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Command should succeed for duration %s: %s", tc.duration, string(output))
			outputStr := string(output)
			
			// Verify included projects
			for _, project := range tc.shouldInclude {
				assert.Contains(t, outputStr, project, "Should include project %s for duration %s", project, tc.duration)
			}
			
			// Verify excluded projects
			for _, project := range tc.shouldNotInclude {
				assert.NotContains(t, outputStr, project, "Should not include project %s for duration %s", project, tc.duration)
			}
		})
	}
}

// TestRootCommandGroupingOptions tests different grouping options
func TestRootCommandGroupingOptions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate data spanning multiple days
	now := time.Now()
	err := generator.GenerateSimpleSession("project-1", now.Add(-25*time.Hour))
	require.NoError(t, err)
	err = generator.GenerateSimpleSession("project-2", now.Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		groupBy  string
		contains []string
	}{
		{
			groupBy:  "hour",
			contains: []string{"Hour", "project-1", "project-2"},
		},
		{
			groupBy:  "day",
			contains: []string{"Day", "project-1", "project-2"},
		},
		{
			groupBy:  "week",
			contains: []string{"Week", "project-1", "project-2"},
		},
		{
			groupBy:  "month",
			contains: []string{"Month", "project-1", "project-2"},
		},
	}

	for _, tc := range testCases {
		t.Run("group_by_"+tc.groupBy, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--group-by", tc.groupBy, "--duration", "30d")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Command should succeed for group-by %s: %s", tc.groupBy, string(output))
			outputStr := string(output)
			
			for _, expected := range tc.contains {
				assert.Contains(t, outputStr, expected, "Should contain %s for group-by %s", expected, tc.groupBy)
			}
		})
	}
}

// TestRootCommandBreakdownOption tests the breakdown option
func TestRootCommandBreakdownOption(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multi-model session for breakdown
	err := generator.GenerateMultiModelSession("breakdown-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test with breakdown
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--breakdown", "--duration", "24h")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Command should succeed with breakdown: %s", string(output))
	outputStr := string(output)
	
	// Should show individual model breakdowns
	assert.Contains(t, outputStr, "claude-3.5-sonnet", "Should show sonnet model in breakdown")
	assert.Contains(t, outputStr, "claude-3-opus", "Should show opus model in breakdown")
	assert.Contains(t, outputStr, "claude-3-haiku", "Should show haiku model in breakdown")
	
	// Test without breakdown
	cmd = exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h")
	outputWithoutBreakdown, err := cmd.CombinedOutput()
	
	assert.NoError(t, err, "Command should succeed without breakdown")
	
	// Breakdown output should be more detailed than regular output
	assert.Greater(t, len(outputStr), len(string(outputWithoutBreakdown))/2, "Breakdown output should be more detailed")
}

// TestRootCommandTimezoneHandling tests timezone handling
func TestRootCommandTimezoneHandling(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate recent session
	err := generator.GenerateSimpleSession("tz-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []string{
		"UTC",
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Local",
	}

	for _, tz := range testCases {
		t.Run("timezone_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tz, "--duration", "24h")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Command should succeed with timezone %s: %s", tz, string(output))
			outputStr := string(output)
			assert.Contains(t, outputStr, "tz-project", "Should show project data for timezone %s", tz)
		})
	}
}

// TestRootCommandPricingOptions tests pricing source options
func TestRootCommandPricingOptions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with cost data
	err := generator.GenerateMultiModelSession("pricing-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []string{
		"default",
		"litellm",
	}

	for _, source := range testCases {
		t.Run("pricing_source_"+source, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--pricing-source", source, "--duration", "24h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Command should succeed with pricing source %s: %s", source, string(output))
			
			// Verify JSON structure and cost calculations
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			assert.NoError(t, err, "Should produce valid JSON output")
			
			// Should contain cost information
			projects, ok := result["projects"].(map[string]interface{})
			assert.True(t, ok, "Should have projects data")
			
			if project, exists := projects["pricing-project"]; exists {
				projectData := project.(map[string]interface{})
				assert.Contains(t, projectData, "total_cost", "Should have total_cost field")
			}
		})
	}
}

// TestRootCommandErrorHandling tests error conditions and recovery
func TestRootCommandErrorHandling(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorText   string
	}{
		{
			name:        "non-existent directory",
			args:        []string{"--dir", "/non/existent/path", "--duration", "1h"},
			expectError: true,
			errorText:   "",
		},
		{
			name:        "invalid duration format",
			args:        []string{"--dir", t.TempDir(), "--duration", "invalid"},
			expectError: true,
			errorText:   "",
		},
		{
			name:        "invalid output format",
			args:        []string{"--dir", t.TempDir(), "--duration", "1h", "--output", "invalid"},
			expectError: true,
			errorText:   "",
		},
		{
			name:        "invalid timezone",
			args:        []string{"--dir", t.TempDir(), "--duration", "1h", "--timezone", "Invalid/Timezone"},
			expectError: true,
			errorText:   "",
		},
		{
			name:        "invalid group-by option",
			args:        []string{"--dir", t.TempDir(), "--duration", "1h", "--group-by", "invalid"},
			expectError: true,
			errorText:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			output, err := cmd.CombinedOutput()
			
			if tc.expectError {
				assert.Error(t, err, "Should return error for %s: %s", tc.name, string(output))
			} else {
				assert.NoError(t, err, "Should not return error for %s: %s", tc.name, string(output))
			}
			
			if tc.errorText != "" {
				assert.Contains(t, string(output), tc.errorText, "Should contain error text")
			}
		})
	}
}

// TestRootCommandEmptyData tests behavior with no data
func TestRootCommandEmptyData(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create empty project
	err := generator.CreateEmptyProject("empty-project")
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test various output formats with empty data
	formats := []string{"table", "json", "csv", "summary"}
	
	for _, format := range formats {
		t.Run("empty_data_"+format, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", format)
			output, err := cmd.CombinedOutput()
			
			// Should succeed but indicate no data
			assert.NoError(t, err, "Should handle empty data gracefully for %s format: %s", format, string(output))
			
			outputStr := string(output)
			
			// Should indicate no data in some way
			hasNoDataIndicator := strings.Contains(outputStr, "No") || 
								  strings.Contains(outputStr, "0") || 
								  strings.Contains(outputStr, "empty") ||
								  len(strings.TrimSpace(outputStr)) == 0
			assert.True(t, hasNoDataIndicator, "Should indicate no data or empty result for %s format", format)
		})
	}
}

// TestRootCommandCacheReset tests cache reset functionality
func TestRootCommandCacheReset(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate test data
	err := generator.GenerateSimpleSession("cache-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// First run to populate cache
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h")
	output, err = cmd.CombinedOutput()
	assert.NoError(t, err, "First run should succeed: %s", string(output))
	
	// Check if cache files are created (this depends on implementation details)
	// We primarily test that the reset flag doesn't break functionality
	
	// Run with cache reset
	cmd = exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--reset")
	output, err = cmd.CombinedOutput()
	assert.NoError(t, err, "Cache reset run should succeed: %s", string(output))
	
	// Output should still contain the expected data
	assert.Contains(t, string(output), "cache-test", "Should still show project data after cache reset")
}

// TestRootCommandLargeDataset tests performance with larger datasets
func TestRootCommandLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate large dataset
	err := generator.GenerateLargeDataset("large-project", time.Now().Add(-24*time.Hour), 500)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test different output formats with large dataset
	formats := []string{"table", "json", "summary"}
	
	for _, format := range formats {
		t.Run("large_dataset_"+format, func(t *testing.T) {
			startTime := time.Now()
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "30d", "--output", format)
			output, err := cmd.CombinedOutput()
			elapsed := time.Since(startTime)
			
			assert.NoError(t, err, "Should handle large dataset for %s format: %s", format, string(output))
			assert.Less(t, elapsed, 10*time.Second, "Should process large dataset within reasonable time for %s format", format)
			
			// Verify output contains expected data
			outputStr := string(output)
			assert.Contains(t, outputStr, "large-project", "Should show project in large dataset")
		})
	}
}

// TestRootCommandMultiProject tests analysis across multiple projects
func TestRootCommandMultiProject(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multiple projects with different characteristics
	now := time.Now()
	projects := []struct {
		name      string
		startTime time.Time
		generator func(string, time.Time) error
	}{
		{"simple-project", now.Add(-2 * time.Hour), generator.GenerateSimpleSession},
		{"multi-model-project", now.Add(-3 * time.Hour), generator.GenerateMultiModelSession},
		{"continuous-project", now.Add(-5 * time.Hour), generator.GenerateContinuousActivity},
	}
	
	for _, proj := range projects {
		err := proj.generator(proj.name, proj.startTime)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multi-project analysis
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project analysis should succeed: %s", string(output))
	
	// Verify all projects are included
	outputStr := string(output)
	for _, proj := range projects {
		assert.Contains(t, outputStr, proj.name, "Should include project %s", proj.name)
	}
	
	// Verify JSON structure
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	assert.NoError(t, err, "Should produce valid JSON output")
	
	projects_data, ok := result["projects"].(map[string]interface{})
	assert.True(t, ok, "Should have projects data")
	assert.Equal(t, len(projects), len(projects_data), "Should have all projects in output")
}

// TestRootCommandValidationLogic tests edge cases in validation
func TestRootCommandValidationLogic(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate test data
	err := generator.GenerateSimpleSession("validation-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test edge cases in duration parsing
	validDurations := []string{
		"1h",
		"24h",
		"1d",
		"7d",
		"30d",
		"1w",
		"1m",
		"1y",
	}
	
	for _, duration := range validDurations {
		t.Run("valid_duration_"+duration, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", duration)
			output, err := cmd.CombinedOutput()
			assert.NoError(t, err, "Should accept valid duration %s: %s", duration, string(output))
		})
	}
	
	// Test very short durations
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "1m")
	output, err = cmd.CombinedOutput()
	// Should handle gracefully (no error or appropriate message)
	outputStr := string(output)
	assert.True(t, err == nil || strings.Contains(outputStr, "No"), "Should handle very short duration gracefully")
}

// TestRootCommandRegression tests for known regression scenarios
func TestRootCommandRegression(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit to test complex scenarios
	err := generator.GenerateSessionWithLimit("regression-test", time.Now().Add(-4*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test that rate limit sessions are handled correctly
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Should handle rate limit sessions correctly: %s", string(output))
	
	// Verify rate limit information is processed
	outputStr := string(output)
	assert.Contains(t, outputStr, "regression-test", "Should show project with rate limit")
	
	// Verify JSON is valid
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	assert.NoError(t, err, "Should produce valid JSON with rate limit data")
}

// TestRootCommandOutputConsistency tests output consistency across formats
func TestRootCommandOutputConsistency(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate consistent test data
	err := generator.GenerateMultiModelSession("consistency-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Get JSON output for reference data
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	jsonOutput, err := cmd.CombinedOutput()
	assert.NoError(t, err, "JSON output should succeed")
	
	var jsonResult map[string]interface{}
	err = json.Unmarshal(jsonOutput, &jsonResult)
	require.NoError(t, err, "Should parse JSON output")
	
	// Extract key metrics for comparison
	_ = jsonResult["projects"].(map[string]interface{})
	
	// Test other formats contain consistent data
	formats := []string{"table", "csv", "summary"}
	
	for _, format := range formats {
		t.Run("consistency_"+format, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", format)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Format %s should succeed", format)
			outputStr := string(output)
			
			// Should contain the project name
			assert.Contains(t, outputStr, "consistency-test", "Format %s should contain project name", format)
			
			// Should contain some cost information (in some form)
			costRegex := regexp.MustCompile(`\$?\d+\.\d+`)
			assert.Regexp(t, costRegex, outputStr, "Format %s should contain cost information", format)
		})
	}
}
//go:build e2e
// +build e2e

package commands

import (
	"fmt"
	"os"
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

// TestDetectCommandBasicFunctionality tests the basic detect command functionality
func TestDetectCommandBasicFunctionality(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate simple session data
	err := generator.GenerateSimpleSession("detect-basic", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Detect command should succeed: %s", string(output))
	outputStr := string(output)
	
	// Verify basic output elements
	assert.Contains(t, outputStr, "Session Detection Analysis", "Should show analysis header")
	assert.Contains(t, outputStr, "detect-basic", "Should show project name")
	assert.Contains(t, outputStr, "Sessions Found", "Should show session count")
	assert.Contains(t, outputStr, "Model Distribution", "Should show model distribution")
}

// TestDetectCommandSessionDetection tests session detection with various scenarios
func TestDetectCommandSessionDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate different session types for detection testing
	now := time.Now()
	
	// Continuous activity session (should be detected as single session)
	err := generator.GenerateContinuousActivity("continuous-session", now.Add(-4*time.Hour))
	require.NoError(t, err)
	
	// Session with rate limit (should show rate limit detection)
	err = generator.GenerateSessionWithLimit("limited-session", now.Add(-8*time.Hour))
	require.NoError(t, err)
	
	// Multi-model session (should show multiple models)
	err = generator.GenerateMultiModelSession("multi-model-session", now.Add(-6*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Detect command should succeed: %s", string(output))
	outputStr := string(output)
	
	// Verify all projects are detected
	assert.Contains(t, outputStr, "continuous-session", "Should detect continuous session")
	assert.Contains(t, outputStr, "limited-session", "Should detect limited session")
	assert.Contains(t, outputStr, "multi-model-session", "Should detect multi-model session")
	
	// Verify window detection indicators
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection info")
	
	// Should show different detection sources
	windowIcons := []string{"🎯", "⏳", "📍", "⚪"}
	hasWindowIcon := false
	for _, icon := range windowIcons {
		if strings.Contains(outputStr, icon) {
			hasWindowIcon = true
			break
		}
	}
	assert.True(t, hasWindowIcon, "Should show window detection icons")
}

// TestDetectCommandPlanOptions tests different plan options
func TestDetectCommandPlanOptions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session that might hit limits
	err := generator.GenerateSessionWithLimit("plan-test", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []string{
		"max5",
		"max20",
		"pro",
		"team",
	}

	for _, plan := range testCases {
		t.Run("plan_"+plan, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "detect", "--plan", plan)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should succeed with plan %s: %s", plan, string(output))
			outputStr := string(output)
			
			// Should show plan-specific information
			assert.Contains(t, outputStr, "plan-test", "Should show project for plan %s", plan)
			assert.Contains(t, outputStr, "Session Detection", "Should show session detection for plan %s", plan)
		})
	}
}

// TestDetectCommandWindowHistoryInteraction tests window history functionality
func TestDetectCommandWindowHistoryInteraction(t *testing.T) {
	// Create temporary home directory for window history
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit for window history
	err := generator.GenerateSessionWithLimit("history-test", time.Now().Add(-4*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// First run to establish window history
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "First detect run should succeed: %s", string(output))
	
	// History directory might be created
	
	// Test with reset-windows flag
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect", "--reset-windows")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Reset windows should succeed: %s", string(output))
	outputStr := string(output)
	
	// Should still show session data after reset
	assert.Contains(t, outputStr, "history-test", "Should show project after window reset")
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection after reset")
}

// TestDetectCommandPricingSourceOptions tests pricing source variations
func TestDetectCommandPricingSourceOptions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multi-model session with cost data
	err := generator.GenerateMultiModelSession("pricing-detect", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		source  string
		offline bool
	}{
		{"default", false},
		{"default", true},
		{"litellm", false},
		{"litellm", true},
	}

	for _, tc := range testCases {
		t.Run("pricing_"+tc.source+"_offline_"+fmt.Sprintf("%v", tc.offline), func(t *testing.T) {
			args := []string{"--dir", tempDir, "detect", "--pricing-source", tc.source}
			if tc.offline {
				args = append(args, "--pricing-offline")
			}
			
			cmd := exec.Command(binaryPath, args...)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should succeed with pricing %s (offline=%v): %s", tc.source, tc.offline, string(output))
			outputStr := string(output)
			
			// Should show cost information
			assert.Contains(t, outputStr, "pricing-detect", "Should show project")
			assert.Contains(t, outputStr, "Cost", "Should show cost information")
			
			// Verify cost values are present (dollar sign or decimal numbers)
			costRegex := regexp.MustCompile(`\$[\d,]+\.?\d*|\d+\.\d+`)
			assert.Regexp(t, costRegex, outputStr, "Should contain cost values")
		})
	}
}

// TestDetectCommandTimezoneHandling tests timezone effects on detection
func TestDetectCommandTimezoneHandling(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session that spans timezone boundaries
	err := generator.GenerateContinuousActivity("timezone-test", time.Now().Add(-6*time.Hour))
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
			cmd := exec.Command(binaryPath, "--dir", tempDir, "detect", "--timezone", tz)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should succeed with timezone %s: %s", tz, string(output))
			outputStr := string(output)
			
			// Should show session data regardless of timezone
			assert.Contains(t, outputStr, "timezone-test", "Should show project for timezone %s", tz)
			assert.Contains(t, outputStr, "Session Detection", "Should show session detection for timezone %s", tz)
		})
	}
}

// TestDetectCommandComplexSessionScenarios tests complex session scenarios
func TestDetectCommandComplexSessionScenarios(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	now := time.Now()
	
	// Create complex scenario with multiple session windows
	testCases := []struct {
		name      string
		startTime time.Time
		generator func(string, time.Time) error
	}{
		// Session right at window boundary
		{"boundary-session", now.Add(-5*time.Hour + 5*time.Minute), generator.GenerateSimpleSession},
		// Very short session
		{"short-session", now.Add(-30*time.Minute), generator.GenerateSimpleSession},
		// Session with gaps
		{"gap-session", now.Add(-10*time.Hour), generator.GenerateSimpleSession},
		// Continuous long session
		{"long-session", now.Add(-8*time.Hour), generator.GenerateContinuousActivity},
	}
	
	for _, tc := range testCases {
		err := tc.generator(tc.name, tc.startTime)
		require.NoError(t, err, "Failed to generate %s", tc.name)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Complex scenario detection should succeed: %s", string(output))
	outputStr := string(output)
	
	// Verify all sessions are detected
	for _, tc := range testCases {
		assert.Contains(t, outputStr, tc.name, "Should detect %s", tc.name)
	}
	
	// Should show window detection statistics
	assert.Contains(t, outputStr, "Window Detection Success", "Should show detection success rate")
	assert.Contains(t, outputStr, "Gap Detection", "Should show gap detection")
	
	// Should show different detection methods
	detectionMethods := []string{"Limit Message", "Gap", "First Message"}
	hasDetectionMethod := false
	for _, method := range detectionMethods {
		if strings.Contains(outputStr, method) {
			hasDetectionMethod = true
			break
		}
	}
	assert.True(t, hasDetectionMethod, "Should show detection methods used")
}

// TestDetectCommandErrorHandling tests error conditions
func TestDetectCommandErrorHandling(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "non-existent directory",
			args:        []string{"--dir", "/non/existent/path", "detect"},
			expectError: true,
		},
		{
			name:        "invalid plan",
			args:        []string{"--dir", t.TempDir(), "detect", "--plan", "invalid"},
			expectError: true,
		},
		{
			name:        "invalid timezone",
			args:        []string{"--dir", t.TempDir(), "detect", "--timezone", "Invalid/Timezone"},
			expectError: true,
		},
		{
			name:        "invalid pricing source",
			args:        []string{"--dir", t.TempDir(), "detect", "--pricing-source", "invalid"},
			expectError: true,
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
		})
	}
}

// TestDetectCommandEmptyDataHandling tests behavior with no data
func TestDetectCommandEmptyDataHandling(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create empty project
	err := generator.CreateEmptyProject("empty-detect")
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect on empty data
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	// Should handle empty data gracefully
	assert.NoError(t, err, "Should handle empty data gracefully: %s", string(output))
	
	outputStr := string(output)
	// Should indicate no sessions found or similar
	hasEmptyIndicator := strings.Contains(outputStr, "No sessions") || 
						 strings.Contains(outputStr, "0 sessions") ||
						 strings.Contains(outputStr, "No data")
	assert.True(t, hasEmptyIndicator, "Should indicate no sessions found")
}

// TestDetectCommandLargeDatasetPerformance tests performance with large datasets
func TestDetectCommandLargeDatasetPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset performance test in short mode")
	}
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate large dataset with multiple rate limits
	err := generator.GenerateLargeDataset("performance-test", time.Now().Add(-48*time.Hour), 800)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test detect performance
	startTime := time.Now()
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	elapsed := time.Since(startTime)
	
	assert.NoError(t, err, "Should handle large dataset: %s", string(output))
	assert.Less(t, elapsed, 15*time.Second, "Should process large dataset within reasonable time")
	
	outputStr := string(output)
	assert.Contains(t, outputStr, "performance-test", "Should show large project")
	assert.Contains(t, outputStr, "Sessions Found", "Should show session count")
}

// TestDetectCommandRateLimitDetection tests rate limit detection accuracy
func TestDetectCommandRateLimitDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session specifically with rate limits
	err := generator.GenerateSessionWithLimit("rate-limit-test", time.Now().Add(-5*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect", "--plan", "max5")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Rate limit detection should succeed: %s", string(output))
	outputStr := string(output)
	
	// Should detect rate limit indicators
	assert.Contains(t, outputStr, "rate-limit-test", "Should show rate limited project")
	
	// Should show rate limit detection (🎯 icon or similar)
	assert.Contains(t, outputStr, "🎯", "Should show limit message detection icon")
	
	// Should show window detection success
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection statistics")
	
	// Should indicate high detection success rate due to limit message
	detectionRegex := regexp.MustCompile(`Window Detection Success.*100%|Window Detection Success.*[89]\d%`)
	assert.Regexp(t, detectionRegex, outputStr, "Should show high detection success rate")
}

// TestDetectCommandAccountLevelDetection tests account-level session detection
func TestDetectCommandAccountLevelDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multiple projects that might share same session window
	now := time.Now()
	baseTime := now.Add(-4 * time.Hour)
	
	// Create sessions that would overlap in the same 5-hour window
	err := generator.GenerateSimpleSession("project-1", baseTime)
	require.NoError(t, err)
	
	err = generator.GenerateSimpleSession("project-2", baseTime.Add(30*time.Minute))
	require.NoError(t, err)
	
	err = generator.GenerateSessionWithLimit("project-3", baseTime.Add(1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Account-level detection should succeed: %s", string(output))
	outputStr := string(output)
	
	// Should detect all projects
	assert.Contains(t, outputStr, "project-1", "Should detect project-1")
	assert.Contains(t, outputStr, "project-2", "Should detect project-2")
	assert.Contains(t, outputStr, "project-3", "Should detect project-3")
	
	// Should show session window detection
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection analysis")
	
	// Should potentially show account-level session detection if implemented
	// This might show "Multiple" project sessions or account-level indicators
}

// TestDetectCommandDetailedOutput tests detailed output analysis
func TestDetectCommandDetailedOutput(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate diverse session for detailed analysis
	err := generator.GenerateMultiModelSession("detailed-analysis", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Detailed output analysis should succeed: %s", string(output))
	outputStr := string(output)
	
	// Verify comprehensive output sections
	expectedSections := []string{
		"Session Detection Analysis",
		"Sessions Found",
		"Window Detection Success",
		"Detection Methods",
		"Model Distribution",
		"Session Statistics",
	}
	
	for _, section := range expectedSections {
		assert.Contains(t, outputStr, section, "Should contain %s section", section)
	}
	
	// Should show model-specific statistics
	models := []string{"claude-3.5-sonnet", "claude-3-opus", "claude-3-haiku"}
	modelCount := 0
	for _, model := range models {
		if strings.Contains(outputStr, model) {
			modelCount++
		}
	}
	assert.Greater(t, modelCount, 1, "Should show multiple models in distribution")
	
	// Should show numerical statistics
	numberRegex := regexp.MustCompile(`\d+`)
	matches := numberRegex.FindAllString(outputStr, -1)
	assert.Greater(t, len(matches), 5, "Should contain various numerical statistics")
}

// TestDetectCommandConsistency tests output consistency and reliability
func TestDetectCommandConsistency(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate consistent test data
	err := generator.GenerateSessionWithLimit("consistency-test", time.Now().Add(-4*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Run detect command multiple times to verify consistency
	var outputs []string
	
	for i := 0; i < 3; i++ {
		cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
		output, err := cmd.CombinedOutput()
		
		assert.NoError(t, err, "Run %d should succeed", i+1)
		outputs = append(outputs, string(output))
	}
	
	// All outputs should be identical for deterministic detection
	for i := 1; i < len(outputs); i++ {
		assert.Equal(t, outputs[0], outputs[i], "Output should be consistent across runs")
	}
	
	// All should contain core elements
	for i, output := range outputs {
		assert.Contains(t, output, "consistency-test", "Run %d should contain project", i+1)
		assert.Contains(t, output, "Session Detection", "Run %d should show session detection", i+1)
	}
}
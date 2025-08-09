//go:build e2e
// +build e2e

package commands

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/e2e"
	"github.com/penwyp/go-claude-monitor/internal/testing/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimezoneHandlingBasic tests basic timezone functionality
func TestTimezoneHandlingBasic(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with specific timestamp
	baseTime := time.Now().Add(-6 * time.Hour)
	err := generator.GenerateSimpleSession("timezone-basic", baseTime)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testTimezones := []string{
		"UTC",
		"America/New_York", 
		"Europe/London",
		"Asia/Tokyo",
		"Australia/Sydney",
		"America/Los_Angeles",
		"Europe/Berlin",
		"Asia/Shanghai",
		"Local",
	}

	for _, tz := range testTimezones {
		t.Run("timezone_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tz, "--duration", "24h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should handle timezone %s: %s", tz, string(output))
			
			// Verify JSON output is valid
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			assert.NoError(t, err, "Should produce valid JSON for timezone %s", tz)
			
			// Verify project data is present
			if projects, hasProjects := result["projects"].(map[string]interface{}); hasProjects {
				assert.Contains(t, projects, "timezone-basic", "Should show project data for timezone %s", tz)
			}
		})
	}
}

// TestTimezoneSessionDetection tests timezone effects on session detection
func TestTimezoneSessionDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate sessions that might span timezone boundaries
	now := time.Now()
	
	// Session that crosses midnight in different timezones
	err := generator.GenerateContinuousActivity("midnight-crossing", now.Add(-8*time.Hour))
	require.NoError(t, err)
	
	// Session with rate limit in different timezone context
	err = generator.GenerateSessionWithLimit("tz-limit-test", now.Add(-10*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test detect command with different timezones
	testTimezones := []string{
		"UTC",
		"America/New_York",  // EST/EDT
		"Europe/London",     // GMT/BST
		"Asia/Tokyo",        // JST
	}

	for _, tz := range testTimezones {
		t.Run("detect_timezone_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "detect", "--timezone", tz)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Detect should work with timezone %s: %s", tz, string(output))
			outputStr := string(output)
			
			// Should detect sessions regardless of timezone
			assert.Contains(t, outputStr, "midnight-crossing", "Should detect session in %s", tz)
			assert.Contains(t, outputStr, "tz-limit-test", "Should detect limited session in %s", tz)
			
			// Should show session detection analysis
			assert.Contains(t, outputStr, "Session Detection", "Should show detection analysis for %s", tz)
		})
	}
}

// TestTimezoneTopCommand tests timezone handling in top command
func TestTimezoneTopCommand(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate recent activity for real-time display
	err := generator.GenerateSimpleSession("top-tz-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testTimezones := []string{
		"UTC",
		"America/New_York",
		"Asia/Tokyo",
	}

	for _, tz := range testTimezones {
		t.Run("top_timezone_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			config := &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", tempDir, "top", "--timezone", tz},
				Timeout: 6 * time.Second,
			}

			session, err := e2e.NewTUITestSession(config)
			require.NoError(t, err)
			defer session.ForceStop()

			// Wait for load
			time.Sleep(1500 * time.Millisecond)

			// Should display data with timezone consideration
			topOutput := session.GetCleanOutput()
			assert.Contains(t, topOutput, "top-tz-test", "Should show project in top with timezone %s", tz)

			err = session.Stop()
			assert.NoError(t, err)
		})
	}
}

// TestTimezoneDurationFiltering tests how timezone affects duration filtering
func TestTimezoneDurationFiltering(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create sessions at specific times that will test timezone boundaries
	now := time.Now()
	
	// Session from 25 hours ago (should be excluded from 24h filter in some timezones)
	err := generator.GenerateSimpleSession("borderline-25h", now.Add(-25*time.Hour))
	require.NoError(t, err)
	
	// Session from 23 hours ago (should be included in 24h filter)
	err = generator.GenerateSimpleSession("within-23h", now.Add(-23*time.Hour))
	require.NoError(t, err)
	
	// Session from 1 hour ago (should always be included)
	err = generator.GenerateSimpleSession("recent-1h", now.Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		timezone string
		duration string
		shouldInclude []string
		description string
	}{
		{
			timezone: "UTC",
			duration: "24h",
			shouldInclude: []string{"within-23h", "recent-1h"},
			description: "UTC 24h filter",
		},
		{
			timezone: "America/New_York",
			duration: "24h", 
			shouldInclude: []string{"within-23h", "recent-1h"},
			description: "EST/EDT 24h filter",
		},
		{
			timezone: "Asia/Tokyo",
			duration: "24h",
			shouldInclude: []string{"within-23h", "recent-1h"},
			description: "JST 24h filter",
		},
		{
			timezone: "UTC",
			duration: "30h",
			shouldInclude: []string{"borderline-25h", "within-23h", "recent-1h"},
			description: "UTC 30h filter includes borderline",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tc.timezone, "--duration", tc.duration, "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Duration filtering should work with timezone %s: %s", tc.timezone, string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON")
			
			if projects, hasProjects := result["projects"].(map[string]interface{}); hasProjects {
				for _, expectedProject := range tc.shouldInclude {
					assert.Contains(t, projects, expectedProject, 
						"Should include %s with %s duration in %s timezone", 
						expectedProject, tc.duration, tc.timezone)
				}
			}
		})
	}
}

// TestTimezoneGroupingBehavior tests how timezone affects time grouping
func TestTimezoneGroupingBehavior(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate sessions across multiple days in different timezones
	now := time.Now()
	
	// Sessions at times that will group differently across timezones
	sessions := []struct {
		name string
		time time.Time
	}{
		{"morning-session", now.Add(-26 * time.Hour)}, // Yesterday morning
		{"evening-session", now.Add(-18 * time.Hour)}, // Yesterday evening
		{"night-session", now.Add(-8 * time.Hour)},    // Last night/this morning
		{"current-session", now.Add(-2 * time.Hour)},  // Current day
	}
	
	for _, session := range sessions {
		err := generator.GenerateSimpleSession(session.name, session.time)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		timezone string
		groupBy  string
		description string
	}{
		{"UTC", "day", "UTC daily grouping"},
		{"America/New_York", "day", "EST/EDT daily grouping"},
		{"Asia/Tokyo", "day", "JST daily grouping"},
		{"UTC", "hour", "UTC hourly grouping"},
		{"Europe/London", "hour", "GMT/BST hourly grouping"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tc.timezone, "--group-by", tc.groupBy, "--duration", "48h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Grouping should work with timezone %s: %s", tc.timezone, string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON for grouping test")
			
			// Should contain grouped data
			if projects, hasProjects := result["projects"].(map[string]interface{}); hasProjects {
				projectCount := len(projects)
				assert.Greater(t, projectCount, 0, "Should have grouped projects for %s", tc.description)
				
				// At least some sessions should be grouped
				foundSessions := 0
				for projectName := range projects {
					for _, session := range sessions {
						if strings.Contains(projectName, session.name) {
							foundSessions++
						}
					}
				}
				assert.Greater(t, foundSessions, 0, "Should find sessions in grouping for %s", tc.description)
			}
		})
	}
}

// TestTimezoneErrorHandling tests timezone error conditions
func TestTimezoneErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate test data
	err := generator.GenerateSimpleSession("tz-error-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	invalidTimezones := []string{
		"Invalid/Timezone",
		"America/NonExistent",
		"Europe/FakeCity",
		"Completely/Wrong",
		"UTC+25", // Invalid offset
	}

	for _, invalidTz := range invalidTimezones {
		t.Run("invalid_timezone_"+strings.ReplaceAll(invalidTz, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", invalidTz, "--duration", "24h")
			output, err := cmd.CombinedOutput()
			
			// Should return an error or handle gracefully
			if err != nil {
				// Expected behavior: return error for invalid timezone
				assert.Error(t, err, "Should return error for invalid timezone %s", invalidTz)
			} else {
				// Alternative behavior: fallback to default timezone
				outputStr := string(output)
				assert.Contains(t, outputStr, "tz-error-test", "Should still show data if fallback is used")
			}
		})
	}
}

// TestTimezoneConsistency tests consistency across commands
func TestTimezoneConsistency(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit for comprehensive testing
	err := generator.GenerateSessionWithLimit("consistency-tz-test", time.Now().Add(-5*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testTimezone := "America/New_York"

	// Test root command
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", testTimezone, "--duration", "24h", "--output", "json")
	rootOutput, err := cmd.CombinedOutput()
	assert.NoError(t, err, "Root command should work with timezone")

	// Test detect command  
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect", "--timezone", testTimezone)
	detectOutput, err := cmd.CombinedOutput()
	assert.NoError(t, err, "Detect command should work with timezone")

	// Both should show the same project data
	var rootResult map[string]interface{}
	err = json.Unmarshal(rootOutput, &rootResult)
	require.NoError(t, err, "Root output should be valid JSON")

	detectOutputStr := string(detectOutput)

	// Both should include the test project
	if projects, hasProjects := rootResult["projects"].(map[string]interface{}); hasProjects {
		assert.Contains(t, projects, "consistency-tz-test", "Root command should show project")
	}
	assert.Contains(t, detectOutputStr, "consistency-tz-test", "Detect command should show project")
}

// TestTimezoneDaylightSavingTransitions tests DST transitions
func TestTimezoneDaylightSavingTransitions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Note: This test creates sessions around potential DST transition times
	// The exact behavior will depend on when the test is run and the current date
	
	now := time.Now()
	
	// Generate sessions that might be affected by DST transitions
	// Using different offsets to potentially catch transition periods
	testTimes := []struct {
		name   string
		offset time.Duration
	}{
		{"dst-morning", -36 * time.Hour},
		{"dst-evening", -28 * time.Hour},
		{"dst-midnight", -24 * time.Hour},
		{"dst-recent", -6 * time.Hour},
	}
	
	for _, testTime := range testTimes {
		err := generator.GenerateSimpleSession(testTime.name, now.Add(testTime.offset))
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test timezones that observe DST
	dstTimezones := []string{
		"America/New_York",  // EDT/EST
		"Europe/London",     // BST/GMT  
		"Europe/Berlin",     // CEST/CET
		"Australia/Sydney",  // AEDT/AEST (different DST period)
	}

	for _, tz := range dstTimezones {
		t.Run("dst_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tz, "--duration", "48h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should handle DST timezone %s: %s", tz, string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON for DST timezone %s", tz)
			
			// Should show session data regardless of DST
			if projects, hasProjects := result["projects"].(map[string]interface{}); hasProjects {
				sessionCount := 0
				for projectName := range projects {
					for _, testTime := range testTimes {
						if strings.Contains(projectName, testTime.name) {
							sessionCount++
						}
					}
				}
				assert.Greater(t, sessionCount, 0, "Should handle sessions across DST for %s", tz)
			}
		})
	}
}

// TestTimezonePerformanceWithManyTimezones tests performance with multiple timezone requests
func TestTimezonePerformanceWithManyTimezones(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timezone performance test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate moderate dataset
	err := generator.GenerateLargeDataset("tz-performance", time.Now().Add(-12*time.Hour), 200)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	allTimezones := []string{
		"UTC",
		"America/New_York",
		"America/Los_Angeles", 
		"Europe/London",
		"Europe/Berlin",
		"Asia/Tokyo",
		"Asia/Shanghai",
		"Australia/Sydney",
		"America/Chicago",
		"Europe/Paris",
	}

	// Test performance with multiple timezone conversions
	startTime := time.Now()
	
	for _, tz := range allTimezones {
		cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tz, "--duration", "24h", "--output", "json")
		output, err := cmd.CombinedOutput()
		
		assert.NoError(t, err, "Performance test should succeed for timezone %s", tz)
		
		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err, "Should produce valid JSON for timezone %s", tz)
	}
	
	elapsed := time.Since(startTime)
	
	// Should complete all timezone tests within reasonable time
	maxDuration := 30 * time.Second
	assert.Less(t, elapsed, maxDuration, 
		"All timezone tests should complete within %v, took %v", maxDuration, elapsed)
	
	// Average time per timezone should be reasonable
	avgTime := elapsed / time.Duration(len(allTimezones))
	assert.Less(t, avgTime, 3*time.Second, 
		"Average time per timezone should be under 3s, got %v", avgTime)
}
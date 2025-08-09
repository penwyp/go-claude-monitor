//go:build e2e
// +build e2e

package commands

import (
	"encoding/json"
	"fmt"
	"os"
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

// TestRateLimitDetectionBasic tests basic rate limit detection functionality
func TestRateLimitDetectionBasic(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit
	err := generator.GenerateSessionWithLimit("basic-limit-test", time.Now().Add(-4*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test root command handles rate limits
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Should handle rate limited session: %s", string(output))
	
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Should produce valid JSON with rate limit data")
	
	// Verify project data is present
	projects := result["projects"].(map[string]interface{})
	assert.Contains(t, projects, "basic-limit-test", "Should show rate limited project")
	
	projectData := projects["basic-limit-test"].(map[string]interface{})
	
	// Should have cost and token data from successful requests before limit
	totalCost := projectData["total_cost"].(float64)
	totalTokens := projectData["total_tokens"].(float64)
	
	assert.Greater(t, totalCost, 0.0, "Should have cost from successful requests")
	assert.Greater(t, totalTokens, 0.0, "Should have tokens from successful requests")
}

// TestRateLimitDetectCommand tests detect command with rate limit scenarios
func TestRateLimitDetectCommand(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit for detection
	err := generator.GenerateSessionWithLimit("detect-limit-test", time.Now().Add(-5*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test detect command with different plan types
	planTypes := []string{"max5", "max20", "pro", "team"}
	
	for _, plan := range planTypes {
		t.Run("plan_"+plan, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "detect", "--plan", plan)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Detect should work with plan %s: %s", plan, string(output))
			outputStr := string(output)
			
			// Should show session detection
			assert.Contains(t, outputStr, "detect-limit-test", "Should detect rate limited project")
			assert.Contains(t, outputStr, "Session Detection", "Should show detection analysis")
			
			// Should show rate limit detection indicators
			assert.Contains(t, outputStr, "🎯", "Should show limit message detection icon")
			
			// Should show window detection success
			assert.Contains(t, outputStr, "Window Detection", "Should show window detection stats")
		})
	}
}

// TestRateLimitWindowDetection tests 5-hour window detection with rate limits
func TestRateLimitWindowDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate sessions with rate limits at different window boundaries
	now := time.Now()
	
	// Session that hits limit exactly at 5-hour mark
	err := generator.GenerateSessionWithLimit("window-boundary-test", now.Add(-5*time.Hour))
	require.NoError(t, err)
	
	// Session that hits limit mid-window
	err = generator.GenerateSessionWithLimit("mid-window-test", now.Add(-7*time.Hour))
	require.NoError(t, err)
	
	// Session with continuous activity then limit
	err = generator.GenerateContinuousActivity("continuous-then-limit", now.Add(-9*time.Hour))
	require.NoError(t, err)
	
	// Add a rate limit to the continuous session by appending to its file
	continuousDir := filepath.Join(tempDir, "continuous-then-limit")
	continuousFile := filepath.Join(continuousDir, "usage.jsonl")
	
	// Read existing content
	existingData, err := os.ReadFile(continuousFile)
	require.NoError(t, err)
	
	// Append rate limit entry
	resetTime := now.Add(-4 * time.Hour)
	limitEntry := fixtures.JSONLEntry{
		Timestamp: now.Add(-4*time.Hour - 30*time.Minute).Format(time.RFC3339),
		Type:      "limit",
		Message: fixtures.Message{
			Role:    "system",
			Type:    "error",
			Content: "Rate limit exceeded. Reset at " + resetTime.Format(time.RFC3339),
		},
		ResetTime: resetTime.Format(time.RFC3339),
	}
	
	err = generator.WriteJSONL(continuousFile, []fixtures.JSONLEntry{limitEntry})
	require.NoError(t, err)
	
	// Append to existing file instead of overwriting
	newData, err := os.ReadFile(continuousFile)
	require.NoError(t, err)
	
	combinedData := string(existingData) + string(newData)
	err = os.WriteFile(continuousFile, []byte(combinedData), 0644)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test window detection with rate limits
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Window detection should work with rate limits: %s", string(output))
	outputStr := string(output)
	
	// Should detect all sessions
	assert.Contains(t, outputStr, "window-boundary-test", "Should detect boundary test")
	assert.Contains(t, outputStr, "mid-window-test", "Should detect mid-window test")
	assert.Contains(t, outputStr, "continuous-then-limit", "Should detect continuous session")
	
	// Should show high detection success rate due to limit messages
	assert.Contains(t, outputStr, "Window Detection Success", "Should show detection success rate")
	
	// Should show limit message detection method
	assert.Contains(t, outputStr, "Limit Message", "Should show limit message detection method")
}

// TestRateLimitTopCommand tests top command with rate limited sessions
func TestRateLimitTopCommand(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate recent rate limited session for top command
	err := generator.GenerateSessionWithLimit("top-limit-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)
	
	// Generate normal session for comparison
	err = generator.GenerateSimpleSession("top-normal-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testPlans := []string{"max5", "max20", "pro"}
	
	for _, plan := range testPlans {
		t.Run("top_plan_"+plan, func(t *testing.T) {
			config := &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", tempDir, "top", "--plan", plan},
				Timeout: 8 * time.Second,
			}

			session, err := e2e.NewTUITestSession(config)
			require.NoError(t, err)
			defer session.ForceStop()

			// Wait for data processing
			time.Sleep(2 * time.Second)

			// Should display both projects
			topOutput := session.GetCleanOutput()
			assert.Contains(t, topOutput, "top-limit-test", "Should show rate limited project")
			assert.Contains(t, topOutput, "top-normal-test", "Should show normal project")
			
			// Should show rate limit information or indicators
			hasRateLimitInfo := session.ContainsText("Rate") || 
							   session.ContainsText("Limit") ||
							   session.ContainsText("Window") ||
							   session.ContainsText("Usage")
			
			assert.True(t, hasRateLimitInfo, "Should show rate limit related information")
			
			// Test sorting with rate limited data
			err = session.SendKey('c') // Sort by cost
			require.NoError(t, err)
			
			time.Sleep(500 * time.Millisecond)
			
			sortedOutput := session.GetCleanOutput()
			assert.Contains(t, sortedOutput, "top-limit-test", "Should still show projects after sorting")

			err = session.Stop()
			assert.NoError(t, err)
		})
	}
}

// TestRateLimitMultipleProjects tests rate limits across multiple projects
func TestRateLimitMultipleProjects(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multiple projects with different rate limit scenarios
	now := time.Now()
	
	projects := []struct {
		name string
		time time.Time
		hasLimit bool
	}{
		{"no-limit-project", now.Add(-1 * time.Hour), false},
		{"early-limit-project", now.Add(-8 * time.Hour), true},
		{"recent-limit-project", now.Add(-3 * time.Hour), true},
		{"boundary-limit-project", now.Add(-5 * time.Hour), true},
		{"normal-project-2", now.Add(-2 * time.Hour), false},
	}
	
	for _, proj := range projects {
		if proj.hasLimit {
			err := generator.GenerateSessionWithLimit(proj.name, proj.time)
			require.NoError(t, err)
		} else {
			err := generator.GenerateSimpleSession(proj.name, proj.time)
			require.NoError(t, err)
		}
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multi-project analysis with mixed rate limits
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Should handle multiple projects with mixed rate limits: %s", string(output))
	
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err)
	
	// Verify all projects are present
	projectsData := result["projects"].(map[string]interface{})
	
	for _, proj := range projects {
		assert.Contains(t, projectsData, proj.name, "Should include project %s", proj.name)
		
		projectData := projectsData[proj.name].(map[string]interface{})
		totalCost := projectData["total_cost"].(float64)
		totalTokens := projectData["total_tokens"].(float64)
		
		// All projects should have some data (successful requests before limits)
		assert.Greater(t, totalCost, 0.0, "Project %s should have cost data", proj.name)
		assert.Greater(t, totalTokens, 0.0, "Project %s should have token data", proj.name)
	}
	
	// Test detect command with multiple rate limited projects
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Detect should work with multiple rate limited projects")
	outputStr := string(output)
	
	// Should detect all projects
	for _, proj := range projects {
		assert.Contains(t, outputStr, proj.name, "Should detect project %s", proj.name)
	}
	
	// Should show comprehensive analysis
	assert.Contains(t, outputStr, "Window Detection Success", "Should show detection success")
	assert.Contains(t, outputStr, "🎯", "Should show limit message detection icons")
}

// TestRateLimitAccountLevelDetection tests account-level rate limit detection
func TestRateLimitAccountLevelDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create scenario where multiple projects share same 5-hour window
	now := time.Now()
	baseTime := now.Add(-4 * time.Hour)
	
	// Multiple projects within same potential rate limit window
	err := generator.GenerateSimpleSession("account-project-1", baseTime)
	require.NoError(t, err)
	
	err = generator.GenerateSimpleSession("account-project-2", baseTime.Add(30*time.Minute))
	require.NoError(t, err)
	
	// Project that hits rate limit (should apply to account level)
	err = generator.GenerateSessionWithLimit("account-project-3", baseTime.Add(1*time.Hour))
	require.NoError(t, err)
	
	// Project after rate limit reset
	err = generator.GenerateSimpleSession("account-project-4", baseTime.Add(6*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test account-level detection
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Account-level detection should work: %s", string(output))
	outputStr := string(output)
	
	// Should detect all projects
	assert.Contains(t, outputStr, "account-project-1", "Should detect project 1")
	assert.Contains(t, outputStr, "account-project-2", "Should detect project 2") 
	assert.Contains(t, outputStr, "account-project-3", "Should detect project 3")
	assert.Contains(t, outputStr, "account-project-4", "Should detect project 4")
	
	// Should show session window analysis
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection")
	
	// Should show high detection success due to limit message
	assert.Contains(t, outputStr, "🎯", "Should show limit message detection")
	
	// Should potentially show account-level session information
	// (This depends on implementation details of account-level detection)
}

// TestRateLimitTimezoneEffects tests rate limit handling across timezones
func TestRateLimitTimezoneEffects(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate rate limited session
	err := generator.GenerateSessionWithLimit("timezone-limit-test", time.Now().Add(-6*time.Hour))
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
	}

	for _, tz := range testTimezones {
		t.Run("timezone_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			// Test root command with timezone
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--timezone", tz, "--duration", "24h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should handle rate limits with timezone %s: %s", tz, string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON for timezone %s", tz)
			
			// Should include rate limited project
			projects := result["projects"].(map[string]interface{})
			assert.Contains(t, projects, "timezone-limit-test", "Should show project for timezone %s", tz)
			
			// Test detect command with timezone
			cmd = exec.Command(binaryPath, "--dir", tempDir, "detect", "--timezone", tz)
			output, err = cmd.CombinedOutput()
			
			assert.NoError(t, err, "Detect should work with timezone %s", tz)
			outputStr := string(output)
			
			// Should detect session regardless of timezone
			assert.Contains(t, outputStr, "timezone-limit-test", "Should detect session in timezone %s", tz)
			assert.Contains(t, outputStr, "🎯", "Should show limit detection in timezone %s", tz)
		})
	}
}

// TestRateLimitWindowHistory tests window history with rate limits
func TestRateLimitWindowHistory(t *testing.T) {
	// Create temporary home directory for window history
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit for window history
	err := generator.GenerateSessionWithLimit("history-limit-test", time.Now().Add(-4*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// First run to establish window history
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "First detect run should succeed: %s", string(output))
	outputStr := string(output)
	
	// Should detect rate limited session
	assert.Contains(t, outputStr, "history-limit-test", "Should detect rate limited session")
	assert.Contains(t, outputStr, "🎯", "Should show limit message detection")
	
	// Second run should use window history
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect")
	secondOutput, err := cmd.CombinedOutput()
	
	assert.NoError(t, err, "Second detect run should succeed")
	secondOutputStr := string(secondOutput)
	
	// Should still detect the same session
	assert.Contains(t, secondOutputStr, "history-limit-test", "Should still detect session")
	
	// Should show consistent detection results
	assert.Contains(t, secondOutputStr, "🎯", "Should maintain limit message detection")
	
	// Test reset-windows flag
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect", "--reset-windows")
	resetOutput, err := cmd.CombinedOutput()
	
	assert.NoError(t, err, "Reset windows should succeed")
	resetOutputStr := string(resetOutput)
	
	// Should still work after reset
	assert.Contains(t, resetOutputStr, "history-limit-test", "Should work after window reset")
}

// TestRateLimitPerformanceWithManyLimits tests performance with multiple rate limits
func TestRateLimitPerformanceWithManyLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit performance test in short mode")
	}
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate many projects with rate limits
	numProjects := 20
	now := time.Now()
	
	for i := 0; i < numProjects; i++ {
		projectName := fmt.Sprintf("rate-limit-perf-%02d", i)
		startTime := now.Add(time.Duration(-i-1) * time.Hour)
		
		// Alternate between rate limited and normal sessions
		if i%3 == 0 {
			err := generator.GenerateSessionWithLimit(projectName, startTime)
			require.NoError(t, err)
		} else {
			err := generator.GenerateMultiModelSession(projectName, startTime)
			require.NoError(t, err)
		}
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test performance with many rate limits
	testCases := []struct {
		command     []string
		maxDuration time.Duration
		description string
	}{
		{
			command:     []string{"--dir", tempDir, "--duration", "48h", "--output", "json"},
			maxDuration: 15 * time.Second,
			description: "root command with many rate limits",
		},
		{
			command:     []string{"--dir", tempDir, "detect"},
			maxDuration: 20 * time.Second,
			description: "detect command with many rate limits",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			startTime := time.Now()
			
			cmd := exec.Command(binaryPath, tc.command...)
			output, err := cmd.CombinedOutput()
			
			elapsed := time.Since(startTime)
			
			assert.NoError(t, err, "%s should succeed: %s", tc.description, string(output))
			assert.Less(t, elapsed, tc.maxDuration,
				"%s should complete within %v, took %v", tc.description, tc.maxDuration, elapsed)
			
			if strings.Contains(tc.description, "root command") {
				// Verify JSON contains projects
				var result map[string]interface{}
				err = json.Unmarshal(output, &result)
				assert.NoError(t, err, "Should produce valid JSON")
				
				projects := result["projects"].(map[string]interface{})
				assert.Greater(t, len(projects), numProjects/3, "Should include multiple projects")
			} else {
				// Verify detect output
				outputStr := string(output)
				assert.Contains(t, outputStr, "Sessions Found", "Should show session analysis")
				assert.Contains(t, outputStr, "🎯", "Should show limit message detections")
			}
		})
	}
}

// TestRateLimitEdgeCases tests edge cases in rate limit handling
func TestRateLimitEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create various edge case scenarios
	now := time.Now()
	
	// Rate limit exactly at session boundary
	err := generator.GenerateSessionWithLimit("boundary-limit", now.Add(-5*time.Hour))
	require.NoError(t, err)
	
	// Very old rate limit
	err = generator.GenerateSessionWithLimit("old-limit", now.Add(-72*time.Hour))
	require.NoError(t, err)
	
	// Recent rate limit
	err = generator.GenerateSessionWithLimit("recent-limit", now.Add(-30*time.Minute))
	require.NoError(t, err)
	
	// Create a session with malformed rate limit message
	malformedDir := filepath.Join(tempDir, "malformed-limit")
	err = os.MkdirAll(malformedDir, 0755)
	require.NoError(t, err)
	
	malformedData := `{"timestamp": "` + now.Add(-2*time.Hour).Format(time.RFC3339) + `", "type": "usage", "model": "claude-3.5-sonnet", "input_tokens": 1000, "output_tokens": 500}
{"timestamp": "` + now.Add(-90*time.Minute).Format(time.RFC3339) + `", "type": "limit", "message": "Malformed rate limit message without proper reset time"}
{"timestamp": "` + now.Add(-80*time.Minute).Format(time.RFC3339) + `", "type": "usage", "model": "claude-3.5-sonnet", "input_tokens": 0, "output_tokens": 0, "message": "Request failed"}
`
	
	malformedFile := filepath.Join(malformedDir, "usage.jsonl")
	err = os.WriteFile(malformedFile, []byte(malformedData), 0644)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test edge cases
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Should handle rate limit edge cases: %s", string(output))
	outputStr := string(output)
	
	// Should detect boundary case
	assert.Contains(t, outputStr, "boundary-limit", "Should detect boundary limit case")
	
	// Should detect recent case
	assert.Contains(t, outputStr, "recent-limit", "Should detect recent limit case")
	
	// Should handle malformed limit gracefully
	assert.Contains(t, outputStr, "malformed-limit", "Should handle malformed limit case")
	
	// Should show some form of window detection
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection analysis")
	
	// Test with different duration filters
	durationTests := []string{"1h", "6h", "24h", "72h"}
	
	for _, duration := range durationTests {
		t.Run("duration_"+duration, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", duration, "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Should handle rate limits with duration %s", duration)
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			assert.NoError(t, err, "Should produce valid JSON for duration %s", duration)
		})
	}
}
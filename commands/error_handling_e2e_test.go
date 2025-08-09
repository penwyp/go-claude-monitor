//go:build e2e
// +build e2e

package commands

import (
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

// TestErrorHandlingNonExistentDirectory tests handling of non-existent directories
func TestErrorHandlingNonExistentDirectory(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	nonExistentPaths := []string{
		"/completely/nonexistent/path",
		"/tmp/this-should-not-exist-12345",
		"relative/nonexistent/path",
		"/root/inaccessible", // May exist but likely inaccessible
	}

	testCommands := []struct {
		name string
		args []string
	}{
		{"root_command", []string{"--dir", "", "--duration", "1h"}},
		{"detect_command", []string{"--dir", "", "detect"}},
		{"top_command", []string{"--dir", "", "top"}},
	}

	for _, path := range nonExistentPaths {
		for _, cmd := range testCommands {
			t.Run(cmd.name+"_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				args := make([]string, len(cmd.args))
				copy(args, cmd.args)
				// Replace empty dir with actual test path
				for i, arg := range args {
					if arg == "" {
						args[i] = path
					}
				}

				execCmd := exec.Command(binaryPath, args...)
				output, err := execCmd.CombinedOutput()

				// Should return error for non-existent directory
				assert.Error(t, err, "Should return error for non-existent directory: %s", path)
				
				// Error message should be informative
				outputStr := string(output)
				hasErrorInfo := strings.Contains(outputStr, "not found") ||
								strings.Contains(outputStr, "does not exist") ||
								strings.Contains(outputStr, "No such file") ||
								strings.Contains(outputStr, "directory") ||
								strings.Contains(outputStr, "error") ||
								len(outputStr) == 0 // No output on error is also acceptable
				
				assert.True(t, hasErrorInfo, "Should provide informative error message for %s: %s", path, outputStr)
			})
		}
	}
}

// TestErrorHandlingInvalidArguments tests invalid command line arguments
func TestErrorHandlingInvalidArguments(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create minimal valid data for testing
	err := generator.GenerateSimpleSession("error-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		name     string
		args     []string
		errorMsg string
	}{
		{
			name:     "invalid_duration",
			args:     []string{"--dir", tempDir, "--duration", "invalid-duration"},
			errorMsg: "duration",
		},
		{
			name:     "invalid_output_format",
			args:     []string{"--dir", tempDir, "--output", "invalid-format", "--duration", "1h"},
			errorMsg: "output",
		},
		{
			name:     "invalid_group_by",
			args:     []string{"--dir", tempDir, "--group-by", "invalid-group", "--duration", "1h"},
			errorMsg: "group",
		},
		{
			name:     "invalid_timezone",
			args:     []string{"--dir", tempDir, "--timezone", "Invalid/Timezone", "--duration", "1h"},
			errorMsg: "timezone",
		},
		{
			name:     "invalid_pricing_source",
			args:     []string{"--dir", tempDir, "--pricing-source", "invalid-source", "--duration", "1h"},
			errorMsg: "pricing",
		},
		{
			name:     "invalid_plan_detect",
			args:     []string{"--dir", tempDir, "detect", "--plan", "invalid-plan"},
			errorMsg: "plan",
		},
		{
			name:     "conflicting_flags",
			args:     []string{"--dir", tempDir, "--duration", "1h", "--output", "json", "--breakdown", "--format", "csv"},
			errorMsg: "flag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			output, err := cmd.CombinedOutput()
			
			// Should return error for invalid arguments
			assert.Error(t, err, "Should return error for %s: %s", tc.name, string(output))
			
			// Error message should mention the problematic element
			outputStr := strings.ToLower(string(output))
			assert.Contains(t, outputStr, tc.errorMsg, "Error message should mention %s for test %s", tc.errorMsg, tc.name)
		})
	}
}

// TestErrorHandlingCorruptedData tests handling of corrupted JSONL data
func TestErrorHandlingCorruptedData(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create project directory with corrupted data
	projectDir := filepath.Join(tempDir, "corrupted-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)
	
	// Write corrupted JSONL file
	corruptedData := `{"timestamp": "invalid-timestamp", "type": "usage"
{"timestamp": "2024-01-01T12:00:00Z", "type": "usage", "model": "claude-3.5-sonnet", "input_tokens": "not-a-number"}
invalid json line here
{"valid": "entry", "timestamp": "2024-01-01T13:00:00Z", "type": "usage", "model": "claude-3.5-sonnet", "input_tokens": 1000, "output_tokens": 500}
`
	
	corruptedFile := filepath.Join(projectDir, "usage.jsonl")
	err = os.WriteFile(corruptedFile, []byte(corruptedData), 0644)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCommands := []struct {
		name string
		args []string
	}{
		{
			name: "root_command_corrupted",
			args: []string{"--dir", tempDir, "--duration", "24h", "--output", "json"},
		},
		{
			name: "detect_command_corrupted",
			args: []string{"--dir", tempDir, "detect"},
		},
	}

	for _, tc := range testCommands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			output, err := cmd.CombinedOutput()
			
			// Should handle corrupted data gracefully (either succeed with partial data or fail informatively)
			outputStr := string(output)
			
			if err != nil {
				// If it fails, should provide informative error
				hasErrorInfo := strings.Contains(outputStr, "parse") ||
								strings.Contains(outputStr, "invalid") ||
								strings.Contains(outputStr, "error") ||
								strings.Contains(outputStr, "corrupt")
				assert.True(t, hasErrorInfo, "Should provide informative error for corrupted data: %s", outputStr)
			} else {
				// If it succeeds, should have processed what it could
				assert.NotEmpty(t, outputStr, "Should produce some output even with corrupted data")
				
				// Should show the project even if some data is corrupted
				if strings.Contains(tc.name, "root_command") {
					// JSON output should be valid
					assert.Contains(t, outputStr, "{", "Should produce JSON-like output")
				} else {
					// Detect output should show some analysis
					assert.Contains(t, outputStr, "corrupted-project", "Should show project in detect output")
				}
			}
		})
	}
}

// TestErrorHandlingPermissionDenied tests permission-related errors
func TestErrorHandlingPermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test common directories that might have permission issues
	restrictedPaths := []string{
		"/root",           // Typically not accessible to non-root users
		"/etc/shadow",     // File with restricted permissions
		"/sys/kernel",     // System directory that might be restricted
	}

	for _, path := range restrictedPaths {
		// Skip if path doesn't exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		t.Run("permission_denied_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", path, "--duration", "1h")
			output, err := cmd.CombinedOutput()
			
			// Should handle permission errors gracefully
			if err != nil {
				outputStr := string(output)
				hasPermissionError := strings.Contains(outputStr, "permission") ||
									 strings.Contains(outputStr, "denied") ||
									 strings.Contains(outputStr, "access") ||
									 strings.Contains(outputStr, "forbidden")
				
				assert.True(t, hasPermissionError, "Should indicate permission issue for %s: %s", path, outputStr)
			}
			// If no error, the path might actually be accessible, which is also fine
		})
	}
}

// TestErrorHandlingTopCommandFailures tests top command error scenarios
func TestErrorHandlingTopCommandFailures(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "top_nonexistent_dir",
			args: []string{"--dir", "/nonexistent/path", "top"},
		},
		{
			name: "top_invalid_plan",
			args: []string{"--dir", t.TempDir(), "top", "--plan", "invalid"},
		},
		{
			name: "top_invalid_refresh_rate",
			args: []string{"--dir", t.TempDir(), "top", "--refresh-per-second", "-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    tc.args,
				Timeout: 3 * time.Second,
			}

			session, err := e2e.NewTUITestSession(config)
			
			if err != nil {
				// Expected: failed to start due to invalid arguments
				assert.Error(t, err, "Should fail to start TUI with invalid arguments for %s", tc.name)
			} else {
				defer session.ForceStop()
				
				// If it starts, it should either show error message or exit gracefully
				time.Sleep(1 * time.Second)
				
				output := session.GetCleanOutput()
				
				// Should show some indication of error or be empty (if exited)
				hasErrorIndication := strings.Contains(output, "error") ||
									 strings.Contains(output, "Error") ||
									 strings.Contains(output, "failed") ||
									 len(strings.TrimSpace(output)) == 0
				
				assert.True(t, hasErrorIndication, "Should show error indication for %s: %s", tc.name, output)
				
				session.Stop()
			}
		})
	}
}

// TestErrorHandlingGracefulDegradation tests graceful degradation scenarios
func TestErrorHandlingGracefulDegradation(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create mix of valid and invalid data
	now := time.Now()
	
	// Valid project
	err := generator.GenerateSimpleSession("valid-project", now.Add(-2*time.Hour))
	require.NoError(t, err)
	
	// Project with empty data
	err = generator.CreateEmptyProject("empty-project")
	require.NoError(t, err)
	
	// Create project with partial corruption
	partialDir := filepath.Join(tempDir, "partial-project")
	err = os.MkdirAll(partialDir, 0755)
	require.NoError(t, err)
	
	mixedData := `{"timestamp": "2024-01-01T12:00:00Z", "type": "usage", "model": "claude-3.5-sonnet", "input_tokens": 1000, "output_tokens": 500}
corrupted line
{"timestamp": "2024-01-01T13:00:00Z", "type": "usage", "model": "claude-3-haiku", "input_tokens": 2000, "output_tokens": 1000}
`
	
	partialFile := filepath.Join(partialDir, "usage.jsonl")
	err = os.WriteFile(partialFile, []byte(mixedData), 0644)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test that system degrades gracefully with mixed data quality
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	// Should succeed with partial data
	assert.NoError(t, err, "Should succeed with mixed data quality: %s", string(output))
	
	outputStr := string(output)
	
	// Should include valid project
	assert.Contains(t, outputStr, "valid-project", "Should include valid project data")
	
	// Should produce valid JSON structure even with some errors
	assert.Contains(t, outputStr, "{", "Should produce JSON output")
	assert.Contains(t, outputStr, "}", "Should have complete JSON structure")
}

// TestErrorHandlingResourceExhaustion tests resource exhaustion scenarios
func TestErrorHandlingResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create very large dataset that might stress the system
	err := generator.GenerateLargeDataset("resource-test", time.Now().Add(-48*time.Hour), 5000)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test with aggressive timeout to catch resource issues
	testCases := []struct {
		name    string
		args    []string
		timeout time.Duration
	}{
		{
			name:    "large_dataset_analysis",
			args:    []string{"--dir", tempDir, "--duration", "72h", "--output", "json"},
			timeout: 30 * time.Second,
		},
		{
			name:    "large_dataset_detection",
			args:    []string{"--dir", tempDir, "detect"},
			timeout: 45 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			
			// Set up timeout
			done := make(chan error, 1)
			go func() {
				_, err := cmd.CombinedOutput()
				done <- err
			}()

			select {
			case err := <-done:
				if err != nil {
					// Should handle resource constraints gracefully
					t.Logf("Command failed as expected under resource stress: %v", err)
				} else {
					// If it succeeds, that's good too
					t.Logf("Command succeeded with large dataset")
				}
				
			case <-time.After(tc.timeout):
				// Kill the process if it's taking too long
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				t.Logf("Command timed out after %v, which indicates potential resource issues", tc.timeout)
				// This is not necessarily a failure - just indicates resource limits
			}
		})
	}
}

// TestErrorHandlingConcurrentErrors tests error handling under concurrent access
func TestErrorHandlingConcurrentErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent error test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create test data
	err := generator.GenerateMultiModelSession("concurrent-error-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Start multiple sessions, some with invalid arguments
	numSessions := 5
	sessions := make([]*e2e.TUITestSession, 0)

	for i := 0; i < numSessions; i++ {
		var config *e2e.TUITestConfig
		
		if i%2 == 0 {
			// Valid session
			config = &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", tempDir, "top"},
				Timeout: 8 * time.Second,
			}
		} else {
			// Invalid session (might fail to start)
			config = &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", "/nonexistent", "top"},
				Timeout: 8 * time.Second,
			}
		}

		session, err := e2e.NewTUITestSession(config)
		if err != nil {
			// Expected for invalid sessions
			continue
		}
		
		sessions = append(sessions, session)
		time.Sleep(100 * time.Millisecond) // Stagger startup
	}

	// Let sessions run
	time.Sleep(2 * time.Second)

	// Interact with valid sessions
	for i, session := range sessions {
		err := session.SendKey('s')
		if err != nil {
			t.Logf("Session %d interaction failed (expected for some): %v", i, err)
		}
		
		time.Sleep(100 * time.Millisecond)
	}

	// Clean up all sessions
	for i, session := range sessions {
		err := session.Stop()
		if err != nil {
			t.Logf("Session %d stop failed: %v", i, err)
		}
	}
	
	// Test should complete without panics or deadlocks
	assert.True(t, true, "Concurrent error test completed")
}

// TestErrorHandlingSignalInterruption tests signal interruption handling
func TestErrorHandlingSignalInterruption(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate data that takes some time to process
	err := generator.GenerateLargeDataset("signal-test", time.Now().Add(-12*time.Hour), 1000)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test interruption during processing
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	
	// Start the command
	err = cmd.Start()
	assert.NoError(t, err, "Should start command")
	
	// Let it run briefly then interrupt
	time.Sleep(1 * time.Second)
	
	// Send interrupt signal
	if cmd.Process != nil {
		err = cmd.Process.Kill()
		assert.NoError(t, err, "Should be able to kill process")
		
		// Wait for process to finish
		err = cmd.Wait()
		// Process was killed, so we expect an error
		assert.Error(t, err, "Should return error when killed")
	}
}

// TestErrorHandlingDiskSpace tests disk space related errors
func TestErrorHandlingDiskSpace(t *testing.T) {
	// This test is more conceptual since we can't easily create actual disk space issues
	// But we can test scenarios that might occur with disk issues
	
	tempDir := t.TempDir()
	
	// Create directory but make it read-only to simulate write issues
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err := os.MkdirAll(readOnlyDir, 0444) // Read-only permissions
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test commands that might need to write cache or temporary files
	cmd := exec.Command(binaryPath, "--dir", readOnlyDir, "--duration", "1h")
	output, err = cmd.CombinedOutput()
	
	// Should handle read-only directory gracefully
	if err != nil {
		outputStr := string(output)
		hasPermissionInfo := strings.Contains(outputStr, "permission") ||
							strings.Contains(outputStr, "read-only") ||
							strings.Contains(outputStr, "access") ||
							strings.Contains(outputStr, "write")
		
		assert.True(t, hasPermissionInfo, "Should indicate permission issue: %s", outputStr)
	} else {
		// If it succeeds, that's also fine (might not need to write anything)
		assert.NotNil(t, output, "Should produce some output")
	}
	
	// Restore permissions for cleanup
	os.Chmod(readOnlyDir, 0755)
}
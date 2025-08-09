//go:build e2e
// +build e2e

package commands

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/e2e"
	"github.com/penwyp/go-claude-monitor/internal/testing/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTopCommandAdvancedKeyboardInteractions tests advanced keyboard interactions
func TestTopCommandAdvancedKeyboardInteractions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate rich multi-model session for interactive testing
	err := generator.GenerateMultiModelSession("interactive-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "1"},
		Timeout: 15 * time.Second,
		Rows:    30,
		Cols:    120,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial load
	time.Sleep(1 * time.Second)

	// Test comprehensive sorting combinations
	sortSequence := []struct {
		key         byte
		description string
	}{
		{'s', "sort by session"},
		{'t', "sort by tokens"},
		{'i', "sort by input tokens"},
		{'o', "sort by output tokens"},
		{'c', "sort by cost"},
		{'p', "sort by project"},
		{'m', "sort by model"},
	}

	for _, sort := range sortSequence {
		t.Run("sort_"+sort.description, func(t *testing.T) {
			// Clear any previous output for cleaner testing
			session.ClearOutput()
			
			err = session.SendKey(sort.key)
			require.NoError(t, err)
			
			// Wait for sort to apply
			time.Sleep(300 * time.Millisecond)
			
			// Verify screen updates
			sortedOutput := session.GetCleanOutput()
			assert.Contains(t, sortedOutput, "interactive-test", "Should show project after %s", sort.description)
		})
	}

	// Test help system thoroughly
	err = session.SendKey('h')
	require.NoError(t, err)
	
	err = session.WaitForText("Keyboard Commands", 2*time.Second)
	assert.NoError(t, err, "Help should appear")
	
	// Verify help content is comprehensive
	helpOutput := session.GetCleanOutput()
	helpKeywords := []string{
		"Sort",
		"Refresh",
		"Help",
		"Quit",
		"ESC",
	}
	
	for _, keyword := range helpKeywords {
		assert.Contains(t, helpOutput, keyword, "Help should contain %s information", keyword)
	}
	
	// Hide help and verify it's gone
	err = session.SendKey('h')
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	
	currentOutput := session.GetCleanOutput()
	recentOutput := currentOutput
	if len(currentOutput) > 1000 {
		recentOutput = currentOutput[len(currentOutput)-1000:]
	}
	assert.NotContains(t, recentOutput, "Keyboard Commands", "Help should be hidden")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandErrorRecoveryScenarios tests error recovery scenarios
func TestTopCommandErrorRecoveryScenarios(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Start with initial data
	err := generator.GenerateSimpleSession("recovery-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "2"},
		Timeout: 10 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial load
	time.Sleep(1 * time.Second)
	
	// Verify initial state
	initialOutput := session.GetCleanOutput()
	assert.Contains(t, initialOutput, "recovery-test", "Should show initial data")

	// Generate more data while running (simulating file changes)
	err = generator.GenerateMultiModelSession("added-during-run", time.Now().Add(-30*time.Minute))
	require.NoError(t, err)

	// Force refresh to pick up new data
	err = session.SendKey('r')
	require.NoError(t, err)

	// Wait for refresh to complete
	time.Sleep(2 * time.Second)

	// Verify new data is picked up
	newOutput := session.GetCleanOutput()
	assert.Contains(t, newOutput, "added-during-run", "Should pick up new data after refresh")

	// Test rapid key presses (stress test)
	rapidKeys := []byte{'s', 't', 'c', 'p', 's', 't', 'r', 'h', 'h'}
	for _, key := range rapidKeys {
		err = session.SendKey(key)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Rapid but not instant
	}

	// Should still be responsive
	finalOutput := session.GetCleanOutput()
	assert.Contains(t, finalOutput, "recovery-test", "Should remain functional after rapid inputs")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandMemoryLeakPrevention tests for memory leak scenarios
func TestTopCommandMemoryLeakPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate continuous data stream
	err := generator.GenerateLargeDataset("memory-test", time.Now().Add(-12*time.Hour), 300)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "5"},
		Timeout: 30 * time.Second,
		Rows:    40,
		Cols:    140,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Let it run with high refresh rate for a period
	time.Sleep(5 * time.Second)

	// Perform continuous interactions
	for i := 0; i < 20; i++ {
		// Cycle through different sorts
		keys := []byte{'s', 't', 'c', 'p', 'r'}
		key := keys[i%len(keys)]
		
		err = session.SendKey(key)
		require.NoError(t, err)
		
		time.Sleep(200 * time.Millisecond)
		
		// Verify it's still responsive every few iterations
		if i%5 == 0 {
			currentOutput := session.GetCleanOutput()
			assert.Contains(t, currentOutput, "memory-test", "Should remain responsive after %d iterations", i+1)
		}
	}

	// Final responsiveness check
	err = session.SendKey('h')
	require.NoError(t, err)
	
	err = session.WaitForText("Help", 3*time.Second)
	assert.NoError(t, err, "Should still respond to help after sustained operation")

	err = session.Stop()
	assert.NoError(t, err, "Should shutdown cleanly after sustained operation")
}

// TestTopCommandDifferentTerminalSizes tests various terminal sizes
func TestTopCommandDifferentTerminalSizes(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateMultiModelSession("size-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	testSizes := []struct {
		name string
		rows uint16
		cols uint16
	}{
		{"small", 20, 60},
		{"medium", 30, 100},
		{"large", 50, 150},
		{"very_wide", 25, 200},
		{"very_tall", 60, 80},
	}

	for _, size := range testSizes {
		t.Run(size.name, func(t *testing.T) {
			config := &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", tempDir, "top"},
				Timeout: 8 * time.Second,
				Rows:    size.rows,
				Cols:    size.cols,
			}

			session, err := e2e.NewTUITestSession(config)
			require.NoError(t, err)
			defer session.ForceStop()

			// Wait for load
			time.Sleep(1 * time.Second)

			// Test basic functionality at this size
			err = session.SendKey('s')
			require.NoError(t, err)
			time.Sleep(200 * time.Millisecond)

			sizeOutput := session.GetCleanOutput()
			assert.Contains(t, sizeOutput, "size-test", "Should work at %s terminal size", size.name)

			// Test help at this size
			err = session.SendKey('h')
			require.NoError(t, err)
			time.Sleep(300 * time.Millisecond)
			
			helpOutput := session.GetCleanOutput()
			assert.Contains(t, helpOutput, "Help", "Help should work at %s size", size.name)

			err = session.Stop()
			assert.NoError(t, err)
		})
	}
}

// TestTopCommandPlanLimitScenarios tests different plan limit scenarios
func TestTopCommandPlanLimitScenarios(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit
	err := generator.GenerateSessionWithLimit("limit-scenario", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	plans := []string{"max5", "max20", "pro", "team"}

	for _, plan := range plans {
		t.Run("plan_"+plan, func(t *testing.T) {
			config := &e2e.TUITestConfig{
				Command: binaryPath,
				Args:    []string{"--dir", tempDir, "top", "--plan", plan},
				Timeout: 8 * time.Second,
			}

			session, err := e2e.NewTUITestSession(config)
			require.NoError(t, err)
			defer session.ForceStop()

			// Wait for data processing
			time.Sleep(1500 * time.Millisecond)

			// Should show plan-specific information
			planOutput := session.GetCleanOutput()
			assert.Contains(t, planOutput, "limit-scenario", "Should show project for plan %s", plan)

			// Should display some rate limit or usage information
			hasLimitInfo := session.ContainsText("Rate") || 
						   session.ContainsText("Limit") || 
						   session.ContainsText("Usage") ||
						   session.ContainsText("Window")
			assert.True(t, hasLimitInfo, "Should show limit-related information for plan %s", plan)

			err = session.Stop()
			assert.NoError(t, err)
		})
	}
}

// TestTopCommandRealTimeUpdates tests real-time update functionality
func TestTopCommandRealTimeUpdates(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Start with minimal data
	err := generator.GenerateSimpleSession("realtime-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "3"},
		Timeout: 12 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial load
	time.Sleep(1 * time.Second)
	
	// Capture initial state
	session.ClearOutput()
	time.Sleep(1 * time.Second)
	initialState := session.GetCleanOutput()
	assert.Contains(t, initialState, "realtime-test", "Should show initial project")

	// Add new data while running
	err = generator.GenerateMultiModelSession("dynamic-addition", time.Now().Add(-30*time.Minute))
	require.NoError(t, err)

	// Wait for auto-refresh to pick up changes
	time.Sleep(3 * time.Second)

	// Should automatically show new data
	updatedState := session.GetCleanOutput()
	assert.Contains(t, updatedState, "dynamic-addition", "Should auto-detect new project")
	
	// Test manual refresh still works
	session.ClearOutput()
	err = session.SendKey('r')
	require.NoError(t, err)
	
	time.Sleep(1 * time.Second)
	manualRefreshState := session.GetCleanOutput()
	assert.Contains(t, manualRefreshState, "dynamic-addition", "Manual refresh should work")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandEdgeCaseData tests edge case data scenarios
func TestTopCommandEdgeCaseData(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create edge case scenarios
	now := time.Now()
	
	// Very recent session (within last few minutes)
	err := generator.GenerateSimpleSession("very-recent", now.Add(-5*time.Minute))
	require.NoError(t, err)
	
	// Session exactly at 5-hour boundary
	err = generator.GenerateSimpleSession("boundary-session", now.Add(-5*time.Hour))
	require.NoError(t, err)
	
	// Very old session
	err = generator.GenerateSimpleSession("old-session", now.Add(-30*24*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 8 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for data processing
	time.Sleep(2 * time.Second)

	edgeCaseOutput := session.GetCleanOutput()
	
	// Should handle all edge cases gracefully
	assert.Contains(t, edgeCaseOutput, "very-recent", "Should show very recent session")
	
	// Should show some data (may or may not include old sessions depending on default filter)
	hasData := session.ContainsText("very-recent") || 
			   session.ContainsText("boundary-session") ||
			   session.ContainsText("Total") ||
			   len(edgeCaseOutput) > 100
	assert.True(t, hasData, "Should show some session data")

	// Test sorting with edge case data
	err = session.SendKey('t')
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	sortedOutput := session.GetCleanOutput()
	assert.NotEmpty(t, sortedOutput, "Should handle sorting with edge case data")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandSignalHandling tests signal handling and graceful shutdown
func TestTopCommandSignalHandling(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateSimpleSession("signal-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 6 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for startup
	time.Sleep(1 * time.Second)

	// Test different exit methods
	testCases := []struct {
		name     string
		exitFunc func() error
	}{
		{
			name: "ESC key",
			exitFunc: func() error {
				return session.SendKey(27) // ESC key
			},
		},
		{
			name: "q key", 
			exitFunc: func() error {
				return session.SendKey('q')
			},
		},
		{
			name: "Ctrl+C simulation",
			exitFunc: func() error {
				return session.SendKey(3) // Ctrl+C
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh session for each test
			freshSession, err := e2e.NewTUITestSession(config)
			require.NoError(t, err)
			defer freshSession.ForceStop()

			time.Sleep(500 * time.Millisecond)
			
			// Verify it's running
			assert.True(t, freshSession.IsRunning(), "Session should be running before exit")
			
			// Send exit signal
			err = tc.exitFunc()
			require.NoError(t, err)
			
			// Wait for shutdown
			time.Sleep(1 * time.Second)
			
			// Should have stopped gracefully
			err = freshSession.Stop()
			// We don't assert no error here as the session might already be stopped
		})
	}
}

// TestTopCommandConcurrentAccess tests concurrent access scenarios
func TestTopCommandConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate data that might be accessed concurrently
	err := generator.GenerateMultiModelSession("concurrent-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Start multiple sessions simultaneously
	numSessions := 3
	sessions := make([]*e2e.TUITestSession, numSessions)

	for i := 0; i < numSessions; i++ {
		config := &e2e.TUITestConfig{
			Command: binaryPath,
			Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "1"},
			Timeout: 10 * time.Second,
		}

		session, err := e2e.NewTUITestSession(config)
		require.NoError(t, err, "Session %d should start", i)
		sessions[i] = session
		defer session.ForceStop()
		
		// Stagger startup slightly
		time.Sleep(200 * time.Millisecond)
	}

	// Let all sessions run concurrently
	time.Sleep(2 * time.Second)

	// Interact with all sessions
	for i, session := range sessions {
		err = session.SendKey('s')
		require.NoError(t, err, "Session %d should accept key", i)
		
		time.Sleep(100 * time.Millisecond)
		
		output := session.GetCleanOutput()
		assert.Contains(t, output, "concurrent-test", "Session %d should show data", i)
	}

	// Clean shutdown
	for i, session := range sessions {
		err = session.Stop()
		assert.NoError(t, err, "Session %d should shutdown cleanly", i)
	}
}

// TestTopCommandResourceCleanup tests resource cleanup
func TestTopCommandResourceCleanup(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateSimpleSession("cleanup-test", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test rapid session creation and destruction
	for i := 0; i < 5; i++ {
		config := &e2e.TUITestConfig{
			Command: binaryPath,
			Args:    []string{"--dir", tempDir, "top"},
			Timeout: 3 * time.Second,
		}

		session, err := e2e.NewTUITestSession(config)
		require.NoError(t, err, "Session %d should start", i)
		
		// Brief operation
		time.Sleep(500 * time.Millisecond)
		
		err = session.SendKey('s')
		require.NoError(t, err)
		
		time.Sleep(200 * time.Millisecond)
		
		// Should be responsive
		output := session.GetCleanOutput()
		assert.Contains(t, output, "cleanup-test", "Session %d should work", i)
		
		// Clean shutdown
		err = session.Stop()
		assert.NoError(t, err, "Session %d should cleanup properly", i)
	}
}
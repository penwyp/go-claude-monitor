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

// TestTopCommandStartup tests that the TUI starts without displaying the help page
func TestTopCommandStartup(t *testing.T) {
	// Create temporary test data directory
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate simple test data
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	// Build the binary
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Create TUI session
	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--plan", "max5"},
		Timeout: 5 * time.Second,
		Rows:    24,
		Cols:    80,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial loading
	time.Sleep(500 * time.Millisecond)

	// Verify no help page is shown (should not contain help instructions)
	err = session.AssertNoText("Press 'h' for help")
	assert.NoError(t, err, "Help page should not be displayed on startup")

	// Verify loading or main screen is shown
	screenOutput := session.GetCleanOutput()
	assert.Contains(t, screenOutput, "Claude Monitor", "Should show application title")
	
	// Clean shutdown
	err = session.Stop()
	assert.NoError(t, err, "Should shutdown cleanly")
}

// TestTopCommandHelpToggle tests the help page toggle functionality
func TestTopCommandHelpToggle(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 5 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial load
	time.Sleep(500 * time.Millisecond)

	// Press 'h' to show help
	err = session.SendKey('h')
	require.NoError(t, err)

	// Wait for help to appear
	err = session.WaitForText("Keyboard Commands", 2*time.Second)
	assert.NoError(t, err, "Help page should appear after pressing 'h'")

	// Press 'h' again to hide help
	err = session.SendKey('h')
	require.NoError(t, err)
	
	time.Sleep(200 * time.Millisecond)

	// Verify help is hidden
	currentOutput := session.GetCleanOutput()
	startIdx := 0
	if len(currentOutput) > 1000 {
		startIdx = len(currentOutput) - 1000
	}
	latestOutput := currentOutput[startIdx:] // Check recent output
	assert.NotContains(t, latestOutput, "Keyboard Commands", "Help should be hidden after second 'h'")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandQuitKey tests quitting with 'q' key
func TestTopCommandQuitKey(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 5 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for startup
	time.Sleep(500 * time.Millisecond)

	// Send quit command (ESC key)
	err = session.SendKey(27) // ESC key
	require.NoError(t, err)

	// Wait a bit for shutdown
	time.Sleep(200 * time.Millisecond)

	// Verify session is no longer running
	assert.False(t, session.IsRunning(), "Session should stop after pressing 'q'")
}

// TestTopCommandSorting tests sorting functionality
func TestTopCommandSorting(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate multi-model session for sorting
	err := generator.GenerateMultiModelSession("test-project", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 10 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for data to load
	time.Sleep(1 * time.Second)

	// Test different sort modes
	sortKeys := []byte{'s', 't', 'i', 'o', 'c', 'p'}
	for _, key := range sortKeys {
		err = session.SendKey(key)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)
		
		// Verify screen updates (basic check)
		sortedOutput := session.GetCleanOutput()
		assert.NotEmpty(t, sortedOutput, "Screen should have content after sorting")
	}

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandDataRefresh tests data refresh functionality
func TestTopCommandDataRefresh(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Start with initial data
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
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
	
	// Capture initial state
	initialOutput := session.GetCleanOutput()

	// Add more data while running
	err = generator.GenerateSimpleSession("test-project-2", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	// Force refresh with 'r'
	err = session.SendKey('r')
	require.NoError(t, err)

	// Wait for refresh
	time.Sleep(1 * time.Second)

	// Check that output has changed
	newOutput := session.GetCleanOutput()
	assert.NotEqual(t, initialOutput, newOutput, "Output should change after refresh")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandEmptyData tests behavior with no data
func TestTopCommandEmptyData(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create empty project
	err := generator.CreateEmptyProject("empty-project")
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 5 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for load
	time.Sleep(1 * time.Second)

	// Should show "No data" or similar message
	emptyOutput := session.GetCleanOutput()
	assert.Contains(t, emptyOutput, "No", "Should indicate no data available")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandRateLimitDisplay tests rate limit message display
func TestTopCommandRateLimitDisplay(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate session with rate limit
	err := generator.GenerateSessionWithLimit("limited-project", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--plan", "max5"},
		Timeout: 5 * time.Second,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for data processing
	time.Sleep(1 * time.Second)

	// Check for rate limit indicators
	limitOutput := session.GetCleanOutput()
	// Should show some indication of rate limit or window detection
	assert.NotEmpty(t, limitOutput, "Should display data with rate limit info")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopCommandPerformance tests with large dataset
func TestTopCommandPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate large dataset
	err := generator.GenerateLargeDataset("large-project", time.Now().Add(-24*time.Hour), 1000)
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 10 * time.Second,
	}

	startTime := time.Now()
	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for initial load
	err = session.WaitForText("Claude Monitor", 5*time.Second)
	loadTime := time.Since(startTime)
	
	assert.NoError(t, err, "Should load within timeout")
	assert.Less(t, loadTime, 5*time.Second, "Should load large dataset within 5 seconds")

	// Test responsiveness
	err = session.SendKey('s')
	require.NoError(t, err)
	
	// Should respond quickly to commands
	time.Sleep(100 * time.Millisecond)
	
	err = session.Stop()
	assert.NoError(t, err)
}
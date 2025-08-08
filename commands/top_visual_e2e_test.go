//go:build e2e
// +build e2e

package commands

import (
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

// TestTopVisualNoHelpFlash replaces test_tui_visual.sh
// Tests that help page doesn't flash on startup and transitions are clean
func TestTopVisualNoHelpFlash(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Generate test data
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	// Build binary
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top", "--plan", "max5", "--timezone", "Asia/Shanghai", "--time-format", "24h"},
		Timeout: 5 * time.Second,
		Rows:    24,
		Cols:    80,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Capture output at different stages
	var outputs []string
	
	// Capture immediately after start (should catch any flash)
	time.Sleep(50 * time.Millisecond)
	outputs = append(outputs, session.GetCleanOutput())
	
	// Capture after 100ms
	time.Sleep(50 * time.Millisecond)
	outputs = append(outputs, session.GetCleanOutput())
	
	// Capture after 200ms
	time.Sleep(100 * time.Millisecond)
	outputs = append(outputs, session.GetCleanOutput())
	
	// Capture after 500ms (should be stable)
	time.Sleep(300 * time.Millisecond)
	outputs = append(outputs, session.GetCleanOutput())

	// Check for help page artifacts in early outputs
	helpKeywords := []string{
		"Press 'h' for help",
		"Keyboard Commands",
		"ESC or q",
		"Sort by",
	}

	// The first few outputs should NOT contain help text
	for i, output := range outputs[:2] {
		for _, keyword := range helpKeywords {
			assert.NotContains(t, output, keyword, 
				"Output at stage %d should not contain help keyword '%s' (help page flash detected)", i, keyword)
		}
	}

	// Should show loading or main content instead
	lastOutput := outputs[len(outputs)-1]
	assert.Contains(t, lastOutput, "Claude Monitor", "Should show main application content")

	// Test that help can be toggled properly after startup
	err = session.SendKey('h')
	require.NoError(t, err)
	
	time.Sleep(200 * time.Millisecond)
	helpOutput := session.GetCleanOutput()
	assert.Contains(t, helpOutput, "Keyboard Commands", "Help should appear when requested")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopVisualCleanTransitions tests clean screen transitions
func TestTopVisualCleanTransitions(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateMultiModelSession("test-project", time.Now().Add(-2*time.Hour))
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

	// Wait for stable state
	time.Sleep(1 * time.Second)
	
	// Test transition when sorting
	beforeSort := session.Screenshot()
	
	err = session.SendKey('s') // Sort by start time
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)
	
	afterSort := session.Screenshot()
	
	// Should have changed but not have artifacts
	assert.NotEqual(t, beforeSort, afterSort, "Screen should update after sorting")
	
	// Check for common artifacts
	artifacts := []string{"\\033[", "\x1b[", "^["}
	for _, artifact := range artifacts {
		assert.NotContains(t, afterSort, artifact, "Should not have escape code artifacts visible")
	}

	err = session.Stop()
	assert.NoError(t, err)
}

// TestTopVisualNoResidualText tests that there's no residual text after operations
func TestTopVisualNoResidualText(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	err := generator.GenerateSimpleSession("test-project", time.Now().Add(-1*time.Hour))
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
	time.Sleep(1 * time.Second)

	// Show help
	err = session.SendKey('h')
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)
	
	helpScreen := session.Screenshot()
	assert.Contains(t, helpScreen, "Keyboard Commands", "Help should be visible")

	// Hide help
	err = session.SendKey('h')
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)
	
	mainScreen := session.Screenshot()
	
	// Should not have help text residuals
	helpLines := strings.Split(helpScreen, "\n")
	mainLines := strings.Split(mainScreen, "\n")
	
	// Check that help-specific lines are completely gone
	for _, helpLine := range helpLines {
		if strings.Contains(helpLine, "Keyboard Commands") || 
		   strings.Contains(helpLine, "ESC or q") {
			assert.NotContains(t, mainScreen, helpLine, 
				"Help text should be completely cleared, not just overwritten")
		}
	}

	// Main screen should be properly restored
	assert.Contains(t, mainScreen, "Claude Monitor", "Main content should be restored")
	
	// Check line count is reasonable (no excessive blank lines)
	nonEmptyLines := 0
	for _, line := range mainLines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}
	assert.Greater(t, nonEmptyLines, 5, "Should have substantial content, not mostly blank")

	err = session.Stop()
	assert.NoError(t, err)
}
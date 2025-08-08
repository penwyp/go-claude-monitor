//go:build e2e
// +build e2e

package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/e2e"
	"github.com/stretchr/testify/require"
)

// TestTopCommandBasicStart tests if the TUI can start at all
func TestTopCommandBasicStart(t *testing.T) {
	// Create empty temp dir (no data)
	tempDir := t.TempDir()

	// Build the binary first if not exists
	if _, err := exec.LookPath("../bin/go-claude-monitor"); err != nil {
		buildCmd := exec.Command("make", "-C", "..", "build")
		if output, err := buildCmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to build binary: %v\n%s", err, output)
		}
	}

	// Use the already-built binary
	binaryPath, err := filepath.Abs("../bin/go-claude-monitor")
	require.NoError(t, err)
	
	// Create TUI session - try using just "top" without --dir
	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"top"},
		Env:     []string{fmt.Sprintf("HOME=%s", tempDir)}, // Try setting HOME instead
		Timeout: 5 * time.Second,
		Rows:    24,
		Cols:    80,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err, "Failed to start TUI session")
	defer session.ForceStop()

	// Just wait a moment to see if it starts
	time.Sleep(500 * time.Millisecond)

	// Check if we got any output
	output := session.GetCleanOutput()
	t.Logf("Got output (len=%d): %s", len(output), output)
	
	// Try to stop gracefully
	err = session.Stop()
	if err != nil {
		t.Logf("Stop error: %v", err)
		// Force stop if graceful didn't work
		session.ForceStop()
	}
}

// TestTopCommandBinaryDirectly tests running the binary directly
func TestTopCommandBinaryDirectly(t *testing.T) {
	tempDir := t.TempDir()
	
	// Run the binary directly with a timeout
	cmd := exec.Command("../bin/go-claude-monitor", "--dir", tempDir, "top")
	
	// Set up a goroutine to kill it after a short time
	done := make(chan error, 1)
	go func() {
		done <- cmd.Start()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected - command started
	}
	
	// Let it run briefly
	time.Sleep(500 * time.Millisecond)
	
	// Kill the process
	if cmd.Process != nil {
		err := cmd.Process.Kill()
		t.Logf("Kill result: %v", err)
	}
}

// TestTopCommandWithMinimalPTY tests with minimal PTY setup
func TestTopCommandWithMinimalPTY(t *testing.T) {
	tempDir := t.TempDir()
	
	// Build test binary
	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build: %s", string(output))
	
	// Try with minimal PTY config
	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 3 * time.Second,
		Rows:    10,  // Smaller terminal
		Cols:    40,
	}
	
	session, err := e2e.NewTUITestSession(config)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer session.ForceStop()
	
	// Check if alive
	time.Sleep(200 * time.Millisecond)
	if !session.IsRunning() {
		t.Fatal("Session died immediately")
	}
	
	// Get some output
	output2 := session.GetOutput()
	t.Logf("Raw output: %q", output2)
	
	// Force stop
	session.ForceStop()
}
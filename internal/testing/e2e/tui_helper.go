package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

// TUITestSession represents a test session for TUI testing
type TUITestSession struct {
	cmd        *exec.Cmd
	ptmx       *os.File
	output     *bytes.Buffer
	outputLock sync.RWMutex
	reader     *bufio.Reader
	writer     io.Writer
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	runningLock sync.Mutex
}

// TUITestConfig contains configuration for TUI testing
type TUITestConfig struct {
	// Command and arguments to run
	Command string
	Args    []string
	
	// Working directory
	WorkDir string
	
	// Environment variables
	Env []string
	
	// Terminal size
	Rows uint16
	Cols uint16
	
	// Timeout for the entire test
	Timeout time.Duration
}

// NewTUITestSession creates a new TUI test session
func NewTUITestSession(config *TUITestConfig) (*TUITestSession, error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Rows == 0 {
		config.Rows = 24
	}
	if config.Cols == 0 {
		config.Cols = 80
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	
	cmd := exec.CommandContext(ctx, config.Command, config.Args...)
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}
	if len(config.Env) > 0 {
		cmd.Env = append(os.Environ(), config.Env...)
	}

	// Start command with PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: config.Rows,
		Cols: config.Cols,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	session := &TUITestSession{
		cmd:    cmd,
		ptmx:   ptmx,
		output: &bytes.Buffer{},
		reader: bufio.NewReader(ptmx),
		writer: ptmx,
		ctx:    ctx,
		cancel: cancel,
		running: true,
	}

	// Start output capture
	go session.captureOutput()

	return session, nil
}

// captureOutput continuously reads from PTY and stores output
func (s *TUITestSession) captureOutput() {
	buf := make([]byte, 4096)
	for s.IsRunning() {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.outputLock.Lock()
			s.output.Write(buf[:n])
			s.outputLock.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				// Log error but continue
			}
			break
		}
	}
}

// SendKey sends a key press to the TUI
func (s *TUITestSession) SendKey(key byte) error {
	if !s.IsRunning() {
		return fmt.Errorf("session not running")
	}
	_, err := s.writer.Write([]byte{key})
	return err
}

// SendString sends a string to the TUI
func (s *TUITestSession) SendString(str string) error {
	if !s.IsRunning() {
		return fmt.Errorf("session not running")
	}
	_, err := s.writer.Write([]byte(str))
	return err
}

// WaitForText waits for specific text to appear in output
func (s *TUITestSession) WaitForText(text string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.ContainsText(text) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for text: %s", text)
}

// WaitForPattern waits for a pattern to appear in output
func (s *TUITestSession) WaitForPattern(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := s.GetOutput()
		if strings.Contains(output, pattern) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for pattern: %s", pattern)
}

// ContainsText checks if the output contains specific text
func (s *TUITestSession) ContainsText(text string) bool {
	s.outputLock.RLock()
	defer s.outputLock.RUnlock()
	return strings.Contains(s.output.String(), text)
}

// GetOutput returns the current output buffer
func (s *TUITestSession) GetOutput() string {
	s.outputLock.RLock()
	defer s.outputLock.RUnlock()
	return s.output.String()
}

// GetCleanOutput returns output with ANSI escape codes removed
func (s *TUITestSession) GetCleanOutput() string {
	output := s.GetOutput()
	return StripANSI(output)
}

// GetLastLines returns the last N lines of output
func (s *TUITestSession) GetLastLines(n int) []string {
	output := s.GetCleanOutput()
	lines := strings.Split(output, "\n")
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// Screenshot captures the current terminal screen state
func (s *TUITestSession) Screenshot() string {
	s.outputLock.RLock()
	defer s.outputLock.RUnlock()
	
	// Parse the output to reconstruct the screen
	screen := ParseTerminalOutput(s.output.String())
	return screen.Render()
}

// IsRunning checks if the session is still running
func (s *TUITestSession) IsRunning() bool {
	s.runningLock.Lock()
	defer s.runningLock.Unlock()
	return s.running
}

// Stop gracefully stops the TUI session
func (s *TUITestSession) Stop() error {
	s.runningLock.Lock()
	s.running = false
	s.runningLock.Unlock()

	// Send quit command (ESC key)
	s.SendKey(27) // ESC
	
	// Wait a bit for graceful shutdown
	time.Sleep(100 * time.Millisecond)
	
	// Cancel context to force stop if needed
	s.cancel()
	
	// Close PTY
	if s.ptmx != nil {
		s.ptmx.Close()
	}
	
	// Wait for command to finish
	return s.cmd.Wait()
}

// ForceStop forcefully terminates the TUI session
func (s *TUITestSession) ForceStop() error {
	s.runningLock.Lock()
	s.running = false
	s.runningLock.Unlock()

	s.cancel()
	if s.ptmx != nil {
		s.ptmx.Close()
	}
	
	// Kill the process if it's still running
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	
	return s.cmd.Wait()
}

// ExpectScreen waits for the screen to match expected content
func (s *TUITestSession) ExpectScreen(expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		screen := s.Screenshot()
		if strings.Contains(screen, expected) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("screen did not match expected content within timeout")
}

// AssertNoText ensures specific text does not appear
func (s *TUITestSession) AssertNoText(text string) error {
	if s.ContainsText(text) {
		return fmt.Errorf("unexpected text found: %s", text)
	}
	return nil
}

// ClearOutput clears the output buffer
func (s *TUITestSession) ClearOutput() {
	s.outputLock.Lock()
	defer s.outputLock.Unlock()
	s.output.Reset()
}
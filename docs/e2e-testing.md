# End-to-End Testing for TUI

## Overview

This document describes the end-to-end (e2e) testing approach for the go-claude-monitor TUI (Terminal User Interface). The e2e tests use pseudo-terminals (PTY) to simulate real terminal interactions, enabling automated testing of the interactive TUI components.

## Why E2E Testing for TUI?

Traditional unit tests cannot adequately test TUI applications because:
- TUIs require a real terminal (TTY) environment
- Keyboard input and screen output need to be tested together
- Visual artifacts and screen transitions are important for user experience
- Interactive features like help toggles and sorting need integration testing

## Technology Stack

### Core Libraries
- **github.com/creack/pty**: Provides pseudo-terminal support for Go
- **testify**: Assertion library for test validation

### Custom Components
- **internal/testing/e2e/tui_helper.go**: TUI test session management
- **internal/testing/e2e/ansi_parser.go**: ANSI escape code parsing and screen rendering
- **internal/testing/fixtures/jsonl_generator.go**: Test data generation

## Test Infrastructure

### TUI Test Helper

The `TUITestSession` provides:
- PTY creation and management
- Keyboard input simulation
- Output capture and parsing
- Screen state verification
- Timeout handling

Key methods:
```go
// Create a test session
session, err := e2e.NewTUITestSession(config)

// Send keyboard input
session.SendKey('q')
session.SendString("search term")

// Wait for content
session.WaitForText("Loading", 2*time.Second)

// Verify screen state
screenshot := session.Screenshot()
session.AssertNoText("error")
```

### ANSI Parser

The ANSI parser handles:
- Escape sequence parsing
- Virtual screen buffer management
- Cursor position tracking
- Screen clearing operations
- Clean text extraction

### Test Fixtures

The fixture generator creates:
- Simple sessions with regular activity
- Sessions with rate limits
- Multi-model sessions
- Large datasets for performance testing
- Empty projects

## Test Categories

### 1. Startup Tests
- Verifies no help page flash on startup
- Checks clean loading screen display
- Validates initial rendering

### 2. Interaction Tests
- Keyboard command processing (q, h, s, etc.)
- Sort functionality
- Help page toggle
- Navigation

### 3. Data Display Tests
- Session data rendering
- Metrics calculation display
- Refresh cycle behavior
- Rate limit indication

### 4. Visual Tests
- Screen transition cleanliness
- No residual text after operations
- No escape code artifacts
- Proper screen clearing

### 5. Performance Tests
- Large dataset handling
- Response time verification
- Memory usage (indirect)

## Running E2E Tests

### Make Commands

```bash
# Run all e2e tests
make test-e2e

# Run quick TUI startup test
make test-tui-quick

# Run all tests (unit + e2e)
make test-all

# Run specific e2e test
go test -v -tags=e2e ./commands -run TestTopCommandStartup
```

### Build Tags

E2E tests use the `e2e` build tag to separate them from unit tests:
```go
//go:build e2e
// +build e2e
```

This ensures e2e tests only run when explicitly requested.

## Writing New E2E Tests

### Basic Structure

```go
func TestTopCommandFeature(t *testing.T) {
    // 1. Setup test data
    tempDir := t.TempDir()
    generator := fixtures.NewTestDataGenerator(tempDir)
    err := generator.GenerateSimpleSession("test-project", time.Now())
    require.NoError(t, err)

    // 2. Build binary
    binaryPath := filepath.Join(t.TempDir(), "test-monitor")
    buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
    err = buildCmd.Run()
    require.NoError(t, err)

    // 3. Create TUI session
    config := &e2e.TUITestConfig{
        Command: binaryPath,
        Args:    []string{"top", "--dir", tempDir},
        Timeout: 5 * time.Second,
    }
    session, err := e2e.NewTUITestSession(config)
    require.NoError(t, err)
    defer session.ForceStop()

    // 4. Perform interactions and assertions
    time.Sleep(500 * time.Millisecond) // Wait for startup
    
    err = session.SendKey('h')
    require.NoError(t, err)
    
    err = session.WaitForText("Help", 2*time.Second)
    assert.NoError(t, err)

    // 5. Clean shutdown
    err = session.Stop()
    assert.NoError(t, err)
}
```

### Best Practices

1. **Use appropriate timeouts**: Allow enough time for operations but fail fast
2. **Clean up resources**: Always use `defer session.ForceStop()`
3. **Build binary once**: Cache binary builds when testing multiple scenarios
4. **Use fixtures**: Generate predictable test data
5. **Test incrementally**: Verify each step before proceeding
6. **Capture screenshots**: Use `session.Screenshot()` for debugging failures

## Migration from Shell Scripts

The e2e tests replace the manual shell scripts:

| Old Script | New E2E Test | Purpose |
|------------|--------------|---------|
| test_tui_fix.sh | TestTopCommandStartup | Verify no help page flash |
| test_tui_visual.sh | TestTopVisualNoHelpFlash | Visual artifact testing |
| (manual testing) | TestTopCommandQuitKey | Quit functionality |
| (manual testing) | TestTopCommandSorting | Sort operations |

## CI/CD Integration

The e2e tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  run: |
    make build
    make test-e2e
```

Note: CI environments must support PTY creation. Most Linux-based CI systems support this.

## Debugging E2E Tests

### Viewing Output
```go
// Print current output for debugging
fmt.Println("Current output:", session.GetCleanOutput())

// Get screenshot of current screen state
fmt.Println("Screen:", session.Screenshot())
```

### Running Individual Tests
```bash
# Run with verbose output
go test -v -tags=e2e ./commands -run TestTopCommandStartup

# Run with timeout override
go test -v -tags=e2e -timeout 30s ./commands
```

### Common Issues

1. **PTY creation fails**: Ensure the test environment supports PTY
2. **Timeouts**: Increase timeouts for slower systems
3. **Binary not found**: Ensure `make build` runs before tests
4. **Screen size issues**: Adjust Rows/Cols in TUITestConfig

## Future Enhancements

Potential improvements to the e2e testing framework:

1. **Record and replay**: Capture sessions for regression testing
2. **Visual regression**: Screenshot comparison between versions
3. **Performance benchmarks**: Automated performance regression detection
4. **Coverage integration**: Measure code coverage during e2e tests
5. **Parallel execution**: Run multiple TUI sessions concurrently
6. **Cross-platform testing**: Ensure Windows and macOS compatibility

## Conclusion

The e2e testing framework provides comprehensive automated testing for the TUI, replacing manual testing scripts with repeatable, CI-friendly tests. This ensures the TUI remains stable and provides a good user experience across updates.
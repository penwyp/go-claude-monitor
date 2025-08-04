# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

go-claude-monitor is a command-line tool for monitoring and analyzing Claude Code usage. It processes JSONL log files from Claude projects, provides real-time monitoring (like Linux `top`), and generates detailed usage reports with cost analysis.

## Development Commands

### Building and Installation

```bash
make build          # Build binary to bin/go-claude-monitor
make install        # Build and install to GOBIN (requires prior build)
make clean          # Remove build artifacts
```

### Testing

```bash
make test           # Run all tests
make coverage       # Generate test coverage report (HTML output in bin/coverage.html)
go test ./internal/session -v    # Run specific package tests
go test -run TestSessionDetection ./internal/session -v  # Run specific test
```

### Code Quality

```bash
make fmt            # Format code with go fmt
make lint           # Run go vet for static analysis
make check          # Run fmt, lint, and test combined
```

### Running the Tool

```bash
# Analyze usage (main command)
./bin/go-claude-monitor --duration 7d --breakdown

# Real-time monitoring (like Linux top)
./bin/go-claude-monitor top --plan max5

# Debug session detection (hidden command)
./bin/go-claude-monitor detect
```

### Release Process

```bash
make release v0.0.1  # Creates git tag and pushes (triggers GitHub Actions)
```

## Architecture

### Core Components

1. **Entry Point** (`/cmd/main.go`): Simple CLI entry that delegates to commands package

2. **Commands** (`/commands`): CLI command implementations using Cobra
    - `root.go`: Main analysis command with time filtering, grouping, and output formatting
    - `top.go`: Real-time monitoring interface with keyboard controls
    - `detect.go`: Hidden debug command for session detection analysis

3. **Session Management** (`/internal/session`): Core logic for Claude session tracking
    - Window detection: Identifies 5-hour session boundaries using limit messages, time gaps, or hour alignment
    - Real-time tracking: Monitors active sessions, calculates burn rates, and projects costs
    - Deduplication: Handles duplicate entries across multiple log files
    - Caching: Memory-based cache for efficient data processing

4. **Data Layer** (`/internal/data`):
    - **Scanner** (`/scanner`): Finds and reads JSONL files concurrently
    - **Parser** (`/parser`): Parses JSONL entries with caching and error resilience
    - **Aggregator** (`/aggregator`): Groups data by time period and model
    - **Cache** (`/cache`): File-based cache with SHA256 hashing

5. **Presentation Layer** (`/internal/presentation`):
    - **Formatter** (`/formatter`): Output formatting (table, JSON, CSV, summary)
    - **Layout** (`/layout`): Terminal UI layout strategies for real-time monitoring
    - **Components** (`/components`): Reusable UI components (headers, tables, status bars)

6. **Business Logic** (`/internal/business`):
    - **Analyzer** (`/analyzer`): Main orchestrator coordinating data processing
    - **Pricing** (`/pricing`): Modular pricing system with provider abstraction
    - **Calculator** (`/calculator`): Token and cost calculations

7. **Utilities** (`/internal/util`):
    - `time.go`: Global timezone-aware time handling via `TimeProvider`
    - `model.go`: Model name simplification and sorting
    - `format.go`: Number, currency, and duration formatting

### Key Design Patterns

- **Concurrent Processing**: Uses worker pools for parallel file processing with goroutines
- **Caching Strategy**: Two-level caching - file-based (analyzer) and in-memory (parser)
- **Time Handling**: All time operations go through `TimeProvider` for consistent timezone support
- **Error Resilience**: Continues processing on partial failures, logs errors for debugging
- **Interface-based Design**: Heavy use of interfaces for testability (e.g., `PricingProvider`)
- **Strategy Pattern**: Layout strategies for different terminal sizes in real-time mode

### Session Window Detection

The tool uses sophisticated logic to detect 5-hour session windows:

1. **Limit Messages** (🎯): Most accurate - extracts reset time from Claude's limit messages
2. **Time Gaps** (⏳): Detects >5 hour gaps between messages
3. **First Message** (📍): Uses first message timestamp for initial sessions
4. **Hour Alignment** (⚪): Fallback - rounds to nearest hour

## Data Directory Structure

The tool expects JSONL files in:

- Default: `~/.claude/projects/*/`
- Custom: Specified via `--dir` flag
- Test data: `/data` folder contains sample JSONL files for verification

## Important Notes

- Use `data` folder to verify source JSONL files during development
- All time operations must use `internal/util/time.go` for consistency
- The `detect` command is hidden but useful for debugging session detection
- Real-time monitoring (`top` command) supports plan-based limits and projections
- Caching significantly improves performance for large datasets
- Tests use `testify` for assertions - follow existing patterns
- JSON processing uses `bytedance/sonic` for performance
- Terminal UI uses raw terminal control codes for efficiency
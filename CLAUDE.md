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

3. **Application Layer** (`/internal/application/top`): Orchestrates top command functionality
    - `orchestrator.go`: Main coordination logic for the real-time monitoring
    - `data_loader.go`: Handles data loading and processing
    - `refresh_controller.go`: Manages refresh and update logic
    - `state_manager.go`: Application state management
    - `config.go`: Configuration for top command
    - `interfaces.go`: Defined interfaces for clean architecture

4. **Core Components** (`/internal/core`):
    - **Session** (`/session`): Core session detection and calculation logic
        - `detector.go`: Detects 5-hour session windows with priority-based logic
        - `calculator.go`: Calculates metrics, burn rates, and projections
        - `window_history.go`: Persists historical window information
        - `limit_parser.go`: Parses rate limit messages
    - **Timeline** (`/timeline`): Unified timeline construction for account-level detection
        - `builder.go`: Builds global timelines from multiple data sources
    - **Cache** (`/cache`): Memory caching layer
        - `memory.go`: In-memory cache with entry management
    - **Monitoring** (`/monitoring`): File system monitoring
        - `watcher.go`: Watches for JSONL file changes

5. **Data Layer** (`/internal/data`):
    - **Scanner** (`/scanner`): Finds and reads JSONL files concurrently
    - **Parser** (`/parser`): Parses JSONL entries with caching and error resilience
    - **Aggregator** (`/aggregator`): Groups data by time period and model
    - **Cache** (`/cache`): File-based cache with SHA256 hashing

6. **Presentation Layer** (`/internal/presentation`):
    - **Formatter** (`/formatter`): Output formatting (table, JSON, CSV, summary)
    - **Layout** (`/layout`): Terminal UI layout strategies for real-time monitoring
    - **Components** (`/components`): Reusable UI components (headers, tables, status bars)
    - **Display** (`/display`): Terminal display logic (moved from session)
    - **Interaction** (`/interaction`): Keyboard input and sorting (moved from session)

7. **Business Logic** (`/internal/business`):
    - **Analyzer** (`/analyzer`): Main orchestrator coordinating data processing
    - **Pricing** (`/pricing`): Modular pricing system with provider abstraction
    - **Calculator** (`/calculator`): Token and cost calculations

8. **Utilities** (`/internal/util`):
    - `time.go`: Global timezone-aware time handling via `TimeProvider`
    - `model.go`: Model name simplification and sorting
    - `format.go`: Number, currency, and duration formatting

### Key Design Patterns

- **Layered Architecture**: Clear separation between core, application, and presentation layers
- **Concurrent Processing**: Uses worker pools for parallel file processing with goroutines
- **Caching Strategy**: Two-level caching - file-based (analyzer) and in-memory (parser)
- **Time Handling**: All time operations go through `TimeProvider` for consistent timezone support
- **Error Resilience**: Continues processing on partial failures, logs errors for debugging
- **Interface-based Design**: Heavy use of interfaces for testability (e.g., `PricingProvider`)
- **Strategy Pattern**: Layout strategies for different terminal sizes in real-time mode
- **Single Responsibility**: Each module has a focused, well-defined purpose (refactored from monolithic Manager)

### Session Window Detection

The tool uses sophisticated logic to detect 5-hour session windows with strict enforcement:

#### Core Principles

1. **Strict 5-Hour Windows**: Each session is exactly 5 hours (EndTime = StartTime + 18000 seconds)
2. **No Session Merging**: Each window is independent, even if adjacent
3. **No Window Overlap**: Windows cannot overlap in time
4. **Limit Messages are Authoritative**: Reset times from limit messages are the most accurate source

#### Detection Priority System

Windows are detected and prioritized as follows:

1. **Priority 10 - Historical Limit Windows** (🎯): Account-level limit windows from history
2. **Priority 9 - Current Limit Messages** (🎯): Newly detected limit messages with reset times
3. **Priority 7 - Historical Account Windows**: Other account-level windows from history
4. **Priority 5 - Time Gaps** (⏳): Detected >5 hour gaps between messages
5. **Priority 3 - First Message** (📍): Uses first message timestamp for initial sessions

#### Account-Level Session Detection

The tool supports account-level session detection, which identifies when multiple projects share the same 5-hour limit window:

- **Global Timeline**: Merges logs from all projects into a unified timeline for accurate cross-project session detection
- **Multi-Project Sessions**: Sessions spanning multiple projects are marked as "Multiple" and tracked as account-level
- **Window History**: Account-level windows (especially from limit messages) are preserved and used to improve future detection
- **No Automatic Merging**: Windows are never merged, maintaining strict 5-hour boundaries
- **Historical Learning**: The tool learns from past limit messages to accurately identify account-wide rate limits

#### Implementation Details

Key files for session detection:

- `internal/core/session/detector.go`: Core detection logic with `detectSessionsFromGlobalTimeline`
- `internal/core/session/window_history.go`: Persistent window history management
- `internal/core/session/timeline_builder.go`: Constructs unified timelines from multiple sources
- `internal/core/session/limit_parser.go`: Parses limit messages and extracts reset times

#### Performance Optimizations

- **Incremental Detection**: Only reprocesses sessions affected by changed files
- **Window Caching**: Caches detected windows to avoid redundant detection
- **Memory Cache**: In-memory cache layer for frequently accessed data
- **Parallel Processing**: Concurrent file processing with worker pools

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
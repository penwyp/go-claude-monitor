# go-claude-monitor

A command-line tool for monitoring and analyzing Claude Code usage, providing detailed cost analysis and real-time
session tracking.

[中文文档](./README_zh.md)

## Features

- 📊 **Usage Analysis**: Analyze Claude Code usage with detailed token and cost breakdowns
- 🔄 **Real-time Monitoring**: Live dashboard similar to Linux `top` command
- 💰 **Cost Tracking**: Track costs by model, project, and time period
- 📈 **Session Detection**: Automatic 5-hour session window detection
- 🚀 **High Performance**: Concurrent processing with intelligent caching

## 🚀 Installation

### Installation Methods

#### Using Homebrew (macOS/Linux)

```bash
brew tap penwyp/go-claude-monitor
brew install go-claude-monitor
```

#### Using Go

```bash
go install github.com/penwyp/go-claude-monitor@latest
```

#### Download Binary

Download the latest release from [GitHub Releases](https://github.com/penwyp/go-claude-monitor/releases) for your
platform.

#### Verify Installation

```bash
go-claude-monitor --version
```

## Quick Start

### Basic Usage Analysis

```bash
# Analyze all usage with default settings
go-claude-monitor

# Analyze last 7 days with cost breakdown
go-claude-monitor --duration 7d --breakdown

# Output as JSON
go-claude-monitor --output json

# Clear cache and re-analyze
go-claude-monitor --reset
```

### Real-time Monitoring

```bash
# Monitor with default settings
go-claude-monitor top

# Monitor with specific plan limits
go-claude-monitor top --plan max5

# Use specific timezone
go-claude-monitor top --timezone Asia/Shanghai
```

## Command Options

### Analysis Command (default)

| Option        | Short | Description                                 | Default              |
|---------------|-------|---------------------------------------------|----------------------|
| `--dir`       |       | Claude project directory                    | `~/.claude/projects` |
| `--duration`  | `-d`  | Time duration (e.g., 7d, 2w, 1m)            | All time             |
| `--output`    | `-o`  | Output format (table, json, csv, summary)   | `table`              |
| `--breakdown` | `-b`  | Show model cost breakdown                   | `false`              |
| `--group-by`  |       | Group by (model, project, day, week, month) | `day`                |
| `--timezone`  |       | Timezone (e.g., UTC, Asia/Shanghai)         | `Local`              |

### Top Command

| Option           | Description                          | Default  |
|------------------|--------------------------------------|----------|
| `--plan`         | Plan type (pro, max5, max20, custom) | `custom` |
| `--refresh-rate` | Data refresh interval in seconds     | `10`     |
| `--timezone`     | Timezone setting                     | `Local`  |

## Examples

### Time-based Analysis

```bash
# Last 24 hours
go-claude-monitor --duration 24h

# Last week
go-claude-monitor --duration 7d

# Last month with daily breakdown
go-claude-monitor --duration 1m --group-by day
```

### Output Formats

```bash
# Table format (default)
go-claude-monitor

# JSON for programmatic use
go-claude-monitor --output json > usage.json

# CSV for spreadsheets
go-claude-monitor --output csv > usage.csv

# Summary only
go-claude-monitor --output summary
```

### Grouping and Sorting

```bash
# Group by model
go-claude-monitor --group-by model

# Group by project
go-claude-monitor --group-by project
```

## Session Windows

Claude Code uses 5-hour session windows. This tool automatically detects session boundaries using:

- 🎯 **Limit messages** from Claude
- ⏳ **Time gaps** greater than 5 hours
- 📍 **First message** timestamps
- ⚪ **Hour alignment** (fallback)

## Development

```bash
# Run tests
make test

# Format code
make fmt

# Run linter
make lint

# Generate coverage report
make coverage
```

## License

MIT License

## Author

[penwyp](https://github.com/penwyp)
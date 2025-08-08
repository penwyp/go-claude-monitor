package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/penwyp/go-claude-monitor/internal/analyzer"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"github.com/spf13/cobra"
)

var (
	// Logging related
	debug bool

	// Data path
	dataDir string

	// Output related
	outputFormat string
	timezone     string

	// Filtering and grouping
	duration  string
	groupBy   string
	limit     int
	breakdown bool
	reset     bool

	// Pricing related
	pricingSource      string
	pricingOfflineMode bool

	rootCmd = &cobra.Command{
		Use:   "go-claude-monitor [flags]",
		Short: "Claude Code usage monitoring tool",
		Long: `go-claude-monitor is a command-line tool for monitoring and analyzing Claude Code usage.

This tool scans JSONL files in the Claude project directory, analyzes API usage, and generates detailed reports.

Examples:
  go-claude-monitor                                    # Analyze with default settings
  go-claude-monitor --dir /path/to/claude/projects      # Analyze specified directory
  go-claude-monitor --output json --group-by model      # Output in JSON format, grouped by model
  go-claude-monitor --duration 12h                     # Analyze last 12 hours
  go-claude-monitor --duration 7d                      # Analyze last 7 days
  go-claude-monitor --duration 2w3d                    # Analyze last 2 weeks and 3 days
  go-claude-monitor --duration 1d12h                   # Analyze last 1 day and 12 hours
  go-claude-monitor --duration 1m --breakdown          # Analyze last month with cost breakdown`,
		RunE: runAnalyze,
	}
)

const (
	defaultLogFile  = "~/.go-claude-monitor/logs/app.log"
	defaultCacheDir = "~/.go-claude-monitor/cache"
	defaultDataDir  = "~/.claude/projects"
)

func init() {
	// Input data configuration
	rootCmd.PersistentFlags().StringVar(&dataDir, "dir", defaultDataDir,
		"Claude project directory path")

	// Time filtering
	rootCmd.Flags().StringVarP(&duration, "duration", "d", "",
		"Time duration to look back (e.g., 12h, 7d, 2w, 1m, 3m2w1d, 1d12h)")

	// Data organization and analysis
	rootCmd.Flags().StringVar(&groupBy, "group-by", "day",
		"Group by field (model, project, day, week, month, hour)")
	rootCmd.Flags().IntVar(&limit, "limit", 0,
		"Limit result count (0 = unlimited)")
	rootCmd.Flags().BoolVarP(&breakdown, "breakdown", "b", false,
		"Show model cost breakdown")

	// Output configuration
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "table",
		"Output format (table, json, csv, summary)")
	rootCmd.Flags().StringVar(&outputFormat, "format", "",
		"Alias for --output")
	rootCmd.Flags().StringVar(&timezone, "timezone", "Local",
		"Timezone setting (e.g., Asia/Shanghai, UTC)")

	// System and debugging
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"Enable debug mode")
	rootCmd.Flags().BoolVarP(&reset, "reset", "r", false,
		"Clear cache before analysis")

	// Pricing configuration
	rootCmd.Flags().StringVar(&pricingSource, "pricing-source", "default",
		"Pricing source (default, litellm)")
	rootCmd.Flags().BoolVar(&pricingOfflineMode, "pricing-offline", false,
		"Use offline pricing mode")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Determine log level based on debug flag
	logLevel := "info"
	if debug {
		logLevel = "debug"
	}

	// Handle format alias
	if format := cmd.Flags().Lookup("format"); format != nil && format.Changed {
		outputFormat = format.Value.String()
	}

	// Initialize logging
	logFile := expandPath(defaultLogFile)
	ensureDir(filepath.Dir(logFile))
	util.InitLogger(logLevel, logFile, debug)
	util.InitializeTimeProvider(timezone)

	// Expand paths
	dataDir = expandPath(dataDir)
	cacheDir := expandPath(defaultCacheDir)

	// Ensure cache directory exists
	if err := ensureDir(cacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Clear cache if needed
	if reset {
		if err := clearCache(cacheDir); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		util.LogInfo("Cache cleared")
	}

	// Create analyzer config
	config := &analyzer.Config{
		DataDir:            dataDir,
		CacheDir:           cacheDir,
		OutputFormat:       outputFormat,
		Timezone:           timezone,
		Duration:           duration,
		GroupBy:            groupBy,
		Limit:              limit,
		Breakdown:          breakdown,
		Concurrency:        runtime.NumCPU(),
		PricingSource:      pricingSource,
		PricingOfflineMode: pricingOfflineMode,
	}

	// Create and run analyzer
	a := analyzer.New(config)
	return a.Run()
}

func Execute() error {
	return rootCmd.Execute()
}

// Helper functions

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func clearCache(cacheDir string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			path := filepath.Join(cacheDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}

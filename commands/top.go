package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"github.com/spf13/cobra"
)

var (
	// Plan related flags
	topPlan              string
	topCustomLimitTokens int

	// Display related flags
	topTimezone         string
	topTimeFormat       string
	topRefreshRate      int
	topRefreshPerSecond float64

	// Pricing related flags
	topPricingSource      string
	topPricingOfflineMode bool
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Monitor Claude Code usage in real-time",
	Long: `Similar to Linux top command, displays active Claude sessions in real-time,
including token usage, cost, rates, and other key metrics.

Session definition:
- Session duration: 5-hour window
- Session start: First message timestamp rounded down to hour
- Supports tracking multiple concurrent sessions`,
	RunE: runTop,
}

func init() {
	rootCmd.AddCommand(topCmd)

	// Plan flags
	topCmd.Flags().StringVar(&topPlan, "plan", "custom",
		"Plan type (pro, max5, max20, custom)")
	topCmd.Flags().IntVar(&topCustomLimitTokens, "custom-limit-tokens", 0,
		"Token limit for custom plan")

	// Display flags
	topCmd.Flags().StringVar(&topTimezone, "timezone", "Local",
		"Timezone setting (e.g., Asia/Shanghai, UTC)")
	topCmd.Flags().StringVar(&topTimeFormat, "time-format", "24h",
		"Time format (12h or 24h)")
	topCmd.Flags().IntVar(&topRefreshRate, "refresh-rate", 10,
		"Data refresh rate in seconds")
	topCmd.Flags().Float64Var(&topRefreshPerSecond, "refresh-per-second", 0.75,
		"Display refresh rate (0.1-20 Hz)")

	// Pricing flags
	topCmd.Flags().StringVar(&topPricingSource, "pricing-source", "default",
		"Pricing source (default, litellm)")
	topCmd.Flags().BoolVar(&topPricingOfflineMode, "pricing-offline", false,
		"Use offline pricing mode")
}

func runTop(cmd *cobra.Command, args []string) error {
	// Handle debug mode (inherited from root command)
	logLevel := "info"
	if debug {
		logLevel = "debug"
	}

	// Initialize logging (reuse root logic)
	logFile := expandPath(defaultLogFile)
	ensureDir(filepath.Dir(logFile))
	util.InitLogger(logLevel, logFile, debug)
	util.InitializeTimeProvider(topTimezone)

	// Handle timezone
	if topTimezone == "auto" {
		topTimezone = "Local"
	}

	// Validate refresh rate
	if topRefreshPerSecond < 0.1 || topRefreshPerSecond > 20 {
		return fmt.Errorf("refresh-per-second must be between 0.1 and 20")
	}

	// Create configuration
	config := &session.TopConfig{
		DataDir:             expandPath(dataDir),
		CacheDir:            expandPath(defaultCacheDir),
		Plan:                topPlan,
		CustomLimitTokens:   topCustomLimitTokens,
		Timezone:            topTimezone,
		TimeFormat:          topTimeFormat,
		DataRefreshInterval: time.Duration(topRefreshRate) * time.Second,
		UIRefreshRate:       topRefreshPerSecond,
		Concurrency:         runtime.NumCPU(),
		PricingSource:       topPricingSource,
		PricingOfflineMode:  topPricingOfflineMode,
	}

	// Run with framework switching support
	for {
		// Create session manager
		manager := session.NewManager(config)

		// Set up signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)

		go func() {
			<-sigChan
			cancel()
		}()

		// Run main loop
		err := manager.Run(ctx)

		return err
	}
}

package commands

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/application/top"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"github.com/spf13/cobra"
)

var (
	// Detect command flags
	detectPlan           string
	detectTimezone       string
	detectPricingSource  string
	detectPricingOffline bool
	detectResetWindows   bool
)

var detectCmd = &cobra.Command{
	Use:    "detect",
	Short:  "Debug command to analyze sessions and print results",
	Long:   `Analyzes Claude sessions and prints comprehensive metrics to console without UI.`,
	Hidden: true, // Hidden from help
	RunE:   runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)

	// Plan flags
	detectCmd.Flags().StringVar(&detectPlan, "plan", "max5",
		"Plan type (pro, max5, max20, custom)")

	// Display flags
	detectCmd.Flags().StringVar(&detectTimezone, "timezone", "Local",
		"Timezone setting (e.g., Asia/Shanghai, UTC)")

	// Pricing flags
	detectCmd.Flags().StringVar(&detectPricingSource, "pricing-source", "default",
		"Pricing source (default, litellm)")
	detectCmd.Flags().BoolVar(&detectPricingOffline, "pricing-offline", false,
		"Use offline pricing mode")
	
	// Window history flags
	detectCmd.Flags().BoolVar(&detectResetWindows, "reset-windows", false,
		"Reset window history before analysis")

}

func runDetect(cmd *cobra.Command, args []string) error {
	// Handle debug mode (inherited from root command)
	logLevel := "info"
	if debug {
		logLevel = "debug"
	}

	// Initialize logging (reuse root logic)
	logFile := expandPath(defaultLogFile)
	ensureDir(filepath.Dir(logFile))
	util.InitLogger(logLevel, logFile, debug)
	util.InitializeTimeProvider(detectTimezone)
	
	// Handle window history reset if requested
	if detectResetWindows {
		if err := resetWindowHistoryQuiet(); err != nil {
			return fmt.Errorf("failed to reset window history: %w", err)
		}
	}

	// Create configuration
	config := &top.TopConfig{
		DataDir:             expandPath(dataDir),
		CacheDir:            expandPath(defaultCacheDir),
		Plan:                detectPlan,
		Timezone:            detectTimezone,
		TimeFormat:          "24h",            // Always use 24h for detect
		DataRefreshInterval: 10 * time.Second, // Not used in detect
		UIRefreshRate:       1.0,              // Not used in detect
		Concurrency:         runtime.NumCPU(),
		PricingSource:       detectPricingSource,
		PricingOfflineMode:  detectPricingOffline,
	}

	// Create orchestrator
	orchestrator, err := top.NewOrchestrator(config)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Load and analyze data
	planLimit := pricing.GetPlan(detectPlan)
	fmt.Println(util.FormatSectionSeparator())
	fmt.Println(util.FormatHeaderTitle("=== Claude Monitor Session Detection ==="))
	fmt.Printf("Timestamp: %s\n", util.GetTimeProvider().Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf("Plan: %s, Cost Limit: %v, Token Limit:%v\n", detectPlan, planLimit.CostLimit, util.FormatNumber(planLimit.TokenLimit))
	fmt.Println(util.FormatSectionSeparator())

	// Load and analyze sessions
	sessions, err := orchestrator.LoadAndAnalyzeData()
	if err != nil {
		return fmt.Errorf("failed to load and analyze data: %w", err)
	}

	// Get aggregated metrics
	aggregated := orchestrator.GetAggregatedMetrics(sessions)

	// Print window detection analysis
	printWindowAnalysis(sessions)
	fmt.Println(util.FormatSectionSeparator())

	// Print results
	printSummary(aggregated, len(sessions))
	fmt.Println(util.FormatSectionSeparator())

	printSessions(sessions, aggregated, config.Timezone, config.TimeFormat)
	fmt.Println(util.FormatSectionSeparator())
	printModelStatistics(aggregated)
	fmt.Println(util.FormatSectionSeparator())

	return nil
}

// getWindowIcon returns an icon based on the window detection source
func getWindowIcon(source string) string {
	switch source {
	case "limit_message":
		return "🎯"
	case "gap":
		return "⏳"
	case "first_message":
		return "📍"
	default:
		return "⚪"
	}
}

// printWindowAnalysis prints detailed window detection analysis
func printWindowAnalysis(sessions []*session.Session) {
	fmt.Println(util.FormatDiagnosticTitle("=== Window Detection Analysis ==="))

	detectedCount := 0
	limitMessageCount := 0
	gapDetectionCount := 0
	firstMessageCount := 0
	roundedHourCount := 0

	for _, sess := range sessions {
		if sess.IsGap {
			continue
		}

		if sess.IsWindowDetected {
			detectedCount++
			switch sess.WindowSource {
			case "limit_message":
				limitMessageCount++
			case "gap":
				gapDetectionCount++
			case "first_message":
				firstMessageCount++
			}
		} else {
			roundedHourCount++
		}
	}

	fmt.Printf("Total Sessions: %d (excluding gaps)\n", len(sessions)-countGaps(sessions))
	fmt.Printf("Windows Detected: %d (%.1f%%)\n", detectedCount,
		float64(detectedCount)/float64(len(sessions)-countGaps(sessions))*100)

	if detectedCount > 0 {
		fmt.Println("\nDetection Methods:")
		if limitMessageCount > 0 {
			fmt.Printf("  🎯 Limit Messages: %d sessions\n", limitMessageCount)
		}
		if gapDetectionCount > 0 {
			fmt.Printf("  ⏳ Time Gaps: %d sessions\n", gapDetectionCount)
		}
		if firstMessageCount > 0 {
			fmt.Printf("  📍 First Message: %d sessions\n", firstMessageCount)
		}
	}
	if roundedHourCount > 0 {
		fmt.Printf("  ⚪ Rounded Hour: %d sessions (fallback)\n", roundedHourCount)
	}

	// Show gap analysis
	gapCount := countGaps(sessions)
	if gapCount > 0 {
		fmt.Printf("\nGap Sessions: %d detected\n", gapCount)
		for i, sess := range sessions {
			if sess.IsGap && i > 0 && i < len(sessions)-1 {
				gapDuration := time.Unix(sess.EndTime, 0).Sub(time.Unix(sess.StartTime, 0))
				fmt.Printf("  Gap between session %d and %d: %s\n",
					i, i+2, util.FormatDuration(gapDuration))
			}
		}
	}
	
	// Show window history stats
	printWindowHistoryStats()
}

// printWindowHistoryStats displays window history statistics
func printWindowHistoryStats() {
	// Get home directory for display
	homeDir, _ := os.UserHomeDir()
	historyPath := filepath.Join(homeDir, ".go-claude-monitor", "history", "window_history.json")
	
	fmt.Println("\nWindow History:")
	fmt.Printf("  File: %s\n", historyPath)
	
	// Check if file exists
	if info, err := os.Stat(historyPath); err == nil {
		fmt.Printf("  Size: %d bytes\n", info.Size())
		fmt.Printf("  Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
		
		// Try to get window history stats through detector
		// Note: This is a simplified approach since we can't directly access the windowHistory
		fmt.Println("  Use --reset-windows flag to clear history")
	} else if os.IsNotExist(err) {
		fmt.Println("  Status: No history file found")
	} else {
		fmt.Printf("  Status: Error accessing file: %v\n", err)
	}
}

// countGaps counts the number of gap sessions
func countGaps(sessions []*session.Session) int {
	count := 0
	for _, sess := range sessions {
		if sess.IsGap {
			count++
		}
	}
	return count
}

func printSummary(aggregated *model.AggregatedMetrics, sessionCount int) {
	activeSessions := 0
	for i := 0; i < sessionCount; i++ {
		// Count active sessions (this is simplified, would need actual session data)
		activeSessions++
	}

	fmt.Println(util.FormatOverviewTitle("=== Summary ==="))
	fmt.Printf("Active Sessions: %d\n", aggregated.ActiveSessions)
	fmt.Printf("Total Sessions: %d\n", aggregated.TotalSessions)
	fmt.Printf("Total Cost: %s\n", util.FormatCurrency(aggregated.TotalCost))
	fmt.Printf("Total Tokens: %s\n", util.FormatNumber(aggregated.TotalTokens))
	fmt.Printf("Total Messages: %d\n", aggregated.TotalMessages)

	if aggregated.AverageBurnRate > 0 {
		fmt.Printf("Average Burn Rate: %s\n", util.FormatBurnRate(aggregated.AverageBurnRate))
	}

	// Show limit status
	fmt.Println("\nLimit Status:")
	if aggregated.CostLimit > 0 {
		costPercentage := (aggregated.TotalCost / aggregated.CostLimit) * 100
		fmt.Printf("  Cost: %s/%s (%.1f%%)\n",
			util.FormatCurrency(aggregated.TotalCost),
			util.FormatCurrency(aggregated.CostLimit),
			costPercentage)
	}
	if aggregated.TokenLimit > 0 {
		tokenPercentage := float64(aggregated.TotalTokens) / float64(aggregated.TokenLimit) * 100
		fmt.Printf("  Tokens: %s/%s (%.1f%%)\n",
			util.FormatNumber(aggregated.TotalTokens),
			util.FormatNumber(aggregated.TokenLimit),
			tokenPercentage)
	}
	if aggregated.MessageLimit > 0 {
		messagePercentage := float64(aggregated.TotalMessages) / float64(aggregated.MessageLimit) * 100
		fmt.Printf("  Messages: %d/%d (%.1f%%)\n",
			aggregated.TotalMessages,
			aggregated.MessageLimit,
			messagePercentage)
	}

	// Show limit exceeded status
	if aggregated.LimitExceeded {
		fmt.Printf("\n⚠️  %s\n", aggregated.LimitExceededReason)
	}

	// Show predicted end time
	if aggregated.PredictedEndTime > 0 && aggregated.HasActiveSession {
		predictedEnd := time.Unix(aggregated.PredictedEndTime, 0)
		timeToPredictedEnd := predictedEnd.Sub(time.Now())
		fmt.Printf("\nPredicted End Time: %s", predictedEnd.Format("2006-01-02 15:04:05 MST"))
		if timeToPredictedEnd > 0 {
			fmt.Printf(" (in %s)\n", util.FormatDuration(timeToPredictedEnd))
		} else {
			fmt.Printf(" (reached)\n")
		}
	}
}

func printSessions(sessions []*session.Session, aggregated *model.AggregatedMetrics, timezone, timeFormat string) {
	fmt.Println(util.FormatDataTitle("=== Sessions ==="))

	for i, sess := range sessions {
		status := "COMPLETED"
		if sess.IsActive {
			status = "ACTIVE"
		} else if sess.IsGap {
			status = "GAP"
		}

		fmt.Printf("Session #%d [%s]\n", i+1, status)
		fmt.Printf("  ID: %s\n", sess.ID)
		fmt.Printf("  Project: %s\n", sess.ProjectName)

		startTime := time.Unix(sess.StartTime, 0)
		startHour := time.Unix(sess.StartHour, 0)
		endTime := time.Unix(sess.EndTime, 0)
		fmt.Printf("  Start: %s\n", startTime.Format("2006-01-02 15:04:05"))
		if sess.StartHour != sess.StartTime && sess.StartHour > 0 {
			fmt.Printf("  StartHour: %s\n", startHour.Format("2006-01-02 15:04:05"))
		}

		if sess.IsActive {
			if sess.PredictedEndTime > 0 {
				predictedEnd := time.Unix(sess.PredictedEndTime, 0)
				timeToPredicted := predictedEnd.Sub(time.Now())
				fmt.Printf("  End: %s (projected", predictedEnd.Format("2006-01-02 15:04:05"))
				if timeToPredicted > 0 {
					fmt.Printf(", in %s)\n", util.FormatDuration(timeToPredicted))
				} else {
					fmt.Printf(", reached)\n")
				}
			} else {
				fmt.Printf("  End: %s\n", endTime.Format("2006-01-02 15:04:05"))
			}
		} else {
			fmt.Printf("  End: %s\n", endTime.Format("2006-01-02 15:04:05"))
		}

		// Window Detection Information
		fmt.Println("  \n  Window Detection:")
		if sess.IsWindowDetected {
			windowIcon := getWindowIcon(sess.WindowSource)
			fmt.Printf("    Status: %s Detected via %s\n", windowIcon, sess.WindowSource)
			if sess.WindowStartTime != nil {
				windowStart := time.Unix(*sess.WindowStartTime, 0)
				fmt.Printf("    Window Start: %s (exact)\n", windowStart.Format("2006-01-02 15:04:05"))
			}
		} else {
			fmt.Printf("    Status: ⚪ Using rounded hour alignment\n")
			fmt.Printf("    Window Start: %s (estimated)\n", startTime.Format("2006-01-02 15:04:05"))
		}

		// First Entry Time (for sliding window analysis)
		if sess.FirstEntryTime > 0 {
			firstEntry := time.Unix(sess.FirstEntryTime, 0)
			fmt.Printf("    First Message: %s\n", firstEntry.Format("2006-01-02 15:04:05"))
		}

		// Reset Time Information
		resetTime := time.Unix(sess.EndTime, 0)
		timeUntilReset := resetTime.Sub(time.Now())
		if timeUntilReset > 0 {
			fmt.Printf("    Reset Time: %s (in %s)\n", resetTime.Format("2006-01-02 15:04:05"), util.FormatDuration(timeUntilReset))
		} else {
			fmt.Printf("    Reset Time: %s (expired)\n", resetTime.Format("2006-01-02 15:04:05"))
		}

		// Limit Messages (if any)
		if len(sess.LimitMessages) > 0 {
			fmt.Printf("    Limit Messages: %d detected\n", len(sess.LimitMessages))
			for j, limitMsg := range sess.LimitMessages {
				msgType, typeOk := limitMsg["type"]
				timestamp, tsOk := limitMsg["timestamp"]
				if typeOk && tsOk {
					var msgTime time.Time
					switch ts := timestamp.(type) {
					case float64:
						msgTime = time.Unix(int64(ts), 0)
					case int64:
						msgTime = time.Unix(ts, 0)
					default:
						continue
					}
					fmt.Printf("      %d. [%s] at %s\n", j+1, msgType, msgTime.Format("15:04:05"))

					if resetTimeVal, ok := limitMsg["resetTime"]; ok {
						switch rt := resetTimeVal.(type) {
						case float64:
							if rt > 0 {
								resetTime := time.Unix(int64(rt), 0)
								fmt.Printf("         Reset at: %s\n", resetTime.Format("2006-01-02 15:04:05"))
							}
						case int64:
							if rt > 0 {
								resetTime := time.Unix(rt, 0)
								fmt.Printf("         Reset at: %s\n", resetTime.Format("2006-01-02 15:04:05"))
							}
						}
					}
				}
			}
		}

		// Duration
		duration := time.Duration(0)
		if sess.ActualEndTime != nil {
			duration = time.Unix(*sess.ActualEndTime, 0).Sub(startTime)
		} else if sess.IsActive {
			duration = time.Since(startTime)
		} else {
			duration = endTime.Sub(startTime)
		}
		fmt.Printf("  Duration: %s\n", util.FormatDuration(duration))

		fmt.Println("  \n  Metrics:")
		tokenPercentage := 0.0
		if aggregated.TotalTokens > 0 {
			tokenPercentage = float64(sess.TotalTokens) / float64(aggregated.TotalTokens) * 100
		}
		fmt.Printf("    Tokens: %s (%.1f%% of total)\n",
			util.FormatNumber(sess.TotalTokens),
			tokenPercentage)

		costPercentage := 0.0
		if aggregated.TotalCost > 0 {
			costPercentage = sess.TotalCost / aggregated.TotalCost * 100
		}
		fmt.Printf("    Cost: %s (%.1f%% of total)\n",
			util.FormatCurrency(sess.TotalCost),
			costPercentage)
		fmt.Printf("    Messages: %d\n", sess.MessageCount)

		if sess.CostPerHour > 0 {
			fmt.Printf("    Cost Burn Rate: %s/hour\n", util.FormatCurrency(sess.CostPerHour))
		}

		// Model distribution
		if len(sess.ModelDistribution) > 0 {
			fmt.Println("    \n  Model Distribution:")
			models := make([]string, 0)
			for model := range sess.ModelDistribution {
				models = append(models, model)
			}
			for _, model := range models {
				stats := sess.ModelDistribution[model]
				percentage := float64(stats.Tokens) / float64(sess.TotalTokens) * 100
				fmt.Printf("    %s: %.0f%% (%s tokens, %s)\n",
					model, percentage,
					util.FormatNumber(stats.Tokens),
					util.FormatCurrency(stats.Cost))
			}
		}

		// Progress for active sessions
		if sess.IsActive {
			fmt.Println("  \n  Active Session Info:")
			if sess.TimeRemaining > 0 {
				fmt.Printf("    Time Until Reset: %s\n", util.FormatDuration(sess.TimeRemaining))
			}

			// Show burn rates
			if sess.CostPerMinute > 0 {
				fmt.Printf("    Cost Burn Rate: %s/min (%s/hour)\n",
					util.FormatCurrency(sess.CostPerMinute),
					util.FormatCurrency(sess.CostPerHour))
			}
			if sess.TokensPerMinute > 0 {
				fmt.Printf("    Token Burn Rate: %.1f tokens/min\n", sess.TokensPerMinute)
			}

			// Show projections
			if sess.ProjectedTokens > 0 {
				fmt.Printf("    Projected Total: %s tokens, %s\n",
					util.FormatNumber(sess.ProjectedTokens),
					util.FormatCurrency(sess.ProjectedCost))
			}

			// Show limit status for this session
			if aggregated != nil {
				if aggregated.CostLimit > 0 {
					costUsage := (sess.TotalCost / aggregated.CostLimit) * 100
					fmt.Printf("    Cost Usage: %.1f%% of limit\n", costUsage)
				}
				if aggregated.TokenLimit > 0 {
					tokenUsage := float64(sess.TotalTokens) / float64(aggregated.TokenLimit) * 100
					fmt.Printf("    Token Usage: %.1f%% of limit\n", tokenUsage)
				}
			}
		}

		fmt.Println()
	}
}

func printModelStatistics(aggregated *model.AggregatedMetrics) {
	if len(aggregated.ModelDistribution) == 0 {
		return
	}

	fmt.Println(util.FormatDataTitle("=== Model Statistics ==="))
	models := make([]string, 0)
	for model := range aggregated.ModelDistribution {
		models = append(models, model)
	}
	sort.Strings(models)
	for _, model := range models {
		stats := aggregated.ModelDistribution[model]
		fmt.Printf("%s:\n", model)
		fmt.Printf("  Total Tokens: %s\n", util.FormatNumber(stats.Tokens))
		fmt.Printf("  Total Cost: %s\n", util.FormatCurrency(stats.Cost))
		fmt.Printf("  Messages: %d\n", stats.Count)
		if stats.Count > 0 {
			avgTokens := stats.Tokens / stats.Count
			fmt.Printf("  Average per Message: %s tokens\n", util.FormatNumber(avgTokens))
		}
		fmt.Println()
	}
}

// resetWindowHistoryQuiet resets window history without prompting
func resetWindowHistoryQuiet() error {
	// Get history file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	
	historyPath := filepath.Join(homeDir, ".go-claude-monitor", "history", "window_history.json")
	
	// Check if file exists
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		fmt.Println("No window history found. Nothing to reset.")
		return nil
	}
	
	// Instead of removing the file, preserve limit_message entries
	// Create a temporary window history manager to load and filter
	tempManager := session.NewWindowHistoryManager(filepath.Join(homeDir, ".go-claude-monitor", "cache"))
	if err := tempManager.Load(); err != nil {
		// If can't load, just remove the file
		if err := os.Remove(historyPath); err != nil {
			return fmt.Errorf("failed to remove window history: %w", err)
		}
		fmt.Println("Window history reset successfully.")
		return nil
	}
	
	// Get all limit-reached windows from the last 3 days
	limitWindows := tempManager.GetLimitReachedWindows()
	currentTime := time.Now().Unix()
	minTime := currentTime - 3*24*3600 // 3 days
	
	// Filter to keep only recent limit_message windows
	var preservedWindows []session.WindowRecord
	for _, window := range limitWindows {
		if window.EndTime >= minTime && window.Source == "limit_message" {
			preservedWindows = append(preservedWindows, window)
		}
	}
	
	fmt.Printf("Preserving %d limit_message entries from the last 3 days.\n", len(preservedWindows))
	
	// Create new window history with only preserved entries
	newManager := session.NewWindowHistoryManager(filepath.Join(homeDir, ".go-claude-monitor", "cache"))
	for _, window := range preservedWindows {
		newManager.AddOrUpdateWindow(window)
	}
	
	// Save the new history
	if err := newManager.Save(); err != nil {
		return fmt.Errorf("failed to save cleared window history: %w", err)
	}
	
	fmt.Println("Window history reset successfully, limit messages preserved.")
	return nil
}

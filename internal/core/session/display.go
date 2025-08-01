package session

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/presentation/layout"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
)

type TerminalDisplay struct {
	config               *TopConfig
	lastDraw             int64
	inAlternateScreen    bool
	lastLayoutStyle      int
	lastResetTime        int64    // Cache for last known reset time
	lastPredictedEndTime int64    // Cache for last known predicted end time
	smartRenderEnabled   bool     // Enable smart rendering mode
	previousScreen       []string // Previous screen content for differential updates
	previousShowHelp     bool     // Track previous help state for transition detection
}

func NewTerminalDisplay(config *TopConfig) *TerminalDisplay {
	return &TerminalDisplay{
		config:             config,
		smartRenderEnabled: true, // Enable smart rendering by default
		previousScreen:     make([]string, 0),
	}
}

// EnterAlternateScreen switches to alternate screen buffer
func (td *TerminalDisplay) EnterAlternateScreen() {
	if !td.inAlternateScreen {
		// Clear scrollback buffer
		fmt.Print(util.ClearScrollback)
		// Enter alternate screen buffer
		fmt.Print("\033[?1049h")
		// Clear screen
		fmt.Print(util.ClearScreen)
		// Reset scroll region
		fmt.Print(util.ResetScrollRegion)
		// Disable scrollback
		fmt.Print(util.DisableScrollback)
		// Hide cursor for cleaner display
		fmt.Print(util.HideCursor)
		// Move cursor to home
		fmt.Print(util.MoveCursorHome)
		td.inAlternateScreen = true
	}
}

// ExitAlternateScreen returns to normal screen buffer
func (td *TerminalDisplay) ExitAlternateScreen() {
	if td.inAlternateScreen {
		// Clear screen before exiting
		fmt.Print(util.ClearScreen)
		fmt.Print(util.MoveCursorHome)
		// Enable scrollback
		fmt.Print(util.EnableScrollback)
		// Show cursor
		fmt.Print(util.ShowCursor)
		// Exit alternate screen buffer
		fmt.Print("\033[?1049l")
		td.inAlternateScreen = false
	}
}

// ClearScreen clears the alternate screen buffer
func (td *TerminalDisplay) ClearScreen() {
	if td.inAlternateScreen {
		fmt.Print(util.ClearScreen)
		fmt.Print(util.MoveCursorHome)
	}
}

func (td *TerminalDisplay) RenderWithState(sessions []*Session, state model.InteractionState) {
	// Check if we're transitioning from help to normal mode
	helpTransition := td.previousShowHelp && !state.ShowHelp

	// If smart rendering is disabled, layout style changed, or transitioning from help, use full clear
	if !td.smartRenderEnabled || td.lastLayoutStyle != state.LayoutStyle || helpTransition {
		td.ClearScreen()
		td.lastLayoutStyle = state.LayoutStyle
		td.previousScreen = make([]string, 0) // Reset previous screen
	} else {
		// Smart render: just move cursor to home
		fmt.Print(util.MoveCursorHome)
	}

	// Show help if requested
	if state.ShowHelp {
		td.ClearScreen()
		td.renderHelp()
		td.previousShowHelp = true
		return
	}

	// Update help state tracking
	td.previousShowHelp = false

	// Calculate aggregated metrics
	aggregated := td.calculateAggregatedMetrics(sessions)

	// Render based on layout style using Strategy Pattern
	layoutParam := model.LayoutParam{Plan: td.config.Plan, Timezone: td.config.Timezone, TimeFormat: td.config.TimeFormat}
	layoutStrategy := layout.GetLayoutStrategy(state.LayoutStyle)

	// For smart rendering, we need to capture the output and compare
	if td.smartRenderEnabled {
		td.smartRender(layoutStrategy, aggregated, layoutParam)
	} else {
		layoutStrategy.Render(aggregated, layoutParam)
	}

	td.lastDraw = time.Now().Unix()
}

// smartRender performs differential rendering to preserve text selection
func (td *TerminalDisplay) smartRender(strategy layout.LayoutStrategy, aggregated *model.AggregatedMetrics, param model.LayoutParam) {
	// For now, use regular rendering but with cursor positioning
	// This prevents full screen clear while maintaining updates
	fmt.Print(util.SaveCursor)
	strategy.Render(aggregated, param)
	fmt.Print(util.RestoreCursor)
}

// calculateAggregatedMetrics calculates combined metrics from all sessions
func (td *TerminalDisplay) calculateAggregatedMetrics(sessions []*Session) *model.AggregatedMetrics {
	if len(sessions) == 0 {
		return &model.AggregatedMetrics{
			ModelDistribution: make(map[string]*model.ModelStats),
		}
	}

	// Get plan limits from pricing package
	plan := pricing.GetPlan(td.config.Plan)
	planLimits := pricing.Plan{
		Name:       plan.Name,
		TokenLimit: plan.TokenLimit,
		CostLimit:  plan.CostLimit,
	}

	aggregated := &model.AggregatedMetrics{
		ModelDistribution: make(map[string]*model.ModelStats),
		CostLimit:         planLimits.CostLimit,
		TokenLimit:        planLimits.TokenLimit,
		MessageLimit:      plan.MessageLimit,
	}

	// Calculate totals
	var totalBurnRate float64
	currentTime := time.Now().Unix()
	hasActiveSession := false

	// Find the first active session (earliest by start time)
	var firstActiveSession *Session
	for _, sess := range sessions {
		if sess.IsActive {
			if firstActiveSession == nil || sess.StartTime < firstActiveSession.StartTime {
				firstActiveSession = sess
			}
		}
	}

	// Count all sessions but only aggregate metrics from the first active session
	for _, sess := range sessions {
		aggregated.TotalSessions++

		if sess.IsActive {
			aggregated.ActiveSessions++
			hasActiveSession = true
		}

		// Check if session is not expired (reset time is in the future)
		if sess.ResetTime > currentTime {
			hasActiveSession = true
		}

		totalBurnRate += sess.BurnRate

		// Combine model distributions
		for _model, stats := range sess.ModelDistribution {
			if existing, ok := aggregated.ModelDistribution[_model]; ok {
				existing.Tokens += stats.Tokens
				existing.Cost += stats.Cost
				existing.Count += stats.Count
			} else {
				aggregated.ModelDistribution[_model] = &model.ModelStats{
					Model:  _model,
					Tokens: stats.Tokens,
					Cost:   stats.Cost,
					Count:  stats.Count,
				}
			}
		}
	}

	// Only use metrics from the first active session for totals
	if firstActiveSession != nil {
		aggregated.TotalCost = firstActiveSession.TotalCost
		aggregated.TotalTokens = firstActiveSession.TotalTokens
		aggregated.TotalMessages = firstActiveSession.MessageCount
	}

	aggregated.AverageBurnRate = totalBurnRate / float64(len(sessions))

	// Calculate burn rates and reset time
	if firstActiveSession != nil {
		// Use burn rates from the first active session only
		aggregated.CostBurnRate = firstActiveSession.CostPerHour / 60.0
		aggregated.CostPerMinute = firstActiveSession.CostPerMinute
		aggregated.TokenBurnRate = firstActiveSession.TokensPerMinute
		aggregated.MessageBurnRate = float64(firstActiveSession.MessageCount) / 300.0 // 5 hours

		// Calculate PredictedEndTime based on first active session
		currentTime := time.Now().Unix()
		if planLimits.CostLimit > 0 && firstActiveSession.CostPerMinute > 0 {
			remainingCost := planLimits.CostLimit - firstActiveSession.TotalCost
			if remainingCost > 0 {
				minutesToLimit := remainingCost / firstActiveSession.CostPerMinute
				aggregated.PredictedEndTime = currentTime + int64(minutesToLimit*60)
			} else {
				// Cost limit reached, set PredictedEndTime to ResetTime
				aggregated.PredictedEndTime = firstActiveSession.ResetTime
			}
		} else if planLimits.TokenLimit > 0 && firstActiveSession.TokensPerMinute > 0 {
			remainingTokens := float64(planLimits.TokenLimit) - float64(firstActiveSession.TotalTokens)
			if remainingTokens > 0 {
				minutesToLimit := remainingTokens / firstActiveSession.TokensPerMinute
				aggregated.PredictedEndTime = currentTime + int64(minutesToLimit*60)
			} else {
				// Token limit reached, set PredictedEndTime to ResetTime
				aggregated.PredictedEndTime = firstActiveSession.ResetTime
			}
		}

		// Use reset time and window information from the first active session
		aggregated.ResetTime = firstActiveSession.ResetTime
		aggregated.WindowSource = firstActiveSession.WindowSource
		aggregated.IsWindowDetected = firstActiveSession.IsWindowDetected

		util.LogDebug(fmt.Sprintf("Display using session %s - EndTime: %s, ResetTime: %s, PredictedEndTime: %s, WindowSource: %s",
			firstActiveSession.ID,
			time.Unix(firstActiveSession.EndTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(firstActiveSession.ResetTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(firstActiveSession.PredictedEndTime, 0).Format("2006-01-02 15:04:05"),
			firstActiveSession.WindowSource))

		// Update cache with the first active session info
		td.lastResetTime = firstActiveSession.ResetTime
		td.lastPredictedEndTime = aggregated.PredictedEndTime
	}

	// Check if limits exceeded
	aggregated.LimitExceeded = false
	if planLimits.CostLimit > 0 && aggregated.TotalCost >= planLimits.CostLimit {
		aggregated.LimitExceeded = true
		aggregated.LimitExceededReason = "COST LIMIT EXCEEDED"
	} else if planLimits.TokenLimit > 0 && aggregated.TotalTokens >= planLimits.TokenLimit {
		aggregated.LimitExceeded = true
		aggregated.LimitExceededReason = "TOKEN LIMIT EXCEEDED"
	} else if aggregated.TotalMessages >= 1000 {
		aggregated.LimitExceeded = true
		aggregated.LimitExceededReason = "MESSAGE LIMIT EXCEEDED"
	}

	// Set session status
	aggregated.HasActiveSession = hasActiveSession

	return aggregated
}

func (td *TerminalDisplay) renderHelp() {
	// Move cursor to home position first
	fmt.Print(util.MoveCursorHome)

	fmt.Println("Claude Monitor Top - Help")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Println("Keyboard Shortcuts:")
	fmt.Println()
	fmt.Println("  q/Esc/Ctrl+C - Quit the program")
	fmt.Println("  r         - Force refresh data")
	fmt.Println("  t         - Change layout style (Full → Minimal)")
	fmt.Println("  c         - Clear cache and reload")
	fmt.Println("  p         - Pause/unpause auto-refresh")
	fmt.Println("  h         - Show this help")
	fmt.Println("  ESC       - Close help/details (or quit if nothing is open)")
	fmt.Println()
	fmt.Println("Layout Styles:")
	fmt.Println("  Full Dashboard - Complete view with progress bars and detailed metrics")
	fmt.Println("  Minimal        - Ultra-compact view for quick checks")
	fmt.Println()
	fmt.Println("Status Colors:")
	fmt.Println("  🟢 Green  - Normal usage (below 60% of limit)")
	fmt.Println("  🟡 Yellow - Warning (60-90% of limit)")
	fmt.Println("  🔴 Red    - Critical (above 90% of limit)")
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("Press any key to return...")

	// Clear remaining lines to ensure no dashboard content shows through
	for i := 0; i < 10; i++ {
		fmt.Println(strings.Repeat(" ", 80))
	}
}

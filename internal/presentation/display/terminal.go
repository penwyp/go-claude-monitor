package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"github.com/penwyp/go-claude-monitor/internal/presentation/layout"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// Session is temporarily duplicated here to avoid circular import
// TODO: Move Session to model package to properly break the cycle
type Session struct {
	ID               string
	StartTime        int64
	StartHour        int64
	EndTime          int64
	ActualEndTime    *int64
	IsActive         bool
	IsGap            bool
	ProjectName      string
	SentMessageCount int
	Projects         map[string]*ProjectStats
	WindowStartTime  *int64
	IsWindowDetected bool
	WindowSource     string
	FirstEntryTime   int64
	TotalTokens      int
	TotalCost        float64
	ProjectTokens    int
	ProjectCost      float64
	ModelsUsed       map[string]int
	EntriesCount     int
	WindowPriority   int

	// Real-time metrics
	ResetTime         int64
	BurnRate          float64
	ModelDistribution map[string]*model.ModelStats
	MessageCount      int
	CostPerHour       float64
	CostPerMinute     float64
	TokensPerMinute   float64
	PredictedEndTime  int64
}

type ProjectStats struct {
	TokenCount   int
	Cost         float64
	MessageCount int
	ModelsUsed   map[string]int
}

type TerminalDisplay struct {
	config               *DisplayConfig
	lastDraw             int64
	inAlternateScreen    bool
	lastLayoutStyle      int
	lastResetTime        int64    // Cache for last known reset time
	lastPredictedEndTime int64    // Cache for last known predicted end time
	smartRenderEnabled   bool     // Enable smart rendering mode
	previousScreen       []string // Previous screen content for differential updates
	isFirstRender        bool     // Track if this is the first render
	currentMode          model.DisplayMode // Track current display mode for proper transitions
}

func NewTerminalDisplay(config *DisplayConfig) *TerminalDisplay {
	return &TerminalDisplay{
		config:             config,
		smartRenderEnabled: true, // Enable smart rendering by default
		previousScreen:     make([]string, 0),
		isFirstRender:      true, // Mark as first render
		currentMode:        model.ModeNormal, // Start in normal mode
	}
}

// EnterAlternateScreen switches to alternate screen buffer
func (td *TerminalDisplay) EnterAlternateScreen() {
	if !td.inAlternateScreen {
		// Enter alternate screen buffer first
		fmt.Print("\033[?1049h")
		// Clear entire screen completely
		fmt.Print("\033[2J")
		// Move cursor to home
		fmt.Print("\033[H")
		// Clear scrollback buffer
		fmt.Print(util.ClearScrollback)
		// Reset scroll region
		fmt.Print(util.ResetScrollRegion)
		// Disable scrollback
		fmt.Print(util.DisableScrollback)
		// Hide cursor for cleaner display
		fmt.Print(util.HideCursor)
		// Clear screen once more to ensure it's completely clean
		fmt.Print(util.ClearScreen)
		// Move cursor to home again
		fmt.Print(util.MoveCursorHome)
		td.inAlternateScreen = true
		// Mark as first render to ensure clean start
		td.isFirstRender = true
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

// ClearForTransition performs comprehensive screen clearing for mode transitions
func (td *TerminalDisplay) ClearForTransition() {
	if td.inAlternateScreen {
		// Clear entire screen
		fmt.Print("\033[2J")
		// Clear scrollback buffer
		fmt.Print("\033[3J")
		// Move cursor to home
		fmt.Print("\033[H")
		// Reset cursor position
		fmt.Print(util.MoveCursorHome)
		// Reset previous screen buffer for smart rendering
		td.previousScreen = make([]string, 0)
	}
}

// determineDisplayMode determines the current display mode based on interaction state
func (td *TerminalDisplay) determineDisplayMode(state model.InteractionState) model.DisplayMode {
	// Priority order: Dialog > Help > Loading > Normal
	if state.ConfirmDialog != nil {
		return model.ModeDialog
	}
	if state.ShowHelp {
		return model.ModeHelp
	}
	if state.DisplayStatus == model.StatusLoading || state.IsLoading {
		return model.ModeLoading
	}
	return model.ModeNormal
}

func (td *TerminalDisplay) RenderWithState(sessions []*Session, state model.InteractionState) {
	// Determine the new display mode based on state
	newMode := td.determineDisplayMode(state)
	
	// Check if we're transitioning between modes
	modeTransition := newMode != td.currentMode
	
	// Always clear screen on first render or mode transitions
	if td.isFirstRender || modeTransition {
		td.ClearForTransition()
		td.lastLayoutStyle = state.LayoutStyle
		td.isFirstRender = false
		td.currentMode = newMode
	} else if !td.smartRenderEnabled || td.lastLayoutStyle != state.LayoutStyle {
		// If smart rendering is disabled or layout style changed, use full clear
		td.ClearScreen()
		td.lastLayoutStyle = state.LayoutStyle
		td.previousScreen = make([]string, 0) // Reset previous screen
	} else {
		// Smart render: just move cursor to home
		fmt.Print(util.MoveCursorHome)
	}

	// Show confirm dialog if present
	if state.ConfirmDialog != nil {
		td.renderConfirmDialog(state.ConfirmDialog)
		return
	}

	// Show help if requested
	if state.ShowHelp {
		td.renderHelp()
		return
	}

	// Handle different display statuses
	switch state.DisplayStatus {
	case model.StatusLoading:
		// Initial loading - show full loading screen
		td.renderLoadingScreen(state.StatusIndicator)
		return
	case model.StatusRefreshing, model.StatusClearing:
		// Keep showing data with status indicator
		// The indicator will be shown in the aggregated metrics
		// Continue to render normal display below
	default:
		// Normal display or warning - continue below
	}

	// For backward compatibility, check old IsLoading flag
	if state.IsLoading && state.DisplayStatus == model.StatusNormal {
		td.renderLoadingScreen(state.LoadingMessage)
		return
	}

	// Calculate aggregated metrics
	aggregated := td.CalculateAggregatedMetrics(sessions)

	// Add status indicator to aggregated metrics for display
	if state.DisplayStatus == model.StatusRefreshing || state.DisplayStatus == model.StatusClearing {
		aggregated.LimitExceeded = false // Clear any limit warning when refreshing
		aggregated.StatusIndicator = state.StatusIndicator
	}

	// Render based on layout style using Strategy Pattern
	layoutParam := model.LayoutParam{Plan: td.config.Plan, Timezone: td.config.Timezone, TimeFormat: td.config.TimeFormat}
	layoutStrategy := layout.GetLayoutStrategy(state.LayoutStyle)

	// For smart rendering, we need to capture the output and compare
	if td.smartRenderEnabled {
		td.smartRender(layoutStrategy, aggregated, layoutParam)
	} else {
		layoutStrategy.Render(aggregated, layoutParam)
	}

	// Show status message if present
	if state.StatusMessage != "" {
		td.renderStatusMessage(state.StatusMessage)
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
func (td *TerminalDisplay) CalculateAggregatedMetrics(sessions []*Session) *model.AggregatedMetrics {
	// Get plan limits from pricing package (always needed)
	plan := pricing.GetPlan(td.config.Plan)
	planLimits := pricing.Plan{
		Name:       plan.Name,
		TokenLimit: plan.TokenLimit,
		CostLimit:  plan.CostLimit,
	}

	if len(sessions) == 0 {
		return &model.AggregatedMetrics{
			ModelDistribution: make(map[string]*model.ModelStats),
			CostLimit:         planLimits.CostLimit,
			TokenLimit:        planLimits.TokenLimit,
			MessageLimit:      plan.MessageLimit,
		}
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
	}

	// Use metrics from the first active session for primary display values
	if firstActiveSession != nil {
		aggregated.TotalCost = firstActiveSession.TotalCost
		aggregated.TotalTokens = firstActiveSession.TotalTokens
		aggregated.TotalMessages = firstActiveSession.MessageCount
		
		// Use model distribution from the first active session only
		// This ensures consistency between total metrics and model distribution
		if firstActiveSession.ModelDistribution != nil {
			for _model, stats := range firstActiveSession.ModelDistribution {
				if stats != nil {
					aggregated.ModelDistribution[_model] = &model.ModelStats{
						Model:  _model,
						Tokens: stats.Tokens,
						Cost:   stats.Cost,
						Count:  stats.Count,
					}
				}
			}
		}
	} else {
		// If no active session, combine model distributions from all sessions
		for _, sess := range sessions {
			if sess.ModelDistribution != nil {
				for _model, stats := range sess.ModelDistribution {
					if stats != nil {
						if existing, exists := aggregated.ModelDistribution[_model]; exists {
							// Add to existing model stats
							existing.Tokens += stats.Tokens
							existing.Cost += stats.Cost
							existing.Count += stats.Count
						} else {
							// Create new model stats entry
							aggregated.ModelDistribution[_model] = &model.ModelStats{
								Model:  _model,
								Tokens: stats.Tokens,
								Cost:   stats.Cost,
								Count:  stats.Count,
							}
						}
					}
				}
			}
		}
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
	} else if plan.MessageLimit > 0 && aggregated.TotalMessages >= plan.MessageLimit {
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

	// Save cursor for smart rendering
	fmt.Print(util.SaveCursor)

	fmt.Println("Claude Monitor Top - Help")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Println("Keyboard Shortcuts:")
	fmt.Println()
	fmt.Println("  q/Esc/Ctrl+C - Quit the program")
	fmt.Println("  r         - Force refresh data")
	fmt.Println("  t         - Change layout style (Full → Minimal)")
	fmt.Println("  c         - Clear memory cache")
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
	fmt.Println("  🟡 Yellow - Warning (60-80% of limit)")
	fmt.Println("  🔴 Red    - Critical (above 80% of limit)")
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("Press 'h' to return...")

	// Instead of clearing remaining lines, just clear from cursor to end of screen
	// This preserves text selection while ensuring clean display
	fmt.Print("\033[J") // Clear from cursor to end of screen

	// Restore cursor position
	fmt.Print(util.RestoreCursor)
}

func (td *TerminalDisplay) renderConfirmDialog(dialog *model.ConfirmDialog) {
	// Clear screen for dialog
	td.ClearScreen()

	// Center the dialog
	termWidth := 80 // Assume 80 chars width
	boxWidth := 60
	padding := (termWidth - boxWidth) / 2

	// Move cursor down a bit
	fmt.Print("\n\n\n\n\n")

	// Draw dialog box
	fmt.Printf("%s╔%s╗\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))
	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), util.CenterText(dialog.Title, boxWidth-2))
	fmt.Printf("%s╠%s╣\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))
	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), strings.Repeat(" ", boxWidth-2))

	// Wrap message text
	messageLines := wrapText(dialog.Message, boxWidth-4)
	for _, line := range messageLines {
		fmt.Printf("%s║ %s%s ║\n", strings.Repeat(" ", padding), line, strings.Repeat(" ", boxWidth-4-len(line)))
	}

	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), strings.Repeat(" ", boxWidth-2))
	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), util.CenterText("(Y)es / (N)o", boxWidth-2))
	fmt.Printf("%s╚%s╝\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))
}

func (td *TerminalDisplay) renderStatusMessage(message string) {
	// Save cursor position
	fmt.Print(util.SaveCursor)

	// Move to bottom of screen
	fmt.Print("\033[999;1H") // Move to row 999 (will stop at bottom)
	fmt.Print("\033[1A")     // Move up one line

	// Clear line and print status
	fmt.Print(util.ClearLine)
	fmt.Printf("  Status: %s", message)

	// Restore cursor position
	fmt.Print(util.RestoreCursor)
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) []string {
	if text == "" {
		return []string{}
	}

	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// renderLoadingScreen displays a loading message with animation
func (td *TerminalDisplay) renderLoadingScreen(message string) {
	// Move cursor to home position
	fmt.Print(util.MoveCursorHome)

	// Clear screen content
	fmt.Print("\033[2J") // Clear entire screen
	fmt.Print(util.MoveCursorHome)

	// Center the loading message vertically and horizontally
	termHeight := 24 // Assume terminal height
	termWidth := 80  // Assume terminal width

	// Move to vertical center
	for i := 0; i < termHeight/2-5; i++ {
		fmt.Println()
	}

	// Display loading box
	boxWidth := 50
	padding := (termWidth - boxWidth) / 2

	// Top border
	fmt.Printf("%s╔%s╗\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))

	// Loading title
	title := "Claude Monitor"
	titlePadding := (boxWidth - 2 - len(title)) / 2
	fmt.Printf("%s║%s%s%s║\n",
		strings.Repeat(" ", padding),
		strings.Repeat(" ", titlePadding),
		title,
		strings.Repeat(" ", boxWidth-2-titlePadding-len(title)))

	// Separator
	fmt.Printf("%s╠%s╣\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))

	// Empty line
	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), strings.Repeat(" ", boxWidth-2))

	// Loading message
	if message == "" {
		message = "Loading data..."
	}

	// Add loading animation
	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	animIndex := int(time.Now().Unix()/1) % len(loadingChars) // Change every second
	animatedMessage := fmt.Sprintf("%s %s", loadingChars[animIndex], message)

	msgPadding := (boxWidth - 2 - len(animatedMessage)) / 2
	fmt.Printf("%s║%s%s%s║\n",
		strings.Repeat(" ", padding),
		strings.Repeat(" ", msgPadding),
		animatedMessage,
		strings.Repeat(" ", boxWidth-2-msgPadding-len(animatedMessage)))

	// Empty line
	fmt.Printf("%s║%s║\n", strings.Repeat(" ", padding), strings.Repeat(" ", boxWidth-2))

	// Instruction
	instruction := "Press 'q' to quit"
	instrPadding := (boxWidth - 2 - len(instruction)) / 2
	fmt.Printf("%s║%s%s%s║\n",
		strings.Repeat(" ", padding),
		strings.Repeat(" ", instrPadding),
		instruction,
		strings.Repeat(" ", boxWidth-2-instrPadding-len(instruction)))

	// Bottom border
	fmt.Printf("%s╚%s╝\n", strings.Repeat(" ", padding), strings.Repeat("═", boxWidth-2))
}

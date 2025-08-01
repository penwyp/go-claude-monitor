package layout

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"strings"
	"time"
)

// FullLayoutStrategy implements the full dashboard layout
type FullLayoutStrategy struct {
	BaseStrategy
}

func (s *FullLayoutStrategy) GetName() string {
	return "Full Dashboard"
}

func (s *FullLayoutStrategy) Render(aggregated *model.AggregatedMetrics, param model.LayoutParam) {
	now := util.GetTimeProvider().Now()
	timeStr := now.Format("15:04:05")
	if param.TimeFormat == "12h" {
		timeStr = now.Format("3:04:05 PM")
	}

	maxWidth := s.GetSizer().GetMaxWidth()

	// If no active session, create a zero-value metrics object
	if !aggregated.HasActiveSession {
		aggregated = s.CreateZeroMetrics(aggregated)
	}

	s.topBorder(maxWidth)                                                          // Top border
	s.header(param, timeStr, maxWidth)                                             // Header line with proper spacing
	sep := s.separator(maxWidth)                                                   // Separator
	costPercent, tokenPercent, _ := s.resourceUsageData(aggregated, maxWidth, sep) // Resource usage section
	s.costLine(aggregated, costPercent, maxWidth)                                  // Cost line with progress bar
	s.tokenLine(aggregated, tokenPercent, maxWidth)                                // Token line with progress bar
	//s.messageLine(aggregated, messagePercent, maxWidth)                                         // Message line with progress bar
	s.sessionLine(aggregated, maxWidth) // Session line with progress bar

	s.performanceSection(aggregated, param, sep, maxWidth, now) // Performance metrics section
	s.modelDistribution(aggregated, sep, maxWidth)              // Model distribution section
	s.predictionsSection(aggregated, param, sep, maxWidth)      // Predictions section
	s.bottomBorder(maxWidth)                                    // Bottom border

}

func (s *FullLayoutStrategy) bottomBorder(maxWidth int) {
	fmt.Println("╰" + strings.Repeat("─", maxWidth-2) + "╯")
}

func (s *FullLayoutStrategy) predictionsSection(aggregated *model.AggregatedMetrics, param model.LayoutParam, sep string, maxWidth int) {
	//fmt.Println(sep)
	//predHeader := "│ PREDICTIONS" + strings.Repeat(" ", maxWidth-14) + "│"
	//fmt.Println(predHeader)
	fmt.Println(sep)

	tokensRunOut := aggregated.GetTokensRunOut(param)

	resetAt := aggregated.FormatResetTime(param)
	resetAt = aggregated.AppendWindowIndicator(resetAt)

	// Build prediction columns with dynamic width calculation
	var leftPredCol1, rightPredCol1 string

	// First row: Time Until Limit and Limit Reset
	leftPredCol1 = fmt.Sprintf("🔮 Time Until Limit: %s", tokensRunOut)
	rightPredCol1 = ""
	rightPredCol1Colored := ""

	// Second row: Only show if there's a limit exceeded warning
	if aggregated.LimitExceeded {
		rightPredCol1 = fmt.Sprintf("⚠️  %s", aggregated.LimitExceededReason)
		rightPredCol1Colored = fmt.Sprintf("⚠️  %s%s%s", util.ColorRed, aggregated.LimitExceededReason, util.ColorReset)
	}

	// Calculate display widths using plain text (without color codes)
	predLeftWidth1 := getDisplayWidth(leftPredCol1)
	predRightWidth1 := getDisplayWidth(rightPredCol1)

	// First, determine the divider position (centered)
	// Format: "│ " + leftContent + " │ " + rightContent + " │"
	// Total fixed chars: 2 (left border + space) + 3 (space + divider + space) + 2 (space + right border) = 7
	predAvailableContentWidth := maxWidth - 7
	predLeftColumnWidth := predAvailableContentWidth / 2
	predRightColumnWidth := predAvailableContentWidth - predLeftColumnWidth

	// Find the maximum width needed for each column's content
	predMaxLeftContentWidth := predLeftWidth1
	predMaxRightContentWidth := predRightWidth1

	// Use the allocated widths, but ensure content fits
	if predMaxLeftContentWidth > predLeftColumnWidth {
		predLeftColumnWidth = predMaxLeftContentWidth
	}
	if predMaxRightContentWidth > predRightColumnWidth {
		predRightColumnWidth = predMaxRightContentWidth
	}

	// Format first prediction line
	predLeftPadding1 := predLeftColumnWidth - predLeftWidth1
	predRightPadding1 := predRightColumnWidth - predRightWidth1
	// Use colored version for display if available
	displayRightPredCol1 := rightPredCol1
	if rightPredCol1Colored != "" {
		displayRightPredCol1 = rightPredCol1Colored
	}
	predLine1 := fmt.Sprintf("│ %s%s │ %s%s │",
		leftPredCol1, strings.Repeat(" ", predLeftPadding1),
		displayRightPredCol1, strings.Repeat(" ", predRightPadding1))
	fmt.Println(predLine1)
}

func (s *FullLayoutStrategy) modelDistribution(aggregated *model.AggregatedMetrics, sep string, maxWidth int) {
	if len(aggregated.ModelDistribution) > 0 {
		//fmt.Println(sep)
		//modelHeader := "│ MODEL DISTRIBUTION" + strings.Repeat(" ", maxWidth-21) + "│"
		//fmt.Println(modelHeader)
		fmt.Println(sep)

		// Sort models by tokens
		var models []string
		for model := range aggregated.ModelDistribution {
			models = append(models, model)
		}
		models = util.SortModels(models)

		// Calculate total tokens for current model distribution
		var currentModelTokens int
		for _, stats := range aggregated.ModelDistribution {
			currentModelTokens += stats.Tokens
		}

		// Find maximum model name width for consistent alignment
		maxModelNameWidth := 0
		for _, _model := range models {
			simplifiedModel := util.SimplifyModelName(_model)
			if len(simplifiedModel) > maxModelNameWidth {
				maxModelNameWidth = len(simplifiedModel)
			}
		}

		// Fixed bar width for all models
		barWidth := 28
		for _, _model := range models {
			stats := aggregated.ModelDistribution[_model]
			// Use current model tokens total for percentage calculation
			percentage := float64(stats.Tokens) / float64(currentModelTokens) * 100
			simplifiedModel := util.SimplifyModelName(_model)

			// Create progress bar with fixed width
			filled := int(percentage * float64(barWidth) / 100)
			if filled > barWidth {
				filled = barWidth
			}
			modelBar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled) + "]"

			modelEmoji := "🧠"
			if strings.Contains(simplifiedModel, "Opus") {
				modelEmoji = "🎯"
			}
			modelLine := fmt.Sprintf("│ %s %-*s    %s %.1f%%", modelEmoji, maxModelNameWidth, simplifiedModel, modelBar, percentage)
			// Calculate proper padding using display width
			currentWidth := getDisplayWidth(modelLine)
			paddingNeeded := maxWidth - currentWidth - 2
			if paddingNeeded > 0 {
				modelLine = modelLine + strings.Repeat(" ", paddingNeeded) + " │"
			} else {
				modelLine = modelLine + " │"
			}
			fmt.Println(modelLine)
		}
	}
}

func (s *FullLayoutStrategy) performanceSection(aggregated *model.AggregatedMetrics, param model.LayoutParam, sep string, maxWidth int, now time.Time) {
	//fmt.Println(sep)
	//perfHeader := "│ PERFORMANCE METRICS" + strings.Repeat(" ", maxWidth-22) + "│"
	//fmt.Println(perfHeader)
	fmt.Println(sep)

	resetAt := aggregated.FormatResetTime(param)
	resetAt = aggregated.AppendWindowIndicator(resetAt)

	// Performance metrics - two columns with dynamic width calculation
	// First, collect all content to find the maximum widths
	leftCol1 := fmt.Sprintf("⚡ Burn Rate: %s", util.FormatBurnRate(aggregated.TokenBurnRate))
	rightCol1 := fmt.Sprintf("💲 Cost Rate: $%.2f/min", aggregated.CostPerMinute)
	leftCol2 := fmt.Sprintf("⏱️  Time to Reset: %s", resetAt)
	rightCol2 := fmt.Sprintf("🔥 Active: %d sessions", aggregated.ActiveSessions)

	// Calculate the display widths
	leftWidth1 := getDisplayWidth(leftCol1)
	rightWidth1 := getDisplayWidth(rightCol1)
	leftWidth2 := getDisplayWidth(leftCol2)
	rightWidth2 := getDisplayWidth(rightCol2)

	// First, determine the divider position (centered)
	// Format: "│ " + leftContent + " │ " + rightContent + " │"
	// Total fixed chars: 2 (left border + space) + 3 (space + divider + space) + 2 (space + right border) = 7
	availableContentWidth := maxWidth - 7
	leftColumnWidth := availableContentWidth / 2
	rightColumnWidth := availableContentWidth - leftColumnWidth

	// Find the maximum width needed for each column's content
	maxLeftContentWidth := leftWidth1
	if leftWidth2 > maxLeftContentWidth {
		maxLeftContentWidth = leftWidth2
	}
	maxRightContentWidth := rightWidth1
	if rightWidth2 > maxRightContentWidth {
		maxRightContentWidth = rightWidth2
	}

	// Use the allocated widths, but ensure content fits
	if maxLeftContentWidth > leftColumnWidth {
		leftColumnWidth = maxLeftContentWidth
	}
	if maxRightContentWidth > rightColumnWidth {
		rightColumnWidth = maxRightContentWidth
	}

	// Format first line with proper padding
	leftPadding1 := leftColumnWidth - leftWidth1
	rightPadding1 := rightColumnWidth - rightWidth1
	perfLine1 := fmt.Sprintf("│ %s%s │ %s%s │",
		leftCol1, strings.Repeat(" ", leftPadding1),
		rightCol1, strings.Repeat(" ", rightPadding1))
	fmt.Println(perfLine1)

	// Format second line with proper padding
	leftPadding2 := leftColumnWidth - leftWidth2
	rightPadding2 := rightColumnWidth - rightWidth2
	perfLine2 := fmt.Sprintf("│ %s%s │ %s%s │",
		leftCol2, strings.Repeat(" ", leftPadding2),
		rightCol2, strings.Repeat(" ", rightPadding2))
	fmt.Println(perfLine2)
}

func (s *FullLayoutStrategy) messageLine(aggregated *model.AggregatedMetrics, messagePercent float64, maxWidth int) {
	messageBar := CreateProgressBar(messagePercent, 40)
	messageValues := fmt.Sprintf("%d / %d", aggregated.TotalMessages, aggregated.MessageLimit)
	messageLine := fmt.Sprintf("│ 📨 Messages %s %s %.1f%%",
		getPercentageEmoji(messagePercent), messageBar, messagePercent)
	// Calculate spacing to align values using display width
	spacing := maxWidth - getDisplayWidth(messageLine) - getDisplayWidth(messageValues) - 3
	if spacing < 2 {
		spacing = 2
	}
	messageLine = fmt.Sprintf("%s%s%s  │", messageLine, strings.Repeat(" ", spacing), messageValues)
	fmt.Println(messageLine)
}

func (s *FullLayoutStrategy) tokenLine(aggregated *model.AggregatedMetrics, tokenPercent float64, maxWidth int) int {
	tokenBar := CreateProgressBar(tokenPercent, 40)
	tokenValues := fmt.Sprintf("%s / %s", util.FormatNumber(aggregated.TotalTokens), util.FormatNumber(aggregated.TokenLimit))
	tokenLine := fmt.Sprintf("│ 🪙 Tokens   %s %s %.1f%%",
		getPercentageEmoji(tokenPercent), tokenBar, tokenPercent)
	// Calculate spacing to align values using display width
	spacing := maxWidth - getDisplayWidth(tokenLine) - getDisplayWidth(tokenValues) - 3
	if spacing < 2 {
		spacing = 2
	}
	tokenLine = fmt.Sprintf("%s%s%s  │", tokenLine, strings.Repeat(" ", spacing), tokenValues)
	fmt.Println(tokenLine)
	return spacing
}

func (s *FullLayoutStrategy) costLine(aggregated *model.AggregatedMetrics, costPercent float64, maxWidth int) {
	costBar := CreateProgressBar(costPercent, 40)
	costValues := fmt.Sprintf("$%.2f / $%.2f", aggregated.TotalCost, aggregated.CostLimit)
	costLine := fmt.Sprintf("│ 💰 Cost     %s %s %.1f%%",
		getPercentageEmoji(costPercent), costBar, costPercent)
	// Calculate spacing to align values using display width
	spacing := maxWidth - getDisplayWidth(costLine) - getDisplayWidth(costValues) - 3
	if spacing < 2 {
		spacing = 2
	}
	costLine = fmt.Sprintf("%s%s%s  │", costLine, strings.Repeat(" ", spacing), costValues)
	fmt.Println(costLine)
}

func (s *FullLayoutStrategy) resourceUsageData(aggregated *model.AggregatedMetrics, maxWidth int, sep string) (float64, float64, float64) {
	//resHeader := "│ RESOURCE USAGE" + strings.Repeat(" ", maxWidth-17) + "│"
	//fmt.Println(resHeader)
	//fmt.Println(sep)

	// Calculate percentages and progress bars
	costPercent := aggregated.GetCostPercentage()
	tokenPercent := aggregated.GetTokenPercentage()
	messagePercent := aggregated.GetMessagePercentage()
	return costPercent, tokenPercent, messagePercent
}

func (s *FullLayoutStrategy) separator(maxWidth int) string {
	sep := "├" + strings.Repeat("─", maxWidth-2) + "┤"
	fmt.Println(sep)
	return sep
}

func (s *FullLayoutStrategy) header(param model.LayoutParam, timeStr string, maxWidth int) {
	planName := getPlanType(param.Plan)

	// Two columns with merged content
	leftCol := fmt.Sprintf("🤖 CLAUDE MONITOR  │  %s Plan", planName)
	rightCol := fmt.Sprintf("  %s  │    %s", param.Timezone, timeStr)

	// Calculate display widths
	leftWidth := getDisplayWidth(leftCol)
	rightWidth := getDisplayWidth(rightCol)

	// First, determine the divider position (centered)
	// Format: "│ " + leftContent + " │ " + rightContent + " │"
	// Total fixed chars: 2 (left border + space) + 3 (space + divider + space) + 2 (space + right border) = 7
	availableContentWidth := maxWidth - 7
	leftColumnWidth := availableContentWidth / 2
	rightColumnWidth := availableContentWidth - leftColumnWidth

	// Find the maximum width needed for each column's content
	maxLeftContentWidth := leftWidth
	maxRightContentWidth := rightWidth

	// Use the allocated widths, but ensure content fits
	if maxLeftContentWidth > leftColumnWidth {
		leftColumnWidth = maxLeftContentWidth
	}
	if maxRightContentWidth > rightColumnWidth {
		rightColumnWidth = maxRightContentWidth
	}

	// Format with proper padding
	leftPadding := leftColumnWidth - leftWidth
	rightPadding := rightColumnWidth - rightWidth
	headerLine := fmt.Sprintf("│ %s%s │ %s%s │",
		leftCol, strings.Repeat(" ", leftPadding),
		rightCol, strings.Repeat(" ", rightPadding))

	fmt.Println(headerLine)
}

func (s *FullLayoutStrategy) sessionLine(aggregated *model.AggregatedMetrics, maxWidth int) {
	// Calculate session duration (5 hours total)
	totalSessionDuration := 5 * time.Hour

	// Calculate elapsed time using the common function
	elapsedTime, remainingTime := CalculateSessionElapsedTime(aggregated.ResetTime)

	var sessionValues string
	var sessionLine string
	var sessionPercent float64

	if aggregated.ResetTime == 0 {
		// No active session
		sessionBar := CreateProgressBar(0, 40)
		sessionValues = "No active session"
		sessionLine = fmt.Sprintf("│ ⏰ Session  %s %s %.1f%%",
			getPercentageEmoji(0), sessionBar, 0.0)
	} else {
		// Active session
		sessionPercent = CalculateSessionPercentage(elapsedTime)
		sessionBar := CreateProgressBar(sessionPercent, 40)

		if remainingTime == 0 && elapsedTime == totalSessionDuration {
			// Session expired
			sessionValues = fmt.Sprintf("%s / %s (expired)",
				util.FormatDuration(elapsedTime), util.FormatDuration(totalSessionDuration))
		} else {
			// Normal session
			sessionValues = fmt.Sprintf("%s / %s",
				util.FormatDuration(elapsedTime), util.FormatDuration(totalSessionDuration))
		}

		sessionLine = fmt.Sprintf("│ ⏰ Session  %s %s %.1f%%",
			getPercentageEmoji(sessionPercent), sessionBar, sessionPercent)
	}

	// Calculate spacing to align values using display width
	spacing := maxWidth - getDisplayWidth(sessionLine) - getDisplayWidth(sessionValues) - 3
	if spacing < 2 {
		spacing = 2
	}
	sessionLine = fmt.Sprintf("%s%s%s  │", sessionLine, strings.Repeat(" ", spacing), sessionValues)
	fmt.Println(sessionLine)
}

func (s *FullLayoutStrategy) topBorder(maxWidth int) {
	border := "╭" + strings.Repeat("─", maxWidth-2) + "╮"
	fmt.Println(border)
}

package layout

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// MinimalLayoutStrategy implements the mininal dashboard layout
type MinimalLayoutStrategy struct {
	BaseStrategy
}

func (s *MinimalLayoutStrategy) GetName() string {
	return "Minimal Dashboard"
}

func (s *MinimalLayoutStrategy) Render(aggregated *model.AggregatedMetrics, param model.LayoutParam) {
	tp := util.GetTimeProvider()

	// Get current time
	currentTimeStr := tp.FormatNow("15:04:05")
	if param.TimeFormat == "12h" {
		currentTimeStr = tp.FormatNow("3:04:05 PM")
	}

	// If no active session, create a zero-value metrics object
	if !aggregated.HasActiveSession {
		aggregated = s.CreateZeroMetrics(aggregated)
	}

	// Format token info
	percentage := aggregated.GetTokenPercentage()

	tokenInfo := fmt.Sprintf("%s/%s (%.1f%%)",
		util.FormatNumber(aggregated.TotalTokens),
		util.FormatNumber(aggregated.TokenLimit),
		percentage)
	if aggregated.TokenLimit <= 0 {
		tokenInfo = util.FormatNumber(aggregated.TotalTokens)
	}

	// Format cost info
	costInfo := fmt.Sprintf("$%.2f/$%.2f", aggregated.TotalCost, aggregated.CostLimit)

	// Build the single line
	line := fmt.Sprintf("Claude: 💰 %s | 🪙 %s | ⚡️ %s | 🔮 %s | ⏰ %s | %s",
		costInfo,
		tokenInfo,
		util.FormatBurnRate(aggregated.TokenBurnRate),
		aggregated.GetTokensRunOut(param),
		aggregated.FormatResetTime(param),
		currentTimeStr)

	// Print the single line
	fmt.Println(line)
}

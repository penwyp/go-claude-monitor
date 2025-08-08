package layout

import (
	"fmt"
	"strings"
	
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// BaseStrategy provides common functionality for all layout strategies
type BaseStrategy struct {
}


// CreateZeroMetrics creates a zero-value metrics object when no active session
func (b *BaseStrategy) CreateZeroMetrics(original *model.AggregatedMetrics) *model.AggregatedMetrics {
	return &model.AggregatedMetrics{
		ModelDistribution: make(map[string]*model.ModelStats),
		CostLimit:         original.CostLimit,
		TokenLimit:        original.TokenLimit,
		MessageLimit:      original.MessageLimit,
		// All other fields remain zero
	}
}

// GetSizer returns the shared sizer instance
func (b *BaseStrategy) GetSizer() *Sizer {
	return sharedSizer
}

// SeparatorLine creates a separator line
func (b *BaseStrategy) SeparatorLine() string {
	return "─────────────────────────────────────────────────────────────────────────"
}

// BoxHeader creates a boxed header with the given title
func (b *BaseStrategy) BoxHeader(title string, width int) string {
	if width <= 2 {
		return strings.Repeat("│", width)
	}
	availableWidth := width - 2 // Account for borders
	if len(title) >= availableWidth {
		// Truncate title if it's too long
		return "│" + title[:availableWidth] + "│"
	}
	padding := availableWidth - len(title)
	leftPad := padding / 2
	rightPad := padding - leftPad
	return "│" + strings.Repeat(" ", leftPad) + title + strings.Repeat(" ", rightPad) + "│"
}

// CenterText centers text within the given width
func (b *BaseStrategy) CenterText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(text) >= width {
		// Truncate text if it's longer than width
		return text[:width]
	}
	padding := width - len(text)
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
}

// FormatPercentage formats a percentage value
func (b *BaseStrategy) FormatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

// FormatTokens formats token count
func (b *BaseStrategy) FormatTokens(tokens int) string {
	return util.FormatNumber(tokens)
}

// FormatCurrency formats a currency value
func (b *BaseStrategy) FormatCurrency(amount float64) string {
	return util.FormatCurrency(amount)
}

// ProgressBar creates a progress bar with optional label
func (b *BaseStrategy) ProgressBar(percentage float64, width int, label string) string {
	bar := util.CreateProgressBar(percentage, width)
	if label != "" {
		// Add label to the progress bar if provided
		return fmt.Sprintf("%s %s", bar, label)
	}
	return bar
}

// GetModelIcon returns the icon for a model
func (b *BaseStrategy) GetModelIcon(model string) string {
	switch {
	case strings.Contains(strings.ToLower(model), "opus"):
		return "🎭"
	case strings.Contains(strings.ToLower(model), "sonnet"):
		return "🎵"
	case strings.Contains(strings.ToLower(model), "haiku"):
		return "🍃"
	default:
		return "🤖"
	}
}

// FormatCostInfo formats cost information
func (b *BaseStrategy) FormatCostInfo(metrics *model.AggregatedMetrics, params model.LayoutParam) string {
	percentage := 0.0
	if metrics.CostLimit > 0 {
		percentage = (metrics.TotalCost / metrics.CostLimit) * 100
	}
	return fmt.Sprintf("💰 Cost: %s / %s (%.1f%%)", 
		util.FormatCurrency(metrics.TotalCost), 
		util.FormatCurrency(metrics.CostLimit), 
		percentage)
}

// FormatTokenInfo formats token information
func (b *BaseStrategy) FormatTokenInfo(metrics *model.AggregatedMetrics, params model.LayoutParam) string {
	percentage := 0.0
	if metrics.TokenLimit > 0 {
		percentage = (float64(metrics.TotalTokens) / float64(metrics.TokenLimit)) * 100
	}
	return fmt.Sprintf("🔤 Tokens: %s / %s (%.1f%%)", 
		util.FormatNumber(metrics.TotalTokens), 
		util.FormatNumber(metrics.TokenLimit), 
		percentage)
}

// FormatMessageInfo formats message information
func (b *BaseStrategy) FormatMessageInfo(metrics *model.AggregatedMetrics, params model.LayoutParam) string {
	percentage := 0.0
	if metrics.MessageLimit > 0 {
		percentage = (float64(metrics.TotalMessages) / float64(metrics.MessageLimit)) * 100
	}
	return fmt.Sprintf("💬 Messages: %d / %d (%.1f%%)", 
		metrics.TotalMessages, 
		metrics.MessageLimit, 
		percentage)
}

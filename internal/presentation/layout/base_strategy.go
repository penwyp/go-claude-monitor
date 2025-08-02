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

// NewBaseStrategy creates a new BaseStrategy instance
func NewBaseStrategy() *BaseStrategy {
	return &BaseStrategy{}
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
	padding := width - len(title) - 2
	leftPad := padding / 2
	rightPad := padding - leftPad
	return "│" + strings.Repeat(" ", leftPad) + title + strings.Repeat(" ", rightPad) + "│"
}

// CenterText centers text within the given width
func (b *BaseStrategy) CenterText(text string, width int) string {
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

// ProgressBar creates a progress bar
func (b *BaseStrategy) ProgressBar(percentage float64, width int) string {
	return util.CreateProgressBar(percentage, width)
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
func (b *BaseStrategy) FormatCostInfo(cost float64, limit float64, percentage float64) string {
	return fmt.Sprintf("💰 Cost: %s / %s (%.1f%%)", 
		util.FormatCurrency(cost), 
		util.FormatCurrency(limit), 
		percentage)
}

// FormatTokenInfo formats token information
func (b *BaseStrategy) FormatTokenInfo(tokens int, limit int, percentage float64) string {
	return fmt.Sprintf("🔤 Tokens: %s / %s (%.1f%%)", 
		util.FormatNumber(tokens), 
		util.FormatNumber(limit), 
		percentage)
}

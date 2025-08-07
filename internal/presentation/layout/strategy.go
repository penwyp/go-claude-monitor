package layout

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

// LayoutStrategy defines the interface for different layout rendering strategies
type LayoutStrategy interface {
	Render(aggregated *model.AggregatedMetrics, param model.LayoutParam)
	GetName() string
}

// GetLayoutStrategy returns the appropriate layout strategy based on the style
func GetLayoutStrategy(layoutStyle int) LayoutStrategy {
	strategies := map[int]LayoutStrategy{
		0: &FullLayoutStrategy{},
		1: &MinimalLayoutStrategy{},
	}

	if strategy, exists := strategies[layoutStyle]; exists {
		return strategy
	}

	// Default to full dashboard if invalid style
	return &FullLayoutStrategy{}
}

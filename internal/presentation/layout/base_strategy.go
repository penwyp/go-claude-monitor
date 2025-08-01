package layout

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
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

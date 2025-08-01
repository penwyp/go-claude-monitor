package pricing

import (
	"testing"

	"github.com/penwyp/go-claude-monitor/internal/core/model"

	"github.com/stretchr/testify/assert"
)

func TestGetPricing(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  ModelPricing
	}{
		{
			name:  "opus pricing",
			model: model.ModelOpus4,
			want: ModelPricing{
				Input:         15.00,
				Output:        75.00,
				CacheCreation: 18.75,
				CacheRead:     1.50, // 修复为与实际代码中一致的值
			},
		},
		{
			name:  "sonnet pricing",
			model: model.ModelSonnet4,
			want: ModelPricing{
				Input:         3.00,
				Output:        15.00,
				CacheCreation: 3.75,
				CacheRead:     0.30,
			},
		},
		{
			name:  "haiku pricing",
			model: model.ModelHaiku35,
			want: ModelPricing{
				Input:         0.80,
				Output:        4.00,
				CacheCreation: 1.00,
				CacheRead:     0.08,
			},
		},
		{
			name:  "unknown model defaults to sonnet",
			model: "unknown-model",
			want: ModelPricing{
				Input:         3.00,
				Output:        15.00,
				CacheCreation: 3.75,
				CacheRead:     0.30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPricing(tt.model)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPlan(t *testing.T) {
	tests := []struct {
		name     string
		planName string
		want     Plan
	}{
		{
			name:     "pro plan",
			planName: model.PlanPro,
			want: Plan{
				Name:         "Claude Pro",
				TokenLimit:   4 * 1000 * 1000,
				CostLimit:    18.00,
				MessageLimit: 40,
			},
		},
		{
			name:     "max5 plan",
			planName: model.PlanMax5,
			want: Plan{
				Name:         "Claude Max 5",
				TokenLimit:   20 * 1000 * 1000,
				CostLimit:    35.00,
				MessageLimit: 200,
			},
		},
		{
			name:     "max20 plan",
			planName: model.PlanMax20,
			want: Plan{
				Name:         "Claude Max 20",
				TokenLimit:   80 * 1000 * 1000,
				CostLimit:    140.00,
				MessageLimit: 800,
			},
		},
		{
			name:     "unknown plan defaults to pro",
			planName: "unknown-plan",
			want: Plan{
				Name:         "Claude Pro",
				TokenLimit:   4 * 1000 * 1000,
				CostLimit:    18.00,
				MessageLimit: 40,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPlan(tt.planName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPricingConsistency(t *testing.T) {
	// Verify that output is more expensive than input for all models
	pricings := GetAllPricings()

	for model, pricing := range pricings {
		assert.Greater(t, pricing.Output, pricing.Input,
			"Output should be more expensive than input for model %s", model)

		// Cache creation should be more expensive than cache read
		assert.Greater(t, pricing.CacheCreation, pricing.CacheRead,
			"Cache creation should be more expensive than cache read for model %s", model)
	}
}

package pricing

import "github.com/penwyp/go-claude-monitor/internal/core/model"

type SourceConfig struct {
	PricingSource      string `json:"pricingSource"`
	PricingOfflineMode bool   `json:"pricingOfflineMode"`
}

// ModelPricing defines token pricing for different Claude models
type ModelPricing struct {
	Input         float64 // Per million tokens
	Output        float64 // Per million tokens
	CacheCreation float64 // Per million tokens
	CacheRead     float64 // Per million tokens
}

// Plan represents a subscription plan with token and cost limits
type Plan struct {
	Name         string  `json:"name"`
	TokenLimit   int     `json:"token_limit"`
	CostLimit    float64 `json:"cost_limit"`
	MessageLimit int     `json:"message_limit"`
}

// modelPricingMap stores pricing for all Claude models
var modelPricingMap = map[string]ModelPricing{
	model.ModelDefault: {
		Input:         3.00,  // $3 per million tokens
		Output:        15.00, // $15 per million tokens
		CacheCreation: 3.75,  // $3.75 per million tokens
		CacheRead:     0.30,  // $0.30 per million tokens
	},
	model.ModelSonnet35: {
		Input:         3.00,  // $3 per million tokens
		Output:        15.00, // $15 per million tokens
		CacheCreation: 3.75,  // $3.75 per million tokens
		CacheRead:     0.30,  // $0.30 per million tokens
	},
	model.ModelHaiku35: {
		Input:         0.80, // $0.80 per million tokens
		Output:        4.00, // $4.00 per million tokens
		CacheCreation: 1.00, // $1.00 per million tokens
		CacheRead:     0.08, // $0.08 per million tokens
	},
	model.ModelSonnet4: {
		Input:         3.00,  // $3 per million tokens
		Output:        15.00, // $15 per million tokens
		CacheCreation: 3.75,  // $3.75 per million tokens
		CacheRead:     0.30,  // $0.30 per million tokens
	},
	model.ModelOpus4: {
		Input:         15.00, // $15 per million tokens
		Output:        75.00, // $75 per million tokens
		CacheCreation: 18.75, // $18.75 per million tokens
		CacheRead:     1.50,  // $1.5 per million tokens
	},
	model.ModelOpus41: {
		Input:         15.00, // $15 per million tokens
		Output:        75.00, // $75 per million tokens
		CacheCreation: 18.75, // $18.75 per million tokens
		CacheRead:     1.50,  // $1.5 per million tokens
	},
}

// planMap stores all available subscription plans
var planMap = map[string]Plan{
	model.PlanPro: {
		Name:         "Claude Pro",
		TokenLimit:   4 * 1000 * 1000,
		CostLimit:    18.00,
		MessageLimit: 40,
	},
	model.PlanMax5: {
		Name:         "Claude Max 5",
		TokenLimit:   20 * 1000 * 1000,
		CostLimit:    35.00,
		MessageLimit: 200,
	},
	model.PlanMax20: {
		Name:         "Claude Max 20",
		TokenLimit:   80 * 1000 * 1000,
		CostLimit:    140.00,
		MessageLimit: 800,
	},
	"custom": {
		Name:         "Custom",
		TokenLimit:   20 * 1000 * 1000, // Default, can be overridden with P90
		CostLimit:    35.00,
		MessageLimit: 200,
	},
}

// GetPricing returns the pricing for a specific model
func GetPricing(modelName string) ModelPricing {
	if pricing, ok := modelPricingMap[modelName]; ok {
		return pricing
	}
	// Default to Sonnet pricing if model not found
	return modelPricingMap[model.ModelDefault]
}

// GetPlan returns a specific subscription plan
func GetPlan(planName string) Plan {
	if plan, ok := planMap[planName]; ok {
		return plan
	}
	// Default to Pro plan if not found
	return planMap[model.PlanPro]
}

// GetPlanWithDefault returns a specific subscription plan
func GetPlanWithDefault(planName string, customLimitTokens int) Plan {
	if plan, ok := planMap[planName]; ok {
		return plan
	}
	return Plan{
		Name:       "Custom",
		TokenLimit: customLimitTokens,
	}
}

// GetAllPricings returns all model pricings
func GetAllPricings() map[string]ModelPricing {
	// Return a copy to prevent external modification
	result := make(map[string]ModelPricing)
	for k, v := range modelPricingMap {
		result[k] = v
	}
	return result
}

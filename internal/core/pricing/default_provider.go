package pricing

import (
	"context"
)

// DefaultProvider implements PricingProvider using the static pricing data
type DefaultProvider struct{}

// NewDefaultProvider creates a new default pricing provider
func NewDefaultProvider() PricingProvider {
	return &DefaultProvider{}
}

// GetPricing returns the pricing for a specific model
func (p *DefaultProvider) GetPricing(ctx context.Context, modelName string) (ModelPricing, error) {
	pricing := GetPricing(modelName)
	return pricing, nil
}

// GetAllPricings returns all available model pricings
func (p *DefaultProvider) GetAllPricings(ctx context.Context) (map[string]ModelPricing, error) {
	return GetAllPricings(), nil
}

// RefreshPricing is a no-op for the default provider
func (p *DefaultProvider) RefreshPricing(ctx context.Context) error {
	return nil
}

// GetProviderName returns the name of this pricing provider
func (p *DefaultProvider) GetProviderName() string {
	return "default"
}

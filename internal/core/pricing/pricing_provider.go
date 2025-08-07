package pricing

import (
	"context"
	"errors"
)

// PricingProvider defines the interface for fetching model pricing information
type PricingProvider interface {
	// GetPricing returns the pricing for a specific model
	GetPricing(ctx context.Context, modelName string) (ModelPricing, error)

	// GetAllPricings returns all available model pricings
	GetAllPricings(ctx context.Context) (map[string]ModelPricing, error)

	// RefreshPricing forces a refresh of pricing data (for remote providers)
	RefreshPricing(ctx context.Context) error

	// GetProviderName returns the name of this pricing provider
	GetProviderName() string
}

// ErrPricingNotFound is returned when pricing for a model is not found
var ErrPricingNotFound = errors.New("pricing not found for model")

// ErrPricingUnavailable is returned when pricing data is temporarily unavailable
var ErrPricingUnavailable = errors.New("pricing data temporarily unavailable")

package pricing

import (
	"context"
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"sync"
	"time"
)

// CachedProvider wraps another provider with caching capabilities
type CachedProvider struct {
	provider     PricingProvider
	cacheManager *CacheManager
	useOffline   bool

	// Synchronization for cache updates
	updateMu       sync.Mutex
	lastUpdateTime time.Time
	updateInterval time.Duration
}

// NewCachedProvider creates a new cached pricing provider
func NewCachedProvider(provider PricingProvider, cacheManager *CacheManager, useOffline bool) *CachedProvider {
	return &CachedProvider{
		provider:       provider,
		cacheManager:   cacheManager,
		useOffline:     useOffline,
		updateInterval: 1 * time.Minute, // Default: update cache at most once per minute
	}
}

// GetPricing returns the pricing for a specific model
func (p *CachedProvider) GetPricing(ctx context.Context, modelName string) (ModelPricing, error) {
	// If offline mode is requested, try cache first
	if p.useOffline {
		cache, err := p.cacheManager.LoadPricing(ctx)
		if err == nil {
			if pricing, ok := cache.Pricing[modelName]; ok {
				util.LogDebugf("Using cached pricing for model %s from %s", modelName, cache.Source)
				return pricing, nil
			}
			if pricing, ok := cache.Pricing[modelName]; ok {
				util.LogDebugf("Using cached pricing for normalized model %s from %s", modelName, cache.Source)
				return pricing, nil
			}
		}
		util.LogDebugf("Cached pricing not found for model %s, falling back to provider", modelName)
	}

	// Get pricing from the underlying provider
	pricing, err := p.provider.GetPricing(ctx, modelName)
	if err != nil {
		// If provider fails and we have cache, try to use it as fallback
		if !p.useOffline && p.cacheManager.HasCache() {
			util.LogInfof("Primary pricing provider failed, attempting to use cached data")
			cache, cacheErr := p.cacheManager.LoadPricing(ctx)
			if cacheErr == nil {
				if cachedPricing, ok := cache.Pricing[modelName]; ok {
					util.LogInfof("Using fallback cached pricing for model %s", modelName)
					return cachedPricing, nil
				}
			}
		}
		return ModelPricing{}, err
	}

	// If not in offline mode and provider succeeded, update cache
	if !p.useOffline && p.provider.GetProviderName() != "default" {
		go p.updateCacheIfNeeded()
	}

	return pricing, nil
}

// GetAllPricings returns all available model pricings
func (p *CachedProvider) GetAllPricings(ctx context.Context) (map[string]ModelPricing, error) {
	// If offline mode is requested, try cache first
	if p.useOffline {
		cache, err := p.cacheManager.LoadPricing(ctx)
		if err == nil {
			util.LogDebugf("Using cached pricing data from %s with %d models", cache.Source, len(cache.Pricing))
			return cache.Pricing, nil
		}
		util.LogDebugf("Failed to load cached pricing: %v", err)
	}

	// Get pricing from the underlying provider
	pricing, err := p.provider.GetAllPricings(ctx)
	if err != nil {
		// If provider fails and we have cache, try to use it as fallback
		if !p.useOffline && p.cacheManager.HasCache() {
			util.LogInfof("Primary pricing provider failed, attempting to use cached data")
			cache, cacheErr := p.cacheManager.LoadPricing(ctx)
			if cacheErr == nil {
				util.LogInfof("Using fallback cached pricing data")
				return cache.Pricing, nil
			}
		}
		return nil, err
	}

	// If not in offline mode and provider succeeded, update cache
	if !p.useOffline && p.provider.GetProviderName() != "default" {
		go p.updateCacheIfNeeded()
	}

	return pricing, nil
}

// RefreshPricing forces a refresh of pricing data
func (p *CachedProvider) RefreshPricing(ctx context.Context) error {
	if p.useOffline {
		return fmt.Errorf("cannot refresh pricing in offline mode")
	}

	// Refresh the underlying provider
	if err := p.provider.RefreshPricing(ctx); err != nil {
		return err
	}

	// Update cache
	allPricing, err := p.provider.GetAllPricings(ctx)
	if err != nil {
		return err
	}

	return p.cacheManager.SavePricing(ctx, p.provider.GetProviderName(), allPricing)
}

// GetProviderName returns the name of this pricing provider
func (p *CachedProvider) GetProviderName() string {
	if p.useOffline {
		return fmt.Sprintf("%s-offline", p.provider.GetProviderName())
	}
	return fmt.Sprintf("%s-cached", p.provider.GetProviderName())
}

// updateCacheIfNeeded updates the cache if enough time has passed since last update
func (p *CachedProvider) updateCacheIfNeeded() {
	p.updateMu.Lock()
	defer p.updateMu.Unlock()

	// Check if enough time has passed since last update
	now := time.Now()
	if !p.lastUpdateTime.IsZero() && now.Sub(p.lastUpdateTime) < p.updateInterval {
		// Skip update - too soon
		return
	}

	// Update cache
	ctx := context.Background()
	allPricing, err := p.provider.GetAllPricings(ctx)
	if err != nil {
		util.LogDebugf("Failed to fetch pricing data for cache update: %v", err)
		return
	}

	if err := p.cacheManager.SavePricing(ctx, p.provider.GetProviderName(), allPricing); err != nil {
		util.LogDebugf("Failed to update pricing cache: %v", err)
	} else {
		util.LogDebugf("Updated pricing cache from %s provider", p.provider.GetProviderName())
		p.lastUpdateTime = now
	}
}

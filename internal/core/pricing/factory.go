package pricing

import (
	"fmt"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

// CreatePricingProvider creates a pricing provider based on configuration
func CreatePricingProvider(cfg *SourceConfig, cacheDir string) (PricingProvider, error) {
	// Create base provider based on source
	var baseProvider PricingProvider

	switch cfg.PricingSource {
	case "default", "":
		baseProvider = NewDefaultProvider()
	case "litellm":
		baseProvider = NewLiteLLMProvider()
	default:
		return nil, fmt.Errorf("unknown pricing source: %s", cfg.PricingSource)
	}

	// If offline mode or non-default provider, wrap with caching
	if cfg.PricingOfflineMode || cfg.PricingSource != "default" {
		util.LogDebug(fmt.Sprintf("Enabling pricing cache: offline_mode=%t, source=%s, cache_file=~/.go-claude-monitor/pricing.json",
			cfg.PricingOfflineMode, cfg.PricingSource))

		cacheManager, err := NewCacheManager(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache manager: %w", err)
		}

		cachedProvider := NewCachedProvider(baseProvider, cacheManager, cfg.PricingOfflineMode)
		return cachedProvider, nil
	}

	util.LogDebug("Successfully created basic pricing provider")
	return baseProvider, nil
}

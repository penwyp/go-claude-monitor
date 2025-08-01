package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

const (
	liteLLMPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	cacheExpiration   = 24 * time.Hour // Cache pricing data for 24 hours
)

// LiteLLMProvider implements PricingProvider by fetching pricing from LiteLLM's repository
type LiteLLMProvider struct {
	mu            sync.RWMutex
	pricing       map[string]ModelPricing
	lastFetchTime time.Time
	httpClient    *http.Client
}

// liteLLMModel represents the structure of a model in LiteLLM's pricing data
type liteLLMModel struct {
	InputCostPerToken           *float64 `json:"input_cost_per_token"`
	OutputCostPerToken          *float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost *float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     *float64 `json:"cache_read_input_token_cost"`
}

// NewLiteLLMProvider creates a new LiteLLM pricing provider
func NewLiteLLMProvider() *LiteLLMProvider {
	return &LiteLLMProvider{
		pricing: make(map[string]ModelPricing),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetPricing returns the pricing for a specific model
func (p *LiteLLMProvider) GetPricing(ctx context.Context, modelName string) (ModelPricing, error) {
	// Ensure pricing data is loaded
	if err := p.ensurePricingLoaded(ctx); err != nil {
		return ModelPricing{}, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try exact match
	if pricing, ok := p.pricing[modelName]; ok {
		return pricing, nil
	}

	// Try with provider prefix variations
	variations := []string{
		modelName,
		fmt.Sprintf("anthropic/%s", modelName),
		fmt.Sprintf("claude-3-5-%s", modelName),
		fmt.Sprintf("claude-3-%s", modelName),
		fmt.Sprintf("claude-%s", modelName),
	}

	for _, variant := range variations {
		if pricing, ok := p.pricing[variant]; ok {
			return pricing, nil
		}
	}

	// Try partial matches
	modelLower := strings.ToLower(modelName)
	for key, pricing := range p.pricing {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, modelLower) || strings.Contains(modelLower, keyLower) {
			return pricing, nil
		}
	}

	return ModelPricing{}, fmt.Errorf("%w: %s", ErrPricingNotFound, modelName)
}

// GetAllPricings returns all available model pricings
func (p *LiteLLMProvider) GetAllPricings(ctx context.Context) (map[string]ModelPricing, error) {
	// Ensure pricing data is loaded
	if err := p.ensurePricingLoaded(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]ModelPricing)
	for k, v := range p.pricing {
		result[k] = v
	}
	return result, nil
}

// RefreshPricing forces a refresh of pricing data
func (p *LiteLLMProvider) RefreshPricing(ctx context.Context) error {
	return p.fetchPricing(ctx)
}

// GetProviderName returns the name of this pricing provider
func (p *LiteLLMProvider) GetProviderName() string {
	return "litellm"
}

// ensurePricingLoaded checks if pricing data needs to be loaded or refreshed
func (p *LiteLLMProvider) ensurePricingLoaded(ctx context.Context) error {
	p.mu.RLock()
	needsRefresh := time.Since(p.lastFetchTime) > cacheExpiration || len(p.pricing) == 0
	currentCount := len(p.pricing)
	lastFetch := p.lastFetchTime
	p.mu.RUnlock()

	if needsRefresh {
		if currentCount == 0 {
			util.LogDebug("LiteLLM pricing data not loaded, fetching...")
		} else {
			util.LogDebug(fmt.Sprintf("LiteLLM pricing data expired (last fetch: %s), refreshing...",
				lastFetch.Format("2006-01-02 15:04:05")))
		}
		return p.fetchPricing(ctx)
	}

	util.LogDebug(fmt.Sprintf("Using cached LiteLLM pricing data (%d models, last updated: %s)",
		currentCount, lastFetch.Format("2006-01-02 15:04:05")))
	return nil
}

// fetchPricing fetches the latest pricing data from LiteLLM
func (p *LiteLLMProvider) fetchPricing(ctx context.Context) error {
	util.LogDebug(fmt.Sprintf("Starting to fetch pricing data from LiteLLM: %s", liteLLMPricingURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, liteLLMPricingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to fetch pricing data: %v", err))
		return fmt.Errorf("failed to fetch pricing data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		util.LogDebug(fmt.Sprintf("Unexpected HTTP status code: %d", resp.StatusCode))
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	util.LogDebug("Successfully downloaded pricing data, parsing JSON...")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON data
	var rawData map[string]json.RawMessage
	if err := sonic.Unmarshal(body, &rawData); err != nil {
		return fmt.Errorf("failed to parse pricing data: %w", err)
	}

	util.LogDebug(fmt.Sprintf("Parsed %d model entries from LiteLLM data", len(rawData)))

	// Convert to our pricing format
	newPricing := make(map[string]ModelPricing)
	processedCount := 0

	for modelName, rawModel := range rawData {
		var model liteLLMModel
		if err := sonic.Unmarshal(rawModel, &model); err != nil {
			// Skip models that don't match our expected structure
			continue
		}

		// Only process models with pricing information
		if model.InputCostPerToken == nil || model.OutputCostPerToken == nil {
			continue
		}

		// Convert from cost per token to cost per million tokens
		pricing := ModelPricing{
			Input:  *model.InputCostPerToken * 1_000_000,
			Output: *model.OutputCostPerToken * 1_000_000,
		}

		// Add cache pricing if available
		if model.CacheCreationInputTokenCost != nil {
			pricing.CacheCreation = *model.CacheCreationInputTokenCost * 1_000_000
		} else {
			// Default to 1.25x input cost if not specified
			pricing.CacheCreation = pricing.Input * 1.25
		}

		if model.CacheReadInputTokenCost != nil {
			pricing.CacheRead = *model.CacheReadInputTokenCost * 1_000_000
		} else {
			// Default to 0.1x input cost if not specified
			pricing.CacheRead = pricing.Input * 0.1
		}

		newPricing[modelName] = pricing
		processedCount++
	}

	// Update the cached pricing
	p.mu.Lock()
	p.pricing = newPricing
	p.lastFetchTime = time.Now()
	p.mu.Unlock()

	util.LogDebug(fmt.Sprintf("Successfully loaded %d model pricings from LiteLLM (processed %d out of %d entries)",
		len(newPricing), processedCount, len(rawData)))

	return nil
}

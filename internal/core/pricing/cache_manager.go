package pricing

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

// CacheManager handles caching of pricing data for offline use
type CacheManager struct {
	mu        sync.RWMutex
	cacheFile string
}

// PricingCache represents the cached pricing data
type PricingCache struct {
	Source    string                  `json:"source"`
	UpdatedAt time.Time               `json:"updated_at"`
	Pricing   map[string]ModelPricing `json:"pricing"`
}

// NewCacheManager creates a new pricing cache manager
func NewCacheManager(baseDir string) (*CacheManager, error) {
	// Use ~/.go-claude-monitor/pricing.json as the cache file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	pricingDir := filepath.Join(homeDir, ".go-claude-monitor")
	pricingFile := filepath.Join(pricingDir, "pricing.json")

	// Ensure pricing directory exists
	if err := os.MkdirAll(pricingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pricing directory: %w", err)
	}

	return &CacheManager{
		cacheFile: pricingFile,
	}, nil
}

// SavePricing saves pricing data to cache
func (m *CacheManager) SavePricing(ctx context.Context, source string, pricing map[string]ModelPricing) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	util.LogDebug(fmt.Sprintf("Saving %s pricing data to %s (%d models)", source, m.cacheFile, len(pricing)))

	cache := PricingCache{
		Source:    source,
		UpdatedAt: time.Now(),
		Pricing:   pricing,
	}

	data, err := sonic.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pricing cache: %w", err)
	}

	// Write to temporary file first
	tmpFile := m.cacheFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Rename to final location (atomic operation)
	if err := os.Rename(tmpFile, m.cacheFile); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	util.LogDebug(fmt.Sprintf("Successfully saved pricing data to %s", m.cacheFile))
	return nil
}

// LoadPricing loads pricing data from cache
func (m *CacheManager) LoadPricing(ctx context.Context) (*PricingCache, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	util.LogDebug(fmt.Sprintf("Loading pricing data from %s", m.cacheFile))

	data, err := os.ReadFile(m.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			util.LogDebug(fmt.Sprintf("Pricing cache file not found: %s", m.cacheFile))
			return nil, fmt.Errorf("no cached pricing data available at %s", m.cacheFile)
		}
		return nil, fmt.Errorf("failed to read cache file %s: %w", m.cacheFile, err)
	}

	var cache PricingCache
	if err := sonic.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pricing cache: %w", err)
	}

	util.LogDebug(fmt.Sprintf("Successfully loaded pricing data: source=%s, models=%d, updated_at=%s",
		cache.Source, len(cache.Pricing), cache.UpdatedAt.Format("2006-01-02 15:04:05")))

	return &cache, nil
}

// HasCache checks if cached pricing data exists
func (m *CacheManager) HasCache() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, err := os.Stat(m.cacheFile)
	return err == nil
}

// GetCacheAge returns how old the cached data is
func (m *CacheManager) GetCacheAge() (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, err := os.Stat(m.cacheFile)
	if err != nil {
		return 0, err
	}

	return time.Since(info.ModTime()), nil
}

// ClearCache removes the cached pricing data
func (m *CacheManager) ClearCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := os.Remove(m.cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	return nil
}

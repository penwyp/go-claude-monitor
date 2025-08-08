package top

import "time"

// TopConfig contains configuration for the top command
type TopConfig struct {
	// Data directories
	DataDir  string
	CacheDir string

	// Plan configuration
	Plan              string
	CustomLimitTokens int

	// Display settings
	Timezone   string
	TimeFormat string

	// Refresh settings
	DataRefreshInterval time.Duration
	UIRefreshRate       float64

	// Performance settings
	Concurrency int

	// Pricing configuration
	PricingSource      string // default, litellm
	PricingOfflineMode bool   // Enable offline pricing mode
}

// Validate checks if the configuration is valid
func (c *TopConfig) Validate() error {
	if c.DataDir == "" {
		c.DataDir = "~/.claude/projects"
	}
	if c.CacheDir == "" {
		c.CacheDir = "~/.go-claude-monitor/cache"
	}
	if c.Timezone == "" {
		c.Timezone = "Local"
	}
	if c.TimeFormat == "" {
		c.TimeFormat = "24h"
	}
	if c.DataRefreshInterval == 0 {
		c.DataRefreshInterval = 10 * time.Second
	}
	if c.UIRefreshRate == 0 {
		c.UIRefreshRate = 0.75
	}
	if c.Concurrency == 0 {
		c.Concurrency = 4
	}
	if c.PricingSource == "" {
		c.PricingSource = "default"
	}
	return nil
}
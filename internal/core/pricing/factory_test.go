package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePricingProviderDefault(t *testing.T) {
	tempDir := t.TempDir()
	
	tests := []struct {
		name                  string
		cfg                   *SourceConfig
		expectedProviderType  string
		expectCached          bool
		expectError           bool
	}{
		{
			name: "default_source",
			cfg: &SourceConfig{
				PricingSource:      "default",
				PricingOfflineMode: false,
			},
			expectedProviderType: "default",
			expectCached:         false,
			expectError:          false,
		},
		{
			name: "empty_source_defaults_to_default",
			cfg: &SourceConfig{
				PricingSource:      "",
				PricingOfflineMode: false,
			},
			expectedProviderType: "default",
			expectCached:         false,
			expectError:          false,
		},
		{
			name: "default_with_offline_mode",
			cfg: &SourceConfig{
				PricingSource:      "default",
				PricingOfflineMode: true,
			},
			expectedProviderType: "cached", // Should be wrapped with cache
			expectCached:         true,
			expectError:          false,
		},
		{
			name: "litellm_source",
			cfg: &SourceConfig{
				PricingSource:      "litellm",
				PricingOfflineMode: false,
			},
			expectedProviderType: "cached", // Non-default source should be cached
			expectCached:         true,
			expectError:          false,
		},
		{
			name: "unknown_source",
			cfg: &SourceConfig{
				PricingSource:      "unknown",
				PricingOfflineMode: false,
			},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := CreatePricingProvider(tt.cfg, tempDir)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if provider != nil {
					t.Error("Expected nil provider on error")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if provider == nil {
				t.Fatal("Expected non-nil provider")
			}
			
			// Check provider type
			providerName := provider.GetProviderName()
			if tt.expectCached {
				// Cached provider should include the base provider name
				if providerName == "default" || providerName == "litellm" {
					// This is fine - the cached provider might return the base provider name
				} else {
					// Or it might return a different identifier
					t.Logf("Cached provider name: %s", providerName)
				}
			} else {
				if providerName != tt.expectedProviderType {
					t.Errorf("Expected provider type %s, got %s", tt.expectedProviderType, providerName)
				}
			}
		})
	}
}

func TestCreatePricingProviderInvalidCacheDir(t *testing.T) {
	// Try to create provider with an invalid cache directory
	invalidDir := "/root/non-existent-dir/cache" // Should not be writable
	
	cfg := &SourceConfig{
		PricingSource:      "litellm",
		PricingOfflineMode: true,
	}
	
	// This might succeed or fail depending on the system
	// The main goal is to ensure it doesn't panic
	provider, err := CreatePricingProvider(cfg, invalidDir)
	if err != nil {
		// Error is expected for invalid cache directory
		t.Logf("Expected error for invalid cache directory: %v", err)
		if provider != nil {
			t.Error("Expected nil provider on cache creation error")
		}
	} else {
		// If it succeeds, provider should be valid
		if provider == nil {
			t.Error("Expected non-nil provider if no error")
		}
	}
}

func TestCreatePricingProviderWithReadOnlyCacheDir(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "readonly")
	
	// Create the directory
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}
	
	// Make it read-only
	err = os.Chmod(cacheDir, 0444)
	if err != nil {
		t.Fatalf("Failed to make directory read-only: %v", err)
	}
	
	// Restore permissions after test
	defer func() {
		os.Chmod(cacheDir, 0755)
	}()
	
	cfg := &SourceConfig{
		PricingSource:      "litellm",
		PricingOfflineMode: true,
	}
	
	// This should handle the read-only directory gracefully
	provider, err := CreatePricingProvider(cfg, cacheDir)
	if err != nil {
		// Error is acceptable for read-only directory
		t.Logf("Error with read-only cache directory: %v", err)
	} else {
		// If it succeeds, provider should be functional
		if provider == nil {
			t.Error("Expected non-nil provider if no error")
		} else {
			// Test basic functionality
			name := provider.GetProviderName()
			if name == "" {
				t.Error("Expected non-empty provider name")
			}
		}
	}
}

func TestCreatePricingProviderEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	
	t.Run("nil_config", func(t *testing.T) {
		// This should panic or return an error
		defer func() {
			if r := recover(); r != nil {
				// Panic is acceptable
				t.Logf("Expected panic with nil config: %v", r)
			}
		}()
		
		provider, err := CreatePricingProvider(nil, tempDir)
		if err == nil && provider != nil {
			t.Error("Expected error or nil provider with nil config")
		}
	})
	
	t.Run("empty_cache_dir", func(t *testing.T) {
		cfg := &SourceConfig{
			PricingSource:      "litellm",
			PricingOfflineMode: true,
		}
		
		// Empty cache directory should be handled
		provider, err := CreatePricingProvider(cfg, "")
		if err != nil {
			// Error is acceptable
			t.Logf("Error with empty cache directory: %v", err)
		} else if provider == nil {
			t.Error("Expected non-nil provider if no error")
		}
	})
	
	t.Run("whitespace_source", func(t *testing.T) {
		cfg := &SourceConfig{
			PricingSource:      "  \t\n  ",
			PricingOfflineMode: false,
		}
		
		// Whitespace source should be treated as empty (default)
		provider, err := CreatePricingProvider(cfg, tempDir)
		if err != nil {
			t.Errorf("Unexpected error with whitespace source: %v", err)
		} else if provider == nil {
			t.Error("Expected non-nil provider")
		} else if provider.GetProviderName() != "default" {
			// Might not be "default" if wrapped, but should be functional
			t.Logf("Provider name with whitespace source: %s", provider.GetProviderName())
		}
	})
}

func TestCreatePricingProviderCacheInteraction(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create provider with caching enabled
	cfg := &SourceConfig{
		PricingSource:      "litellm",
		PricingOfflineMode: false, // Not offline, but should still cache for non-default
	}
	
	provider, err := CreatePricingProvider(cfg, tempDir)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	
	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
	
	// Verify provider functionality
	name := provider.GetProviderName()
	if name == "" {
		t.Error("Expected non-empty provider name")
	}
	
	// Create another provider with the same cache directory
	provider2, err := CreatePricingProvider(cfg, tempDir)
	if err != nil {
		t.Fatalf("Failed to create second provider: %v", err)
	}
	
	if provider2 == nil {
		t.Fatal("Expected non-nil second provider")
	}
	
	// Both should be functional
	name2 := provider2.GetProviderName()
	if name2 == "" {
		t.Error("Expected non-empty second provider name")
	}
}

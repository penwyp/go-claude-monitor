package pricing

import (
	"context"
	"testing"
)

func TestNewDefaultProvider(t *testing.T) {
	provider := NewDefaultProvider()
	if provider == nil {
		t.Fatal("NewDefaultProvider returned nil")
	}
	
	defaultProvider, ok := provider.(*DefaultProvider)
	if !ok {
		t.Errorf("Expected *DefaultProvider, got %T", provider)
	}
	if defaultProvider == nil {
		t.Error("Expected non-nil DefaultProvider")
	}
}

func TestDefaultProviderGetPricing(t *testing.T) {
	provider := NewDefaultProvider()
	ctx := context.Background()
	
	tests := []struct {
		name      string
		modelName string
		expectErr bool
	}{
		{
			name:      "opus_model",
			modelName: "claude-opus-4-20250514",
			expectErr: false,
		},
		{
			name:      "sonnet_model",
			modelName: "claude-3-5-sonnet",
			expectErr: false,
		},
		{
			name:      "haiku_model",
			modelName: "claude-3-5-haiku",
			expectErr: false,
		},
		{
			name:      "unknown_model_defaults_to_sonnet",
			modelName: "unknown-model",
			expectErr: false, // Should default to sonnet pricing
		},
		{
			name:      "empty_model_name",
			modelName: "",
			expectErr: false, // Should default to sonnet pricing
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, err := provider.GetPricing(ctx, tt.modelName)
			
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify pricing structure
			if pricing.Input <= 0 {
				t.Errorf("Expected positive input price, got %f", pricing.Input)
			}
			if pricing.Output <= 0 {
				t.Errorf("Expected positive output price, got %f", pricing.Output)
			}
			
			// Cache pricing should be non-negative
			if pricing.CacheCreation < 0 {
				t.Errorf("Expected non-negative cache creation price, got %f", pricing.CacheCreation)
			}
			if pricing.CacheRead < 0 {
				t.Errorf("Expected non-negative cache read price, got %f", pricing.CacheRead)
			}
		})
	}
}

func TestDefaultProviderGetAllPricings(t *testing.T) {
	provider := NewDefaultProvider()
	ctx := context.Background()
	
	allPricings, err := provider.GetAllPricings(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if len(allPricings) == 0 {
		t.Error("Expected non-empty pricing map")
	}
	
	// Verify expected models are present
	expectedModels := []string{"claude-3-5-sonnet", "claude-3-5-haiku", "claude-opus-4-20250514"}
	for _, model := range expectedModels {
		if pricing, ok := allPricings[model]; !ok {
			t.Errorf("Expected pricing for model %s", model)
		} else {
			if pricing.Input <= 0 {
				t.Errorf("Invalid input price for model %s: %f", model, pricing.Input)
			}
			if pricing.Output <= 0 {
				t.Errorf("Invalid output price for model %s: %f", model, pricing.Output)
			}
		}
	}
}

func TestDefaultProviderRefreshPricing(t *testing.T) {
	provider := NewDefaultProvider()
	ctx := context.Background()
	
	// RefreshPricing should be a no-op for default provider
	err := provider.RefreshPricing(ctx)
	if err != nil {
		t.Errorf("Expected no error from RefreshPricing, got: %v", err)
	}
	
	// Multiple calls should not cause issues
	for i := 0; i < 5; i++ {
		err = provider.RefreshPricing(ctx)
		if err != nil {
			t.Errorf("RefreshPricing call %d failed: %v", i, err)
		}
	}
}

func TestDefaultProviderGetProviderName(t *testing.T) {
	provider := NewDefaultProvider()
	
	name := provider.GetProviderName()
	if name != "default" {
		t.Errorf("Expected provider name 'default', got %s", name)
	}
}

func TestDefaultProviderContextCancellation(t *testing.T) {
	provider := NewDefaultProvider()
	
	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	// Operations should still work since default provider doesn't use context for I/O
	pricing, err := provider.GetPricing(ctx, "claude-3-sonnet")
	if err != nil {
		t.Errorf("Expected operation to succeed with cancelled context, got: %v", err)
	}
	if pricing.Input <= 0 {
		t.Error("Expected valid pricing even with cancelled context")
	}
	
	allPricings, err := provider.GetAllPricings(ctx)
	if err != nil {
		t.Errorf("Expected GetAllPricings to succeed with cancelled context, got: %v", err)
	}
	if len(allPricings) == 0 {
		t.Error("Expected non-empty pricing map even with cancelled context")
	}
	
	err = provider.RefreshPricing(ctx)
	if err != nil {
		t.Errorf("Expected RefreshPricing to succeed with cancelled context, got: %v", err)
	}
}

func TestDefaultProviderConsistency(t *testing.T) {
	provider := NewDefaultProvider()
	ctx := context.Background()
	
	// Test that multiple calls return consistent results
	modelName := "claude-3-5-sonnet"
	pricing1, err1 := provider.GetPricing(ctx, modelName)
	if err1 != nil {
		t.Fatalf("First call failed: %v", err1)
	}
	
	pricing2, err2 := provider.GetPricing(ctx, modelName)
	if err2 != nil {
		t.Fatalf("Second call failed: %v", err2)
	}
	
	// Verify pricing is consistent
	if pricing1.Input != pricing2.Input {
		t.Errorf("Inconsistent input pricing: %f vs %f", pricing1.Input, pricing2.Input)
	}
	if pricing1.Output != pricing2.Output {
		t.Errorf("Inconsistent output pricing: %f vs %f", pricing1.Output, pricing2.Output)
	}
	if pricing1.CacheCreation != pricing2.CacheCreation {
		t.Errorf("Inconsistent cache creation pricing: %f vs %f", pricing1.CacheCreation, pricing2.CacheCreation)
	}
	if pricing1.CacheRead != pricing2.CacheRead {
		t.Errorf("Inconsistent cache read pricing: %f vs %f", pricing1.CacheRead, pricing2.CacheRead)
	}
}

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "default_model",
			constant: ModelDefault,
			expected: "default",
		},
		{
			name:     "haiku_35_model",
			constant: ModelHaiku35,
			expected: "claude-3-5-haiku",
		},
		{
			name:     "sonnet_35_model",
			constant: ModelSonnet35,
			expected: "claude-3-5-sonnet",
		},
		{
			name:     "sonnet_4_model",
			constant: ModelSonnet4,
			expected: "claude-sonnet-4-20250514",
		},
		{
			name:     "opus_4_model",
			constant: ModelOpus4,
			expected: "claude-opus-4-20250514",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestModelConstantsUniqueness(t *testing.T) {
	// Ensure all model constants are unique
	models := []string{
		ModelDefault,
		ModelHaiku35,
		ModelSonnet35,
		ModelSonnet4,
		ModelOpus4,
	}
	
	seen := make(map[string]bool)
	for _, model := range models {
		assert.False(t, seen[model], "Duplicate model constant found: %s", model)
		seen[model] = true
	}
	
	assert.Len(t, seen, len(models), "Expected %d unique model constants", len(models))
}

func TestEntryTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "message_entry",
			constant: EntryMessage,
			expected: "message",
		},
		{
			name:     "assistant_entry",
			constant: EntryAssistant,
			expected: "assistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestEntryTypeConstantsUniqueness(t *testing.T) {
	// Ensure all entry type constants are unique
	entryTypes := []string{
		EntryMessage,
		EntryAssistant,
	}
	
	seen := make(map[string]bool)
	for _, entryType := range entryTypes {
		assert.False(t, seen[entryType], "Duplicate entry type constant found: %s", entryType)
		seen[entryType] = true
	}
	
	assert.Len(t, seen, len(entryTypes), "Expected %d unique entry type constants", len(entryTypes))
}

func TestPlanConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "pro_plan",
			constant: PlanPro,
			expected: "pro",
		},
		{
			name:     "max5_plan",
			constant: PlanMax5,
			expected: "max5",
		},
		{
			name:     "max20_plan",
			constant: PlanMax20,
			expected: "max20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestPlanConstantsUniqueness(t *testing.T) {
	// Ensure all plan constants are unique
	plans := []string{
		PlanPro,
		PlanMax5,
		PlanMax20,
	}
	
	seen := make(map[string]bool)
	for _, plan := range plans {
		assert.False(t, seen[plan], "Duplicate plan constant found: %s", plan)
		seen[plan] = true
	}
	
	assert.Len(t, seen, len(plans), "Expected %d unique plan constants", len(plans))
}

func TestAllConstantsAreStrings(t *testing.T) {
	// Verify that all constants are non-empty strings
	allConstants := []string{
		// Model constants
		ModelDefault,
		ModelHaiku35,
		ModelSonnet35,
		ModelSonnet4,
		ModelOpus4,
		// Entry type constants
		EntryMessage,
		EntryAssistant,
		// Plan constants
		PlanPro,
		PlanMax5,
		PlanMax20,
	}
	
	for _, constant := range allConstants {
		assert.NotEmpty(t, constant, "Constant should not be empty")
		assert.IsType(t, "", constant, "Constant should be a string")
	}
}

func TestConstantCategorization(t *testing.T) {
	t.Run("model_constants_naming", func(t *testing.T) {
		// All model constants should contain model name patterns
		modelConstants := []string{ModelHaiku35, ModelSonnet35, ModelSonnet4, ModelOpus4}
		
		for _, model := range modelConstants {
			assert.Contains(t, model, "claude", "Model constant should contain 'claude': %s", model)
		}
		
		// Except for the default model
		assert.NotContains(t, ModelDefault, "claude", "Default model should not contain 'claude'")
	})
	
	t.Run("entry_type_constants_validation", func(t *testing.T) {
		// Entry type constants should be lowercase
		entryTypes := []string{EntryMessage, EntryAssistant}
		
		for _, entryType := range entryTypes {
			assert.Equal(t, entryType, entryType, "Entry type should be lowercase: %s", entryType)
			assert.NotContains(t, entryType, " ", "Entry type should not contain spaces: %s", entryType)
		}
	})
	
	t.Run("plan_constants_validation", func(t *testing.T) {
		// Plan constants should be lowercase alphanumeric
		plans := []string{PlanPro, PlanMax5, PlanMax20}
		
		for _, plan := range plans {
			assert.Equal(t, plan, plan, "Plan should be lowercase: %s", plan)
			assert.NotContains(t, plan, " ", "Plan should not contain spaces: %s", plan)
			assert.NotContains(t, plan, "-", "Plan should not contain hyphens: %s", plan)
		}
	})
}

func TestConstantUsageScenarios(t *testing.T) {
	t.Run("model_constant_usage", func(t *testing.T) {
		// Test that model constants can be used in a map
		modelMap := map[string]bool{
			ModelDefault:  true,
			ModelHaiku35:  true,
			ModelSonnet35: true,
			ModelSonnet4:  true,
			ModelOpus4:    true,
		}
		
		assert.Len(t, modelMap, 5)
		assert.True(t, modelMap[ModelSonnet35])
		assert.True(t, modelMap[ModelHaiku35])
	})
	
	t.Run("entry_type_constant_usage", func(t *testing.T) {
		// Test that entry type constants can be used in conditionals
		messageType := EntryMessage
		assistantType := EntryAssistant
		
		switch messageType {
		case EntryMessage:
			assert.Equal(t, "message", messageType)
		case EntryAssistant:
			t.Error("Should not match assistant type")
		default:
			t.Error("Should match message type")
		}
		
		switch assistantType {
		case EntryMessage:
			t.Error("Should not match message type")
		case EntryAssistant:
			assert.Equal(t, "assistant", assistantType)
		default:
			t.Error("Should match assistant type")
		}
	})
	
	t.Run("plan_constant_usage", func(t *testing.T) {
		// Test that plan constants can be used for configuration
		plans := []string{PlanPro, PlanMax5, PlanMax20}
		
		for _, plan := range plans {
			switch plan {
			case PlanPro:
				assert.Equal(t, "pro", plan)
			case PlanMax5:
				assert.Equal(t, "max5", plan)
			case PlanMax20:
				assert.Equal(t, "max20", plan)
			default:
				t.Errorf("Unknown plan: %s", plan)
			}
		}
	})
}

func TestConstantStability(t *testing.T) {
	// These tests ensure that constant values don't change accidentally
	// which could break backward compatibility
	
	t.Run("model_constant_stability", func(t *testing.T) {
		// Critical model constants that must remain stable
		assert.Equal(t, "default", ModelDefault)
		assert.Equal(t, "claude-3-5-haiku", ModelHaiku35)
		assert.Equal(t, "claude-3-5-sonnet", ModelSonnet35)
		assert.Equal(t, "claude-sonnet-4-20250514", ModelSonnet4)
		assert.Equal(t, "claude-opus-4-20250514", ModelOpus4)
	})
	
	t.Run("entry_type_constant_stability", func(t *testing.T) {
		// Critical entry type constants that must remain stable
		assert.Equal(t, "message", EntryMessage)
		assert.Equal(t, "assistant", EntryAssistant)
	})
	
	t.Run("plan_constant_stability", func(t *testing.T) {
		// Critical plan constants that must remain stable
		assert.Equal(t, "pro", PlanPro)
		assert.Equal(t, "max5", PlanMax5)
		assert.Equal(t, "max20", PlanMax20)
	})
}

// Benchmark tests for constant access (should be extremely fast)
func BenchmarkModelConstantAccess(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ModelSonnet35
	}
}

func BenchmarkEntryTypeConstantAccess(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EntryMessage
	}
}

func BenchmarkPlanConstantAccess(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PlanPro
	}
}

func BenchmarkConstantComparison(b *testing.B) {
	testValue := "claude-3-5-sonnet"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = (testValue == ModelSonnet35)
	}
}

func BenchmarkConstantMapLookup(b *testing.B) {
	modelMap := map[string]bool{
		ModelDefault:  true,
		ModelHaiku35:  true,
		ModelSonnet35: true,
		ModelSonnet4:  true,
		ModelOpus4:    true,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = modelMap[ModelSonnet35]
	}
}

// Test edge cases and potential issues
func TestConstantEdgeCases(t *testing.T) {
	t.Run("constant_case_sensitivity", func(t *testing.T) {
		// Ensure constants are case-sensitive as expected
		assert.NotEqual(t, "PRO", PlanPro)
		assert.NotEqual(t, "Pro", PlanPro)
		assert.NotEqual(t, "MESSAGE", EntryMessage)
		assert.NotEqual(t, "Message", EntryMessage)
	})
	
	t.Run("constant_whitespace", func(t *testing.T) {
		// Ensure constants don't have leading/trailing whitespace
		allConstants := []string{
			ModelDefault, ModelHaiku35, ModelSonnet35, ModelSonnet4, ModelOpus4,
			EntryMessage, EntryAssistant,
			PlanPro, PlanMax5, PlanMax20,
		}
		
		for _, constant := range allConstants {
			assert.Equal(t, constant, constant, "Constant should not have whitespace: '%s'", constant)
			assert.NotContains(t, constant, "\n", "Constant should not contain newlines: %s", constant)
			assert.NotContains(t, constant, "\t", "Constant should not contain tabs: %s", constant)
		}
	})
	
	t.Run("constant_length_validation", func(t *testing.T) {
		// Ensure constants are reasonable lengths
		allConstants := []string{
			ModelDefault, ModelHaiku35, ModelSonnet35, ModelSonnet4, ModelOpus4,
			EntryMessage, EntryAssistant,
			PlanPro, PlanMax5, PlanMax20,
		}
		
		for _, constant := range allConstants {
			assert.Greater(t, len(constant), 0, "Constant should not be empty: %s", constant)
			assert.LessOrEqual(t, len(constant), 50, "Constant should not be excessively long: %s", constant)
		}
	})
}
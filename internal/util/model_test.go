package util

import (
	"reflect"
	"testing"
)

func TestSimplifyModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Standard patterns with 8-digit dates
		{"claude-opus-4-20250514", "Opus-4"},
		{"claude-sonnet-4-20250514", "Sonnet-4"},
		{"claude-haiku-3-20250101", "Haiku-3"},

		// Future model versions
		{"claude-opus-4.5-20250514", "Opus-4.5"},
		{"claude-opus-5-20251122", "Opus-5"},
		{"claude-opus-4-20262233", "Opus-4"},
		{"claude-anthori-4-20255544", "Anthori-4"},

		// Special cases
		{"synthetic", "synthetic"},

		// Non-matching patterns should return original
		{"unknown-model", "unknown-model"},
		{"claude-opus-4-2025", "claude-opus-4-2025"}, // 4-digit date doesn't match
		{"opus-4-20250514", "opus-4-20250514"},       // Missing claude- prefix
		{"claude-opus-4", "claude-opus-4"},           // Missing date suffix
	}

	for _, test := range tests {
		result := SimplifyModelName(test.input)
		if result != test.expected {
			t.Errorf("SimplifyModelName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestGetModelOrder(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		// Opus models have highest priority
		{"claude-opus-4-20250514", 1},
		{"claude-opus-4.5-20250514", 1},
		{"claude-opus-5-20251122", 1},

		// Sonnet models have second priority
		{"claude-sonnet-4-20250514", 2},
		{"claude-3-5-sonnet-20250101", 2},

		// Haiku models have third priority
		{"claude-haiku-3-20250101", 3},

		// Synthetic has very low priority
		{"synthetic", 999},

		// Unknown models go last
		{"unknown-model", 100},
		{"claude-anthori-4-20255544", 100}, // New model type
	}

	for _, test := range tests {
		result := GetModelOrder(test.input)
		if result != test.expected {
			t.Errorf("GetModelOrder(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestSortModels(t *testing.T) {
	input := []string{
		"synthetic",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"unknown",
		"claude-haiku-3-20250101",
		"claude-anthori-4-20255544",
	}
	expected := []string{
		"claude-opus-4-20250514",
		"claude-sonnet-4-20250514",
		"claude-haiku-3-20250101",
		"claude-anthori-4-20255544",
		"unknown",
		"synthetic",
	}

	result := SortModels(input)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("SortModels(%v) = %v, expected %v", input, result, expected)
	}

	// Verify original slice is not modified
	originalInput := []string{
		"synthetic",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"unknown",
		"claude-haiku-3-20250101",
		"claude-anthori-4-20255544",
	}
	if !reflect.DeepEqual(input, originalInput) {
		t.Errorf("Original slice was modified: %v", input)
	}
}

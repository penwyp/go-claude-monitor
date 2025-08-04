package util

import (
	"regexp"
	"sort"
	"strings"
)

// SimplifyModelName transforms model names according to the specified rules
// Pattern: claude-{model-name}-{date} -> {Model-name} (first letter capitalized)
func SimplifyModelName(modelName string) string {
	// Handle special cases first
	if modelName == "synthetic" {
		return "synthetic"
	}

	// Use regex to match claude- prefix and -date suffix
	re := regexp.MustCompile(`^claude-(.+)-(\d{8})$`)
	matches := re.FindStringSubmatch(modelName)

	if len(matches) == 3 {
		modelPart := matches[1]
		// Capitalize first letter
		if len(modelPart) > 0 {
			return strings.ToUpper(string(modelPart[0])) + modelPart[1:]
		}
		return modelPart
	}

	// If no match, return original name
	return modelName
}

// GetModelOrder returns the sort order for a model (lower number = higher priority)
func GetModelOrder(modelName string) int {
	simplified := SimplifyModelName(modelName)

	// Handle synthetic first
	if simplified == "synthetic" {
		return 999
	}

	// Extract base model type and version for ordering
	lower := strings.ToLower(simplified)

	// Opus models have highest priority
	if strings.Contains(lower, "opus") {
		return 1
	}

	// Sonnet models have second priority
	if strings.Contains(lower, "sonnet") {
		return 2
	}

	// Haiku models have third priority
	if strings.Contains(lower, "haiku") {
		return 3
	}

	// Unknown models go last
	return 100
}

// SortModels sorts a slice of model names according to the specified order
func SortModels(models []string) []string {
	sorted := make([]string, len(models))
	copy(sorted, models)

	sort.Slice(sorted, func(i, j int) bool {
		orderI := GetModelOrder(sorted[i])
		orderJ := GetModelOrder(sorted[j])

		if orderI != orderJ {
			return orderI < orderJ
		}

		// If same order, sort alphabetically
		return sorted[i] < sorted[j]
	})

	return sorted
}

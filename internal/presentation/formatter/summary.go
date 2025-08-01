package formatter

import (
	"fmt"
	"sort"
	"strings"
)

// SummaryFormatter is responsible for formatting and outputting summary reports.
type SummaryFormatter struct{}

// NewSummaryFormatter creates a new instance of SummaryFormatter.
func NewSummaryFormatter() *SummaryFormatter {
	return &SummaryFormatter{}
}

// Format formats and outputs the summary information of grouped data.
func (f *SummaryFormatter) Format(data []GroupedData) error {
	// Calculate totals for all fields.
	var totalInput, totalOutput, totalCacheCreate, totalCacheRead, totalTokens int
	var totalCost float64
	modelStats := make(map[string]*ModelDetail)

	for _, row := range data {
		totalInput += row.InputTokens
		totalOutput += row.OutputTokens
		totalCacheCreate += row.CacheCreation
		totalCacheRead += row.CacheRead
		totalTokens += row.TotalTokens
		totalCost += row.Cost

		// Initialize model statistics if not already present.
		for _, model := range row.Models {
			if _, ok := modelStats[model]; !ok {
				modelStats[model] = &ModelDetail{Model: model}
			}
		}

		// Accumulate model details for summary reports.
		for _, detail := range row.ModelDetails {
			if stat, ok := modelStats[detail.Model]; ok {
				stat.InputTokens += detail.InputTokens
				stat.OutputTokens += detail.OutputTokens
				stat.CacheCreation += detail.CacheCreation
				stat.CacheRead += detail.CacheRead
				stat.TotalTokens += detail.TotalTokens
				stat.Cost += detail.Cost
			}
		}
	}

	// Output the summary report in English.
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Claude Code Usage Summary Report")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	fmt.Printf("Total Input Tokens:      %s\n", formatNumber(totalInput))
	fmt.Printf("Total Output Tokens:     %s\n", formatNumber(totalOutput))
	fmt.Printf("Cache Creation Tokens:   %s\n", formatNumber(totalCacheCreate))
	fmt.Printf("Cache Read Tokens:       %s\n", formatNumber(totalCacheRead))
	fmt.Printf("Total Tokens:            %s\n", formatNumber(totalTokens))
	fmt.Printf("Total Cost:              $%.2f USD\n", totalCost)
	fmt.Println()

	if len(modelStats) > 0 {
		fmt.Println("Statistics by Model:")
		fmt.Println(strings.Repeat("-", 60))

		var models []string
		for model := range modelStats {
			models = append(models, model)
		}
		sort.Strings(models)

		for _, model := range models {
			stat := modelStats[model]
			fmt.Printf("\n%s:\n", model)
			fmt.Printf("  Input Tokens:         %s\n", formatNumber(stat.InputTokens))
			fmt.Printf("  Output Tokens:        %s\n", formatNumber(stat.OutputTokens))
			fmt.Printf("  Cache Creation:       %s\n", formatNumber(stat.CacheCreation))
			fmt.Printf("  Cache Read:           %s\n", formatNumber(stat.CacheRead))
			fmt.Printf("  Total Tokens:         %s\n", formatNumber(stat.TotalTokens))
			fmt.Printf("  Cost:                 $%.2f USD\n", stat.Cost)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

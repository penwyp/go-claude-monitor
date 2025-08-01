package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

type TableFormatter struct {
	headers []string
	widths  []int
}

func NewTableFormatter() *TableFormatter {
	return &TableFormatter{
		headers: []string{
			"Date", "Models", "Input", "Output",
			"Cache Create", "Cache Read", "Total Tokens", "Cost (USD)",
		},
	}
}

func (f *TableFormatter) Format(data []GroupedData) error {
	// Calculate optimal column widths based on content
	widths := f.calculateColumnWidths(data)

	// Print top border
	f.printBorder(widths, "top")

	// Print header
	f.printRow(f.headers, widths, "header")

	// Print header separator
	f.printBorder(widths, "middle")

	var totalInput, totalOutput, totalCacheCreate, totalCacheRead, totalTokens int
	var totalCost float64

	for i, row := range data {
		// Transform and sort model names, filtering out synthetic models with zero tokens
		filteredModels := f.filterNonZeroTokenModels(row.Models, row.ModelDetails, row.ShowBreakdown)
		simplifiedModels := make([]string, len(filteredModels))
		for j, model := range filteredModels {
			simplifiedModels[j] = util.SimplifyModelName(model)
		}
		sortedModels := util.SortModels(simplifiedModels)

		var modelStr string
		if row.ShowBreakdown && len(row.ModelDetails) > 0 {
			modelStr = "ALL"
		} else {
			modelStr = strings.Join(sortedModels, ", ")
		}

		// Print main row
		rowData := []string{
			row.Date,
			modelStr,
			formatNumber(row.InputTokens),
			formatNumber(row.OutputTokens),
			formatNumber(row.CacheCreation),
			formatNumber(row.CacheRead),
			formatNumber(row.TotalTokens),
			formatCost(row.Cost),
		}
		f.printRow(rowData, widths, "data")

		if row.ShowBreakdown && len(row.ModelDetails) > 0 {
			// Filter and sort model details by the specified order
			filteredDetails := f.filterNonZeroTokenModelDetails(row.ModelDetails)
			sortedDetails := make([]ModelDetail, len(filteredDetails))
			copy(sortedDetails, filteredDetails)
			sort.Slice(sortedDetails, func(i, j int) bool {
				return util.GetModelOrder(sortedDetails[i].Model) < util.GetModelOrder(sortedDetails[j].Model)
			})

			for _, detail := range sortedDetails {
				simplifiedModel := util.SimplifyModelName(detail.Model)
				breakdownData := []string{
					"",
					"└ " + simplifiedModel,
					formatNumber(detail.InputTokens),
					formatNumber(detail.OutputTokens),
					formatNumber(detail.CacheCreation),
					formatNumber(detail.CacheRead),
					formatNumber(detail.TotalTokens),
					formatCost(detail.Cost),
				}
				f.printRow(breakdownData, widths, "breakdown")
			}

			if i < len(data)-1 {
				f.printBorder(widths, "middle")
			}
		}

		totalInput += row.InputTokens
		totalOutput += row.OutputTokens
		totalCacheCreate += row.CacheCreation
		totalCacheRead += row.CacheRead
		totalTokens += row.TotalTokens
		totalCost += row.Cost
	}

	// Print total row
	f.printBorder(widths, "middle")
	totalData := []string{
		"Total",
		"",
		formatNumber(totalInput),
		formatNumber(totalOutput),
		formatNumber(totalCacheCreate),
		formatNumber(totalCacheRead),
		formatNumber(totalTokens),
		formatCost(totalCost),
	}
	f.printRow(totalData, widths, "data")

	// Print bottom border
	f.printBorder(widths, "bottom")

	return nil
}

// calculateColumnWidths determines optimal width for each column based on content
func (f *TableFormatter) calculateColumnWidths(data []GroupedData) []int {
	widths := make([]int, len(f.headers))

	// Initialize with header widths
	for i, header := range f.headers {
		widths[i] = len(header)
	}

	// Calculate totals for proper width calculation
	var totalInput, totalOutput, totalCacheCreate, totalCacheRead, totalTokens int
	var totalCost float64

	// Check data rows
	for _, row := range data {
		// Transform and sort model names, filtering out synthetic models with zero tokens
		filteredModels := f.filterNonZeroTokenModels(row.Models, row.ModelDetails, row.ShowBreakdown)
		simplifiedModels := make([]string, len(filteredModels))
		for j, model := range filteredModels {
			simplifiedModels[j] = util.SimplifyModelName(model)
		}
		sortedModels := util.SortModels(simplifiedModels)

		var modelStr string
		if row.ShowBreakdown && len(row.ModelDetails) > 0 {
			modelStr = "ALL"
		} else {
			modelStr = strings.Join(sortedModels, ", ")
		}

		// Check main row values
		values := []string{
			row.Date,
			modelStr,
			formatNumber(row.InputTokens),
			formatNumber(row.OutputTokens),
			formatNumber(row.CacheCreation),
			formatNumber(row.CacheRead),
			formatNumber(row.TotalTokens),
			formatCost(row.Cost),
		}

		for i, value := range values {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}

		// Check breakdown rows if present
		if row.ShowBreakdown && len(row.ModelDetails) > 0 {
			filteredDetails := f.filterNonZeroTokenModelDetails(row.ModelDetails)
			for _, detail := range filteredDetails {
				simplifiedModel := util.SimplifyModelName(detail.Model)
				breakdownValues := []string{
					"",
					"└ " + simplifiedModel,
					formatNumber(detail.InputTokens),
					formatNumber(detail.OutputTokens),
					formatNumber(detail.CacheCreation),
					formatNumber(detail.CacheRead),
					formatNumber(detail.TotalTokens),
					formatCost(detail.Cost),
				}

				for i, value := range breakdownValues {
					if len(value) > widths[i] {
						widths[i] = len(value)
					}
				}
			}
		}

		// Accumulate totals
		totalInput += row.InputTokens
		totalOutput += row.OutputTokens
		totalCacheCreate += row.CacheCreation
		totalCacheRead += row.CacheRead
		totalTokens += row.TotalTokens
		totalCost += row.Cost
	}

	// Check actual "Total" row values
	totalValues := []string{
		"Total",
		"",
		formatNumber(totalInput),
		formatNumber(totalOutput),
		formatNumber(totalCacheCreate),
		formatNumber(totalCacheRead),
		formatNumber(totalTokens),
		formatCost(totalCost),
	}
	for i, value := range totalValues {
		if len(value) > widths[i] {
			widths[i] = len(value)
		}
	}

	// Apply minimum widths for readability
	minWidths := []int{8, 8, 8, 8, 8, 8, 8, 8}
	for i, minWidth := range minWidths {
		if widths[i] < minWidth {
			widths[i] = minWidth
		}
	}

	return widths
}

// printBorder prints table borders (top, middle, bottom)
func (f *TableFormatter) printBorder(widths []int, borderType string) {
	var left, middle, right, separator string

	switch borderType {
	case "top":
		left, middle, right, separator = "┌", "┬", "┐", "─"
	case "middle":
		left, middle, right, separator = "├", "┼", "┤", "─"
	case "bottom":
		left, middle, right, separator = "└", "┴", "┘", "─"
	}

	fmt.Print(left)
	for i, width := range widths {
		fmt.Print(strings.Repeat(separator, width+2)) // +2 for padding spaces
		if i < len(widths)-1 {
			fmt.Print(middle)
		}
	}
	fmt.Println(right)
}

// printRow prints a data row with proper alignment
func (f *TableFormatter) printRow(values []string, widths []int, rowType string) {
	fmt.Print("│")
	for i, value := range values {
		// Special handling for breakdown rows to ensure proper alignment
		if rowType == "breakdown" && i == 1 {
			// For breakdown rows, Models column should be left-aligned with proper indentation
			fmt.Printf(" %-*s │", widths[i], value)
		} else if i == 0 || i == 1 {
			// Date and Models columns are left-aligned
			fmt.Printf(" %-*s │", widths[i], value)
		} else {
			// Numeric columns are right-aligned
			fmt.Printf(" %*s │", widths[i], value)
		}
	}
	fmt.Println()
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result []byte
	for i, digit := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, digit)
	}

	return string(result)
}

func formatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

// filterNonZeroTokenModels filters out synthetic models that have zero tokens
func (f *TableFormatter) filterNonZeroTokenModels(models []string, modelDetails []ModelDetail, showBreakdown bool) []string {
	if !showBreakdown {
		// When not showing breakdown, check if synthetic models have zero total tokens
		// by looking at corresponding model details or assume non-zero for non-synthetic
		filtered := make([]string, 0, len(models))
		for _, model := range models {
			if util.SimplifyModelName(model) == "<synthetic>" {
				// Find corresponding detail to check if it has tokens
				hasTokens := false
				for _, detail := range modelDetails {
					if detail.Model == model && detail.TotalTokens > 0 {
						hasTokens = true
						break
					}
				}
				if hasTokens {
					filtered = append(filtered, model)
				}
			} else {
				// Non-synthetic models are always included
				filtered = append(filtered, model)
			}
		}
		return filtered
	}
	// When showing breakdown, we include all models since breakdown will be filtered
	return models
}

// filterNonZeroTokenModelDetails filters out synthetic model details that have zero tokens
func (f *TableFormatter) filterNonZeroTokenModelDetails(modelDetails []ModelDetail) []ModelDetail {
	filtered := make([]ModelDetail, 0, len(modelDetails))
	for _, detail := range modelDetails {
		// Only filter out synthetic models with zero tokens
		if util.SimplifyModelName(detail.Model) == "<synthetic>" && detail.TotalTokens == 0 {
			continue
		}
		filtered = append(filtered, detail)
	}
	return filtered
}

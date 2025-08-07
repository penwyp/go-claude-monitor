package formatter

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

type CSVFormatter struct{}

func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

func (f *CSVFormatter) Format(data []GroupedData) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	headers := []string{
		"Date", "Models", "Input", "Output",
		"Cache Create", "Cache Read", "Total Tokens", "Cost (USD)",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, row := range data {
		// Simplify model names
		simplifiedModels := make([]string, len(row.Models))
		for i, model := range row.Models {
			simplifiedModels[i] = util.SimplifyModelName(model)
		}
		sortedModels := util.SortModels(simplifiedModels)
		
		record := []string{
			row.Date,
			strings.Join(sortedModels, ", "),
			fmt.Sprintf("%d", row.InputTokens),
			fmt.Sprintf("%d", row.OutputTokens),
			fmt.Sprintf("%d", row.CacheCreation),
			fmt.Sprintf("%d", row.CacheRead),
			fmt.Sprintf("%d", row.TotalTokens),
			fmt.Sprintf("%.2f", row.Cost),
		}
		if err := w.Write(record); err != nil {
			return err
		}

		if row.ShowBreakdown {
			for _, detail := range row.ModelDetails {
				simplifiedModel := util.SimplifyModelName(detail.Model)
				record := []string{
					"  └─ " + row.Date,
					simplifiedModel,
					fmt.Sprintf("%d", detail.InputTokens),
					fmt.Sprintf("%d", detail.OutputTokens),
					fmt.Sprintf("%d", detail.CacheCreation),
					fmt.Sprintf("%d", detail.CacheRead),
					fmt.Sprintf("%d", detail.TotalTokens),
					fmt.Sprintf("%.2f", detail.Cost),
				}
				if err := w.Write(record); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

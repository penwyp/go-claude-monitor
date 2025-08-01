package util

import (
	"fmt"
	"time"
)

// Helper functions
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
}

func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func FormatBurnRate(rate float64) string {
	if rate < 1000 {
		return fmt.Sprintf("%.1f tokens/min", rate)
	} else if rate < 1000000 {
		return fmt.Sprintf("%.1fK tokens/min", rate/1000)
	} else {
		return fmt.Sprintf("%.1fM tokens/min", rate/1000000)
	}
}

func FormatCurrency(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

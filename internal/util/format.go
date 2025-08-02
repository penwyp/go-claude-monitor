package util

import (
	"fmt"
	"strings"
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
	// Format with comma separators for thousands
	// First format with 2 decimal places
	str := fmt.Sprintf("%.2f", amount)
	
	// Split into integer and decimal parts
	parts := strings.Split(str, ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = parts[1]
	}
	
	// Add commas to integer part
	if len(intPart) > 3 {
		result := ""
		for i, c := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				result += ","
			}
			result += string(c)
		}
		intPart = result
	}
	
	// Combine with decimal part
	if decPart != "" {
		return fmt.Sprintf("$%s.%s", intPart, decPart)
	}
	return fmt.Sprintf("$%s.00", intPart)
}

package layout

import (
	"github.com/penwyp/go-claude-monitor/internal/util"
	"time"
)

// getDisplayWidth is now available from util.GetDisplayWidth
func getDisplayWidth(text string) int {
	return util.GetDisplayWidth(text)
}

func getPlanType(plan string) string {
	switch plan {
	case "pro":
		return "Pro"
	case "max5":
		return "Max 5"
	case "max20":
		return "Max 20"
	default:
		return "Custom"
	}
}

// CreateProgressBar is now available from util.CreateProgressBar
func CreateProgressBar(percentage float64, width int) string {
	return util.CreateProgressBar(percentage, width)
}

// getPercentageEmoji is now available from util.GetPercentageEmoji
func getPercentageEmoji(percentage float64) string {
	return util.GetPercentageEmoji(percentage)
}

// CalculateSessionElapsedTime is now available from util.CalculateSessionElapsedTime
func CalculateSessionElapsedTime(resetTime int64) (elapsedTime time.Duration, remainingTime time.Duration) {
	return util.CalculateSessionElapsedTime(resetTime)
}

// CalculateSessionPercentage is now available from util.CalculateSessionPercentage
func CalculateSessionPercentage(elapsedTime time.Duration) float64 {
	return util.CalculateSessionPercentage(elapsedTime)
}

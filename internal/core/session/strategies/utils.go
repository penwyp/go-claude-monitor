package strategies

import (
	"fmt"
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// truncateToHour truncates a timestamp to the nearest hour boundary
func truncateToHour(timestamp int64) int64 {
	t := time.Unix(timestamp, 0).UTC()
	truncated := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
	return truncated.Unix()
}

// formatTimestamp formats a Unix timestamp to a readable string
func formatTimestamp(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

// Wrapper functions for util package functions
// These ensure the strategies package can work even if util functions change

// LogDebug logs a debug message
func logDebug(msg string) {
	util.LogDebug(msg)
}

// LogInfo logs an info message
func logInfo(msg string) {
	util.LogInfo(msg)
}

// LogWarn logs a warning message
func logWarn(msg string) {
	util.LogWarn(msg)
}

// FormatDuration formats a duration in seconds to a human-readable string
func formatDuration(seconds int64) string {
	hours := float64(seconds) / 3600
	if hours >= 1 {
		return fmt.Sprintf("%.1f hours", hours)
	}
	minutes := float64(seconds) / 60
	return fmt.Sprintf("%.1f minutes", minutes)
}
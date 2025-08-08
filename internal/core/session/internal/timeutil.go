package internal

import (
	"time"
)

// TruncateToHour rounds down a timestamp to the nearest hour
func TruncateToHour(timestamp int64) int64 {
	return (timestamp / 3600) * 3600
}

// IsWithinDuration checks if a timestamp is within the specified duration from now
func IsWithinDuration(timestamp int64, duration time.Duration, nowTimestamp int64) bool {
	cutoff := nowTimestamp - int64(duration.Seconds())
	return timestamp >= cutoff
}

// FormatDuration formats seconds into a human-readable duration string
func FormatDuration(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	
	if hours > 0 {
		if minutes > 0 {
			return time.Duration(seconds * int64(time.Second)).String()
		}
		return time.Duration(hours * int64(time.Hour)).String()
	}
	if minutes > 0 {
		return time.Duration(seconds * int64(time.Second)).String()
	}
	return time.Duration(secs * int64(time.Second)).String()
}

// CalculateWindowBoundaries calculates the start and end time for a window
// given a timestamp and duration
func CalculateWindowBoundaries(timestamp int64, duration time.Duration) (start, end int64) {
	start = timestamp
	end = timestamp + int64(duration.Seconds())
	return start, end
}

// FormatUnixToString formats a Unix timestamp to a readable string
func FormatUnixToString(unixTime int64) string {
	if unixTime == 0 {
		return ""
	}
	return time.Unix(unixTime, 0).Format("2006-01-02 15:04:05")
}

// IsSameDay checks if two timestamps are on the same day
func IsSameDay(time1, time2 int64) bool {
	t1 := time.Unix(time1, 0)
	t2 := time.Unix(time2, 0)
	return t1.Year() == t2.Year() && t1.YearDay() == t2.YearDay()
}

// GetElapsedTime calculates the elapsed time between two timestamps
func GetElapsedTime(startTime, endTime int64) float64 {
	if endTime <= startTime {
		return 0
	}
	return float64(endTime - startTime)
}

// GetElapsedMinutes calculates the elapsed minutes between two timestamps
func GetElapsedMinutes(startTime, endTime int64) float64 {
	elapsed := GetElapsedTime(startTime, endTime)
	return elapsed / 60.0
}

// GetElapsedHours calculates the elapsed hours between two timestamps
func GetElapsedHours(startTime, endTime int64) float64 {
	elapsed := GetElapsedTime(startTime, endTime)
	return elapsed / 3600.0
}
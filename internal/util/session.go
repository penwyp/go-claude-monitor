package util

import (
	"time"
)

// CalculateSessionElapsedTime calculates how much time has elapsed in the current session
// Returns elapsed time and remaining time until reset
func CalculateSessionElapsedTime(resetTime int64) (elapsedTime time.Duration, remainingTime time.Duration) {
	// Total session duration is 5 hours
	totalSessionDuration := 5 * time.Hour

	// Get current time
	now := GetTimeProvider().Now()

	// Calculate elapsed time from reset time
	// ResetTime is 5 hours in the future, so elapsed = 5 hours - time remaining
	if resetTime != 0 {
		resetTimeObj := time.Unix(resetTime, 0).UTC()
		resetTimeLocal := GetTimeProvider().In(resetTimeObj)
		remainingTime = resetTimeLocal.Sub(now)
		elapsedTime = totalSessionDuration - remainingTime

		// Ensure values are not negative
		if elapsedTime < 0 {
			elapsedTime = 0
		}
		if remainingTime < 0 {
			// Session has expired, show full duration elapsed
			elapsedTime = totalSessionDuration
			remainingTime = 0
		}
	} else {
		// If no reset time, return zero values to indicate no active session
		elapsedTime = 0
		remainingTime = 0
	}

	return elapsedTime, remainingTime
}

// CalculateSessionPercentage calculates the percentage of session time elapsed
func CalculateSessionPercentage(elapsedTime time.Duration) float64 {
	totalSessionDuration := 5 * time.Hour
	sessionPercent := (elapsedTime.Seconds() / totalSessionDuration.Seconds()) * 100
	if sessionPercent > 100 {
		sessionPercent = 100
	}
	if sessionPercent < 0 {
		sessionPercent = 0
	}
	return sessionPercent
}

package internal

import (
	"fmt"
	"math"
	"time"
)

// ValidateTimeRange validates that a time range is valid
func ValidateTimeRange(start, end int64) error {
	if start < 0 || end < 0 {
		return fmt.Errorf("invalid time range: negative timestamps")
	}
	if start > end {
		return fmt.Errorf("invalid time range: start (%d) > end (%d)", start, end)
	}
	return nil
}

// ValidateWindowDuration validates that a window has the expected duration
func ValidateWindowDuration(start, end int64, expected time.Duration) error {
	if err := ValidateTimeRange(start, end); err != nil {
		return err
	}
	
	actual := end - start
	expectedSeconds := int64(expected.Seconds())
	
	// Allow small tolerance (1 second)
	if math.Abs(float64(actual-expectedSeconds)) > 1 {
		return fmt.Errorf("window duration mismatch: expected %v, got %d seconds", 
			expected, actual)
	}
	return nil
}

// ValidateTokenDiscrepancy validates that token counts match within tolerance
func ValidateTokenDiscrepancy(sessionTokens, timelineTokens int64) error {
	if sessionTokens == timelineTokens {
		return nil
	}
	
	// Calculate percentage difference
	var percentDiff float64
	if timelineTokens > 0 {
		percentDiff = math.Abs(float64(sessionTokens-timelineTokens)) / float64(timelineTokens) * 100
	} else if sessionTokens > 0 {
		percentDiff = 100 // If timeline has 0 but session has tokens, that's 100% diff
	}
	
	// Allow up to 1% discrepancy
	if percentDiff > 1 {
		return fmt.Errorf("token count discrepancy: sessions=%d, timeline=%d (%.2f%% difference)",
			sessionTokens, timelineTokens, percentDiff)
	}
	
	return nil
}

// ValidateTimeBounds validates that a timestamp is within specified bounds
func ValidateTimeBounds(timestamp int64, minTime, maxTime int64) error {
	if timestamp < minTime {
		return fmt.Errorf("timestamp %d is before minimum time %d", timestamp, minTime)
	}
	if maxTime > 0 && timestamp > maxTime {
		return fmt.Errorf("timestamp %d is after maximum time %d", timestamp, maxTime)
	}
	return nil
}

// CheckWindowOverlap checks if two windows overlap
func CheckWindowOverlap(start1, end1, start2, end2 int64) bool {
	// Windows overlap if one starts before the other ends
	return start1 < end2 && start2 < end1
}

// AdjustWindowForConflict adjusts a proposed window to avoid conflict with an existing window
func AdjustWindowForConflict(proposedStart, proposedEnd, existingStart, existingEnd int64) (adjustedStart, adjustedEnd int64, adjusted bool) {
	// If no overlap, return as-is
	if !CheckWindowOverlap(proposedStart, proposedEnd, existingStart, existingEnd) {
		return proposedStart, proposedEnd, false
	}
	
	// If proposed window completely contains existing, keep proposed
	if proposedStart <= existingStart && proposedEnd >= existingEnd {
		return proposedStart, proposedEnd, false
	}
	
	// If existing window completely contains proposed, use existing
	if existingStart <= proposedStart && existingEnd >= proposedEnd {
		return existingStart, existingEnd, true
	}
	
	// Partial overlap - adjust to avoid conflict
	// If proposed starts within existing, start after existing ends
	if proposedStart >= existingStart && proposedStart < existingEnd {
		newStart := existingEnd
		newEnd := newStart + (proposedEnd - proposedStart)
		return newStart, newEnd, true
	}
	
	// If proposed ends within existing, end before existing starts
	if proposedEnd > existingStart && proposedEnd <= existingEnd {
		newEnd := existingStart
		newStart := newEnd - (proposedEnd - proposedStart)
		if newStart < 0 {
			newStart = 0
		}
		return newStart, newEnd, true
	}
	
	// Default: no adjustment
	return proposedStart, proposedEnd, false
}

// ValidateSessionData validates basic session data consistency
func ValidateSessionData(messageCount, sentMessageCount, totalTokens int) error {
	if messageCount < 0 || sentMessageCount < 0 || totalTokens < 0 {
		return fmt.Errorf("negative values not allowed in session data")
	}
	
	if sentMessageCount > messageCount {
		return fmt.Errorf("sent message count (%d) cannot exceed total message count (%d)",
			sentMessageCount, messageCount)
	}
	
	// If there are messages, there should typically be tokens (but allow 0 for edge cases)
	// Just log a warning internally if needed
	
	return nil
}
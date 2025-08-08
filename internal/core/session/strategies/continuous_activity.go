package strategies

import (
	"fmt"
)

// ContinuousActivityStrategy ensures strict 5-hour window boundaries
// even when activity is continuous without gaps
type ContinuousActivityStrategy struct {
	BaseStrategy
}

// NewContinuousActivityStrategy creates a new continuous activity strategy
func NewContinuousActivityStrategy() *ContinuousActivityStrategy {
	return &ContinuousActivityStrategy{
		BaseStrategy: NewBaseStrategy(
			"continuous_activity",
			8, // Higher than gap(5) and first_message(3), lower than limit(9-10)
			"Strict 5-hour windows for continuous activity",
		),
	}
}

// Detect generates strict 5-hour windows for all activity periods
func (s *ContinuousActivityStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if len(input.GlobalTimeline) == 0 {
		logDebug("ContinuousActivityStrategy: No timeline data")
		return candidates
	}
	
	firstActivity := input.GlobalTimeline[0].Timestamp
	lastActivity := input.GlobalTimeline[len(input.GlobalTimeline)-1].Timestamp
	sessionSeconds := int64(input.SessionDuration.Seconds())
	
	// Start from the first activity's hour boundary
	currentWindowStart := truncateToHour(firstActivity)
	
	logInfo(fmt.Sprintf("ContinuousActivityStrategy: Generating strict 5-hour windows from %s to %s",
		formatTimestamp(firstActivity),
		formatTimestamp(lastActivity)))
	
	windowCount := 0
	for currentWindowStart <= lastActivity {
		windowEnd := currentWindowStart + sessionSeconds
		
		// Check if this window period has any activity
		hasActivity := false
		activityCount := 0
		
		for _, tl := range input.GlobalTimeline {
			// Activity belongs to this window if: startTime <= activity < endTime
			if tl.Timestamp >= currentWindowStart && tl.Timestamp < windowEnd {
				hasActivity = true
				activityCount++
			}
		}
		
		if hasActivity {
			candidates = append(candidates, WindowCandidate{
				StartTime: currentWindowStart,
				EndTime:   windowEnd,
				Source:    s.Name(),
				Priority:  s.Priority(),
				IsLimit:   false,
				Metadata: map[string]string{
					"activity_count": fmt.Sprintf("%d", activityCount),
					"window_index":   fmt.Sprintf("%d", windowCount),
				},
			})
			
			logDebug(fmt.Sprintf("ContinuousActivityStrategy: Added window %d: %s to %s (%d activities)",
				windowCount,
				formatTimestamp(currentWindowStart),
				formatTimestamp(windowEnd),
				activityCount))
			
			windowCount++
		}
		
		// Move to next 5-hour window boundary
		currentWindowStart = windowEnd
	}
	
	logInfo(fmt.Sprintf("ContinuousActivityStrategy: Generated %d strict 5-hour windows", len(candidates)))
	return candidates
}


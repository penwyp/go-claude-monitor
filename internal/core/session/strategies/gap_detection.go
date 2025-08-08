package strategies

import (
	"fmt"
)

// GapDetectionStrategy detects new session windows based on time gaps between messages
// A gap of 5+ hours indicates a new session window
type GapDetectionStrategy struct {
	BaseStrategy
}

// NewGapDetectionStrategy creates a new gap detection strategy
func NewGapDetectionStrategy() *GapDetectionStrategy {
	return &GapDetectionStrategy{
		BaseStrategy: NewBaseStrategy(
			"gap",
			5,
			"Windows created after 5+ hour gaps",
		),
	}
}

// Detect finds windows based on time gaps in the timeline
func (s *GapDetectionStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if len(input.GlobalTimeline) <= 1 {
		logDebug("GapDetectionStrategy: Not enough timeline entries for gap detection")
		return candidates
	}
	
	sessionSeconds := int64(input.SessionDuration.Seconds())
	gapCount := 0
	
	// Look for gaps between consecutive messages
	for i := 1; i < len(input.GlobalTimeline); i++ {
		prevTimestamp := input.GlobalTimeline[i-1].Timestamp
		currTimestamp := input.GlobalTimeline[i].Timestamp
		gap := currTimestamp - prevTimestamp
		
		// If gap is >= session duration, a new window starts
		if gap >= sessionSeconds {
			gapCount++
			
			// New window starts at the current message (after the gap)
			windowStart := truncateToHour(currTimestamp)
			windowEnd := windowStart + sessionSeconds
			
			candidates = append(candidates, WindowCandidate{
				StartTime: windowStart,
				EndTime:   windowEnd,
				Source:    s.Name(),
				Priority:  s.Priority(),
				IsLimit:   false,
				Metadata: map[string]string{
					"gap_duration": fmt.Sprintf("%.1f hours", float64(gap)/3600),
					"gap_index":    fmt.Sprintf("%d", gapCount),
					"prev_time":    formatTimestamp(prevTimestamp),
					"curr_time":    formatTimestamp(currTimestamp),
				},
			})
			
			logInfo(fmt.Sprintf("GapDetectionStrategy: Detected %.1f hour gap at %s, new window %s-%s",
				float64(gap)/3600,
				formatTimestamp(currTimestamp),
				formatTimestamp(windowStart),
				formatTimestamp(windowEnd)))
		}
	}
	
	logInfo(fmt.Sprintf("GapDetectionStrategy: Detected %d gaps resulting in %d new windows", 
		gapCount, len(candidates)))
	return candidates
}
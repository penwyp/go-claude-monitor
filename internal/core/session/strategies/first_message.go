package strategies

import (
	"fmt"
)

// FirstMessageStrategy creates a window starting from the first message
// This is the fallback strategy when no other context is available
type FirstMessageStrategy struct {
	BaseStrategy
}

// NewFirstMessageStrategy creates a new first message strategy
func NewFirstMessageStrategy() *FirstMessageStrategy {
	return &FirstMessageStrategy{
		BaseStrategy: NewBaseStrategy(
			"first_message",
			3, // Lowest priority
			"Window starting from first activity",
		),
	}
}

// Detect creates a single window starting from the first message
func (s *FirstMessageStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if len(input.GlobalTimeline) == 0 {
		logDebug("FirstMessageStrategy: No timeline data")
		return candidates
	}
	
	firstTimestamp := input.GlobalTimeline[0].Timestamp
	sessionSeconds := int64(input.SessionDuration.Seconds())
	
	// Create a window starting from the first message's hour boundary
	windowStart := truncateToHour(firstTimestamp)
	windowEnd := windowStart + sessionSeconds
	
	candidates = append(candidates, WindowCandidate{
		StartTime: windowStart,
		EndTime:   windowEnd,
		Source:    s.Name(),
		Priority:  s.Priority(),
		IsLimit:   false,
		Metadata: map[string]string{
			"first_activity": formatTimestamp(firstTimestamp),
			"project":        input.GlobalTimeline[0].ProjectName,
		},
	})
	
	logDebug(fmt.Sprintf("FirstMessageStrategy: Created initial window %s-%s from first activity at %s",
		formatTimestamp(windowStart),
		formatTimestamp(windowEnd),
		formatTimestamp(firstTimestamp)))
	
	return candidates
}
package internal

import (
	"time"
)

// WindowDetectionStrategy defines the interface for different window detection strategies
type WindowDetectionStrategy interface {
	// DetectWindows finds potential session windows based on the strategy
	DetectWindows(logs []TimestampedLog, sessionDuration time.Duration) []WindowCandidate
	// GetPriority returns the priority of this strategy (higher is better)
	GetPriority() int
	// GetName returns the name of this strategy
	GetName() string
}

// TimestampedLog represents a log entry with timestamp (simplified from timeline.TimestampedLog)
type TimestampedLog struct {
	Timestamp    int64
	ProjectName  string
	Content      string
	Type         string
	ResetTime    *int64 // For limit messages
}

// WindowCandidate represents a potential session window
type WindowCandidate struct {
	StartTime int64
	EndTime   int64
	Source    string
	Priority  int
	IsLimit   bool
}

// GapDetectionStrategy detects windows based on time gaps between messages
type GapDetectionStrategy struct{}

func NewGapDetectionStrategy() *GapDetectionStrategy {
	return &GapDetectionStrategy{}
}

func (s *GapDetectionStrategy) DetectWindows(logs []TimestampedLog, sessionDuration time.Duration) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	if len(logs) <= 1 {
		return candidates
	}
	
	sessionSeconds := int64(sessionDuration.Seconds())
	for i := 1; i < len(logs); i++ {
		gap := logs[i].Timestamp - logs[i-1].Timestamp
		if gap >= sessionSeconds {
			// Gap detected, new window starts at current message
			windowStart := TruncateToHour(logs[i].Timestamp)
			candidates = append(candidates, WindowCandidate{
				StartTime: windowStart,
				EndTime:   windowStart + sessionSeconds,
				Source:    s.GetName(),
				Priority:  s.GetPriority(),
				IsLimit:   false,
			})
		}
	}
	
	return candidates
}

func (s *GapDetectionStrategy) GetPriority() int {
	return 5
}

func (s *GapDetectionStrategy) GetName() string {
	return "gap"
}

// FirstMessageStrategy detects windows based on the first message
type FirstMessageStrategy struct{}

func NewFirstMessageStrategy() *FirstMessageStrategy {
	return &FirstMessageStrategy{}
}

func (s *FirstMessageStrategy) DetectWindows(logs []TimestampedLog, sessionDuration time.Duration) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	if len(logs) == 0 {
		return candidates
	}
	
	sessionSeconds := int64(sessionDuration.Seconds())
	firstTimestamp := logs[0].Timestamp
	windowStart := TruncateToHour(firstTimestamp)
	
	candidates = append(candidates, WindowCandidate{
		StartTime: windowStart,
		EndTime:   windowStart + sessionSeconds,
		Source:    s.GetName(),
		Priority:  s.GetPriority(),
		IsLimit:   false,
	})
	
	return candidates
}

func (s *FirstMessageStrategy) GetPriority() int {
	return 3
}

func (s *FirstMessageStrategy) GetName() string {
	return "first_message"
}

// LimitMessageStrategy detects windows based on rate limit messages
type LimitMessageStrategy struct {
	parser LimitMessageParser
}

// LimitMessageParser interface for parsing limit messages
type LimitMessageParser interface {
	ParseResetTime(content string, timestamp int64) *int64
}

func NewLimitMessageStrategy(parser LimitMessageParser) *LimitMessageStrategy {
	return &LimitMessageStrategy{
		parser: parser,
	}
}

func (s *LimitMessageStrategy) DetectWindows(logs []TimestampedLog, sessionDuration time.Duration) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	sessionSeconds := int64(sessionDuration.Seconds())
	
	for _, log := range logs {
		if log.ResetTime != nil {
			windowStart := *log.ResetTime - sessionSeconds
			candidates = append(candidates, WindowCandidate{
				StartTime: windowStart,
				EndTime:   *log.ResetTime,
				Source:    s.GetName(),
				Priority:  s.GetPriority(),
				IsLimit:   true,
			})
		}
	}
	
	return candidates
}

func (s *LimitMessageStrategy) GetPriority() int {
	return 9
}

func (s *LimitMessageStrategy) GetName() string {
	return "limit_message"
}

// WindowSelector selects the best non-overlapping windows from candidates
type WindowSelector struct {
	sessionDuration time.Duration
}

func NewWindowSelector(sessionDuration time.Duration) *WindowSelector {
	return &WindowSelector{
		sessionDuration: sessionDuration,
	}
}

func (s *WindowSelector) SelectBestWindows(candidates []WindowCandidate) []WindowCandidate {
	if len(candidates) == 0 {
		return []WindowCandidate{}
	}
	
	// Sort by priority (descending) then by start time (ascending)
	sortWindowCandidates(candidates)
	
	selected := make([]WindowCandidate, 0)
	sessionSeconds := int64(s.sessionDuration.Seconds())
	
	for _, candidate := range candidates {
		// Ensure window is exactly 5 hours
		if candidate.EndTime-candidate.StartTime != sessionSeconds {
			candidate.EndTime = candidate.StartTime + sessionSeconds
		}
		
		// Check for overlap with already selected windows
		if !s.hasOverlap(candidate, selected) {
			selected = append(selected, candidate)
		}
	}
	
	// Sort selected windows by start time
	sortWindowsByStartTime(selected)
	
	return selected
}

func (s *WindowSelector) hasOverlap(candidate WindowCandidate, selected []WindowCandidate) bool {
	for _, sel := range selected {
		if s.windowsOverlap(candidate, sel) {
			return true
		}
	}
	return false
}

func (s *WindowSelector) windowsOverlap(w1, w2 WindowCandidate) bool {
	return w1.StartTime < w2.EndTime && w1.EndTime > w2.StartTime
}

// Helper function to sort candidates by priority and start time
func sortWindowCandidates(candidates []WindowCandidate) {
	// Implementation would use sort.Slice
	// This is a placeholder - actual implementation would be in detector_refactored.go
}

// Helper function to sort windows by start time
func sortWindowsByStartTime(windows []WindowCandidate) {
	// Implementation would use sort.Slice
	// This is a placeholder - actual implementation would be in detector_refactored.go
}
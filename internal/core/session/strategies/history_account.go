package strategies

import (
	"fmt"
	"time"
)

// HistoryAccountStrategy detects windows from historical account-level windows
// that are not from limit messages
type HistoryAccountStrategy struct {
	BaseStrategy
}

// NewHistoryAccountStrategy creates a new history account strategy
func NewHistoryAccountStrategy() *HistoryAccountStrategy {
	return &HistoryAccountStrategy{
		BaseStrategy: NewBaseStrategy(
			"history_account",
			7,
			"Account-level windows from history",
		),
	}
}

// Detect finds account-level windows from history (excluding limit windows)
func (s *HistoryAccountStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if input.WindowHistory == nil {
		logDebug("HistoryAccountStrategy: No window history available")
		return candidates
	}
	
	// Get recent windows from the last 24 hours
	recentWindows := input.WindowHistory.GetRecentWindows(24 * time.Hour)
	logDebug(fmt.Sprintf("HistoryAccountStrategy: Found %d recent windows", len(recentWindows)))
	
	for _, w := range recentWindows {
		// Only interested in account-level windows that are NOT from limit messages
		// (limit messages are handled by HistoryLimitStrategy with higher priority)
		if w.IsAccountLevel && !w.IsLimitReached {
			candidates = append(candidates, WindowCandidate{
				StartTime: w.StartTime,
				EndTime:   w.EndTime,
				Source:    s.Name(),
				Priority:  s.Priority(),
				IsLimit:   false,
				Metadata: map[string]string{
					"session_id":      w.SessionID,
					"original_source": w.Source,
					"account_level":   "true",
				},
			})
			
			logDebug(fmt.Sprintf("HistoryAccountStrategy: Added account window %s-%s",
				formatTimestamp(w.StartTime),
				formatTimestamp(w.EndTime)))
		}
	}
	
	logInfo(fmt.Sprintf("HistoryAccountStrategy: Detected %d account-level windows", len(candidates)))
	return candidates
}
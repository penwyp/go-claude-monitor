package strategies

import (
	"fmt"
)

// HistoryLimitStrategy detects windows from historical limit messages stored in window history
// These are the most authoritative windows as they come from confirmed rate limit hits
type HistoryLimitStrategy struct {
	BaseStrategy
}

// NewHistoryLimitStrategy creates a new history limit strategy
func NewHistoryLimitStrategy() *HistoryLimitStrategy {
	return &HistoryLimitStrategy{
		BaseStrategy: NewBaseStrategy(
			"history_limit",
			10, // Highest priority
			"Historical limit windows from previous rate limit messages",
		),
	}
}

// Detect finds windows from historical limit messages
func (s *HistoryLimitStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if input.WindowHistory == nil {
		logDebug("HistoryLimitStrategy: No window history available")
		return candidates
	}
	
	// Get account-level windows from history
	accountWindows := input.WindowHistory.GetAccountLevelWindows()
	logDebug(fmt.Sprintf("HistoryLimitStrategy: Found %d account-level windows", len(accountWindows)))
	
	for _, w := range accountWindows {
		// Only interested in limit-reached windows from limit messages
		if w.IsLimitReached && w.Source == "limit_message" {
			candidates = append(candidates, WindowCandidate{
				StartTime:    w.StartTime,
				EndTime:      w.EndTime,
				Source:       s.Name(),
				Priority:     s.Priority(),
				IsLimit:      true,
				LimitMessage: w.LimitMessage,
				Metadata: map[string]string{
					"session_id":     w.SessionID,
					"original_source": w.Source,
					"account_level":  "true",
				},
			})
			
			logInfo(fmt.Sprintf("HistoryLimitStrategy: Added historical limit window %s-%s",
				formatTimestamp(w.StartTime),
				formatTimestamp(w.EndTime)))
		}
	}
	
	logInfo(fmt.Sprintf("HistoryLimitStrategy: Detected %d limit windows from history", len(candidates)))
	return candidates
}
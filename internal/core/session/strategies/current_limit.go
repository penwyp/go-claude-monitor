package strategies

import (
	"fmt"
)

// LimitParser interface for parsing limit messages
type LimitParser interface {
	ParseLogs(logs []interface{}) []LimitInfo
}

// LimitInfo contains information extracted from a limit message
type LimitInfo struct {
	Content   string
	Timestamp int64
	ResetTime *int64
}

// CurrentLimitStrategy detects windows from current limit messages in the logs
type CurrentLimitStrategy struct {
	BaseStrategy
	limitParser LimitParser
}

// NewCurrentLimitStrategy creates a new current limit strategy
func NewCurrentLimitStrategy(parser LimitParser) *CurrentLimitStrategy {
	return &CurrentLimitStrategy{
		BaseStrategy: NewBaseStrategy(
			"limit_message",
			9, // Second highest priority
			"Current limit messages with reset times",
		),
		limitParser: parser,
	}
}

// Detect finds windows from current limit messages
func (s *CurrentLimitStrategy) Detect(input DetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	if s.limitParser == nil || len(input.RawLogs) == 0 {
		logDebug("CurrentLimitStrategy: No limit parser or no logs to parse")
		return candidates
	}
	
	// Convert logs to interface{} for parser
	logs := make([]interface{}, len(input.RawLogs))
	for i, log := range input.RawLogs {
		logs[i] = log
	}
	
	// Parse limit messages from logs
	limits := s.limitParser.ParseLogs(logs)
	logInfo(fmt.Sprintf("CurrentLimitStrategy: Parsed %d limit messages", len(limits)))
	
	sessionSeconds := int64(input.SessionDuration.Seconds())
	
	for _, limit := range limits {
		if limit.ResetTime != nil {
			// Window starts 5 hours before reset time
			windowStart := *limit.ResetTime - sessionSeconds
			
			candidates = append(candidates, WindowCandidate{
				StartTime:    windowStart,
				EndTime:      *limit.ResetTime,
				Source:       s.Name(),
				Priority:     s.Priority(),
				IsLimit:      true,
				LimitMessage: limit.Content,
				Metadata: map[string]string{
					"message_time": fmt.Sprintf("%d", limit.Timestamp),
					"reset_time":   fmt.Sprintf("%d", *limit.ResetTime),
				},
			})
			
			logInfo(fmt.Sprintf("CurrentLimitStrategy: Added limit window %s-%s from message",
				formatTimestamp(windowStart),
				formatTimestamp(*limit.ResetTime)))
		}
	}
	
	logInfo(fmt.Sprintf("CurrentLimitStrategy: Detected %d current limit windows", len(candidates)))
	return candidates
}
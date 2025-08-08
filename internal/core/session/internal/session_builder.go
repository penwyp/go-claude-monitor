package internal

import (
	"time"
)

// SessionBuilder handles the creation and population of sessions
type SessionBuilder struct {
	timezone        *time.Location
	sessionDuration time.Duration
}

// NewSessionBuilder creates a new session builder
func NewSessionBuilder(timezone *time.Location, sessionDuration time.Duration) *SessionBuilder {
	return &SessionBuilder{
		timezone:        timezone,
		sessionDuration: sessionDuration,
	}
}

// BuildSession creates a new session for a given window
func (b *SessionBuilder) BuildSession(window WindowCandidate, projectName string) *SessionData {
	session := &SessionData{
		StartTime:       window.StartTime,
		EndTime:         window.EndTime,
		ProjectName:     projectName,
		MessageCount:    0,
		TotalTokens:     0,
		InputTokens:     0,
		OutputTokens:    0,
		CacheReadTokens: 0,
		CacheWriteTokens: 0,
		Models:          make(map[string]int),
		IsLimitReached:  window.IsLimit,
		WindowSource:    window.Source,
	}
	
	// Set formatted times
	b.setFormattedTimes(session)
	
	return session
}

// SessionData represents a simplified session structure for building
type SessionData struct {
	StartTime        int64
	EndTime          int64
	ProjectName      string
	MessageCount     int
	TotalTokens      int
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	Models           map[string]int
	IsLimitReached   bool
	WindowSource     string
	FormattedStart   string
	FormattedEnd     string
	Duration         time.Duration
	IsActive         bool
	IsCrossProject   bool
	IsAccountLevel   bool
}

// AddLog adds a log entry to the session
func (b *SessionBuilder) AddLog(session *SessionData, log TimestampedLog, usage TokenUsage) {
	session.MessageCount++
	
	// Update token counts - calculate directly here
	totalTokens := usage.InputTokens + usage.OutputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
	session.TotalTokens += totalTokens
	session.InputTokens += usage.InputTokens
	session.OutputTokens += usage.OutputTokens
	session.CacheReadTokens += usage.CacheReadInputTokens
	session.CacheWriteTokens += usage.CacheCreationInputTokens
	
	// Track model usage
	if model := usage.Model; model != "" {
		session.Models[model] = session.Models[model] + 1
	}
	
	// Check for cross-project activity
	if session.ProjectName != log.ProjectName && log.ProjectName != "" {
		session.IsCrossProject = true
		if session.ProjectName == "Multiple" || session.ProjectName == "" {
			session.ProjectName = "Multiple"
			session.IsAccountLevel = true
		}
	}
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
	Model                    string
}

// Finalize finalizes the session after all logs have been added
func (b *SessionBuilder) Finalize(session *SessionData) {
	// Calculate actual duration based on messages
	if session.MessageCount > 0 {
		session.Duration = time.Duration(session.EndTime-session.StartTime) * time.Second
	} else {
		session.Duration = b.sessionDuration
	}
	
	// Update formatted times after finalization
	b.setFormattedTimes(session)
}

// setFormattedTimes sets the formatted time strings for display
func (b *SessionBuilder) setFormattedTimes(session *SessionData) {
	startTime := time.Unix(session.StartTime, 0).In(b.timezone)
	endTime := time.Unix(session.EndTime, 0).In(b.timezone)
	
	session.FormattedStart = startTime.Format("2006-01-02 15:04:05")
	session.FormattedEnd = endTime.Format("15:04:05")
	session.Duration = endTime.Sub(startTime)
}

// MarkActive marks a session as active if it's within the active threshold
func (b *SessionBuilder) MarkActive(session *SessionData, nowTimestamp int64, activeThreshold time.Duration) {
	timeSinceEnd := nowTimestamp - session.EndTime
	if timeSinceEnd >= 0 && timeSinceEnd <= int64(activeThreshold.Seconds()) {
		session.IsActive = true
	}
}
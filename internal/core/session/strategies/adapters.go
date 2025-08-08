package strategies

import (
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

// WindowHistoryAdapter adapts the existing WindowHistoryManager to the WindowHistoryAccess interface
type WindowHistoryAdapter struct {
	manager interface{
		GetAccountLevelWindows() []interface{}
		GetRecentWindows(duration time.Duration) []interface{}
		GetLimitReachedWindows() []interface{}
	}
}

// NewWindowHistoryAdapter creates a new adapter
func NewWindowHistoryAdapter(manager interface{}) *WindowHistoryAdapter {
	if mgr, ok := manager.(interface{
		GetAccountLevelWindows() []interface{}
		GetRecentWindows(duration time.Duration) []interface{}
		GetLimitReachedWindows() []interface{}
	}); ok {
		return &WindowHistoryAdapter{manager: mgr}
	}
	return &WindowHistoryAdapter{}
}

// GetAccountLevelWindows returns all account-level windows
func (a *WindowHistoryAdapter) GetAccountLevelWindows() []HistoricalWindow {
	if a.manager == nil {
		return []HistoricalWindow{}
	}
	
	windows := make([]HistoricalWindow, 0)
	for _, w := range a.manager.GetAccountLevelWindows() {
		if window, ok := convertToHistoricalWindow(w); ok {
			windows = append(windows, window)
		}
	}
	return windows
}

// GetRecentWindows returns windows from the specified duration
func (a *WindowHistoryAdapter) GetRecentWindows(duration time.Duration) []HistoricalWindow {
	if a.manager == nil {
		return []HistoricalWindow{}
	}
	
	windows := make([]HistoricalWindow, 0)
	for _, w := range a.manager.GetRecentWindows(duration) {
		if window, ok := convertToHistoricalWindow(w); ok {
			windows = append(windows, window)
		}
	}
	return windows
}

// GetLimitReachedWindows returns all windows where limit was reached
func (a *WindowHistoryAdapter) GetLimitReachedWindows() []HistoricalWindow {
	if a.manager == nil {
		return []HistoricalWindow{}
	}
	
	windows := make([]HistoricalWindow, 0)
	for _, w := range a.manager.GetLimitReachedWindows() {
		if window, ok := convertToHistoricalWindow(w); ok {
			windows = append(windows, window)
		}
	}
	return windows
}

// convertToHistoricalWindow converts a WindowRecord to HistoricalWindow
func convertToHistoricalWindow(w interface{}) (HistoricalWindow, bool) {
	// Use reflection or type assertion based on actual WindowRecord structure
	// This is a simplified version - adjust based on actual types
	if record, ok := w.(interface{
		GetSessionID() string
		GetSource() string
		GetStartTime() int64
		GetEndTime() int64
		GetIsLimitReached() bool
		GetIsAccountLevel() bool
		GetLimitMessage() string
	}); ok {
		return HistoricalWindow{
			SessionID:      record.GetSessionID(),
			Source:         record.GetSource(),
			StartTime:      record.GetStartTime(),
			EndTime:        record.GetEndTime(),
			IsLimitReached: record.GetIsLimitReached(),
			IsAccountLevel: record.GetIsAccountLevel(),
			LimitMessage:   record.GetLimitMessage(),
		}, true
	}
	
	// Fallback for direct field access (if WindowRecord fields are exported)
	type windowRecord struct {
		SessionID      string
		Source         string
		StartTime      int64
		EndTime        int64
		IsLimitReached bool
		IsAccountLevel bool
		LimitMessage   string
	}
	
	if record, ok := w.(*windowRecord); ok {
		return HistoricalWindow{
			SessionID:      record.SessionID,
			Source:         record.Source,
			StartTime:      record.StartTime,
			EndTime:        record.EndTime,
			IsLimitReached: record.IsLimitReached,
			IsAccountLevel: record.IsAccountLevel,
			LimitMessage:   record.LimitMessage,
		}, true
	}
	
	return HistoricalWindow{}, false
}

// LimitParserAdapter adapts the existing LimitParser to the strategy interface
type LimitParserAdapter struct {
	parser interface{
		ParseLogs(logs []model.ConversationLog) []interface{}
	}
}

// NewLimitParserAdapter creates a new adapter
func NewLimitParserAdapter(parser interface{}) *LimitParserAdapter {
	if p, ok := parser.(interface{
		ParseLogs(logs []model.ConversationLog) []interface{}
	}); ok {
		return &LimitParserAdapter{parser: p}
	}
	return &LimitParserAdapter{}
}

// ParseLogs parses logs and returns limit information
func (a *LimitParserAdapter) ParseLogs(logs []interface{}) []LimitInfo {
	if a.parser == nil {
		return []LimitInfo{}
	}
	
	// Convert interface{} logs to model.ConversationLog
	conversationLogs := make([]model.ConversationLog, 0, len(logs))
	for _, log := range logs {
		if convLog, ok := log.(model.ConversationLog); ok {
			conversationLogs = append(conversationLogs, convLog)
		}
	}
	
	// Parse and convert results
	limits := make([]LimitInfo, 0)
	for _, l := range a.parser.ParseLogs(conversationLogs) {
		if limit, ok := convertToLimitInfo(l); ok {
			limits = append(limits, limit)
		}
	}
	return limits
}

// convertToLimitInfo converts parsed limit to LimitInfo
func convertToLimitInfo(l interface{}) (LimitInfo, bool) {
	// Use reflection or type assertion based on actual LimitResult structure
	if limit, ok := l.(interface{
		GetContent() string
		GetTimestamp() int64
		GetResetTime() *int64
	}); ok {
		return LimitInfo{
			Content:   limit.GetContent(),
			Timestamp: limit.GetTimestamp(),
			ResetTime: limit.GetResetTime(),
		}, true
	}
	
	// Fallback for direct field access
	type limitResult struct {
		Content   string
		Timestamp int64
		ResetTime *int64
	}
	
	if limit, ok := l.(*limitResult); ok {
		return LimitInfo{
			Content:   limit.Content,
			Timestamp: limit.Timestamp,
			ResetTime: limit.ResetTime,
		}, true
	}
	
	return LimitInfo{}, false
}
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session/internal"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// WindowRecordV2 represents a refactored session window in history
type WindowRecordV2 struct {
	// Core fields
	SessionID      string `json:"session_id"`
	Source         string `json:"source"`
	StartTime      int64  `json:"start_time"`
	EndTime        int64  `json:"end_time"`
	CreatedAt      int64  `json:"created_at"`
	
	// Limit-related fields
	IsLimitReached bool   `json:"is_limit_reached"`
	LimitMessage   string `json:"limit_message,omitempty"`
	
	// Optional fields
	FirstEntryTime int64  `json:"first_entry_time,omitempty"`
	IsAccountLevel bool   `json:"is_account_level,omitempty"`
	
	// String representations (computed)
	StartTimeStr string `json:"start_time_str"`
	EndTimeStr   string `json:"end_time_str"`
	CreatedAtStr string `json:"created_at_str"`
}

// populateStringFields fills the string representations of timestamps
func (w *WindowRecordV2) populateStringFields() {
	w.StartTimeStr = internal.FormatUnixToString(w.StartTime)
	w.EndTimeStr = internal.FormatUnixToString(w.EndTime)
	w.CreatedAtStr = internal.FormatUnixToString(w.CreatedAt)
}

// WindowHistoryV2 manages the history of session windows
type WindowHistoryV2 struct {
	Windows     []WindowRecordV2 `json:"windows"`
	LastUpdated int64           `json:"last_updated"`
	mu          sync.RWMutex
}

// WindowHistoryManagerV2 handles persistence and validation of window history
type WindowHistoryManagerV2 struct {
	history           *WindowHistoryV2
	historyPath       string
	validator         *internal.WindowValidator
	overlapChecker    *internal.WindowOverlapChecker
	boundsValidator   *internal.WindowBoundsValidator
	mu                sync.Mutex
}

// NewWindowHistoryManagerV2 creates a new refactored window history manager
func NewWindowHistoryManagerV2(cacheDir string) *WindowHistoryManagerV2 {
	historyPath := determineHistoryPath(cacheDir)
	
	return &WindowHistoryManagerV2{
		historyPath:     historyPath,
		history:         &WindowHistoryV2{Windows: make([]WindowRecordV2, 0)},
		validator:       internal.NewWindowValidator(),
		overlapChecker:  internal.NewWindowOverlapChecker(true),
		boundsValidator: internal.NewWindowBoundsValidator(
			constants.LimitWindowRetentionSeconds,
			constants.MaxFutureWindowSeconds,
		),
	}
}

// determineHistoryPath determines the path for the history file
func determineHistoryPath(cacheDir string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(cacheDir, "window_history.json")
	}
	
	historyDir := filepath.Join(homeDir, ".go-claude-monitor", "history")
	return filepath.Join(historyDir, "window_history.json")
}

// Load loads window history from disk
func (m *WindowHistoryManagerV2) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	data, err := os.ReadFile(m.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			util.LogInfo("No existing window history found, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to read window history: %w", err)
	}
	
	var history WindowHistoryV2
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("failed to unmarshal window history: %w", err)
	}
	
	// Populate string fields for all loaded records
	for i := range history.Windows {
		history.Windows[i].populateStringFields()
	}
	
	m.history = &history
	util.LogInfo(fmt.Sprintf("Loaded %d window records from history", len(history.Windows)))
	return nil
}

// Save saves window history to disk using atomic write
func (m *WindowHistoryManagerV2) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.history.mu.Lock()
	m.history.LastUpdated = time.Now().Unix()
	data, err := sonic.MarshalIndent(m.history, "", "  ")
	m.history.mu.Unlock()
	
	if err != nil {
		return fmt.Errorf("failed to marshal window history: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.historyPath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	// Atomic write: write to temp file first, then rename
	tempPath := m.historyPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write window history: %w", err)
	}
	
	if err := os.Rename(tempPath, m.historyPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to save window history: %w", err)
	}
	
	util.LogDebug(fmt.Sprintf("Saved %d window records to history", len(m.history.Windows)))
	return nil
}

// AddOrUpdateWindow adds a new window record or updates existing one
func (m *WindowHistoryManagerV2) AddOrUpdateWindow(record WindowRecordV2) {
	// Validate the window based on its type
	if err := m.validateWindow(record); err != nil {
		util.LogWarn(fmt.Sprintf("Rejecting invalid window: %v", err))
		return
	}
	
	m.history.mu.Lock()
	defer m.history.mu.Unlock()
	
	record.CreatedAt = time.Now().Unix()
	record.populateStringFields()
	
	// Check if window already exists
	for i, existing := range m.history.Windows {
		if existing.SessionID == record.SessionID {
			// Update existing record, preserving important fields
			record = m.mergeRecords(existing, record)
			m.history.Windows[i] = record
			util.LogDebug(fmt.Sprintf("Updated window record: %s (%s)", record.SessionID, record.Source))
			return
		}
	}
	
	// Add new record and sort
	m.history.Windows = append(m.history.Windows, record)
	util.LogDebug(fmt.Sprintf("Added new window record: %s (%s)", record.SessionID, record.Source))
	
	// Sort by start time
	sort.Slice(m.history.Windows, func(i, j int) bool {
		return m.history.Windows[i].StartTime < m.history.Windows[j].StartTime
	})
}

// validateWindow validates a window record based on its type
func (m *WindowHistoryManagerV2) validateWindow(record WindowRecordV2) error {
	if record.IsLimitReached {
		return m.validator.ValidateLimitWindow(record.EndTime)
	}
	return m.validator.ValidateNormalWindow(record.EndTime)
}

// mergeRecords merges two window records, preserving important fields
func (m *WindowHistoryManagerV2) mergeRecords(existing, new WindowRecordV2) WindowRecordV2 {
	// Preserve limit reached status if it exists
	if existing.IsLimitReached && !new.IsLimitReached {
		new.IsLimitReached = true
		new.Source = existing.Source
		new.LimitMessage = existing.LimitMessage
	}
	return new
}

// ValidateNewWindow checks if a proposed window is valid based on history
func (m *WindowHistoryManagerV2) ValidateNewWindow(proposedStart, proposedEnd int64) (validStart, validEnd int64, isValid bool) {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()
	
	validStart, validEnd = proposedStart, proposedEnd
	
	util.LogDebug(fmt.Sprintf("ValidateNewWindow: Proposed %s to %s",
		internal.FormatUnixToString(proposedStart),
		internal.FormatUnixToString(proposedEnd)))
	
	// First, handle limit message windows (highest priority)
	validStart, validEnd = m.adjustForLimitWindows(validStart, validEnd)
	
	// Then, handle other windows
	validStart, validEnd = m.adjustForOtherWindows(validStart, validEnd)
	
	// Ensure valid duration
	if !m.boundsValidator.IsWindowDurationValid(validStart, validEnd) {
		validEnd = m.boundsValidator.EnsureValidDuration(validStart)
	}
	
	// Check if adjusted window is within bounds
	if !m.boundsValidator.IsFutureWindowValid(validEnd) {
		util.LogWarn(fmt.Sprintf("Window adjustment would create invalid future window"))
		return proposedStart, proposedEnd, true
	}
	
	isValid = (validStart == proposedStart)
	m.logValidationResult(proposedStart, proposedEnd, validStart, validEnd, isValid)
	
	return validStart, validEnd, isValid
}

// adjustForLimitWindows adjusts a window to avoid overlapping with limit message windows
func (m *WindowHistoryManagerV2) adjustForLimitWindows(start, end int64) (adjustedStart, adjustedEnd int64) {
	adjustedStart, adjustedEnd = start, end
	
	for _, record := range m.history.Windows {
		if !record.IsLimitReached || record.Source != "limit_message" {
			continue
		}
		
		if !m.boundsValidator.IsHistoricalWindowValid(record.EndTime) {
			continue
		}
		
		// Only adjust for same-day overlaps with limit windows
		if m.overlapChecker.AreSameDay(adjustedStart, record.StartTime) {
			adjustedStart, adjustedEnd = m.overlapChecker.AdjustForOverlap(
				adjustedStart, adjustedEnd, record.StartTime, record.EndTime)
		}
	}
	
	return adjustedStart, adjustedEnd
}

// adjustForOtherWindows adjusts a window to avoid overlapping with other windows
func (m *WindowHistoryManagerV2) adjustForOtherWindows(start, end int64) (adjustedStart, adjustedEnd int64) {
	adjustedStart, adjustedEnd = start, end
	
	for _, record := range m.history.Windows {
		// Skip limit message windows (already handled)
		if record.IsLimitReached && record.Source == "limit_message" {
			continue
		}
		
		if !m.boundsValidator.IsHistoricalWindowValid(record.EndTime) {
			continue
		}
		
		adjustedStart, adjustedEnd = m.overlapChecker.AdjustForOverlap(
			adjustedStart, adjustedEnd, record.StartTime, record.EndTime)
	}
	
	return adjustedStart, adjustedEnd
}

// logValidationResult logs the result of window validation
func (m *WindowHistoryManagerV2) logValidationResult(proposedStart, proposedEnd, validStart, validEnd int64, isValid bool) {
	if !isValid {
		util.LogDebug(fmt.Sprintf("Window adjusted: %s-%s -> %s-%s",
			internal.FormatUnixToString(proposedStart),
			internal.FormatUnixToString(proposedEnd),
			internal.FormatUnixToString(validStart),
			internal.FormatUnixToString(validEnd)))
	} else {
		util.LogDebug(fmt.Sprintf("Window not adjusted: %s-%s",
			internal.FormatUnixToString(validStart),
			internal.FormatUnixToString(validEnd)))
	}
}

// GetLimitReachedWindows returns all windows where limit was reached
func (m *WindowHistoryManagerV2) GetLimitReachedWindows() []WindowRecordV2 {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()
	
	var limitReached []WindowRecordV2
	for _, record := range m.history.Windows {
		if record.IsLimitReached {
			limitReached = append(limitReached, record)
		}
	}
	return limitReached
}

// GetAccountLevelWindows returns all account-level windows
func (m *WindowHistoryManagerV2) GetAccountLevelWindows() []WindowRecordV2 {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()
	
	var accountLevel []WindowRecordV2
	for _, record := range m.history.Windows {
		if record.IsAccountLevel {
			accountLevel = append(accountLevel, record)
		}
	}
	return accountLevel
}

// GetRecentWindows returns all windows from the specified duration
func (m *WindowHistoryManagerV2) GetRecentWindows(duration time.Duration) []WindowRecordV2 {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()
	
	cutoff := time.Now().Unix() - int64(duration.Seconds())
	var recent []WindowRecordV2
	
	for _, record := range m.history.Windows {
		if record.EndTime > cutoff {
			recent = append(recent, record)
		}
	}
	
	return recent
}

// UpdateFromLimitMessage updates window history based on a limit message
func (m *WindowHistoryManagerV2) UpdateFromLimitMessage(resetTime int64, messageTime int64, limitMessage string) {
	windowEnd := resetTime
	windowStart := windowEnd - constants.SessionDurationSeconds
	
	record := WindowRecordV2{
		StartTime:      windowStart,
		EndTime:        windowEnd,
		Source:         "limit_message",
		IsLimitReached: true,
		IsAccountLevel: true,
		LimitMessage:   limitMessage,
		SessionID:      fmt.Sprintf("%d", windowStart),
		CreatedAt:      time.Now().Unix(),
	}
	
	record.populateStringFields()
	m.AddOrUpdateWindow(record)
	
	util.LogInfo(fmt.Sprintf("Updated from limit message: %s to %s (account-level)",
		internal.FormatUnixToString(windowStart),
		internal.FormatUnixToString(windowEnd)))
}

// LoadHistoricalLimitWindows loads limit windows from historical logs
func (m *WindowHistoryManagerV2) LoadHistoricalLimitWindows(logs []model.ConversationLog) int {
	parser := NewLimitParser()
	limits := parser.ParseLogs(logs)
	
	util.LogInfo(fmt.Sprintf("Found %d limit messages in historical logs", len(limits)))
	
	addedCount := 0
	currentTime := time.Now().Unix()
	minAllowedTime := currentTime - constants.HistoricalScanSeconds
	
	for _, limit := range limits {
		if limit.ResetTime == nil || *limit.ResetTime < minAllowedTime {
			continue
		}
		
		windowEnd := *limit.ResetTime
		windowStart := windowEnd - constants.SessionDurationSeconds
		
		// Check if window already exists
		if m.windowExists(windowStart, windowEnd) {
			continue
		}
		
		record := WindowRecordV2{
			StartTime:      windowStart,
			EndTime:        windowEnd,
			Source:         "limit_message",
			IsLimitReached: true,
			IsAccountLevel: true,
			LimitMessage:   limit.Content,
			SessionID:      fmt.Sprintf("%d", windowStart),
			CreatedAt:      limit.Timestamp,
		}
		
		record.populateStringFields()
		m.AddOrUpdateWindow(record)
		addedCount++
		
		util.LogInfo(fmt.Sprintf("Added historical limit window: %s to %s",
			internal.FormatUnixToString(windowStart),
			internal.FormatUnixToString(windowEnd)))
	}
	
	if addedCount > 0 {
		if err := m.Save(); err != nil {
			util.LogWarn(fmt.Sprintf("Failed to save window history: %v", err))
		}
	}
	
	return addedCount
}

// windowExists checks if a window already exists in history
func (m *WindowHistoryManagerV2) windowExists(start, end int64) bool {
	for _, record := range m.history.Windows {
		if record.StartTime == start && record.EndTime == end && record.IsLimitReached {
			return true
		}
	}
	return false
}

// CleanOldWindows removes windows older than the retention period
func (m *WindowHistoryManagerV2) CleanOldWindows() int {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()
	
	currentTime := time.Now().Unix()
	minRetentionTime := currentTime - constants.LimitWindowRetentionSeconds
	
	var kept []WindowRecordV2
	removedCount := 0
	
	for _, record := range m.history.Windows {
		if record.EndTime > minRetentionTime {
			kept = append(kept, record)
		} else {
			removedCount++
			util.LogDebug(fmt.Sprintf("Removing old window: %s-%s (source: %s)",
				internal.FormatUnixToString(record.StartTime),
				internal.FormatUnixToString(record.EndTime),
				record.Source))
		}
	}
	
	m.history.Windows = kept
	
	if removedCount > 0 {
		util.LogInfo(fmt.Sprintf("Cleaned %d old windows, %d remaining", removedCount, len(kept)))
	}
	
	return removedCount
}

// ConvertFromLegacy converts legacy WindowRecord to WindowRecordV2
func ConvertFromLegacy(legacy WindowRecord) WindowRecordV2 {
	v2 := WindowRecordV2{
		SessionID:      legacy.SessionID,
		Source:         legacy.Source,
		StartTime:      legacy.StartTime,
		EndTime:        legacy.EndTime,
		CreatedAt:      legacy.CreatedAt,
		IsLimitReached: legacy.IsLimitReached,
		LimitMessage:   legacy.LimitMessage,
		FirstEntryTime: legacy.FirstEntryTime,
		IsAccountLevel: legacy.IsAccountLevel,
		StartTimeStr:   legacy.StartTimeStr,
		EndTimeStr:     legacy.EndTimeStr,
		CreatedAtStr:   legacy.CreatedAtStr,
	}
	return v2
}

// ConvertToLegacy converts WindowRecordV2 to legacy WindowRecord
func ConvertToLegacy(v2 WindowRecordV2) WindowRecord {
	legacy := WindowRecord{
		SessionID:      v2.SessionID,
		Source:         v2.Source,
		StartTime:      v2.StartTime,
		EndTime:        v2.EndTime,
		CreatedAt:      v2.CreatedAt,
		IsLimitReached: v2.IsLimitReached,
		LimitMessage:   v2.LimitMessage,
		FirstEntryTime: v2.FirstEntryTime,
		IsAccountLevel: v2.IsAccountLevel,
		StartTimeStr:   v2.StartTimeStr,
		EndTimeStr:     v2.EndTimeStr,
		CreatedAtStr:   v2.CreatedAtStr,
	}
	return legacy
}
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// formatUnixToString converts Unix timestamp to human-readable string
func formatUnixToString(unixTime int64) string {
	if unixTime == 0 {
		return ""
	}
	return time.Unix(unixTime, 0).Format("2006-01-02 15:04:05")
}

// populateStringFields fills the string representations of timestamps
func (w *WindowRecord) populateStringFields() {
	w.StartTimeStr = formatUnixToString(w.StartTime)
	w.EndTimeStr = formatUnixToString(w.EndTime)
	w.CreatedAtStr = formatUnixToString(w.CreatedAt)
}

// WindowRecord represents a single session window in history
type WindowRecord struct {
	SessionID      string `json:"session_id"`
	Source         string `json:"source"` // "limit_message", "gap", "first_message", "rounded_hour"
	StartTime      int64  `json:"start_time"`
	EndTime        int64  `json:"end_time"`
	CreatedAt      int64  `json:"created_at"`
	IsLimitReached bool   `json:"is_limit_reached"` // true if from limit message
	LimitMessage   string `json:"limit_message,omitempty"` // Original limit message text (e.g., "Claude AI usage limit reached|1751997600")
	FirstEntryTime int64  `json:"first_entry_time,omitempty"` // Stable first message time for burn rate calculation

	StartTimeStr string `json:"start_time_str"`
	EndTimeStr   string `json:"end_time_str"`
	CreatedAtStr string `json:"created_at_str"`
}

// WindowHistory manages the history of session windows
type WindowHistory struct {
	Windows     []WindowRecord `json:"windows"`
	LastUpdated int64          `json:"last_updated"`
	mu          sync.RWMutex
}

// WindowHistoryManager handles persistence and validation of window history
type WindowHistoryManager struct {
	history     *WindowHistory
	historyPath string
	mu          sync.Mutex
}

// NewWindowHistoryManager creates a new window history manager
func NewWindowHistoryManager(cacheDir string) *WindowHistoryManager {
	// Use ~/.go-claude-monitor/history/ instead of cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to cache directory if home directory is not accessible
		return &WindowHistoryManager{
			historyPath: filepath.Join(cacheDir, "window_history.json"),
			history:     &WindowHistory{Windows: make([]WindowRecord, 0)},
		}
	}

	historyDir := filepath.Join(homeDir, ".go-claude-monitor", "history")
	return &WindowHistoryManager{
		historyPath: filepath.Join(historyDir, "window_history.json"),
		history:     &WindowHistory{Windows: make([]WindowRecord, 0)},
	}
}

// Load loads window history from disk
func (m *WindowHistoryManager) Load() error {
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

	var history WindowHistory
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

// Save saves window history to disk
func (m *WindowHistoryManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history.mu.Lock()
	m.history.LastUpdated = time.Now().Unix()
	data, err := json.MarshalIndent(m.history, "", "  ")
	m.history.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to marshal window history: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.historyPath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write to temp file first, then rename (atomic operation)
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
func (m *WindowHistoryManager) AddOrUpdateWindow(record WindowRecord) {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	// Check time validity based on window type
	currentTime := time.Now().Unix()
	
	if record.IsLimitReached {
		// Limit-reached windows must be historical (not in future)
		if record.EndTime > currentTime {
			util.LogWarn(fmt.Sprintf("Rejecting limit-reached window with future end time: %s",
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
			return
		}
		// Check if it's within retention period
		minAllowedTime := currentTime - constants.LimitWindowRetentionSeconds
		if record.EndTime < minAllowedTime {
			util.LogWarn(fmt.Sprintf("Rejecting old limit-reached window: %s (older than %d day)",
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05"), constants.LimitWindowRetentionDays))
			return
		}
	} else {
		// For normal windows, restrict to session duration in the future
		maxAllowedEnd := currentTime + constants.MaxFutureWindowSeconds
		if record.EndTime > maxAllowedEnd {
			util.LogWarn(fmt.Sprintf("Rejecting window record with end time too far in future: %s (max allowed: %s)",
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05"),
				time.Unix(maxAllowedEnd, 0).Format("2006-01-02 15:04:05")))
			return
		}
	}

	record.CreatedAt = currentTime
	// Populate string fields
	record.populateStringFields()

	// Check if window already exists
	for i, existing := range m.history.Windows {
		if existing.SessionID == record.SessionID {
			// Update existing record, but preserve limit reached status and message
			if existing.IsLimitReached && !record.IsLimitReached {
				record.IsLimitReached = true
				record.Source = existing.Source
				record.LimitMessage = existing.LimitMessage
			}
			m.history.Windows[i] = record
			util.LogDebug(fmt.Sprintf("Updated window record: %s (%s)", record.SessionID, record.Source))
			return
		}
	}

	// Add new record
	m.history.Windows = append(m.history.Windows, record)
	util.LogDebug(fmt.Sprintf("Added new window record: %s (%s)", record.SessionID, record.Source))

	// Sort by start time
	sort.Slice(m.history.Windows, func(i, j int) bool {
		return m.history.Windows[i].StartTime < m.history.Windows[j].StartTime
	})
}

// ValidateNewWindow checks if a proposed window is valid based on history
func (m *WindowHistoryManager) ValidateNewWindow(proposedStart, proposedEnd int64) (validStart, validEnd int64, isValid bool) {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	validStart = proposedStart
	validEnd = proposedEnd

	// Check against all existing windows
	for _, record := range m.history.Windows {
		// Rule 1: New window cannot contain a limit-reached end time
		if record.IsLimitReached && proposedStart < record.EndTime && proposedEnd > record.EndTime {
			util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid confirmed end time %s",
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
			// Adjust window to start after confirmed end time
			validStart = record.EndTime
			validEnd = validStart + constants.SessionDurationSeconds
		}

		// Rule 2: Windows should not overlap
		if validStart < record.EndTime && validEnd > record.StartTime {
			// There's an overlap
			if validStart < record.EndTime {
				util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid overlap with window %s",
					record.SessionID))
				validStart = record.EndTime
				validEnd = validStart + constants.SessionDurationSeconds
			}
		}
	}

	// Validate the window is still session duration
	if validEnd-validStart != constants.SessionDurationSeconds {
		validEnd = validStart + constants.SessionDurationSeconds
	}

	// Check if adjusted window is too far in the future
	currentTime := time.Now().Unix()
	maxAllowedEnd := currentTime + constants.MaxFutureWindowSeconds
	
	if validEnd > maxAllowedEnd {
		// Window adjustment would create a future window beyond reasonable range
		util.LogWarn(fmt.Sprintf("Window validation: Rejected adjustment that would create future window (end: %s, max allowed: %s)",
			time.Unix(validEnd, 0).Format("2006-01-02 15:04:05"),
			time.Unix(maxAllowedEnd, 0).Format("2006-01-02 15:04:05")))
		// Return original proposed window instead
		return proposedStart, proposedEnd, true
	}

	isValid = validStart >= proposedStart // Window was adjusted if start time changed
	return validStart, validEnd, isValid
}

// GetLimitReachedWindows returns all windows where limit was reached (from limit messages)
func (m *WindowHistoryManager) GetLimitReachedWindows() []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	var limitReached []WindowRecord
	for _, record := range m.history.Windows {
		if record.IsLimitReached {
			limitReached = append(limitReached, record)
		}
	}
	return limitReached
}

// GetRecentWindows returns windows from the last N hours
func (m *WindowHistoryManager) GetRecentWindows(hours int) []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	var recent []WindowRecord

	for _, record := range m.history.Windows {
		if record.EndTime > cutoff {
			recent = append(recent, record)
		}
	}
	return recent
}

// CleanupOldWindows removes windows older than the specified days
// Special handling: windows with limit reached (IsLimitReached) are retained for at least LimitWindowRetentionDays
func (m *WindowHistoryManager) CleanupOldWindows(days int) int {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	limitWindowCutoff := time.Now().AddDate(0, 0, -constants.LimitWindowRetentionDays).Unix() // Retention period for limit-reached windows
	originalCount := len(m.history.Windows)

	// Keep windows based on retention rules:
	// 1. Regular windows: keep if newer than cutoff
	// 2. Limit-reached windows: keep if newer than retention period
	var kept []WindowRecord
	for _, record := range m.history.Windows {
		shouldKeep := false
		
		if record.IsLimitReached {
			// Limit-reached windows are kept for retention period
			shouldKeep = record.CreatedAt > limitWindowCutoff
		} else {
			// Regular windows follow the standard retention period
			shouldKeep = record.CreatedAt > cutoff
		}
		
		if shouldKeep {
			kept = append(kept, record)
		}
	}

	m.history.Windows = kept
	removed := originalCount - len(kept)

	if removed > 0 {
		util.LogInfo(fmt.Sprintf("Cleaned up %d old window records (kept %d, including %d limit-reached windows)", 
			removed, len(kept), m.countLimitReachedWindows(kept)))
	}

	return removed
}

// countLimitReachedWindows counts the number of limit-reached windows in a slice
func (m *WindowHistoryManager) countLimitReachedWindows(windows []WindowRecord) int {
	count := 0
	for _, record := range windows {
		if record.IsLimitReached {
			count++
		}
	}
	return count
}

// FindOverlappingWindow finds any window that overlaps with the given time range
func (m *WindowHistoryManager) FindOverlappingWindow(start, end int64) *WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	for _, record := range m.history.Windows {
		if start < record.EndTime && end > record.StartTime {
			return &record
		}
	}
	return nil
}

// UpdateFromLimitMessage updates window history based on a limit message
func (m *WindowHistoryManager) UpdateFromLimitMessage(resetTime int64, messageTime int64, limitMessage string) {
	// Calculate window boundaries from reset time
	windowEnd := resetTime
	windowStart := windowEnd - constants.SessionDurationSeconds

	record := WindowRecord{
		StartTime:      windowStart,
		EndTime:        windowEnd,
		Source:         "limit_message",
		IsLimitReached: true,
		LimitMessage:   limitMessage,
		SessionID:      fmt.Sprintf("%d", windowStart),
		CreatedAt:      time.Now().Unix(),
	}

	// Populate string fields
	record.populateStringFields()

	m.AddOrUpdateWindow(record)

	util.LogInfo(fmt.Sprintf("Updated window history from limit message: %s to %s (message: %s)",
		time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
		time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05"),
		limitMessage))
}

// GetWindowForTime finds the window that contains the given timestamp
func (m *WindowHistoryManager) GetWindowForTime(timestamp int64) *WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	for _, record := range m.history.Windows {
		if timestamp >= record.StartTime && timestamp < record.EndTime {
			return &record
		}
	}
	return nil
}

// LoadHistoricalLimitWindows loads limit windows from historical logs (past LimitWindowRetentionDays)
// This ensures that even if the window history is reset, we can reconstruct
// the most accurate windows from limit messages
func (m *WindowHistoryManager) LoadHistoricalLimitWindows(logs []model.ConversationLog) int {
	// Create a limit parser to find limit messages
	parser := NewLimitParser()
	limits := parser.ParseLogs(logs)
	
	util.LogInfo(fmt.Sprintf("LoadHistoricalLimitWindows: Found %d limit messages in historical logs", len(limits)))
	
	// Track how many new windows were added
	addedCount := 0
	currentTime := time.Now().Unix()
	minAllowedTime := currentTime - constants.HistoricalScanSeconds // Historical scan period
	
	for _, limit := range limits {
		if limit.ResetTime == nil {
			continue
		}
		
		// Skip if too old (older than 1 day)
		if *limit.ResetTime < minAllowedTime {
			continue
		}
		
		// Calculate window boundaries from reset time
		windowEnd := *limit.ResetTime
		windowStart := windowEnd - constants.SessionDurationSeconds
		
		// Check if this window already exists
		existing := false
		for _, record := range m.history.Windows {
			if record.StartTime == windowStart && record.EndTime == windowEnd && record.IsLimitReached {
				existing = true
				break
			}
		}
		
		if !existing {
			record := WindowRecord{
				StartTime:      windowStart,
				EndTime:        windowEnd,
				Source:         "limit_message",
				IsLimitReached: true,
				LimitMessage:   limit.Content,
				SessionID:      fmt.Sprintf("%d", windowStart),
				CreatedAt:      limit.Timestamp, // Use the limit message timestamp
			}
			
			// Populate string fields
			record.populateStringFields()
			
			m.AddOrUpdateWindow(record)
			addedCount++
			
			util.LogInfo(fmt.Sprintf("Added historical limit window: %s to %s",
				time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
				time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05")))
		}
	}
	
	if addedCount > 0 {
		// Save the updated history
		if err := m.Save(); err != nil {
			util.LogWarn(fmt.Sprintf("Failed to save window history after loading historical limits: %v", err))
		}
	}
	
	return addedCount
}

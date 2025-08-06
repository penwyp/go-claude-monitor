package session

import (
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
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
	IsLimitReached bool   `json:"is_limit_reached"`           // true if from limit message
	LimitMessage   string `json:"limit_message,omitempty"`    // Original limit message text (e.g., "Claude AI usage limit reached|1751997600")
	FirstEntryTime int64  `json:"first_entry_time,omitempty"` // Stable first message time for burn rate calculation
	IsAccountLevel bool   `json:"is_account_level,omitempty"` // true if this is an account-level window (across all projects)

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
	data, err := sonic.MarshalIndent(m.history, "", "  ")
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
	
	// Add detailed logging of input
	util.LogDebug(fmt.Sprintf("ValidateNewWindow called - Proposed window: %s to %s",
		time.Unix(proposedStart, 0).Format("2006-01-02 15:04:05"),
		time.Unix(proposedEnd, 0).Format("2006-01-02 15:04:05")))

	// Get current time and reasonable time bounds
	currentTime := time.Now().Unix()
	minReasonableTime := currentTime - constants.LimitWindowRetentionSeconds
	maxReasonableTime := currentTime + constants.MaxFutureWindowSeconds
	
	util.LogDebug(fmt.Sprintf("Time bounds - Current: %s, Min: %s, Max: %s",
		time.Unix(currentTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(minReasonableTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(maxReasonableTime, 0).Format("2006-01-02 15:04:05")))

	// Check against all existing windows, prioritizing limit_message windows
	for _, record := range m.history.Windows {
		// Special handling for limit_message windows - they are authoritative
		if record.IsLimitReached && record.Source == "limit_message" {
			// Skip if the historical window is outside reasonable time bounds
			if record.EndTime < minReasonableTime {
				util.LogDebug(fmt.Sprintf("Skipping old limit_message window %s-%s (too old)",
					time.Unix(record.StartTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
				continue
			}
			
			// Check if windows are on the same day (considering timezone)
			proposedDate := time.Unix(proposedStart, 0).Format("2006-01-02")
			recordDate := time.Unix(record.StartTime, 0).Format("2006-01-02")
			
			if proposedDate != recordDate {
				util.LogDebug(fmt.Sprintf("Skipping limit_message window from different date - Proposed: %s, Record: %s",
					proposedDate, recordDate))
				continue
			}
			
			// Check for overlap on the same day
			if proposedStart < record.EndTime && proposedEnd > record.StartTime {
				// There's an overlap - adjust to start after the limit_message window
				util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid limit_message window %s-%s (same day overlap)",
					time.Unix(record.StartTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
				validStart = record.EndTime
				validEnd = validStart + constants.SessionDurationSeconds
			}
		}
	}

	// After adjusting for limit_message windows, check other windows
	for _, record := range m.history.Windows {
		// Skip limit_message windows as they were already handled
		if record.IsLimitReached && record.Source == "limit_message" {
			continue
		}
		
		// Skip if the historical window is outside reasonable time bounds
		if record.EndTime < minReasonableTime {
			util.LogDebug(fmt.Sprintf("Skipping old window %s (too old)",
				record.SessionID))
			continue
		}

		// Rule 2: Windows should not overlap with other windows
		if validStart < record.EndTime && validEnd > record.StartTime {
			// There's an overlap
			if validStart < record.EndTime {
				util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid overlap with window %s (%s-%s)",
					record.SessionID,
					time.Unix(record.StartTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
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
	// (reuse currentTime and maxReasonableTime from earlier)

	if validEnd > maxReasonableTime {
		// Window adjustment would create a future window beyond reasonable range
		util.LogWarn(fmt.Sprintf("Window validation: Rejected adjustment that would create future window (end: %s, max allowed: %s)",
			time.Unix(validEnd, 0).Format("2006-01-02 15:04:05"),
			time.Unix(maxReasonableTime, 0).Format("2006-01-02 15:04:05")))
		// Return original proposed window instead
		return proposedStart, proposedEnd, true
	}

	isValid = validStart == proposedStart // Window was NOT adjusted if start time is the same
	
	// Log final result
	if !isValid {
		util.LogDebug(fmt.Sprintf("Window validation result: Window was adjusted from %s-%s to %s-%s",
			time.Unix(proposedStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(proposedEnd, 0).Format("2006-01-02 15:04:05"),
			time.Unix(validStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(validEnd, 0).Format("2006-01-02 15:04:05")))
	} else {
		util.LogDebug(fmt.Sprintf("Window validation result: Window was NOT adjusted, using original %s-%s",
			time.Unix(validStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(validEnd, 0).Format("2006-01-02 15:04:05")))
	}
	
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

// GetAccountLevelWindows returns all account-level windows (across all projects)
func (m *WindowHistoryManager) GetAccountLevelWindows() []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	var accountLevel []WindowRecord
	for _, record := range m.history.Windows {
		if record.IsAccountLevel {
			accountLevel = append(accountLevel, record)
		}
	}
	return accountLevel
}

// GetRecentWindows returns all windows from the specified duration
func (m *WindowHistoryManager) GetRecentWindows(duration time.Duration) []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	cutoff := time.Now().Unix() - int64(duration.Seconds())
	var recent []WindowRecord
	
	for _, record := range m.history.Windows {
		if record.EndTime > cutoff {
			recent = append(recent, record)
		}
	}
	
	return recent
}

// GetRecentAccountWindows returns account-level windows from the specified duration
func (m *WindowHistoryManager) GetRecentAccountWindows(duration time.Duration) []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	cutoff := time.Now().Unix() - int64(duration.Seconds())
	var recent []WindowRecord
	
	for _, record := range m.history.Windows {
		if record.IsAccountLevel && record.EndTime > cutoff {
			recent = append(recent, record)
		}
	}
	
	return recent
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
		IsAccountLevel: true, // Limit messages apply to the entire account
		LimitMessage:   limitMessage,
		SessionID:      fmt.Sprintf("%d", windowStart),
		CreatedAt:      time.Now().Unix(),
	}

	// Populate string fields
	record.populateStringFields()

	m.AddOrUpdateWindow(record)

	util.LogInfo(fmt.Sprintf("Updated window history from limit message: %s to %s (account-level, message: %s)",
		time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
		time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05"),
		limitMessage))
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
	
	util.LogDebug(fmt.Sprintf("LoadHistoricalLimitWindows: Scanning period from %s to %s (%d days)",
		time.Unix(minAllowedTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(currentTime, 0).Format("2006-01-02 15:04:05"),
		constants.HistoricalScanDays))

	for _, limit := range limits {
		if limit.ResetTime == nil {
			util.LogDebug(fmt.Sprintf("Skipping limit without reset time: %s", limit.Content))
			continue
		}

		// Skip if too old (older than retention period)
		if *limit.ResetTime < minAllowedTime {
			util.LogDebug(fmt.Sprintf("Skipping old limit message - Reset: %s, Content: %s (older than %d days)",
				time.Unix(*limit.ResetTime, 0).Format("2006-01-02 15:04:05"),
				limit.Content,
				constants.HistoricalScanDays))
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
				IsAccountLevel: true, // Historical limit messages are account-level
				LimitMessage:   limit.Content,
				SessionID:      fmt.Sprintf("%d", windowStart),
				CreatedAt:      limit.Timestamp, // Use the limit message timestamp
			}

			// Populate string fields
			record.populateStringFields()

			m.AddOrUpdateWindow(record)
			addedCount++

			util.LogInfo(fmt.Sprintf("Added historical limit window: %s to %s (account-level, from message: %s)",
				time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
				time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05"),
				limit.Content))
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

// MergeAccountWindows merges overlapping or adjacent windows that belong to the same account
// This is useful when multiple projects share the same 5-hour limit window
func (m *WindowHistoryManager) MergeAccountWindows() {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	if len(m.history.Windows) < 2 {
		return
	}

	// Sort windows by start time
	sort.Slice(m.history.Windows, func(i, j int) bool {
		return m.history.Windows[i].StartTime < m.history.Windows[j].StartTime
	})

	// Merge overlapping or adjacent account-level windows
	var merged []WindowRecord
	current := m.history.Windows[0]

	for i := 1; i < len(m.history.Windows); i++ {
		next := m.history.Windows[i]

		// Check if both windows are account-level and can be merged
		if current.IsAccountLevel && next.IsAccountLevel &&
			current.EndTime >= next.StartTime { // Overlapping or adjacent
			
			// Merge windows
			if next.EndTime > current.EndTime {
				current.EndTime = next.EndTime
			}
			
			// Preserve limit reached status
			if next.IsLimitReached {
				current.IsLimitReached = true
				if next.LimitMessage != "" {
					current.LimitMessage = next.LimitMessage
				}
			}
			
			// Update source to indicate merged window
			if current.Source != next.Source {
				current.Source = "merged"
			}
			
			util.LogDebug(fmt.Sprintf("Merged account windows: %s-%s with %s-%s",
				time.Unix(current.StartTime, 0).Format("15:04"),
				time.Unix(current.EndTime, 0).Format("15:04"),
				time.Unix(next.StartTime, 0).Format("15:04"),
				time.Unix(next.EndTime, 0).Format("15:04")))
		} else {
			// Can't merge, save current and move to next
			current.populateStringFields()
			merged = append(merged, current)
			current = next
		}
	}

	// Don't forget the last window
	current.populateStringFields()
	merged = append(merged, current)

	m.history.Windows = merged
	util.LogInfo(fmt.Sprintf("Window merge complete: %d windows after merge", len(merged)))
}

// CleanOldWindows removes windows older than the retention period
func (m *WindowHistoryManager) CleanOldWindows() int {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	currentTime := time.Now().Unix()
	minRetentionTime := currentTime - constants.LimitWindowRetentionSeconds
	
	var kept []WindowRecord
	removedCount := 0

	for _, record := range m.history.Windows {
		// Keep recent windows and all limit-reached windows within retention
		if record.EndTime > minRetentionTime {
			kept = append(kept, record)
		} else {
			removedCount++
			util.LogDebug(fmt.Sprintf("Removing old window: %s-%s (source: %s)",
				time.Unix(record.StartTime, 0).Format("2006-01-02 15:04"),
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04"),
				record.Source))
		}
	}

	m.history.Windows = kept
	
	if removedCount > 0 {
		util.LogInfo(fmt.Sprintf("Cleaned %d old windows, %d windows remaining", removedCount, len(kept)))
	}

	return removedCount
}

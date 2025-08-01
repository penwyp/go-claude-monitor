package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

// WindowRecord represents a single session window in history
type WindowRecord struct {
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	Source      string `json:"source"`       // "limit_message", "gap", "first_message", "rounded_hour"
	IsConfirmed bool   `json:"is_confirmed"` // true if from limit message
	SessionID   string `json:"session_id"`
	CreatedAt   int64  `json:"created_at"`
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

	record.CreatedAt = time.Now().Unix()

	// Check if window already exists
	for i, existing := range m.history.Windows {
		if existing.SessionID == record.SessionID {
			// Update existing record, but preserve confirmed status
			if existing.IsConfirmed && !record.IsConfirmed {
				record.IsConfirmed = true
				record.Source = existing.Source
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
		// Rule 1: New window cannot contain a confirmed end time
		if record.IsConfirmed && proposedStart < record.EndTime && proposedEnd > record.EndTime {
			util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid confirmed end time %s",
				time.Unix(record.EndTime, 0).Format("2006-01-02 15:04:05")))
			// Adjust window to start after confirmed end time
			validStart = record.EndTime
			validEnd = validStart + 5*3600
		}

		// Rule 2: Windows should not overlap
		if validStart < record.EndTime && validEnd > record.StartTime {
			// There's an overlap
			if validStart < record.EndTime {
				util.LogDebug(fmt.Sprintf("Window validation: Adjusting to avoid overlap with window %s",
					record.SessionID))
				validStart = record.EndTime
				validEnd = validStart + 5*3600
			}
		}
	}

	// Validate the window is still 5 hours
	if validEnd-validStart != 5*3600 {
		validEnd = validStart + 5*3600
	}

	isValid = validStart >= proposedStart // Window was adjusted if start time changed
	return validStart, validEnd, isValid
}

// GetConfirmedWindows returns all confirmed windows (from limit messages)
func (m *WindowHistoryManager) GetConfirmedWindows() []WindowRecord {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	var confirmed []WindowRecord
	for _, record := range m.history.Windows {
		if record.IsConfirmed {
			confirmed = append(confirmed, record)
		}
	}
	return confirmed
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
func (m *WindowHistoryManager) CleanupOldWindows(days int) int {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	originalCount := len(m.history.Windows)

	// Keep windows that are newer than cutoff or are confirmed
	var kept []WindowRecord
	for _, record := range m.history.Windows {
		if record.CreatedAt > cutoff || record.IsConfirmed {
			kept = append(kept, record)
		}
	}

	m.history.Windows = kept
	removed := originalCount - len(kept)

	if removed > 0 {
		util.LogInfo(fmt.Sprintf("Cleaned up %d old window records (kept %d)", removed, len(kept)))
	}

	return removed
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
func (m *WindowHistoryManager) UpdateFromLimitMessage(resetTime int64, messageTime int64) {
	// Calculate window boundaries from reset time
	windowEnd := resetTime
	windowStart := windowEnd - 5*3600

	record := WindowRecord{
		StartTime:   windowStart,
		EndTime:     windowEnd,
		Source:      "limit_message",
		IsConfirmed: true,
		SessionID:   fmt.Sprintf("%d", windowStart),
		CreatedAt:   time.Now().Unix(),
	}

	m.AddOrUpdateWindow(record)

	util.LogInfo(fmt.Sprintf("Updated window history from limit message: %s to %s",
		time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
		time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05")))
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
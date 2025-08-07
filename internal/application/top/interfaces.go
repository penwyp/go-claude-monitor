package top

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/presentation/interaction"
)

// SessionProvider handles session detection and calculation
type SessionProvider interface {
	// DetectSessionsWithLimits detects sessions from timeline data
	DetectSessionsWithLimits(input session.SessionDetectionInput) []*session.Session
	// Calculate calculates metrics for a session
	Calculate(session *session.Session)
}

// DataSource manages data loading and caching
type DataSource interface {
	// LoadFiles loads and processes the specified files
	LoadFiles(files []string) error
	// ScanRecentFiles scans for recent files based on configuration
	ScanRecentFiles() ([]string, error)
	// GetGlobalTimeline returns the global timeline of all logs
	GetGlobalTimeline(secondsBack int64) []timeline.TimestampedLog
	// GetCachedWindowInfo returns cached window detection information
	GetCachedWindowInfo() map[string]*session.WindowDetectionInfo
	// UpdateWindowInfo updates window detection information for a session
	UpdateWindowInfo(sessionID string, info *session.WindowDetectionInfo)
	// IdentifyChangedFiles returns files that have changed since last load
	IdentifyChangedFiles(files []string) []string
}

// DisplayController handles terminal display operations
type DisplayController interface {
	// EnterAlternateScreen switches to alternate terminal screen
	EnterAlternateScreen()
	// ExitAlternateScreen returns to normal terminal screen
	ExitAlternateScreen()
	// ClearScreen clears the terminal screen
	ClearScreen()
	// RenderWithState renders sessions with the given interaction state
	RenderWithState(sessions []*session.Session, state model.InteractionState)
}

// RefreshStrategy manages data refresh operations
type RefreshStrategy interface {
	// RefreshData performs a full or incremental data refresh
	RefreshData() ([]*session.Session, error)
	// IncrementalDetect performs incremental session detection for changed files
	IncrementalDetect(changedFiles []string) ([]*session.Session, error)
	// FullDetect performs full session detection
	FullDetect() ([]*session.Session, error)
}

// StateStore manages application state
type StateStore interface {
	// SetSessions updates active sessions (thread-safe)
	SetSessions(sessions []*session.Session)
	// GetLoadingState returns current loading state and message
	GetLoadingState() (bool, string)
	// SetLoadingState updates loading state and message
	SetLoadingState(isLoading bool, message string)
	// GetInteractionState returns current interaction state
	GetInteractionState() model.InteractionState
}

// InputHandler processes keyboard and other input events
type InputHandler interface {
	// Events returns a channel of keyboard events
	Events() <-chan interaction.KeyEvent
	// Close cleans up input handler resources
	Close() error
}

// FileMonitor watches for file changes
type FileMonitor interface {
	// Events returns a channel of file change events
	Events() <-chan model.FileEvent
	// Close stops monitoring and cleans up resources
	Close() error
}

// SessionSortStrategy handles session sorting
type SessionSortStrategy interface {
	// Sort sorts sessions in place according to current settings
	Sort(sessions []*session.Session)
}
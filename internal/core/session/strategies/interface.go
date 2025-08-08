package strategies

import (
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
)

// DetectionInput contains all the input data needed for window detection
type DetectionInput struct {
	// Global timeline of all logs across projects
	GlobalTimeline []timeline.TimestampedLog
	
	// Raw logs for limit detection
	RawLogs []model.ConversationLog
	
	// Window history manager for historical data
	WindowHistory WindowHistoryAccess
	
	// Session duration (typically 5 hours)
	SessionDuration time.Duration
	
	// Current time for validation
	CurrentTime int64
}

// WindowCandidate represents a potential session window detected by a strategy
type WindowCandidate struct {
	StartTime int64  // Unix timestamp
	EndTime   int64  // Unix timestamp
	Source    string // Strategy name that detected this window
	Priority  int    // Priority of this window (higher is better)
	IsLimit   bool   // Whether this window is from a rate limit message
	
	// Optional metadata
	ProjectName  string            // Project this window belongs to (if specific)
	LimitMessage string            // Original limit message (if applicable)
	Metadata     map[string]string // Additional strategy-specific metadata
}

// WindowDetectionStrategy defines the interface for different window detection strategies
type WindowDetectionStrategy interface {
	// Name returns the unique name of this strategy
	Name() string
	
	// Priority returns the priority of windows detected by this strategy (higher is better)
	Priority() int
	
	// Detect analyzes the input and returns potential session windows
	Detect(input DetectionInput) []WindowCandidate
	
	// Description returns a human-readable description of what this strategy does
	Description() string
}

// WindowHistoryAccess provides read-only access to window history
type WindowHistoryAccess interface {
	// GetAccountLevelWindows returns all account-level windows
	GetAccountLevelWindows() []HistoricalWindow
	
	// GetRecentWindows returns windows from the specified duration
	GetRecentWindows(duration time.Duration) []HistoricalWindow
	
	// GetLimitReachedWindows returns all windows where limit was reached
	GetLimitReachedWindows() []HistoricalWindow
}

// HistoricalWindow represents a window from history
type HistoricalWindow struct {
	SessionID      string
	Source         string
	StartTime      int64
	EndTime        int64
	IsLimitReached bool
	IsAccountLevel bool
	LimitMessage   string
}

// BaseStrategy provides common functionality for all strategies
type BaseStrategy struct {
	name        string
	priority    int
	description string
}

// NewBaseStrategy creates a new base strategy
func NewBaseStrategy(name string, priority int, description string) BaseStrategy {
	return BaseStrategy{
		name:        name,
		priority:    priority,
		description: description,
	}
}

// Name returns the strategy name
func (s BaseStrategy) Name() string {
	return s.name
}

// Priority returns the strategy priority
func (s BaseStrategy) Priority() int {
	return s.priority
}

// Description returns the strategy description
func (s BaseStrategy) Description() string {
	return s.description
}
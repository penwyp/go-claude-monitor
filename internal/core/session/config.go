package session

// SessionConfig provides configuration for session detection behavior
type SessionConfig struct {
	// DataRetentionHours controls how much historical data to retain (in hours)
	// 0 means retain all data
	DataRetentionHours int

	// EnableActiveDetection controls whether to detect and create active sessions
	EnableActiveDetection bool

	// CacheStrategy controls caching behavior
	// "aggressive" - cache everything, unified data sources
	// "conservative" - cache selectively, prioritize memory
	CacheStrategy string

	// TimelineMode controls timeline construction
	// "full" - load all data without time restrictions
	// "recent" - load only recent data (legacy behavior)
	// "optimized" - balance between full and recent
	TimelineMode string

	// EnableIncrementalDetection controls whether to use incremental session detection
	// for changed files rather than full redetection
	EnableIncrementalDetection bool

	// WindowHistoryRetentionDays controls how long to keep window history
	WindowHistoryRetentionDays int
}

// DefaultSessionConfig returns the default configuration
var DefaultSessionConfig = SessionConfig{
	DataRetentionHours:         0,     // Retain all data
	EnableActiveDetection:      true,  // Enable active session detection
	CacheStrategy:              "aggressive", // Use unified data sources
	TimelineMode:               "full", // Load all data
	EnableIncrementalDetection: true,  // Use incremental detection
	WindowHistoryRetentionDays: 7,     // Keep 7 days of window history
}

// GetSessionConfig returns configuration with environment variable overrides
func GetSessionConfig() SessionConfig {
	config := DefaultSessionConfig
	
	// Future: Add environment variable overrides here
	// e.g., if os.Getenv("CLAUDE_MONITOR_TIMELINE_MODE") != "" { ... }
	
	return config
}
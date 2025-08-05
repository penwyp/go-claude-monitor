package session

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"time"
)

// ProjectStats holds statistics for a single project within a session
type ProjectStats struct {
	ProjectName       string
	TotalTokens       int
	TotalCost         float64
	MessageCount      int
	SentMessageCount  int
	ModelDistribution map[string]*model.ModelStats
	PerModelStats     map[string]map[string]interface{}
	HourlyMetrics     []*model.HourlyMetric
	FirstEntryTime    int64 // First message time for this project in the session
	LastEntryTime     int64 // Last message time for this project in the session
}

// Session represents an active Claude usage session (account-level)
type Session struct {
	ID               string
	StartTime        int64  // Unix timestamp
	StartHour        int64  // Unix timestamp rounded down to hour
	EndTime          int64  // Unix timestamp
	ActualEndTime    *int64 // Unix timestamp of last entry
	IsActive         bool
	IsGap            bool // Indicates if this is a gap session
	ProjectName      string // Deprecated: kept for UI compatibility, will contain primary project or "Multiple"
	SentMessageCount int // Number of messages sent in this session

	// Multi-project support
	Projects         map[string]*ProjectStats // Key: project name

	// Sliding window support
	WindowStartTime  *int64 // Actual window start time (not rounded to hour)
	IsWindowDetected bool   // Whether window timing was explicitly detected
	WindowSource     string // Source of window detection: "limit_message", "gap", "first_message", "rounded_hour"
	FirstEntryTime   int64  // Timestamp of the first message in this session

	// Statistics (account-level totals)
	TotalTokens       int
	TotalCost         float64
	MessageCount      int
	ModelDistribution map[string]*model.ModelStats
	PerModelStats     map[string]map[string]interface{} // Detailed per-model statistics
	HourlyMetrics     []*model.HourlyMetric

	// Real-time metrics
	TimeRemaining    time.Duration
	TokensPerMinute  float64
	CostPerHour      float64
	CostPerMinute    float64
	BurnRate         float64
	ProjectedTokens  int
	ProjectedCost    float64
	ResetTime        int64 // Unix timestamp
	PredictedEndTime int64 // Unix timestamp

	// Additional fields from Python
	LimitMessages    []map[string]interface{} // Limit messages detected in this session
	ProjectionData   map[string]interface{}   // Projection data
	BurnRateSnapshot *model.BurnRate          // Snapshot of burn rate
}

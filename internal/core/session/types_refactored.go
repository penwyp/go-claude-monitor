package session

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"time"
)

// WindowInfo contains information about the session window detection
type WindowInfo struct {
	WindowStartTime  *int64 // Actual window start time (not rounded to hour)
	IsWindowDetected bool   // Whether window timing was explicitly detected
	WindowSource     string // Source of window detection: "limit_message", "gap", "first_message", "rounded_hour"
	WindowPriority   int    // Priority of the window source
}

// MetricsInfo contains real-time metrics and calculations
type MetricsInfo struct {
	TimeRemaining    time.Duration            // Time remaining in the session
	TokensPerMinute  float64                  // Token consumption rate
	CostPerHour      float64                  // Cost rate per hour
	CostPerMinute    float64                  // Cost rate per minute
	BurnRate         float64                  // Overall burn rate
	BurnRateSnapshot *model.BurnRate          // Snapshot of burn rate at a point in time
}

// ProjectionInfo contains future projections based on current rates
type ProjectionInfo struct {
	ProjectedTokens  int                    // Projected total tokens at end of session
	ProjectedCost    float64                // Projected total cost at end of session
	PredictedEndTime int64                  // Unix timestamp of predicted end
	ProjectionData   map[string]interface{} // Additional projection data
}

// SessionStatistics contains aggregated statistics for the session
type SessionStatistics struct {
	TotalTokens       int                               // Total tokens consumed
	TotalCost         float64                           // Total cost incurred
	MessageCount      int                               // Total number of messages
	SentMessageCount  int                               // Number of messages sent
	ModelDistribution map[string]*model.ModelStats     // Distribution by model
	PerModelStats     map[string]map[string]interface{} // Detailed per-model statistics
	HourlyMetrics     []*model.HourlyMetric             // Hourly breakdown
	LimitMessages     []map[string]interface{}          // Limit messages detected
}

// SessionTiming contains all timing-related information
type SessionTiming struct {
	StartTime      int64  // Unix timestamp of session start
	StartHour      int64  // Unix timestamp rounded down to hour
	EndTime        int64  // Unix timestamp of session end
	ActualEndTime  *int64 // Unix timestamp of last entry
	FirstEntryTime int64  // Timestamp of the first message
	ResetTime      int64  // Unix timestamp of rate limit reset
}

// SessionV2 represents an improved structure for Claude usage session
// This is the refactored version with better organization
type SessionV2 struct {
	// Core identification
	ID          string // Session identifier
	IsActive    bool   // Whether session is currently active
	IsGap       bool   // Whether this is a gap session
	ProjectName string // Primary project or "Multiple" for multi-project

	// Timing information
	Timing SessionTiming

	// Window detection information
	Window WindowInfo

	// Multi-project support
	Projects map[string]*ProjectStats // Key: project name

	// Session statistics
	Statistics SessionStatistics

	// Real-time metrics
	Metrics MetricsInfo

	// Future projections
	Projection ProjectionInfo

	// Legacy compatibility fields (deprecated)
	ProjectTokens int            // Deprecated: use Statistics.TotalTokens
	ProjectCost   float64        // Deprecated: use Statistics.TotalCost
	ModelsUsed    map[string]int // Deprecated: use Statistics.ModelDistribution
	EntriesCount  int            // Deprecated: use Statistics.MessageCount
}

// Helper methods to maintain backward compatibility

// ToLegacySession converts the refactored SessionV2 back to the original Session struct
func (s *SessionV2) ToLegacySession() *Session {
	return &Session{
		ID:               s.ID,
		StartTime:        s.Timing.StartTime,
		StartHour:        s.Timing.StartHour,
		EndTime:          s.Timing.EndTime,
		ActualEndTime:    s.Timing.ActualEndTime,
		IsActive:         s.IsActive,
		IsGap:            s.IsGap,
		ProjectName:      s.ProjectName,
		SentMessageCount: s.Statistics.SentMessageCount,
		Projects:         s.Projects,
		WindowStartTime:  s.Window.WindowStartTime,
		IsWindowDetected: s.Window.IsWindowDetected,
		WindowSource:     s.Window.WindowSource,
		WindowPriority:   s.Window.WindowPriority,
		FirstEntryTime:   s.Timing.FirstEntryTime,
		TotalTokens:      s.Statistics.TotalTokens,
		TotalCost:        s.Statistics.TotalCost,
		MessageCount:     s.Statistics.MessageCount,
		ModelDistribution: s.Statistics.ModelDistribution,
		PerModelStats:    s.Statistics.PerModelStats,
		HourlyMetrics:    s.Statistics.HourlyMetrics,
		TimeRemaining:    s.Metrics.TimeRemaining,
		TokensPerMinute:  s.Metrics.TokensPerMinute,
		CostPerHour:      s.Metrics.CostPerHour,
		CostPerMinute:    s.Metrics.CostPerMinute,
		BurnRate:         s.Metrics.BurnRate,
		ProjectedTokens:  s.Projection.ProjectedTokens,
		ProjectedCost:    s.Projection.ProjectedCost,
		ResetTime:        s.Timing.ResetTime,
		PredictedEndTime: s.Projection.PredictedEndTime,
		LimitMessages:    s.Statistics.LimitMessages,
		ProjectionData:   s.Projection.ProjectionData,
		BurnRateSnapshot: s.Metrics.BurnRateSnapshot,
		ProjectTokens:    s.ProjectTokens,
		ProjectCost:      s.ProjectCost,
		ModelsUsed:       s.ModelsUsed,
		EntriesCount:     s.EntriesCount,
	}
}

// FromLegacySession creates a SessionV2 from the original Session struct
func FromLegacySession(s *Session) *SessionV2 {
	return &SessionV2{
		ID:          s.ID,
		IsActive:    s.IsActive,
		IsGap:       s.IsGap,
		ProjectName: s.ProjectName,
		Timing: SessionTiming{
			StartTime:      s.StartTime,
			StartHour:      s.StartHour,
			EndTime:        s.EndTime,
			ActualEndTime:  s.ActualEndTime,
			FirstEntryTime: s.FirstEntryTime,
			ResetTime:      s.ResetTime,
		},
		Window: WindowInfo{
			WindowStartTime:  s.WindowStartTime,
			IsWindowDetected: s.IsWindowDetected,
			WindowSource:     s.WindowSource,
			WindowPriority:   s.WindowPriority,
		},
		Projects: s.Projects,
		Statistics: SessionStatistics{
			TotalTokens:       s.TotalTokens,
			TotalCost:         s.TotalCost,
			MessageCount:      s.MessageCount,
			SentMessageCount:  s.SentMessageCount,
			ModelDistribution: s.ModelDistribution,
			PerModelStats:     s.PerModelStats,
			HourlyMetrics:     s.HourlyMetrics,
			LimitMessages:     s.LimitMessages,
		},
		Metrics: MetricsInfo{
			TimeRemaining:    s.TimeRemaining,
			TokensPerMinute:  s.TokensPerMinute,
			CostPerHour:      s.CostPerHour,
			CostPerMinute:    s.CostPerMinute,
			BurnRate:         s.BurnRate,
			BurnRateSnapshot: s.BurnRateSnapshot,
		},
		Projection: ProjectionInfo{
			ProjectedTokens:  s.ProjectedTokens,
			ProjectedCost:    s.ProjectedCost,
			PredictedEndTime: s.PredictedEndTime,
			ProjectionData:   s.ProjectionData,
		},
		ProjectTokens: s.ProjectTokens,
		ProjectCost:   s.ProjectCost,
		ModelsUsed:    s.ModelsUsed,
		EntriesCount:  s.EntriesCount,
	}
}
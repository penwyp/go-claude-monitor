package timeline

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

// TimestampedLog represents a log entry with its timestamp and project info
type TimestampedLog struct {
	Log         model.ConversationLog
	Timestamp   int64  // Unix timestamp for sorting
	ProjectName string // Project this log belongs to
}

// TimelineEntry represents a single point in the timeline
type TimelineEntry struct {
	Timestamp       int64
	ProjectName     string
	Type           string // "message", "limit", "hourly"
	Data           interface{}
	IsSupplementary bool   // Marks if this is supplementary data (e.g., aggregated when raw exists)
}
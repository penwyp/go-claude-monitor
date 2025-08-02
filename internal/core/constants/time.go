package constants

import "time"

const (
	// Session and window durations
	SessionDuration        = 5 * time.Hour
	SessionDurationSeconds = int64(5 * 3600)

	// Limit window retention
	LimitWindowRetentionDays    = 1
	LimitWindowRetentionSeconds = int64(LimitWindowRetentionDays * 24 * 3600)

	// Future window validation
	MaxFutureWindowHours   = 5
	MaxFutureWindowSeconds = int64(MaxFutureWindowHours * 3600)

	// Historical data scanning (same as limit retention)
	HistoricalScanDays    = LimitWindowRetentionDays
	HistoricalScanSeconds = LimitWindowRetentionSeconds
)
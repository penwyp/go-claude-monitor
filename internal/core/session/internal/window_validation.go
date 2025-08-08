package internal

import (
	"fmt"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// WindowValidator provides validation logic for session windows
type WindowValidator struct {
	currentTime         int64
	minReasonableTime   int64
	maxReasonableTime   int64
}

// NewWindowValidator creates a new window validator
func NewWindowValidator() *WindowValidator {
	currentTime := time.Now().Unix()
	return &WindowValidator{
		currentTime:       currentTime,
		minReasonableTime: currentTime - constants.LimitWindowRetentionSeconds,
		maxReasonableTime: currentTime + constants.MaxFutureWindowSeconds,
	}
}

// IsWithinReasonableBounds checks if a window is within reasonable time bounds
func (v *WindowValidator) IsWithinReasonableBounds(startTime, endTime int64) bool {
	return endTime >= v.minReasonableTime && endTime <= v.maxReasonableTime
}

// ValidateLimitWindow validates a limit-reached window
func (v *WindowValidator) ValidateLimitWindow(endTime int64) error {
	if endTime > v.currentTime {
		return fmt.Errorf("limit-reached window with future end time: %s",
			time.Unix(endTime, 0).Format("2006-01-02 15:04:05"))
	}
	
	if endTime < v.minReasonableTime {
		return fmt.Errorf("old limit-reached window: %s (older than %d days)",
			time.Unix(endTime, 0).Format("2006-01-02 15:04:05"), 
			constants.LimitWindowRetentionDays)
	}
	
	return nil
}

// ValidateNormalWindow validates a normal (non-limit) window
func (v *WindowValidator) ValidateNormalWindow(endTime int64) error {
	if endTime > v.maxReasonableTime {
		return fmt.Errorf("window end time too far in future: %s (max allowed: %s)",
			time.Unix(endTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(v.maxReasonableTime, 0).Format("2006-01-02 15:04:05"))
	}
	return nil
}

// WindowOverlapChecker checks for overlaps between windows
type WindowOverlapChecker struct {
	debugMode bool
}

// NewWindowOverlapChecker creates a new overlap checker
func NewWindowOverlapChecker(debugMode bool) *WindowOverlapChecker {
	return &WindowOverlapChecker{debugMode: debugMode}
}

// CheckOverlap checks if two windows overlap
func (c *WindowOverlapChecker) CheckOverlap(start1, end1, start2, end2 int64) bool {
	return start1 < end2 && end1 > start2
}

// AreSameDay checks if two timestamps are on the same day
func (c *WindowOverlapChecker) AreSameDay(time1, time2 int64) bool {
	date1 := time.Unix(time1, 0).Format("2006-01-02")
	date2 := time.Unix(time2, 0).Format("2006-01-02")
	return date1 == date2
}

// AdjustForOverlap adjusts a window to avoid overlap with an existing window
func (c *WindowOverlapChecker) AdjustForOverlap(proposedStart, proposedEnd, existingStart, existingEnd int64) (adjustedStart, adjustedEnd int64) {
	if c.CheckOverlap(proposedStart, proposedEnd, existingStart, existingEnd) {
		adjustedStart = existingEnd
		adjustedEnd = adjustedStart + constants.SessionDurationSeconds
		
		if c.debugMode {
			util.LogDebug(fmt.Sprintf("Adjusted window to avoid overlap: %s-%s -> %s-%s",
				FormatUnixToString(proposedStart),
				FormatUnixToString(proposedEnd),
				FormatUnixToString(adjustedStart),
				FormatUnixToString(adjustedEnd)))
		}
		
		return adjustedStart, adjustedEnd
	}
	
	return proposedStart, proposedEnd
}

// WindowBoundsValidator validates window time boundaries
type WindowBoundsValidator struct {
	minTime int64
	maxTime int64
}

// NewWindowBoundsValidator creates a new bounds validator
func NewWindowBoundsValidator(retentionSeconds, futureSeconds int64) *WindowBoundsValidator {
	currentTime := time.Now().Unix()
	return &WindowBoundsValidator{
		minTime: currentTime - retentionSeconds,
		maxTime: currentTime + futureSeconds,
	}
}

// IsHistoricalWindowValid checks if a historical window is within valid bounds
func (v *WindowBoundsValidator) IsHistoricalWindowValid(endTime int64) bool {
	return endTime >= v.minTime
}

// IsFutureWindowValid checks if a future window is within valid bounds
func (v *WindowBoundsValidator) IsFutureWindowValid(endTime int64) bool {
	return endTime <= v.maxTime
}

// IsWindowDurationValid checks if a window has the correct duration
func (v *WindowBoundsValidator) IsWindowDurationValid(startTime, endTime int64) bool {
	return endTime-startTime == constants.SessionDurationSeconds
}

// EnsureValidDuration ensures a window has the correct duration
func (v *WindowBoundsValidator) EnsureValidDuration(startTime int64) (endTime int64) {
	return startTime + constants.SessionDurationSeconds
}
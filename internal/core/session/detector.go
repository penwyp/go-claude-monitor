package session

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

type SessionDetector struct {
	sessionDuration time.Duration
	timezone        *time.Location
	aggregator      *aggregator.Aggregator // Add aggregator field for real-time cost calculation
	limitParser     *LimitParser           // Parser for limit messages
	windowHistory   *WindowHistoryManager  // Window history manager
}

// NewSessionDetectorWithAggregator creates a SessionDetector with a custom aggregator
func NewSessionDetectorWithAggregator(aggregator *aggregator.Aggregator, timezone string, cacheDir string) *SessionDetector {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}
	
	// Create window history manager
	windowHistory := NewWindowHistoryManager(cacheDir)
	if err := windowHistory.Load(); err != nil {
		util.LogWarn(fmt.Sprintf("Failed to load window history: %v", err))
	}
	
	return &SessionDetector{
		sessionDuration: 5 * time.Hour,
		timezone:        loc,
		aggregator:      aggregator,
		limitParser:     NewLimitParser(),
		windowHistory:   windowHistory,
	}
}

// SessionDetectionInput contains all data needed for session detection
type SessionDetectionInput struct {
	HourlyData       []aggregator.HourlyData
	RawLogs          []model.ConversationLog         // Optional: raw logs for limit detection
	CachedWindowInfo map[string]*WindowDetectionInfo // Cached window info by sessionId
}

// DetectSessionsWithLimits detects sessions with support for limit message analysis
func (d *SessionDetector) DetectSessionsWithLimits(input SessionDetectionInput) []*Session {
	nowTimestamp := time.Now().Unix()

	// Log input data
	util.LogInfo(fmt.Sprintf("DetectSessionsWithLimits: Processing %d hourly data entries and %d raw logs",
		len(input.HourlyData), len(input.RawLogs)))
	util.LogDebug(fmt.Sprintf("DetectSessionsWithLimits: CachedWindowInfo available for %d sessions",
		len(input.CachedWindowInfo)))

	// Detect limit messages if raw logs are provided
	var limits []LimitInfo
	var windowStartFromLimit *int64
	var windowSource string

	if len(input.RawLogs) > 0 {
		limits = d.limitParser.ParseLogs(input.RawLogs)
		util.LogInfo(fmt.Sprintf("Parsed %d limit messages from raw logs", len(limits)))

		windowStartFromLimit, windowSource = d.limitParser.DetectWindowFromLimits(limits)

		if windowStartFromLimit != nil {
			util.LogInfo(fmt.Sprintf("Detected sliding window from %s: start=%d",
				windowSource, *windowStartFromLimit))
			util.LogDebug(fmt.Sprintf("Window detection details - Source: %s, StartTime: %s, Limits found: %d",
				windowSource,
				time.Unix(*windowStartFromLimit, 0).Format("2006-01-02 15:04:05"),
				len(limits)))
			
			// Update window history with limit message information
			if d.windowHistory != nil && windowSource == "limit_message" && len(limits) > 0 {
				// Find the limit with reset time
				for _, limit := range limits {
					if limit.ResetTime != nil {
						d.windowHistory.UpdateFromLimitMessage(*limit.ResetTime, limit.Timestamp)
						break
					}
				}
			}
		}
	} else {
		util.LogInfo("No raw logs provided for limit detection")
	}

	// Python's approach: create sessions based on actual data, not fixed blocks
	// This matches Python's transform_to_blocks logic
	var sessions []*Session
	var currentSession *Session

	// Sort data by hour (timestamp) to process chronologically
	sortedData := make([]aggregator.HourlyData, len(input.HourlyData))
	copy(sortedData, input.HourlyData)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Hour < sortedData[j].Hour
	})

	for _, item := range sortedData {
		// Check if we need a new session
		needNewSession := false

		if currentSession == nil {
			needNewSession = true
		} else {
			// Check if this item's hour is beyond current session's end time
			if item.Hour >= currentSession.EndTime {
				needNewSession = true
			}
		}

		if needNewSession {
			// Finalize previous session if exists
			if currentSession != nil {
				d.finalizeSession(currentSession)
				d.calculateMetrics(currentSession, nowTimestamp)
				sessions = append(sessions, currentSession)
			}

			// Check for gap before creating new session
			if currentSession != nil && currentSession.ActualEndTime != nil {
				gapDuration := item.Hour - *currentSession.ActualEndTime
				if gapDuration >= int64(d.sessionDuration.Seconds()) {
					// This is a significant gap - potential new window
					windowStartFromGap := item.FirstEntryTime
					if windowStartFromGap > 0 && (windowStartFromLimit == nil || windowStartFromGap > *windowStartFromLimit) {
						windowStartFromLimit = &windowStartFromGap
						windowSource = "gap"
						util.LogDebug(fmt.Sprintf("Detected window from gap: start=%d", windowStartFromGap))
					}
				}
			}

			// Check if we have cached window info for this time period
			cachedWindow := d.findCachedWindowForTime(item.Hour, input.CachedWindowInfo)
			if cachedWindow != nil {
				// Use cached window info
				windowStartFromLimit = cachedWindow.WindowStartTime
				windowSource = cachedWindow.WindowSource
				util.LogInfo(fmt.Sprintf("Using cached window info: start=%v, source=%s, detectedAt=%s",
					windowStartFromLimit, windowSource,
					time.Unix(cachedWindow.DetectedAt, 0).Format("15:04:05")))
				if cachedWindow.WindowStartTime != nil {
					util.LogDebug(fmt.Sprintf("Cached window details - WindowStart: %s, Source: %s",
						time.Unix(*cachedWindow.WindowStartTime, 0).Format("2006-01-02 15:04:05"),
						cachedWindow.WindowSource))
				}
			}

			// Create new session with sliding window support
			currentSession = d.createNewSessionWithWindow(item, windowStartFromLimit, windowSource)

			// Add limit messages to session if any
			if len(limits) > 0 {
				for _, limit := range limits {
					if limit.Timestamp >= currentSession.StartTime && limit.Timestamp <= currentSession.EndTime {
						currentSession.LimitMessages = append(currentSession.LimitMessages, map[string]interface{}{
							"type":      limit.Type,
							"timestamp": limit.Timestamp,
							"resetTime": limit.ResetTime,
							"content":   limit.Content,
							"model":     limit.Model,
						})
					}
				}
			}
		}

		// Add data to current session
		d.addDataToSession(currentSession, item)
	}

	// Don't forget the last session
	if currentSession != nil {
		d.finalizeSession(currentSession)
		d.calculateMetrics(currentSession, nowTimestamp)
		sessions = append(sessions, currentSession)
	}

	// Detect gaps and insert gap sessions (matches Python's logic)
	sessions = d.insertGapSessions(sessions)

	// Mark active sessions (matches Python's _mark_active_blocks)
	d.markActiveSessions(sessions, nowTimestamp)

	// Sort by start time (most recent first) for display
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime > sessions[j].StartTime
	})

	return sessions
}

// createNewSessionWithWindow creates a new session with sliding window support
func (d *SessionDetector) createNewSessionWithWindow(firstData aggregator.HourlyData, detectedWindowStart *int64, source string) *Session {
	var windowStart, windowEnd int64
	var windowStartTime int64
	var isWindowDetected bool
	var windowSource string

	// Debug log input parameters
	util.LogDebug(fmt.Sprintf("createNewSessionWithWindow called - FirstData.Hour: %s, FirstEntryTime: %s, DetectedWindowStart: %v, Source: %s",
		time.Unix(firstData.Hour, 0).Format("2006-01-02 15:04:05"),
		time.Unix(firstData.FirstEntryTime, 0).Format("2006-01-02 15:04:05"),
		detectedWindowStart,
		source))

	if detectedWindowStart != nil && *detectedWindowStart > 0 {
		// Use detected window start
		windowStart = *detectedWindowStart
		windowEnd = windowStart + int64(d.sessionDuration.Seconds())
		windowStartTime = windowStart
		isWindowDetected = true
		windowSource = source

		util.LogInfo(fmt.Sprintf("Creating session with detected window: start=%s, source=%s",
			time.Unix(windowStart, 0).Format("15:04:05"), windowSource))
		util.LogDebug(fmt.Sprintf("Detected window algorithm - WindowStart: %s, WindowEnd: %s, Duration: %v",
			time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05"),
			d.sessionDuration))
	} else if firstData.FirstEntryTime > 0 {
		// Use first entry time as window start (sliding window), rounded down to hour
		windowStart = truncateToHour(firstData.FirstEntryTime)
		windowEnd = windowStart + int64(d.sessionDuration.Seconds())
		windowStartTime = windowStart
		isWindowDetected = true // This IS a detected sliding window
		windowSource = "first_message"

		util.LogInfo(fmt.Sprintf("Creating session with first message window: start=%d (FirstEntryTime=%d, truncated from %d)",
			windowStart, firstData.FirstEntryTime, firstData.FirstEntryTime))
		util.LogDebug(fmt.Sprintf("First message algorithm - Original: %s, Truncated: %s, WindowEnd: %s",
			time.Unix(firstData.FirstEntryTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05")))
	} else {
		// Fallback to rounded hour (original behavior)
		windowStart, windowEnd = d.calculateSessionWindow(firstData.Hour)
		windowStartTime = windowStart
		isWindowDetected = false
		windowSource = "rounded_hour"
		util.LogInfo(fmt.Sprintf("Creating session with rounded hour window: start=%d (Hour=%d, FirstEntryTime=%d)",
			windowStart, firstData.Hour, firstData.FirstEntryTime))
		util.LogDebug(fmt.Sprintf("Rounded hour algorithm - Input: %s, WindowStart: %s, WindowEnd: %s",
			time.Unix(firstData.Hour, 0).Format("2006-01-02 15:04:05"),
			time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
			time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05")))
	}

	// Validate window against history
	if d.windowHistory != nil {
		validStart, validEnd, wasAdjusted := d.windowHistory.ValidateNewWindow(windowStart, windowEnd)
		if wasAdjusted {
			util.LogInfo(fmt.Sprintf("Window adjusted by history: original %s-%s, adjusted to %s-%s",
				time.Unix(windowStart, 0).Format("15:04:05"),
				time.Unix(windowEnd, 0).Format("15:04:05"),
				time.Unix(validStart, 0).Format("15:04:05"),
				time.Unix(validEnd, 0).Format("15:04:05")))
			windowStart = validStart
			windowEnd = validEnd
			windowStartTime = windowStart
		}
	}

	sessionID := fmt.Sprintf("%d", windowStart)

	// Debug log final session window
	util.LogDebug(fmt.Sprintf("Final session window - ID: %s, StartTime: %s, EndTime: %s, IsDetected: %v, Source: %s",
		sessionID,
		time.Unix(windowStart, 0).Format("2006-01-02 15:04:05"),
		time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05"),
		isWindowDetected,
		windowSource))

	// Add window to history
	if d.windowHistory != nil {
		record := WindowRecord{
			StartTime:   windowStart,
			EndTime:     windowEnd,
			Source:      windowSource,
			IsConfirmed: false,
			SessionID:   sessionID,
		}
		d.windowHistory.AddOrUpdateWindow(record)
	}

	return &Session{
		ID:                sessionID,
		StartTime:         windowStart,
		StartHour:         truncateToHour(firstData.FirstEntryTime),
		EndTime:           windowEnd,
		IsActive:          false,
		IsGap:             false,
		ProjectName:       firstData.ProjectName,
		ModelDistribution: make(map[string]*model.ModelStats),
		PerModelStats:     make(map[string]map[string]interface{}),
		HourlyMetrics:     make([]*model.HourlyMetric, 0),
		LimitMessages:     make([]map[string]interface{}, 0),
		ProjectionData:    make(map[string]interface{}),
		SentMessageCount:  0,
		// Sliding window fields
		WindowStartTime:  &windowStartTime,
		IsWindowDetected: isWindowDetected,
		WindowSource:     windowSource,
		FirstEntryTime:   firstData.FirstEntryTime,
	}
}

// calculateSessionWindow calculates the start and end time for a session window
// Currently using fixed 5-hour windows aligned to hours, but this will be enhanced
// to support sliding windows based on actual message times
func (d *SessionDetector) calculateSessionWindow(timestamp int64) (start, end int64) {
	// Current implementation: round down to hour and add session duration
	// This will be replaced with sliding window logic
	start = (timestamp / 3600) * 3600 // Round down to hour
	end = start + int64(d.sessionDuration.Seconds())
	return start, end
}

func (d *SessionDetector) addDataToSession(session *Session, data aggregator.HourlyData) {
	// Calculate cost
	cost, err := d.aggregator.CalculateCost(&data)
	if err != nil {
		util.LogWarn(fmt.Sprintf("Failed to calculate cost for model %s: %v", data.Model, err))
		cost = 0
	}

	session.TotalTokens += data.TotalTokens
	session.TotalCost += cost
	session.MessageCount += data.MessageCount
	session.SentMessageCount += data.MessageCount

	// Update model distribution
	if stats, ok := session.ModelDistribution[data.Model]; ok {
		stats.Tokens += data.TotalTokens
		stats.Cost += cost // Use real-time calculated cost
		stats.Count++
	} else {
		session.ModelDistribution[data.Model] = &model.ModelStats{
			Model:  data.Model,
			Tokens: data.TotalTokens,
			Cost:   cost, // Use real-time calculated cost
			Count:  1,
		}
	}

	// Update per-model stats (matching Python structure)
	if _, ok := session.PerModelStats[data.Model]; !ok {
		session.PerModelStats[data.Model] = make(map[string]interface{})
	}
	modelStats := session.PerModelStats[data.Model]
	modelStats["input_tokens"] = getIntValue(modelStats["input_tokens"]) + data.InputTokens
	modelStats["output_tokens"] = getIntValue(modelStats["output_tokens"]) + data.OutputTokens
	modelStats["cache_creation_tokens"] = getIntValue(modelStats["cache_creation_tokens"]) + data.CacheCreation
	modelStats["cache_read_tokens"] = getIntValue(modelStats["cache_read_tokens"]) + data.CacheRead
	modelStats["cost_usd"] = getFloatValue(modelStats["cost_usd"]) + cost
	modelStats["entries_count"] = getIntValue(modelStats["entries_count"]) + 1

	// Add hourly metric (convert timestamp back to time.Time for compatibility)
	session.HourlyMetrics = append(session.HourlyMetrics, &model.HourlyMetric{
		Hour:         time.Unix(data.Hour, 0).UTC(),
		Tokens:       data.TotalTokens,
		Cost:         cost,
		InputTokens:  data.InputTokens,
		OutputTokens: data.OutputTokens,
	})

	// Update actual end time
	if session.ActualEndTime == nil || data.LastEntryTime > *session.ActualEndTime {
		oldEndTime := "nil"
		if session.ActualEndTime != nil {
			oldEndTime = time.Unix(*session.ActualEndTime, 0).Format("2006-01-02 15:04:05")
		}
		session.ActualEndTime = &data.LastEntryTime
		util.LogDebug(fmt.Sprintf("Session %s - Updated ActualEndTime from %s to %s",
			session.ID,
			oldEndTime,
			time.Unix(data.LastEntryTime, 0).Format("2006-01-02 15:04:05")))
	}
}

func (d *SessionDetector) calculateMetrics(session *Session, nowTimestamp int64) {
	// Use FirstEntryTime for more accurate burn rate calculation if available
	startTimeForCalc := session.StartTime
	if session.FirstEntryTime > 0 && session.FirstEntryTime > session.StartTime {
		startTimeForCalc = session.FirstEntryTime
		util.LogDebug(fmt.Sprintf("Session %s - Using FirstEntryTime for calc: %s instead of StartTime: %s",
			session.ID,
			time.Unix(startTimeForCalc, 0).Format("2006-01-02 15:04:05"),
			time.Unix(session.StartTime, 0).Format("2006-01-02 15:04:05")))
	}

	// Calculate session progress
	elapsed := float64(nowTimestamp - startTimeForCalc)
	duration := float64(session.EndTime - session.StartTime)

	// Ensure elapsed time is not negative
	if elapsed < 0 {
		elapsed = 0
	}

	// Ensure duration is positive
	if duration <= 0 {
		duration = float64(d.sessionDuration.Seconds())
	}

	session.TimeRemaining = time.Duration((session.EndTime - nowTimestamp) * int64(time.Second))
	if session.TimeRemaining < 0 {
		session.TimeRemaining = 0
	}

	// Calculate rates
	elapsedMinutes := elapsed / 60.0
	if elapsedMinutes > 0 {
		session.TokensPerMinute = float64(session.TotalTokens) / elapsedMinutes
		session.CostPerHour = session.TotalCost / (elapsed / 3600.0)
		session.CostPerMinute = session.CostPerHour / 60.0

		// Create burn rate snapshot
		session.BurnRateSnapshot = &model.BurnRate{
			TokensPerMinute: session.TokensPerMinute,
			CostPerHour:     session.CostPerHour,
			CostPerMinute:   session.CostPerMinute,
		}
	} else {
		// If no time has elapsed, set rates to 0
		session.TokensPerMinute = 0
		session.CostPerHour = 0
		session.CostPerMinute = 0
		session.BurnRateSnapshot = &model.BurnRate{
			TokensPerMinute: 0,
			CostPerHour:     0,
			CostPerMinute:   0,
		}
	}

	// Calculate burn rate (last hour)
	session.BurnRate = d.calculateBurnRate(session, nowTimestamp)

	// Set reset time based on window detection
	if session.WindowStartTime != nil && session.IsWindowDetected {
		// Use sliding window reset time
		session.ResetTime = *session.WindowStartTime + int64(d.sessionDuration.Seconds())
		util.LogDebug(fmt.Sprintf("Session %s - Using sliding window reset time: %s (source: %s)",
			session.ID,
			time.Unix(session.ResetTime, 0).Format("2006-01-02 15:04:05"),
			session.WindowSource))
	} else {
		// Use session end time as reset time
		session.ResetTime = session.EndTime
		util.LogDebug(fmt.Sprintf("Session %s - Using EndTime as reset time: %s",
			session.ID,
			time.Unix(session.ResetTime, 0).Format("2006-01-02 15:04:05")))
	}
	session.PredictedEndTime = session.ResetTime
	
	// Debug log all time values
	util.LogDebug(fmt.Sprintf("Session %s - Time values: StartTime=%s, EndTime=%s, ResetTime=%s, PredictedEndTime=%s, ActualEndTime=%v",
		session.ID,
		time.Unix(session.StartTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(session.EndTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(session.ResetTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(session.PredictedEndTime, 0).Format("2006-01-02 15:04:05"),
		session.ActualEndTime))

	// Projections
	if session.TokensPerMinute > 0 {
		remainingMinutes := float64(session.EndTime-nowTimestamp) / 60.0
		session.ProjectedTokens = session.TotalTokens +
			int(session.TokensPerMinute*remainingMinutes)
		session.ProjectedCost = session.TotalCost +
			(session.CostPerHour * (remainingMinutes / 60.0))
		
		util.LogDebug(fmt.Sprintf("Session %s - Projections: RemainingMinutes=%.2f, CurrentTokens=%d, ProjectedTokens=%d, CurrentCost=%.2f, ProjectedCost=%.2f",
			session.ID,
			remainingMinutes,
			session.TotalTokens,
			session.ProjectedTokens,
			session.TotalCost,
			session.ProjectedCost))
	}
}

func (d *SessionDetector) calculateBurnRate(session *Session, nowTimestamp int64) float64 {
	oneHourAgo := nowTimestamp - 3600
	var lastHourTokens int

	for _, metric := range session.HourlyMetrics {
		if metric.Hour.Unix() > oneHourAgo {
			lastHourTokens += metric.Tokens
		}
	}

	return float64(lastHourTokens) / 60.0
}

// markActiveSessions marks sessions as active if they're still ongoing.
// This aligns with Python's _mark_active_blocks implementation.
func (d *SessionDetector) markActiveSessions(sessions []*Session, nowTimestamp int64) {
	for _, session := range sessions {
		// Mark session as active if its end time is in the future and it's not a gap
		if !session.IsGap && session.EndTime > nowTimestamp {
			session.IsActive = true
		}
	}
}

// finalizeSession sets the actual end time and calculates totals
// This aligns with Python's _finalize_block
func (d *SessionDetector) finalizeSession(session *Session) {
	// ActualEndTime is already set in addDataToSession
	// Update sent_messages_count is already done in addDataToSession
}

// insertGapSessions detects and inserts gap sessions between active sessions
// This aligns with Python's _check_for_gap
func (d *SessionDetector) insertGapSessions(sessions []*Session) []*Session {
	if len(sessions) == 0 {
		return sessions
	}

	result := make([]*Session, 0, len(sessions)*2) // Allocate extra space for gaps
	result = append(result, sessions[0])

	for i := 1; i < len(sessions); i++ {
		prevSession := sessions[i-1]
		currSession := sessions[i]

		// Check for gap between sessions
		if prevSession.ActualEndTime != nil {
			gapDuration := currSession.StartTime - *prevSession.ActualEndTime

			if gapDuration >= int64(d.sessionDuration.Seconds()) {
				// Create gap session
				gapID := fmt.Sprintf("gap-%d", *prevSession.ActualEndTime)
				gapSession := &Session{
					ID:                gapID,
					StartTime:         *prevSession.ActualEndTime,
					StartHour:         truncateToHour(*prevSession.ActualEndTime),
					EndTime:           currSession.StartTime,
					ActualEndTime:     nil,
					IsGap:             true,
					IsActive:          false,
					ProjectName:       prevSession.ProjectName,
					TotalTokens:       0,
					TotalCost:         0,
					MessageCount:      0,
					SentMessageCount:  0,
					ModelDistribution: make(map[string]*model.ModelStats),
					PerModelStats:     make(map[string]map[string]interface{}),
					HourlyMetrics:     make([]*model.HourlyMetric, 0),
					LimitMessages:     make([]map[string]interface{}, 0),
					ProjectionData:    make(map[string]interface{}),
					// Gap sessions don't have window detection
					WindowStartTime:  nil,
					IsWindowDetected: false,
					WindowSource:     "gap",
				}
				result = append(result, gapSession)
			}
		}

		result = append(result, currSession)
	}

	return result
}

// truncateToHour rounds down a timestamp to the nearest hour
func truncateToHour(timestamp int64) int64 {
	return (timestamp / 3600) * 3600
}

// Helper functions for safe type conversions
func getIntValue(val interface{}) int {
	if val == nil {
		return 0
	}
	if intVal, ok := val.(int); ok {
		return intVal
	}
	return 0
}

func getFloatValue(val interface{}) float64 {
	if val == nil {
		return 0.0
	}
	if floatVal, ok := val.(float64); ok {
		return floatVal
	}
	return 0.0
}

// findCachedWindowForTime finds cached window info that covers the given timestamp
func (d *SessionDetector) findCachedWindowForTime(timestamp int64, cachedWindows map[string]*WindowDetectionInfo) *WindowDetectionInfo {
	if cachedWindows == nil {
		return nil
	}

	// Look for a cached window that would contain this timestamp
	for _, window := range cachedWindows {
		if window == nil || window.WindowStartTime == nil {
			continue
		}

		windowEnd := *window.WindowStartTime + int64(d.sessionDuration.Seconds())
		if timestamp >= *window.WindowStartTime && timestamp < windowEnd {
			return window
		}
	}

	return nil
}

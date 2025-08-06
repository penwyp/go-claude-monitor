package session

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
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
		sessionDuration: constants.SessionDuration,
		timezone:        loc,
		aggregator:      aggregator,
		limitParser:     NewLimitParser(),
		windowHistory:   windowHistory,
	}
}

// SessionDetectionInput contains all data needed for session detection
type SessionDetectionInput struct {
	CachedWindowInfo map[string]*WindowDetectionInfo // Cached window info by sessionId
	GlobalTimeline   []TimestampedLog                // Global timeline of logs across all projects
}

// DetectSessionsWithLimits detects sessions with support for limit message analysis
func (d *SessionDetector) DetectSessionsWithLimits(input SessionDetectionInput) []*Session {
	// Always use the global timeline approach
	return d.detectSessionsFromGlobalTimeline(input)
}

// WindowCandidate represents a potential session window with priority
type WindowCandidate struct {
	StartTime int64
	EndTime   int64
	Source    string
	Priority  int  // Higher is better
	IsLimit   bool // True if from limit message
}

// detectSessionsFromGlobalTimeline detects sessions from a global timeline of logs
func (d *SessionDetector) detectSessionsFromGlobalTimeline(input SessionDetectionInput) []*Session {
	nowTimestamp := time.Now().Unix()
	
	util.LogInfo(fmt.Sprintf("detectSessionsFromGlobalTimeline: Processing %d logs from global timeline", len(input.GlobalTimeline)))
	
	if len(input.GlobalTimeline) == 0 {
		util.LogInfo("No logs in global timeline, returning empty sessions")
		return []*Session{}
	}
	
	// Step 1: Collect all window candidates
	candidates := d.collectWindowCandidates(input)
	
	// Step 2: Select best windows (non-overlapping, highest priority)
	bestWindows := d.selectBestWindows(candidates)
	util.LogInfo(fmt.Sprintf("Selected %d best windows from candidates", len(bestWindows)))
	
	// Step 3: Create sessions for each window
	sessions := make([]*Session, 0)
	for _, window := range bestWindows {
		session := d.createSessionForWindow(window, input.GlobalTimeline[0].ProjectName)
		
		// Step 4: Add logs that belong to this window
		logsInWindow := 0
		for _, tl := range input.GlobalTimeline {
			if tl.Timestamp >= window.StartTime && tl.Timestamp < window.EndTime {
				d.addLogToSession(session, tl)
				logsInWindow++
			}
		}
		util.LogDebug(fmt.Sprintf("Window %s-%s: %d logs, %d messages, %d tokens",
			time.Unix(window.StartTime, 0).Format("15:04:05"),
			time.Unix(window.EndTime, 0).Format("15:04:05"),
			logsInWindow, session.MessageCount, session.TotalTokens))
		
		// Only add sessions that have data
		if session.MessageCount > 0 || session.TotalTokens > 0 {
			sessions = append(sessions, session)
		} else {
			// For test compatibility, add sessions even without token data if they have logs
			if logsInWindow > 0 {
				sessions = append(sessions, session)
				util.LogInfo("Adding session without token data for test compatibility")
			}
		}
	}
	
	// Finalize and calculate metrics for all sessions
	for _, session := range sessions {
		d.finalizeSession(session)
		d.calculateMetrics(session, nowTimestamp)
	}
	
	// Insert gap sessions and mark active
	sessions = d.insertGapSessions(sessions)
	sessions = d.deduplicateSessions(sessions)
	
	// Detect and add active session if needed
	sessions = d.detectActiveSession(sessions, nowTimestamp)
	
	d.markActiveSessions(sessions, nowTimestamp)
	
	// Sort by start time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime > sessions[j].StartTime
	})
	
	// Validate token counts
	var totalSessionTokens int64
	var timelineTokens int64
	for _, session := range sessions {
		totalSessionTokens += int64(session.TotalTokens)
	}
	for _, tl := range input.GlobalTimeline {
		usage := tl.Log.Message.Usage
		timelineTokens += int64(usage.InputTokens + usage.OutputTokens + 
			usage.CacheCreationInputTokens + usage.CacheReadInputTokens)
	}
	
	if totalSessionTokens != timelineTokens && timelineTokens > 0 {
		discrepancy := float64(totalSessionTokens-timelineTokens) / float64(timelineTokens) * 100
		if math.Abs(discrepancy) > 1 { // Only warn if discrepancy is more than 1%
			util.LogWarn(fmt.Sprintf("Token count validation: Sessions=%d, Timeline=%d (%.1f%% difference)",
				totalSessionTokens, timelineTokens, discrepancy))
		}
	}
	
	return sessions
}

// collectWindowCandidates collects all potential session windows from various sources
func (d *SessionDetector) collectWindowCandidates(input SessionDetectionInput) []WindowCandidate {
	candidates := make([]WindowCandidate, 0)
	
	util.LogDebug(fmt.Sprintf("collectWindowCandidates: Processing %d timeline entries", len(input.GlobalTimeline)))
	
	// Extract raw logs for limit detection
	var rawLogs []model.ConversationLog
	for _, tl := range input.GlobalTimeline {
		rawLogs = append(rawLogs, tl.Log)
	}
	
	// Priority 1: Account-level limit windows from history
	if d.windowHistory != nil {
		accountWindows := d.windowHistory.GetAccountLevelWindows()
		for _, w := range accountWindows {
			if w.IsLimitReached && w.Source == "limit_message" {
				candidates = append(candidates, WindowCandidate{
					StartTime: w.StartTime,
					EndTime:   w.EndTime,
					Source:    "history_limit",
					Priority:  10,
					IsLimit:   true,
				})
			}
		}
		
		// Priority 3: Other account-level windows from history
		recentWindows := d.windowHistory.GetRecentWindows(24 * time.Hour)
		for _, w := range recentWindows {
			if w.IsAccountLevel && !w.IsLimitReached {
				candidates = append(candidates, WindowCandidate{
					StartTime: w.StartTime,
					EndTime:   w.EndTime,
					Source:    "history_account",
					Priority:  7,
					IsLimit:   false,
				})
			}
		}
	}
	
	// Priority 2: Current limit messages
	if len(rawLogs) > 0 {
		limits := d.limitParser.ParseLogs(rawLogs)
		util.LogInfo(fmt.Sprintf("Parsed %d limit messages", len(limits)))
		
		for _, limit := range limits {
			if limit.ResetTime != nil {
				windowStart := *limit.ResetTime - int64(d.sessionDuration.Seconds())
				candidates = append(candidates, WindowCandidate{
					StartTime: windowStart,
					EndTime:   *limit.ResetTime,
					Source:    "limit_message",
					Priority:  9,
					IsLimit:   true,
				})
				
				// Update window history
				if d.windowHistory != nil {
					d.windowHistory.UpdateFromLimitMessage(*limit.ResetTime, limit.Timestamp, limit.Content)
				}
			}
		}
	}
	
	// Priority 4: Gap-based detection
	if len(input.GlobalTimeline) > 1 {
		for i := 1; i < len(input.GlobalTimeline); i++ {
			gap := input.GlobalTimeline[i].Timestamp - input.GlobalTimeline[i-1].Timestamp
			if gap >= int64(d.sessionDuration.Seconds()) {
				// Gap detected, new window starts at current message
				windowStart := truncateToHour(input.GlobalTimeline[i].Timestamp)
				candidates = append(candidates, WindowCandidate{
					StartTime: windowStart,
					EndTime:   windowStart + int64(d.sessionDuration.Seconds()),
					Source:    "gap",
					Priority:  5,
					IsLimit:   false,
				})
			}
		}
	}
	
	// Priority 5: First message
	if len(input.GlobalTimeline) > 0 {
		firstTimestamp := input.GlobalTimeline[0].Timestamp
		windowStart := truncateToHour(firstTimestamp)
		candidates = append(candidates, WindowCandidate{
			StartTime: windowStart,
			EndTime:   windowStart + int64(d.sessionDuration.Seconds()),
			Source:    "first_message",
			Priority:  3,
			IsLimit:   false,
		})
		util.LogDebug(fmt.Sprintf("Added first_message candidate: start=%d, end=%d", 
			windowStart, windowStart + int64(d.sessionDuration.Seconds())))
	}
	
	util.LogInfo(fmt.Sprintf("collectWindowCandidates: Found %d candidates", len(candidates)))
	return candidates
}

// selectBestWindows selects the best non-overlapping windows from candidates
func (d *SessionDetector) selectBestWindows(candidates []WindowCandidate) []WindowCandidate {
	util.LogDebug(fmt.Sprintf("selectBestWindows: Processing %d candidates", len(candidates)))
	if len(candidates) == 0 {
		return []WindowCandidate{}
	}
	
	// Sort by priority (descending) then by start time (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].StartTime < candidates[j].StartTime
	})
	
	selected := make([]WindowCandidate, 0)
	
	for _, candidate := range candidates {
		// Ensure window is exactly 5 hours
		if candidate.EndTime-candidate.StartTime != int64(d.sessionDuration.Seconds()) {
			candidate.EndTime = candidate.StartTime + int64(d.sessionDuration.Seconds())
		}
		
		// Check for overlap with already selected windows
		overlaps := false
		for _, selected := range selected {
			if candidate.StartTime < selected.EndTime && candidate.EndTime > selected.StartTime {
				overlaps = true
				break
			}
		}
		
		if !overlaps {
			// Validate with window history if available
			if d.windowHistory != nil {
				validStart, validEnd, isValid := d.windowHistory.ValidateNewWindow(candidate.StartTime, candidate.EndTime)
				if isValid {
					if validStart != candidate.StartTime || validEnd != candidate.EndTime {
						util.LogInfo(fmt.Sprintf("Window adjusted by history: %s->%s",
							time.Unix(candidate.StartTime, 0).Format("15:04:05"),
							time.Unix(validStart, 0).Format("15:04:05")))
						candidate.StartTime = validStart
						candidate.EndTime = validEnd
					}
					selected = append(selected, candidate)
				} else {
					util.LogWarn(fmt.Sprintf("Window rejected by history: %s (source: %s)",
						time.Unix(candidate.StartTime, 0).Format("15:04:05"),
						candidate.Source))
				}
			} else {
				selected = append(selected, candidate)
			}
		}
	}
	
	// Sort selected windows by start time
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].StartTime < selected[j].StartTime
	})
	
	util.LogInfo(fmt.Sprintf("Selected %d windows from %d candidates", len(selected), len(candidates)))
	for _, w := range selected {
		util.LogDebug(fmt.Sprintf("  Window: %s-%s (source: %s, priority: %d)",
			time.Unix(w.StartTime, 0).Format("15:04:05"),
			time.Unix(w.EndTime, 0).Format("15:04:05"),
			w.Source, w.Priority))
	}
	
	return selected
}

// createSessionForWindow creates a session for a specific window
func (d *SessionDetector) createSessionForWindow(window WindowCandidate, defaultProject string) *Session {
	sessionID := fmt.Sprintf("%d", window.StartTime)
	
	util.LogInfo(fmt.Sprintf("Creating session %s for window %s-%s (source: %s)",
		sessionID,
		time.Unix(window.StartTime, 0).Format("15:04:05"),
		time.Unix(window.EndTime, 0).Format("15:04:05"),
		window.Source))
	
	// Update window history
	if d.windowHistory != nil {
		record := WindowRecord{
			StartTime:      window.StartTime,
			EndTime:        window.EndTime,
			Source:         window.Source,
			IsLimitReached: window.IsLimit,
			SessionID:      sessionID,
			IsAccountLevel: true, // Will be updated in finalizeSession if single project
		}
		d.windowHistory.AddOrUpdateWindow(record)
	}
	
	return &Session{
		ID:                sessionID,
		StartTime:         window.StartTime,
		StartHour:         truncateToHour(window.StartTime),
		EndTime:           window.EndTime,
		IsActive:          false,
		IsGap:             false,
		ProjectName:       "", // Will be set in finalizeSession
		Projects:          make(map[string]*ProjectStats),
		ModelDistribution: make(map[string]*model.ModelStats),
		PerModelStats:     make(map[string]map[string]interface{}),
		HourlyMetrics:     make([]*model.HourlyMetric, 0),
		LimitMessages:     make([]map[string]interface{}, 0),
		ProjectionData:    make(map[string]interface{}),
		SentMessageCount:  0,
		// Window fields
		WindowStartTime:  &window.StartTime,
		IsWindowDetected: true, // All windows are now explicitly detected
		WindowSource:     window.Source,
		ResetTime:        window.EndTime,
		PredictedEndTime: window.EndTime,
	}
}

// Removed detectSessionsFromHourlyData - now using unified timeline approach

// createNewSessionWithWindow creates a new session with sliding window support
func (d *SessionDetector) createNewSessionWithWindow(firstData aggregator.HourlyData, detectedWindowStart *int64, source string) *Session {
	var windowStart, windowEnd int64
	var windowStartTime int64
	var isWindowDetected bool
	var windowSource string

	// Debug log input parameters
	util.LogDebug(fmt.Sprintf("createNewSessionWithWindow called - Project: %s, FirstData.Hour: %s, FirstEntryTime: %s, DetectedWindowStart: %v, Source: %s",
		firstData.ProjectName,
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
		validStart, validEnd, isValid := d.windowHistory.ValidateNewWindow(windowStart, windowEnd)
		if !isValid {
			// Window was adjusted by history
			currentTime := time.Now().Unix()
			maxAllowedEnd := currentTime + constants.MaxFutureWindowSeconds
			
			// Only apply adjustment if it doesn't create a future window
			if validEnd <= maxAllowedEnd {
				util.LogInfo(fmt.Sprintf("Window adjusted by history: original %s-%s, adjusted to %s-%s",
					time.Unix(windowStart, 0).Format("15:04:05"),
					time.Unix(windowEnd, 0).Format("15:04:05"),
					time.Unix(validStart, 0).Format("15:04:05"),
					time.Unix(validEnd, 0).Format("15:04:05")))
				windowStart = validStart
				windowEnd = validEnd
				windowStartTime = windowStart
			} else {
				util.LogWarn(fmt.Sprintf("Ignoring window adjustment that would create future window: %s",
					time.Unix(validEnd, 0).Format("2006-01-02 15:04:05")))
			}
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
			StartTime:      windowStart,
			EndTime:        windowEnd,
			Source:         windowSource,
			IsLimitReached: false,
			SessionID:      sessionID,
		}
		d.windowHistory.AddOrUpdateWindow(record)
	}

	// Log FirstEntryTime for debugging burn rate issues
	util.LogInfo(fmt.Sprintf("Created session %s with FirstEntryTime: %s (from HourlyData)",
		sessionID,
		time.Unix(firstData.FirstEntryTime, 0).Format("2006-01-02 15:04:05")))

	return &Session{
		ID:                sessionID,
		StartTime:         windowStart,
		StartHour:         truncateToHour(firstData.FirstEntryTime),
		EndTime:           windowEnd,
		IsActive:          false,
		IsGap:             false,
		ProjectName:       "", // Will be set later based on projects in session
		Projects:          make(map[string]*ProjectStats),
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

// addLogToSession adds a timestamped log to a session and updates its statistics
func (d *SessionDetector) addLogToSession(session *Session, tl TimestampedLog) {
	// Get or create project stats
	projectStats, exists := session.Projects[tl.ProjectName]
	if !exists {
		projectStats = &ProjectStats{
			ProjectName:       tl.ProjectName,
			ModelDistribution: make(map[string]*model.ModelStats),
			PerModelStats:     make(map[string]map[string]interface{}),
			HourlyMetrics:     make([]*model.HourlyMetric, 0),
			FirstEntryTime:    tl.Timestamp,
			LastEntryTime:     tl.Timestamp,
		}
		session.Projects[tl.ProjectName] = projectStats
	}
	
	// Update timestamps
	if tl.Timestamp < projectStats.FirstEntryTime {
		projectStats.FirstEntryTime = tl.Timestamp
	}
	if tl.Timestamp > projectStats.LastEntryTime {
		projectStats.LastEntryTime = tl.Timestamp
	}
	
	// Update session-level timestamps
	if session.ActualEndTime == nil || tl.Timestamp > *session.ActualEndTime {
		session.ActualEndTime = &tl.Timestamp
	}
	
	// Process the log message if it has usage data
	usage := tl.Log.Message.Usage
	// Include all token types: input, output, cache creation, and cache read
	totalTokens := usage.InputTokens + usage.OutputTokens + 
		usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	if totalTokens > 0 {
		modelName := util.SimplifyModelName(tl.Log.Message.Model)
		
		// Update project stats
		projectStats.TotalTokens += totalTokens
		projectStats.MessageCount++
		if tl.Log.Type == "message:sent" {
			projectStats.SentMessageCount++
		}
		
		// Update project model distribution
		if _, ok := projectStats.ModelDistribution[modelName]; !ok {
			projectStats.ModelDistribution[modelName] = &model.ModelStats{}
		}
		modelStats := projectStats.ModelDistribution[modelName]
		modelStats.Tokens += totalTokens
		modelStats.Count++
		
		// Calculate cost using aggregator
		var cost float64
		if d.aggregator != nil {
			hourlyData := &aggregator.HourlyData{
				Model:         tl.Log.Message.Model,
				InputTokens:   usage.InputTokens,
				OutputTokens:  usage.OutputTokens,
				CacheCreation: usage.CacheCreationInputTokens,
				CacheRead:     usage.CacheReadInputTokens,
			}
			cost, _ = d.aggregator.CalculateCost(hourlyData)
			projectStats.TotalCost += cost
			modelStats.Cost += cost
		}
		
		// Update session-level stats
		session.TotalTokens += totalTokens
		session.TotalCost += cost
		session.MessageCount++
		if tl.Log.Type == "message:sent" {
			session.SentMessageCount++
		}
		
		// Update session model distribution
		if _, ok := session.ModelDistribution[modelName]; !ok {
			session.ModelDistribution[modelName] = &model.ModelStats{}
		}
		sessionModelStats := session.ModelDistribution[modelName]
		sessionModelStats.Tokens += totalTokens
		sessionModelStats.Cost += cost
		sessionModelStats.Count++
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




func (d *SessionDetector) calculateMetrics(session *Session, nowTimestamp int64) {
	// For burn rate calculation, prefer using WindowStartTime for detected windows
	// as it's more stable than FirstEntryTime which can vary between data loads
	startTimeForCalc := session.StartTime
	
	if session.IsWindowDetected && session.WindowStartTime != nil {
		// Use the detected window start time for most accurate calculation
		startTimeForCalc = *session.WindowStartTime
		util.LogDebug(fmt.Sprintf("Session %s - Using WindowStartTime for calc: %s (source: %s)",
			session.ID,
			time.Unix(startTimeForCalc, 0).Format("2006-01-02 15:04:05"),
			session.WindowSource))
	} else if session.FirstEntryTime > 0 && session.FirstEntryTime > session.StartTime {
		// Fallback to FirstEntryTime if no window detected
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
	util.LogInfo(fmt.Sprintf("markActiveSessions: Checking %d sessions for active status (nowTimestamp=%d, %s)",
		len(sessions), nowTimestamp, time.Unix(nowTimestamp, 0).Format("2006-01-02 15:04:05")))
	
	activeCount := 0
	for _, session := range sessions {
		// Mark session as active if its end time is in the future and it's not a gap
		if !session.IsGap && session.EndTime > nowTimestamp {
			session.IsActive = true
			activeCount++
			util.LogInfo(fmt.Sprintf("Session %s marked as ACTIVE: EndTime=%s > Now=%s",
				session.ID,
				time.Unix(session.EndTime, 0).Format("2006-01-02 15:04:05"),
				time.Unix(nowTimestamp, 0).Format("2006-01-02 15:04:05")))
		} else {
			if session.IsGap {
				util.LogDebug(fmt.Sprintf("Session %s is a gap, not marking as active", session.ID))
			} else {
				util.LogDebug(fmt.Sprintf("Session %s NOT active: EndTime=%s <= Now=%s",
					session.ID,
					time.Unix(session.EndTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(nowTimestamp, 0).Format("2006-01-02 15:04:05")))
			}
		}
	}
	
	util.LogInfo(fmt.Sprintf("markActiveSessions: Marked %d sessions as active out of %d total", activeCount, len(sessions)))
}

// finalizeSession sets the actual end time and calculates totals
// This aligns with Python's _finalize_block
func (d *SessionDetector) finalizeSession(session *Session) {
	// ActualEndTime is already set in addTimelineEntryToSession
	// Update sent_messages_count is already done in addTimelineEntryToSession
	
	// Set the session's ProjectName based on projects
	if len(session.Projects) == 1 {
		// Single project
		for name := range session.Projects {
			session.ProjectName = name
		}
	} else if len(session.Projects) > 1 {
		// Multiple projects
		session.ProjectName = "Multiple"
		
		// Update window history to mark this as account-level
		if d.windowHistory != nil && session.IsWindowDetected && session.WindowStartTime != nil {
			record := WindowRecord{
				StartTime:      *session.WindowStartTime,
				EndTime:        *session.WindowStartTime + int64(d.sessionDuration.Seconds()),
				Source:         session.WindowSource,
				IsLimitReached: session.WindowSource == "limit_message",
				IsAccountLevel: true, // Multi-project sessions are account-level
				SessionID:      session.ID,
				FirstEntryTime: session.FirstEntryTime,
			}
			d.windowHistory.AddOrUpdateWindow(record)
		}
	}
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
					ProjectName:       "Gap", // Generic name for gap sessions
					Projects:          make(map[string]*ProjectStats), // Empty projects map
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

// deduplicateSessions removes duplicate sessions that cover the same time period
func (d *SessionDetector) deduplicateSessions(sessions []*Session) []*Session {
	if len(sessions) <= 1 {
		return sessions
	}

	util.LogInfo(fmt.Sprintf("deduplicateSessions: Checking %d sessions for duplicates", len(sessions)))

	// Sort sessions by start time for easier comparison
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime < sessions[j].StartTime
	})

	// Keep track of unique sessions by time window (account-level)
	uniqueSessions := make([]*Session, 0, len(sessions))
	sessionMap := make(map[string]*Session)

	for _, session := range sessions {
		// Key is now just based on time window, not project
		key := fmt.Sprintf("%d-%d", session.StartTime, session.EndTime)
		
		if existing, exists := sessionMap[key]; exists {
			// Merge data from duplicate session
			util.LogInfo(fmt.Sprintf("Found duplicate session: ID=%s and ID=%s (key=%s)", existing.ID, session.ID, key))
			
			// Merge projects from duplicate session into existing
			for projectName, projectStats := range session.Projects {
				if existingProject, exists := existing.Projects[projectName]; exists {
					// Merge project data
					existingProject.TotalTokens += projectStats.TotalTokens
					existingProject.TotalCost += projectStats.TotalCost
					existingProject.MessageCount += projectStats.MessageCount
					existingProject.SentMessageCount += projectStats.SentMessageCount
					
					// Update time bounds
					if projectStats.FirstEntryTime < existingProject.FirstEntryTime {
						existingProject.FirstEntryTime = projectStats.FirstEntryTime
					}
					if projectStats.LastEntryTime > existingProject.LastEntryTime {
						existingProject.LastEntryTime = projectStats.LastEntryTime
					}
					
					// Merge model distributions
					for model, stats := range projectStats.ModelDistribution {
						if existingStats, ok := existingProject.ModelDistribution[model]; ok {
							existingStats.Tokens += stats.Tokens
							existingStats.Cost += stats.Cost
							existingStats.Count += stats.Count
						} else {
							existingProject.ModelDistribution[model] = stats
						}
					}
				} else {
					// Add new project
					existing.Projects[projectName] = projectStats
				}
			}
			
			// Update session totals
			existing.TotalTokens += session.TotalTokens
			existing.TotalCost += session.TotalCost
			existing.MessageCount += session.MessageCount
			existing.SentMessageCount += session.SentMessageCount
			
			// Keep the better window detection
			if session.IsWindowDetected && !existing.IsWindowDetected ||
			   (session.WindowSource == "limit_message" && existing.WindowSource != "limit_message") {
				existing.WindowStartTime = session.WindowStartTime
				existing.IsWindowDetected = session.IsWindowDetected
				existing.WindowSource = session.WindowSource
			}
			
			// Update ProjectName for UI
			if len(existing.Projects) > 1 {
				existing.ProjectName = fmt.Sprintf("Multiple (%d projects)", len(existing.Projects))
			}
			
			util.LogInfo(fmt.Sprintf("Merged session %s into %s", session.ID, existing.ID))
		} else {
			// New unique session
			uniqueSessions = append(uniqueSessions, session)
			sessionMap[key] = session
		}
	}

	removedCount := len(sessions) - len(uniqueSessions)
	if removedCount > 0 {
		util.LogInfo(fmt.Sprintf("deduplicateSessions: Removed %d duplicate sessions, %d unique sessions remain", 
			removedCount, len(uniqueSessions)))
	}

	return uniqueSessions
}

// detectActiveSession checks if we should create an active session
func (d *SessionDetector) detectActiveSession(sessions []*Session, nowTimestamp int64) []*Session {
	if len(sessions) == 0 {
		// No historical sessions, create initial active session if within window
		return d.createInitialActiveSession(nowTimestamp)
	}
	
	// Find the most recent session
	var lastSession *Session
	for _, s := range sessions {
		if lastSession == nil || s.EndTime > lastSession.EndTime {
			lastSession = s
		}
	}
	
	// Check if we need a new active session
	if nowTimestamp > lastSession.EndTime {
		// Calculate the next window start
		newStart := lastSession.EndTime
		newEnd := newStart + int64(d.sessionDuration.Seconds())
		
		// Check if we're currently in this window
		if nowTimestamp >= newStart && nowTimestamp < newEnd {
			activeSession := d.createActiveSessionWindow(newStart, newEnd, nowTimestamp)
			if activeSession != nil {
				// Add to beginning of list (most recent first)
				sessions = append([]*Session{activeSession}, sessions...)
				util.LogInfo(fmt.Sprintf("Created active session: %s (%s-%s)",
					activeSession.ID,
					time.Unix(newStart, 0).Format("15:04"),
					time.Unix(newEnd, 0).Format("15:04")))
			}
		}
	}
	
	return sessions
}

// createInitialActiveSession creates the first active session when no history exists
func (d *SessionDetector) createInitialActiveSession(nowTimestamp int64) []*Session {
	// Calculate current 5-hour window
	windowStart := (nowTimestamp / 3600) * 3600 // Round down to hour
	windowEnd := windowStart + int64(d.sessionDuration.Seconds())
	
	// Check if we're within the window
	if nowTimestamp >= windowStart && nowTimestamp < windowEnd {
		activeSession := d.createActiveSessionWindow(windowStart, windowEnd, nowTimestamp)
		if activeSession != nil {
			return []*Session{activeSession}
		}
	}
	
	return []*Session{}
}

// createActiveSessionWindow creates an active session for the given window
func (d *SessionDetector) createActiveSessionWindow(windowStart, windowEnd, nowTimestamp int64) *Session {
	sessionID := fmt.Sprintf("%d", windowStart)
	
	// Create the active session
	activeSession := &Session{
		ID:                sessionID,
		StartTime:         windowStart,
		EndTime:           windowEnd,
		ResetTime:         windowEnd,
		IsActive:          true,
		IsWindowDetected:  true,
		WindowSource:      "active_detection",
		WindowStartTime:   &windowStart,
		FirstEntryTime:    nowTimestamp,
		Projects:          make(map[string]*ProjectStats),
		ModelDistribution: make(map[string]*model.ModelStats),
		PerModelStats:     make(map[string]map[string]interface{}),
		HourlyMetrics:     make([]*model.HourlyMetric, 0),
		LimitMessages:     make([]map[string]interface{}, 0),
		ProjectionData:    make(map[string]interface{}),
	}
	
	// Add to window history if available
	if d.windowHistory != nil {
		record := WindowRecord{
			StartTime:      windowStart,
			EndTime:        windowEnd,
			Source:         "active_detection",
			IsLimitReached: false,
			SessionID:      sessionID,
			IsAccountLevel: true,
		}
		d.windowHistory.AddOrUpdateWindow(record)
	}
	
	// Calculate initial metrics
	d.calculateMetrics(activeSession, nowTimestamp)
	
	return activeSession
}

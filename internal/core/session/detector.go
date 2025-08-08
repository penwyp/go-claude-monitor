package session

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session/internal"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
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

// GetWindowHistory returns the window history manager
func (d *SessionDetector) GetWindowHistory() *WindowHistoryManager {
	return d.windowHistory
}

// SessionDetectionInput contains all data needed for session detection
type SessionDetectionInput struct {
	CachedWindowInfo map[string]*WindowDetectionInfo // Cached window info by sessionId
	GlobalTimeline   []timeline.TimestampedLog                // Global timeline of logs across all projects
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
				d.AddLogToSession(session, tl)
				logsInWindow++
			}
		}
		// Enhanced logging for continuous activity windows
		if window.Source == "continuous_activity" {
			util.LogInfo(fmt.Sprintf("Continuous activity window %s-%s: %d logs assigned, %d messages, %d tokens",
				time.Unix(window.StartTime, 0).Format("2006-01-02 15:04:05"),
				time.Unix(window.EndTime, 0).Format("2006-01-02 15:04:05"),
				logsInWindow, session.MessageCount, session.TotalTokens))
		} else {
			util.LogDebug(fmt.Sprintf("Window %s-%s (source: %s): %d logs, %d messages, %d tokens",
				time.Unix(window.StartTime, 0).Format("15:04:05"),
				time.Unix(window.EndTime, 0).Format("15:04:05"),
				window.Source,
				logsInWindow, session.MessageCount, session.TotalTokens))
		}
		
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
		d.FinalizeSession(session)
		d.CalculateMetrics(session, nowTimestamp)
	}
	
	// Insert gap sessions
	sessions = d.insertGapSessions(sessions)
	
	// Detect and add active session if needed (before deduplication to avoid overlaps)
	sessions = d.detectActiveSession(sessions, nowTimestamp)
	
	// Deduplicate sessions (this will merge any overlapping sessions)
	sessions = d.deduplicateSessions(sessions)
	
	// Mark active sessions
	d.markActiveSessions(sessions, nowTimestamp)
	
	// Sort by start time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime > sessions[j].StartTime
	})
	
	// Validate token counts
	var totalSessionTokens int64
	var timelineTokens int64
	var syntheticCount int
	for _, session := range sessions {
		totalSessionTokens += int64(session.TotalTokens)
	}
	for _, tl := range input.GlobalTimeline {
		usage := tl.Log.Message.Usage
		tokens := int64(internal.CalculateTotalTokens(usage))
		timelineTokens += tokens
		if tl.Log.Type == "synthetic" {
			syntheticCount++
		}
	}
	
	util.LogInfo(fmt.Sprintf("Token validation: Sessions=%d tokens, Timeline=%d tokens (%d entries, %d synthetic)",
		totalSessionTokens, timelineTokens, len(input.GlobalTimeline), syntheticCount))
	
	if totalSessionTokens != timelineTokens && timelineTokens > 0 {
		discrepancy := float64(totalSessionTokens-timelineTokens) / float64(timelineTokens) * 100
		if math.Abs(discrepancy) > 1 { // Only warn if discrepancy is more than 1%
			util.LogWarn(fmt.Sprintf("Token count mismatch: %.1f%% difference (Sessions=%d, Timeline=%d)",
				discrepancy, totalSessionTokens, timelineTokens))
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
		
		// Separate unexpired from expired limits
		unexpiredCount := 0
		currentTime := time.Now().Unix()
		
		for _, limit := range limits {
			if limit.ResetTime != nil {
				windowStart := *limit.ResetTime - int64(d.sessionDuration.Seconds())
				isUnexpired := limit.IsUnexpired()
				
				// Give unexpired limits the highest priority
				priority := 9
				if isUnexpired {
					priority = 10 // Highest priority for unexpired limits
					unexpiredCount++
					util.LogInfo(fmt.Sprintf("Found UNEXPIRED limit message: reset at %s (in %d minutes)",
						time.Unix(*limit.ResetTime, 0).Format("2006-01-02 15:04:05"),
						(*limit.ResetTime-currentTime)/60))
				}
				
				candidates = append(candidates, WindowCandidate{
					StartTime: windowStart,
					EndTime:   *limit.ResetTime,
					Source:    "limit_message",
					Priority:  priority,
					IsLimit:   true,
				})
				
				// Update window history
				if d.windowHistory != nil {
					d.windowHistory.UpdateFromLimitMessage(*limit.ResetTime, limit.Timestamp, limit.Content)
				}
			}
		}
		
		if unexpiredCount > 0 {
			util.LogInfo(fmt.Sprintf("Found %d unexpired limit messages out of %d total", unexpiredCount, len(limits)))
		}
	}
	
	// Priority 3.5: Continuous activity window generation - HIGHEST PRIORITY FOR STRICT 5-HOUR WINDOWS
	// This ensures strict 5-hour window boundaries even for continuous activity
	if len(input.GlobalTimeline) > 0 {
		firstActivity := input.GlobalTimeline[0].Timestamp
		lastActivity := input.GlobalTimeline[len(input.GlobalTimeline)-1].Timestamp
		
		// Start from the first activity's hour boundary
		currentWindowStart := internal.TruncateToHour(firstActivity)
		
		util.LogInfo(fmt.Sprintf("Generating strict 5-hour windows from %s to %s",
			time.Unix(firstActivity, 0).Format("2006-01-02 15:04:05"),
			time.Unix(lastActivity, 0).Format("2006-01-02 15:04:05")))
		
		for currentWindowStart <= lastActivity {
			windowEnd := currentWindowStart + int64(d.sessionDuration.Seconds())
			
			// Check if this window period has any activity
			hasActivity := false
			for _, tl := range input.GlobalTimeline {
				if tl.Timestamp >= currentWindowStart && tl.Timestamp < windowEnd {
					hasActivity = true
					break
				}
			}
			
			if hasActivity {
				candidates = append(candidates, WindowCandidate{
					StartTime: currentWindowStart,
					EndTime:   windowEnd,
					Source:    "continuous_activity",
					Priority:  8, // Higher than gap(5) and first_message(3), lower than limit(9-10)
					IsLimit:   false,
				})
				util.LogDebug(fmt.Sprintf("Added continuous_activity window: %s to %s",
					time.Unix(currentWindowStart, 0).Format("2006-01-02 15:04:05"),
					time.Unix(windowEnd, 0).Format("2006-01-02 15:04:05")))
			}
			
			// Move to next 5-hour window boundary
			currentWindowStart = windowEnd
		}
	}
	
	// Priority 4: Gap-based detection
	if len(input.GlobalTimeline) > 1 {
		for i := 1; i < len(input.GlobalTimeline); i++ {
			gap := input.GlobalTimeline[i].Timestamp - input.GlobalTimeline[i-1].Timestamp
			if gap >= int64(d.sessionDuration.Seconds()) {
				// Gap detected, new window starts at current message
				windowStart := internal.TruncateToHour(input.GlobalTimeline[i].Timestamp)
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
		windowStart := internal.TruncateToHour(firstTimestamp)
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
	
	// Priority 6: Active window detection - add current window if within bounds
	currentTime := time.Now().Unix()
	
	// Check if we should add an active window
	// This happens when current time is not covered by any existing window candidates
	shouldAddActiveWindow := true
	
	// First check if current time is already covered by any candidate
	for _, candidate := range candidates {
		if currentTime >= candidate.StartTime && currentTime < candidate.EndTime {
			shouldAddActiveWindow = false
			util.LogDebug(fmt.Sprintf("Current time already covered by %s window", candidate.Source))
			break
		}
	}
	
	if shouldAddActiveWindow {
		// Try to determine the appropriate window for current time
		// First, check if we have recent windows in history to align with
		var activeWindowStart int64
		var activeWindowEnd int64
		foundAlignment := false
		
		// Look for the most recent window to align with
		if len(candidates) > 0 {
			// Find the most recent window end time
			var mostRecentEnd int64
			for _, candidate := range candidates {
				if candidate.EndTime > mostRecentEnd && candidate.EndTime <= currentTime {
					mostRecentEnd = candidate.EndTime
				}
			}
			
			// If we found a recent window, check if current time would be in the next window
			if mostRecentEnd > 0 {
				nextWindowStart := mostRecentEnd
				nextWindowEnd := nextWindowStart + int64(d.sessionDuration.Seconds())
				
				if currentTime >= nextWindowStart && currentTime < nextWindowEnd {
					activeWindowStart = nextWindowStart
					activeWindowEnd = nextWindowEnd
					foundAlignment = true
					util.LogInfo(fmt.Sprintf("Active window aligned with previous window end: %s to %s",
						time.Unix(activeWindowStart, 0).Format("2006-01-02 15:04:05"),
						time.Unix(activeWindowEnd, 0).Format("2006-01-02 15:04:05")))
				}
			}
		}
		
		// If no alignment found, create a new window starting at current hour
		if !foundAlignment {
			activeWindowStart = internal.TruncateToHour(currentTime)
			activeWindowEnd = activeWindowStart + int64(d.sessionDuration.Seconds())
			
			// Make sure current time is within this window
			if currentTime >= activeWindowStart && currentTime < activeWindowEnd {
				foundAlignment = true
				util.LogInfo(fmt.Sprintf("Active window created at hour boundary: %s to %s",
					time.Unix(activeWindowStart, 0).Format("2006-01-02 15:04:05"),
					time.Unix(activeWindowEnd, 0).Format("2006-01-02 15:04:05")))
			}
		}
		
		// Add the active window candidate if we found a valid window
		if foundAlignment {
			candidates = append(candidates, WindowCandidate{
				StartTime: activeWindowStart,
				EndTime:   activeWindowEnd,
				Source:    "active_window",
				Priority:  6, // Higher than gap(5) and first_message(3), but lower than continuous_activity(8)
				IsLimit:   false,
			})
			util.LogInfo(fmt.Sprintf("Added active_window candidate for current time"))
		}
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
	
	currentTime := time.Now().Unix()
	
	// Phase 1: Separate unexpired limit messages from other candidates
	var unexpiredLimits []WindowCandidate
	var otherCandidates []WindowCandidate
	
	for _, candidate := range candidates {
		// Check if this is an unexpired limit message (priority 9-10 and reset time in future)
		if candidate.IsLimit && candidate.Source == "limit_message" && 
		   candidate.EndTime > currentTime {
			unexpiredLimits = append(unexpiredLimits, candidate)
			util.LogInfo(fmt.Sprintf("Found unexpired limit window: %s-%s (will force override)",
				time.Unix(candidate.StartTime, 0).Format("2006-01-02 15:04:05"),
				time.Unix(candidate.EndTime, 0).Format("2006-01-02 15:04:05")))
		} else {
			otherCandidates = append(otherCandidates, candidate)
		}
	}
	
	// Sort unexpired limits by priority (descending) then by start time (ascending)
	sort.Slice(unexpiredLimits, func(i, j int) bool {
		if unexpiredLimits[i].Priority != unexpiredLimits[j].Priority {
			return unexpiredLimits[i].Priority > unexpiredLimits[j].Priority
		}
		return unexpiredLimits[i].StartTime < unexpiredLimits[j].StartTime
	})
	
	// Sort other candidates by priority (descending) then by start time (ascending)
	sort.Slice(otherCandidates, func(i, j int) bool {
		if otherCandidates[i].Priority != otherCandidates[j].Priority {
			return otherCandidates[i].Priority > otherCandidates[j].Priority
		}
		return otherCandidates[i].StartTime < otherCandidates[j].StartTime
	})
	
	selected := make([]WindowCandidate, 0)
	
	// Phase 2: Add all unexpired limit windows first (they have absolute priority)
	for _, limitCandidate := range unexpiredLimits {
		// Ensure window is exactly 5 hours
		if limitCandidate.EndTime-limitCandidate.StartTime != int64(d.sessionDuration.Seconds()) {
			limitCandidate.EndTime = limitCandidate.StartTime + int64(d.sessionDuration.Seconds())
		}
		
		// Remove any existing windows that conflict with this limit window
		newSelected := make([]WindowCandidate, 0)
		for _, existing := range selected {
			// Check for overlap
			if limitCandidate.StartTime < existing.EndTime && limitCandidate.EndTime > existing.StartTime {
				util.LogInfo(fmt.Sprintf("Removing conflicting window %s-%s (source: %s) in favor of limit window",
					time.Unix(existing.StartTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(existing.EndTime, 0).Format("2006-01-02 15:04:05"),
					existing.Source))
			} else {
				newSelected = append(newSelected, existing)
			}
		}
		selected = newSelected
		
		// Add the limit window (it has absolute priority)
		selected = append(selected, limitCandidate)
		util.LogInfo(fmt.Sprintf("Added unexpired limit window: %s-%s (forced override)",
			time.Unix(limitCandidate.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(limitCandidate.EndTime, 0).Format("2006-01-02 15:04:05")))
	}
	
	// Phase 3: Process other candidates (only add if they don't conflict with limit windows)
	util.LogDebug(fmt.Sprintf("Phase 3: Processing %d other candidates", len(otherCandidates)))
	for i, candidate := range otherCandidates {
		util.LogDebug(fmt.Sprintf("Processing candidate %d/%d: %s (source: %s, priority: %d)",
			i+1, len(otherCandidates),
			time.Unix(candidate.StartTime, 0).Format("2006-01-02 15:04:05"),
			candidate.Source, candidate.Priority))
		
		// Ensure window is exactly 5 hours
		if candidate.EndTime-candidate.StartTime != int64(d.sessionDuration.Seconds()) {
			candidate.EndTime = candidate.StartTime + int64(d.sessionDuration.Seconds())
		}
		
		// Check for overlap with already selected windows
		overlaps := false
		for _, sel := range selected {
			// Special handling for continuous_activity windows
			if candidate.Source == "continuous_activity" || sel.Source == "continuous_activity" {
				// Strict overlap check - boundaries touching is OK
				if candidate.StartTime < sel.EndTime && candidate.EndTime > sel.StartTime {
					// Check if it's just boundary touching
					if candidate.StartTime == sel.EndTime || candidate.EndTime == sel.StartTime {
						// Boundary touching is allowed for continuous_activity windows
						continue
					}
					overlaps = true
					break
				}
			} else {
				// Original logic for other window types
				if candidate.StartTime < sel.EndTime && candidate.EndTime > sel.StartTime {
					overlaps = true
					break
				}
			}
		}
		
		if !overlaps {
			// Validate with window history if available
			if d.windowHistory != nil {
				// Special handling for continuous_activity windows - they should always be valid
				if candidate.Source == "continuous_activity" {
					// Continuous activity windows are strictly enforced 5-hour boundaries
					util.LogInfo(fmt.Sprintf("Accepting continuous_activity window: %s-%s",
						time.Unix(candidate.StartTime, 0).Format("2006-01-02 15:04:05"),
						time.Unix(candidate.EndTime, 0).Format("2006-01-02 15:04:05")))
					selected = append(selected, candidate)
				} else {
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
	
	util.LogInfo(fmt.Sprintf("Selected %d windows from %d candidates (%d unexpired limits forced)",
		len(selected), len(candidates), len(unexpiredLimits)))
	for _, w := range selected {
		util.LogDebug(fmt.Sprintf("  Window: %s-%s (source: %s, priority: %d, isLimit: %v)",
			time.Unix(w.StartTime, 0).Format("15:04:05"),
			time.Unix(w.EndTime, 0).Format("15:04:05"),
			w.Source, w.Priority, w.IsLimit))
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
		StartHour:         internal.TruncateToHour(window.StartTime),
		EndTime:           window.EndTime,
		IsActive:          false,
		IsGap:             window.Source == "gap",
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
// Removed createNewSessionWithWindow - replaced by createSessionForWindow

// addLogToSession adds a timestamped log to a session and updates its statistics
// AddLogToSession adds a log entry to a session
func (d *SessionDetector) AddLogToSession(session *Session, tl timeline.TimestampedLog) {
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
	totalTokens := internal.CalculateTotalTokens(usage)
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
		internal.UpdateModelStats(modelStats, totalTokens, 0) // Cost will be added separately
		
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
			modelStats.Cost += cost // Update cost separately after calculation
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
		internal.UpdateModelStats(sessionModelStats, totalTokens, cost)
	}
}





// CalculateMetrics calculates metrics for a session
func (d *SessionDetector) CalculateMetrics(session *Session, nowTimestamp int64) {
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
// FinalizeSession finalizes session data after all logs have been added
func (d *SessionDetector) FinalizeSession(session *Session) {
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
					StartHour:         internal.TruncateToHour(*prevSession.ActualEndTime),
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
	// Since active windows are now handled in collectWindowCandidates,
	// this method only needs to verify that active sessions are properly marked.
	// We no longer create empty placeholder sessions here.
	
	// Check if current time is already covered by an existing session
	for _, s := range sessions {
		if !s.IsGap && nowTimestamp >= s.StartTime && nowTimestamp < s.EndTime {
			// Current time is already covered by an existing session
			util.LogDebug(fmt.Sprintf("Current time is covered by session %s (%s-%s)",
				s.ID,
				time.Unix(s.StartTime, 0).Format("15:04"),
				time.Unix(s.EndTime, 0).Format("15:04")))
			// Mark this session as potentially active (will be confirmed in markActiveSessions)
			return sessions
		}
	}
	
	// If no session covers current time, that's fine - it means no active window was detected
	// or there's genuinely no activity in the current window period
	util.LogDebug("No session covers current time - no active window detected or no activity in current period")
	
	return sessions
}

// createInitialActiveSession - deprecated, no longer creates sessions
// Active windows are now handled in collectWindowCandidates
func (d *SessionDetector) createInitialActiveSession(nowTimestamp int64) []*Session {
	// No longer creates sessions - active windows are handled in collectWindowCandidates
	return []*Session{}
}

// createActiveSessionWindow - deprecated, no longer creates sessions
// Active windows are now handled in collectWindowCandidates
func (d *SessionDetector) createActiveSessionWindow(windowStart, windowEnd, nowTimestamp int64) *Session {
	// No longer creates sessions - active windows are handled in collectWindowCandidates
	return nil
}

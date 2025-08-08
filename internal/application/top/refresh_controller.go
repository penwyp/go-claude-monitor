package top

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// RefreshController manages data refresh operations and session detection
type RefreshController struct {
	dataLoader    *DataLoader
	detector      *session.SessionDetector
	calculator    *session.MetricsCalculator
	stateManager  *StateManager
	sessionConfig session.SessionConfig
	
	mu           sync.RWMutex
	refreshMutex sync.Mutex // Prevent concurrent refreshes
}

// NewRefreshController creates a new RefreshController instance
func NewRefreshController(dataLoader *DataLoader, detector *session.SessionDetector, calculator *session.MetricsCalculator, stateManager *StateManager) *RefreshController {
	return &RefreshController{
		dataLoader:    dataLoader,
		detector:      detector,
		calculator:    calculator,
		stateManager:  stateManager,
		sessionConfig: session.GetSessionConfig(),
	}
}

// RefreshData performs atomic data refresh with double buffering
func (rc *RefreshController) RefreshData() ([]*session.Session, error) {
	// Acquire refresh mutex to prevent concurrent refreshes
	rc.refreshMutex.Lock()
	defer rc.refreshMutex.Unlock()

	util.LogDebug("RefreshData: Starting refresh operation")

	// Rescan recent files and update
	files, err := rc.dataLoader.ScanRecentFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to scan recent files: %w", err)
	}
	
	util.LogDebug(fmt.Sprintf("RefreshData: Found %d files to process", len(files)))

	// Track which files are new or changed
	changedFiles := rc.dataLoader.IdentifyChangedFiles(files)
	util.LogDebug(fmt.Sprintf("RefreshData: %d files have changed", len(changedFiles)))
	
	// Load files
	if err := rc.dataLoader.LoadFiles(files); err != nil {
		return nil, fmt.Errorf("failed to load files: %w", err)
	}
	
	// Prepare new sessions (double buffering)
	var newSessions []*session.Session
	if rc.sessionConfig.EnableIncrementalDetection && len(changedFiles) > 0 {
		util.LogInfo(fmt.Sprintf("RefreshData: Using incremental detection for %d changed files", len(changedFiles)))
		newSessions, err = rc.IncrementalDetect(changedFiles)
	} else {
		util.LogInfo("RefreshData: Using full detection")
		newSessions, err = rc.FullDetect()
	}
	
	if err != nil {
		util.LogError(fmt.Sprintf("RefreshData: Detection failed: %v", err))
		return nil, err
	}
	
	// Validate results
	if len(newSessions) == 0 {
		util.LogWarn("RefreshData: Detection returned no sessions")
		// Get current sessions for comparison
		currentSessions := rc.stateManager.GetCurrentSessions()
		if len(currentSessions) > 0 {
			util.LogWarn(fmt.Sprintf("RefreshData: Had %d sessions before refresh, returning empty", len(currentSessions)))
		}
	} else {
		util.LogInfo(fmt.Sprintf("RefreshData: Successfully detected %d sessions", len(newSessions)))
	}
	
	return newSessions, nil
}

// IncrementalDetect performs incremental session detection for changed files
func (rc *RefreshController) IncrementalDetect(changedFiles []string) ([]*session.Session, error) {
	if len(changedFiles) == 0 {
		// No changes, use full detection
		return rc.FullDetect()
	}

	// Get current sessions from state manager
	currentSessions := rc.stateManager.GetCurrentSessions()
	
	// Get current memory cache
	memCache := rc.dataLoader.GetMemoryCache()
	
	// Get time range of changed files
	minTime := int64(^uint64(0) >> 1) // Max int64
	maxTime := int64(0)
	
	for _, file := range changedFiles {
		logs := memCache.GetLogsForFile(file)
		for _, log := range logs {
			ts, err := time.Parse(time.RFC3339, log.Timestamp)
			if err != nil {
				continue
			}
			timestamp := ts.Unix()
			if timestamp < minTime {
				minTime = timestamp
			}
			if timestamp > maxTime {
				maxTime = timestamp
			}
		}
	}

	// Check if we have existing windows that cover this time range
	existingWindows := make(map[string]*session.Session)
	for _, sess := range currentSessions {
		if sess.StartTime <= maxTime && sess.EndTime >= minTime {
			existingWindows[sess.ID] = sess
		}
	}

	if len(existingWindows) > 0 && rc.detector.GetWindowHistory() != nil {
		// We have existing windows, just update the statistics
		util.LogInfo(fmt.Sprintf("Incremental update for %d existing windows", len(existingWindows)))
		
		// Get updated global timeline
		globalTimeline := rc.dataLoader.GetGlobalTimeline(6 * 3600)
		
		// Prepare new session list
		newSessions := make([]*session.Session, 0, len(currentSessions))
		
		// Recalculate only affected sessions
		for _, oldSession := range currentSessions {
			if _, isAffected := existingWindows[oldSession.ID]; !isAffected {
				// Session not affected, keep as is
				newSessions = append(newSessions, oldSession)
				continue
			}
			
			// Create new session with same window
			newSession := &session.Session{
				ID:                oldSession.ID,
				StartTime:         oldSession.StartTime,
				EndTime:           oldSession.EndTime,
				Projects:          make(map[string]*session.ProjectStats),
				ModelDistribution: make(map[string]*model.ModelStats),
				PerModelStats:     make(map[string]map[string]interface{}),
				HourlyMetrics:     make([]*model.HourlyMetric, 0),
				LimitMessages:     make([]map[string]interface{}, 0),
				ProjectionData:    make(map[string]interface{}),
				WindowStartTime:   oldSession.WindowStartTime,
				IsWindowDetected:  oldSession.IsWindowDetected,
				WindowSource:      oldSession.WindowSource,
				ResetTime:         oldSession.ResetTime,
			}
			
			// Add logs that belong to this window
			for _, tl := range globalTimeline {
				if tl.Timestamp >= newSession.StartTime && tl.Timestamp < newSession.EndTime {
					rc.detector.AddLogToSession(newSession, tl)
				}
			}
			
			// Finalize and calculate metrics
			rc.detector.FinalizeSession(newSession)
			rc.detector.CalculateMetrics(newSession, time.Now().Unix())
			rc.calculator.Calculate(newSession)
			
			newSessions = append(newSessions, newSession)
		}
		
		return newSessions, nil
	} else {
		// No existing windows or window history, do full detection
		util.LogInfo("No existing windows found, performing full detection")
		return rc.FullDetect()
	}
}

// FullDetect performs full session detection
func (rc *RefreshController) FullDetect() ([]*session.Session, error) {
	memCache := rc.dataLoader.GetMemoryCache()
	
	// First, load historical limit windows from the past 1 day
	if rc.detector.GetWindowHistory() != nil {
		historicalLogs := memCache.GetHistoricalLogs(constants.HistoricalScanSeconds)
		if len(historicalLogs) > 0 {
			addedCount := rc.detector.GetWindowHistory().LoadHistoricalLimitWindows(historicalLogs)
			if addedCount > 0 {
				util.LogInfo(fmt.Sprintf("Loaded %d historical limit windows from past %d day", addedCount, constants.HistoricalScanDays))
			}
		}
	}

	// Get global timeline of ALL logs across all projects
	globalTimeline := rc.dataLoader.GetGlobalTimeline(0) // 0 means no time limit
	util.LogInfo(fmt.Sprintf("Got global timeline with %d entries", len(globalTimeline)))
	
	// Get cached window info
	cachedWindowInfo := rc.dataLoader.GetCachedWindowInfo()

	// Use global timeline for session detection
	input := session.SessionDetectionInput{
		GlobalTimeline:   globalTimeline,
		CachedWindowInfo: cachedWindowInfo,
	}
	newSessions := rc.detector.DetectSessionsWithLimits(input)

	// Calculate metrics for each session and store window info
	currentTime := time.Now().Unix()
	maxFutureTime := currentTime + constants.MaxFutureWindowSeconds
	
	for _, sess := range newSessions {
		rc.calculator.Calculate(sess)

		// Store window detection info back to cache if detected
		if sess.IsWindowDetected && sess.WindowStartTime != nil {
			// Check if window end time is not too far in the future
			windowEndTime := *sess.WindowStartTime + constants.SessionDurationSeconds
			if windowEndTime <= maxFutureTime {
				windowInfo := &session.WindowDetectionInfo{
					WindowStartTime:  sess.WindowStartTime,
					IsWindowDetected: sess.IsWindowDetected,
					WindowSource:     sess.WindowSource,
					DetectedAt:       currentTime,
					FirstEntryTime:   sess.FirstEntryTime,
				}
				rc.dataLoader.UpdateWindowInfo(sess.ID, windowInfo)
			} else {
				util.LogWarn(fmt.Sprintf("Skipping cache for future window: %s (ends at %s, max allowed %s)",
					sess.ID,
					time.Unix(windowEndTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(maxFutureTime, 0).Format("2006-01-02 15:04:05")))
			}
		}
	}

	// Log session details
	rc.logSessionDetails(newSessions)

	return newSessions, nil
}

// logSessionDetails logs detailed information about detected sessions
func (rc *RefreshController) logSessionDetails(sessions []*session.Session) {
	for i, sess := range sessions {
		projectNames := make([]string, 0, len(sess.Projects))
		for name := range sess.Projects {
			projectNames = append(projectNames, name)
		}
		projectsStr := "no projects"
		if len(projectNames) > 0 {
			projectsStr = fmt.Sprintf("%v", projectNames)
		}

		util.LogInfo(fmt.Sprintf("Session %d: ID=%s, Window=%s (%s), ResetTime=%s, Tokens=%d, Cost=%.2f, Projects=%s",
			i+1, sess.ID,
			time.Unix(sess.StartTime, 0).Format("15:04"),
			sess.WindowSource,
			time.Unix(sess.ResetTime, 0).Format("15:04"),
			sess.TotalTokens,
			sess.TotalCost,
			projectsStr))
		
		util.LogDebug(fmt.Sprintf("Session %s details - StartTime: %s, EndTime: %s, ResetTime: %s, IsActive: %v, IsWindowDetected: %v, Projects: %d",
			sess.ID,
			time.Unix(sess.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(sess.EndTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(sess.ResetTime, 0).Format("2006-01-02 15:04:05"),
			sess.IsActive,
			sess.IsWindowDetected,
			len(sess.Projects)))
	}

	util.LogInfo(fmt.Sprintf("Detected %d active sessions", len(sessions)))
}
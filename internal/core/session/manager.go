package session

import (
	"context"
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/data/cache"
	"github.com/penwyp/go-claude-monitor/internal/data/parser"
	"github.com/penwyp/go-claude-monitor/internal/data/scanner"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

type TopConfig struct {
	DataDir             string
	CacheDir            string
	Plan                string
	CustomLimitTokens   int
	Timezone            string
	TimeFormat          string
	DataRefreshInterval time.Duration
	UIRefreshRate       float64
	Concurrency         int
	// Pricing configuration
	PricingSource      string // default, litellm
	PricingOfflineMode bool   // Enable offline pricing mode
}

// extractSessionId extracts the session ID from a file path
// For example: "/path/to/00aec530-0614-436f-a53b-faaa0b32f123.jsonl" -> "00aec530-0614-436f-a53b-faaa0b32f123"
func extractSessionId(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

type Manager struct {
	config         *TopConfig
	sessionConfig  SessionConfig          // Session detection configuration
	fileCache      cache.Cache            // Reuse existing cache.Cache
	memoryCache    *MemoryCache           // Memory cache layer
	scanner        *scanner.FileScanner   // Reuse existing scanner
	parser         *parser.Parser         // Reuse existing parser
	aggregator     *aggregator.Aggregator // Reuse existing aggregator
	detector       *SessionDetector       // Session detector
	calculator     *MetricsCalculator     // Metrics calculator
	display        *TerminalDisplay       // Display component
	watcher        *FileWatcher           // File watcher
	planLimits     pricing.Plan           // Plan limits
	keyboard       *KeyboardReader        // Keyboard input
	sorter         *SessionSorter         // Session sorter

	mu                sync.RWMutex
	activeSessions    []*Session
	previousSessions  []*Session              // Keep previous valid sessions during refresh
	isLoading         bool                   // Loading state flag
	loadingMessage    string                 // Loading status message
	lastDataUpdate    int64                  // Timestamp of last successful data update
	refreshMutex      sync.Mutex             // Prevent concurrent refreshes
	lastCacheSave     int64
	state             model.InteractionState // Interaction state
}

func NewManager(config *TopConfig) *Manager {
	// Initialize file cache (reuse existing implementation)
	fileCache, _ := cache.NewFileCache(config.CacheDir)

	// Determine plan limits
	planLimits := pricing.GetPlanWithDefault(config.Plan, config.CustomLimitTokens)

	// Create aggregator with pricing configuration
	agg, err := aggregator.NewAggregatorWithConfig(
		config.PricingSource,
		config.PricingOfflineMode,
		config.CacheDir,
		config.Timezone,
	)
	if err != nil {
		util.LogError("Failed to create aggregator with pricing config: " + err.Error())
		// Fallback to default aggregator
		agg = aggregator.NewAggregatorWithTimezone(config.Timezone)
	}

	// Get session configuration
	sessionConfig := GetSessionConfig()
	
	return &Manager{
		config:        config,
		sessionConfig: sessionConfig,
		fileCache:     fileCache,
		memoryCache:   NewMemoryCache(),
		scanner:       scanner.NewFileScanner(config.DataDir),
		parser:        parser.NewParser(config.Concurrency),
		aggregator:    agg,
		detector:      NewSessionDetectorWithAggregator(agg, config.Timezone, config.CacheDir), // Create detector with aggregator instance and cache dir
		calculator:    NewMetricsCalculator(planLimits),
		display:       NewTerminalDisplay(config),
		planLimits:    planLimits,
		sorter:        NewSessionSorter(),
		state:         model.InteractionState{},
	}
}

// LoadAndAnalyzeData performs the core session detection workflow without UI
// This method is used by both the top command and the detect debug command
func (m *Manager) LoadAndAnalyzeData() ([]*Session, error) {
	// Initialize global time provider with configured timezone
	if err := util.InitializeTimeProvider(m.config.Timezone); err != nil {
		return nil, fmt.Errorf("failed to initialize timezone: %w", err)
	}

	// Preload data
	if err := m.preload(); err != nil {
		return nil, fmt.Errorf("preload failed: %w", err)
	}

	// Return detected sessions
	m.mu.RLock()
	sessions := make([]*Session, len(m.activeSessions))
	copy(sessions, m.activeSessions)
	m.mu.RUnlock()

	return sessions, nil
}

// GetAggregatedMetrics calculates aggregated metrics from sessions
func (m *Manager) GetAggregatedMetrics(sessions []*Session) *model.AggregatedMetrics {
	return m.display.calculateAggregatedMetrics(sessions)
}

func (m *Manager) Run(ctx context.Context) error {
	util.LogInfo("Starting Claude Monitor Top...")

	// Ensure cleanup on exit
	defer m.Close()

	// Initialize global time provider with configured timezone
	if err := util.InitializeTimeProvider(m.config.Timezone); err != nil {
		return fmt.Errorf("failed to initialize timezone: %w", err)
	}

	// Enter alternate screen mode early to show loading state
	// Phase 1: Initialize keyboard for default framework
	keyboard, err := NewKeyboardReader()
	if err != nil {
		return fmt.Errorf("failed to initialize keyboard: %w", err)
	}
	m.keyboard = keyboard
	defer m.keyboard.Close()

	// Enter alternate screen mode
	m.display.EnterAlternateScreen()
	defer m.display.ExitAlternateScreen()

	// Set initial loading state
	m.mu.Lock()
	m.isLoading = true
	m.loadingMessage = "Initializing and loading data..."
	m.mu.Unlock()

	// Show initial loading screen
	m.updateDisplay()

	// Phase 2: Preload data synchronously (reuse cache.Preload)
	if err := m.preload(); err != nil {
		return fmt.Errorf("preload failed: %w", err)
	}

	// Mark data loading as complete
	m.mu.Lock()
	m.isLoading = false
	m.loadingMessage = ""
	m.lastDataUpdate = time.Now().Unix()
	m.mu.Unlock()

	// Phase 3: Start file monitoring
	if err := m.startWatcher(ctx); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Phase 4: Main loop for default framework
	uiTicker := time.NewTicker(time.Duration(1000/m.config.UIRefreshRate) * time.Millisecond)
	defer uiTicker.Stop()

	dataTicker := time.NewTicker(m.config.DataRefreshInterval)
	defer dataTicker.Stop()

	cacheTicker := time.NewTicker(1 * time.Minute)
	defer cacheTicker.Stop()

	// Initial display with loaded data
	m.updateDisplay()

	for {
		select {
		case <-ctx.Done():
			util.LogInfo("Shutting down Claude Monitor Top...")
			return nil

		case <-uiTicker.C:
			// UI refresh (skip if paused)
			if !m.state.IsPaused {
				m.updateDisplay()
			}

		case <-dataTicker.C:
			// Data refresh (skip if paused)
			if !m.state.IsPaused || m.state.ForceRefresh {
				m.refreshDataAtomic()
				m.state.ForceRefresh = false
			}

		case <-cacheTicker.C:
			// Persist cache
			m.persistCache()

		case event := <-m.watcher.Events():
			// Handle file changes (skip if paused)
			if !m.state.IsPaused {
				m.handleFileChange(event)
			}

		case keyEvent := <-m.keyboard.Events():
			// Handle keyboard input
			if m.handleKeyboard(keyEvent) {
				return nil // Exit requested
			}
			m.updateDisplay() // Update display after keyboard action
		}
	}
}

func (m *Manager) preload() error {
	util.LogInfo("Preloading cache and recent data...")

	// 1. Preload file cache to memory (reuse existing Preload)
	if err := m.fileCache.Preload(); err != nil {
		util.LogWarn(fmt.Sprintf("Cache preload warning: %v", err))
	}

	// 2. Scan recent files
	files, err := m.scanRecentFiles()
	if err != nil {
		return err
	}

	util.LogInfo(fmt.Sprintf("Found %d files to process", len(files)))

	// 3. Load data in parallel (reuse parser's concurrency mechanism)
	m.loadFiles(files)

	// 4. Detect initial sessions
	m.detectSessions()

	return nil
}

func (m *Manager) scanRecentFiles() ([]string, error) {
	// Get all files
	allFiles, err := m.scanner.Scan()
	if err != nil {
		return nil, err
	}

	// Apply configuration-based filtering
	switch m.sessionConfig.TimelineMode {
	case "full":
		// Return ALL files without time filtering
		return allFiles, nil
	case "recent":
		// Legacy behavior: filter by recent modification time
		cutoff := time.Now().Add(-48 * time.Hour).Unix()
		var recentFiles []string
		for _, file := range allFiles {
			info, err := util.GetFileInfo(file)
			if err != nil {
				continue
			}
			if info.ModTime > cutoff {
				recentFiles = append(recentFiles, file)
			}
		}
		return recentFiles, nil
	case "optimized":
		// Balanced approach: load files based on retention config
		if m.sessionConfig.DataRetentionHours > 0 {
			cutoff := time.Now().Add(-time.Duration(m.sessionConfig.DataRetentionHours) * time.Hour).Unix()
			var filteredFiles []string
			for _, file := range allFiles {
				info, err := util.GetFileInfo(file)
				if err != nil {
					continue
				}
				if info.ModTime > cutoff {
					filteredFiles = append(filteredFiles, file)
				}
			}
			return filteredFiles, nil
		}
		return allFiles, nil
	default:
		// Default to full mode
		return allFiles, nil
	}
}

func (m *Manager) loadFiles(files []string) {
	if len(files) == 0 {
		return
	}

	// Batch validate cache (reuse cache.BatchValidate)
	sessionIdMap := make(map[string]string)
	sessionIds := make([]string, 0, len(files))

	for _, file := range files {
		sessionId := extractSessionId(file)
		sessionIdMap[file] = sessionId
		sessionIds = append(sessionIds, sessionId)
	}

	validCache := m.fileCache.BatchValidate(sessionIds)

	// Separate files to parse and cache hits
	var filesToParse []string

	for _, file := range files {
		sessionId := sessionIdMap[file]
		validateResult := validCache[sessionId]
		if validateResult.Valid {
			// Load from cache
			if result := m.fileCache.Get(sessionId); result.Found && result.Data != nil {
				m.memoryCache.Set(sessionId, &MemoryCacheEntry{
					AggregatedData: result.Data,
					LastAccessed:   time.Now().Unix(),
					RawLogs:        nil, // Raw logs not stored in file cache currently
				})
			}
		} else {
			filesToParse = append(filesToParse, file)
		}
	}

	// Parse files that need processing
	if len(filesToParse) > 0 {
		util.LogInfo(fmt.Sprintf("Parsing %d files...", len(filesToParse)))
		m.parseAndCacheFiles(filesToParse, sessionIdMap)
	}
}

func (m *Manager) parseAndCacheFiles(files []string, sessionIdMap map[string]string) {
	parseResults := m.parser.ParseFiles(files)

	for result := range parseResults {
		if result.Error != nil {
			util.LogWarn(fmt.Sprintf("Failed to parse %s: %v", result.File, result.Error))
			continue
		}

		// Use all logs without filtering
		recentLogs := m.filterRecentLogs(result.Logs)
		if len(recentLogs) == 0 {
			continue
		}

		// Aggregate data
		projectName := aggregator.ExtractProjectName(result.File)
		hourlyData := m.aggregator.AggregateByHourAndModel(recentLogs, projectName)

		// Extract limit messages
		limitParser := NewLimitParser()
		limitInfos := limitParser.ParseLogs(recentLogs)
		cachedLimits := make([]aggregator.CachedLimitInfo, 0, len(limitInfos))
		for _, limit := range limitInfos {
			cachedLimits = append(cachedLimits, aggregator.CachedLimitInfo{
				Type:      limit.Type,
				Timestamp: limit.Timestamp,
				ResetTime: limit.ResetTime,
				Content:   limit.Content,
				Model:     limit.Model,
			})
		}

		// Create aggregated data
		sessionId := sessionIdMap[result.File]
		aggregatedData := &aggregator.AggregatedData{
			FileHash:      sessionId, // Using sessionId for now, will rename field later
			FilePath:      result.File,
			ProjectName:   projectName,
			HourlyStats:   hourlyData,
			SessionId:     sessionId,
			LimitMessages: cachedLimits,
		}

		// Save to cache
		if err := m.fileCache.Set(sessionId, aggregatedData); err != nil {
			util.LogWarn(fmt.Sprintf("Failed to cache %s: %v", result.File, err))
		}

		// Update memory cache with raw logs
		m.memoryCache.Set(sessionId, &MemoryCacheEntry{
			AggregatedData: aggregatedData,
			LastAccessed:   time.Now().Unix(),
			RawLogs:        recentLogs,
		})
	}
}

func (m *Manager) filterRecentLogs(logs []model.ConversationLog) []model.ConversationLog {
	// Apply configuration-based filtering
	if m.sessionConfig.DataRetentionHours <= 0 {
		// No filtering - return all logs
		return logs
	}
	
	// Filter based on retention configuration
	cutoff := time.Now().Add(-time.Duration(m.sessionConfig.DataRetentionHours) * time.Hour).Unix()
	var filtered []model.ConversationLog
	
	for _, log := range logs {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}
		
		if ts.Unix() > cutoff {
			filtered = append(filtered, log)
		}
	}
	
	return filtered
}

// incrementalDetectSessionsAtomic performs incremental session detection for changed files without modifying state
func (m *Manager) incrementalDetectSessionsAtomic(changedFiles []string) []*Session {
	if len(changedFiles) == 0 {
		// No changes, use full detection
		return m.fullDetectSessionsAtomic()
	}

	// Get current sessions (safely)
	m.mu.RLock()
	currentSessions := make([]*Session, len(m.activeSessions))
	copy(currentSessions, m.activeSessions)
	m.mu.RUnlock()

	// Get time range of changed files
	minTime := int64(^uint64(0) >> 1) // Max int64
	maxTime := int64(0)
	
	for _, file := range changedFiles {
		logs := m.memoryCache.GetLogsForFile(file)
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
	existingWindows := make(map[string]*Session)
	for _, session := range currentSessions {
		if session.StartTime <= maxTime && session.EndTime >= minTime {
			existingWindows[session.ID] = session
		}
	}

	if len(existingWindows) > 0 && m.detector.windowHistory != nil {
		// We have existing windows, just update the statistics
		util.LogInfo(fmt.Sprintf("Incremental update for %d existing windows", len(existingWindows)))
		
		// Get updated global timeline
		globalTimeline := m.memoryCache.GetGlobalTimeline(6 * 3600)
		
		// Prepare new session list
		newSessions := make([]*Session, 0, len(currentSessions))
		
		// Recalculate only affected sessions
		for _, oldSession := range currentSessions {
			if _, isAffected := existingWindows[oldSession.ID]; !isAffected {
				// Session not affected, keep as is
				newSessions = append(newSessions, oldSession)
				continue
			}
			
			// Create new session with same window
			newSession := &Session{
				ID:                oldSession.ID,
				StartTime:         oldSession.StartTime,
				EndTime:           oldSession.EndTime,
				Projects:          make(map[string]*ProjectStats),
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
					m.detector.addLogToSession(newSession, tl)
				}
			}
			
			// Finalize and calculate metrics
			m.detector.finalizeSession(newSession)
			m.detector.calculateMetrics(newSession, time.Now().Unix())
			m.calculator.Calculate(newSession)
			
			newSessions = append(newSessions, newSession)
		}
		
		return newSessions
	} else {
		// No existing windows or window history, do full detection
		util.LogInfo("No existing windows found, performing full detection")
		return m.fullDetectSessionsAtomic()
	}
}

// incrementalDetectSessions performs incremental session detection for changed files
func (m *Manager) incrementalDetectSessions(changedFiles []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.activeSessions = m.incrementalDetectSessionsAtomic(changedFiles)
}

// fullDetectSessionsAtomic performs full session detection and returns new sessions without modifying state
func (m *Manager) fullDetectSessionsAtomic() []*Session {
	// First, load historical limit windows from the past 1 day
	// This ensures we have the most accurate windows from limit messages
	if m.detector.windowHistory != nil {
		historicalLogs := m.memoryCache.GetHistoricalLogs(constants.HistoricalScanSeconds) // Historical scan period
		if len(historicalLogs) > 0 {
			addedCount := m.detector.windowHistory.LoadHistoricalLimitWindows(historicalLogs)
			if addedCount > 0 {
				util.LogInfo(fmt.Sprintf("Loaded %d historical limit windows from past %d day", addedCount, constants.HistoricalScanDays))
			}
		}
	}

	// Get global timeline of ALL logs across all projects
	// Use 0 to get all data without time restrictions
	globalTimeline := m.memoryCache.GetGlobalTimeline(0) // 0 means no time limit
	util.LogInfo(fmt.Sprintf("Got global timeline with %d entries", len(globalTimeline)))
	
	// Get cached window info
	cachedWindowInfo := m.memoryCache.GetCachedWindowInfo()

	// Use global timeline for session detection
	input := SessionDetectionInput{
		GlobalTimeline:   globalTimeline,
		CachedWindowInfo: cachedWindowInfo,
	}
	newSessions := m.detector.DetectSessionsWithLimits(input)

	// Calculate metrics for each session and store window info
	for _, session := range newSessions {
		m.calculator.Calculate(session)

		// Store window detection info back to cache if detected
		// Only cache windows that are not too far in the future (max session duration ahead)
		currentTime := time.Now().Unix()
		maxFutureTime := currentTime + constants.MaxFutureWindowSeconds

		if session.IsWindowDetected && session.WindowStartTime != nil {
			// Check if window end time is not too far in the future
			windowEndTime := *session.WindowStartTime + constants.SessionDurationSeconds
			if windowEndTime <= maxFutureTime {
				windowInfo := &WindowDetectionInfo{
					WindowStartTime:  session.WindowStartTime,
					IsWindowDetected: session.IsWindowDetected,
					WindowSource:     session.WindowSource,
					DetectedAt:       currentTime,
					FirstEntryTime:   session.FirstEntryTime,
				}
				m.memoryCache.UpdateWindowInfo(session.ID, windowInfo)
			} else {
				util.LogWarn(fmt.Sprintf("Skipping cache for future window: %s (ends at %s, max allowed %s)",
					session.ID,
					time.Unix(windowEndTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(maxFutureTime, 0).Format("2006-01-02 15:04:05")))
			}
		}
	}

	return newSessions
}

// fullDetectSessions performs full session detection (renamed from detectSessions)
func (m *Manager) fullDetectSessions() {
	m.activeSessions = m.fullDetectSessionsAtomic()
}

func (m *Manager) detectSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use full detection as the default
	m.fullDetectSessions()

	// Calculate metrics for each session and store window info
	for _, session := range m.activeSessions {
		m.calculator.Calculate(session)

		// Store window detection info back to cache if detected
		// Only cache windows that are not too far in the future (max session duration ahead)
		currentTime := time.Now().Unix()
		maxFutureTime := currentTime + constants.MaxFutureWindowSeconds

		if session.IsWindowDetected && session.WindowStartTime != nil {
			// Check if window end time is not too far in the future
			windowEndTime := *session.WindowStartTime + constants.SessionDurationSeconds
			if windowEndTime <= maxFutureTime {
				windowInfo := &WindowDetectionInfo{
					WindowStartTime:  session.WindowStartTime,
					IsWindowDetected: session.IsWindowDetected,
					WindowSource:     session.WindowSource,
					DetectedAt:       currentTime,
					FirstEntryTime:   session.FirstEntryTime,
				}
				m.memoryCache.UpdateWindowInfo(session.ID, windowInfo)
			} else {
				util.LogWarn(fmt.Sprintf("Skipping cache for future window: %s (ends at %s, max allowed %s)",
					session.ID,
					time.Unix(windowEndTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(maxFutureTime, 0).Format("2006-01-02 15:04:05")))
			}
		}
	}

	// Log detailed session info
	for i, session := range m.activeSessions {
		projectNames := make([]string, 0, len(session.Projects))
		for name := range session.Projects {
			projectNames = append(projectNames, name)
		}
		projectsStr := strings.Join(projectNames, ", ")
		if projectsStr == "" {
			projectsStr = "no projects"
		}

		util.LogInfo(fmt.Sprintf("Session %d: ID=%s, Window=%s (%s), ResetTime=%s, Tokens=%d, Cost=%.2f, Projects=%s",
			i+1, session.ID,
			time.Unix(session.StartTime, 0).Format("15:04"),
			session.WindowSource,
			time.Unix(session.ResetTime, 0).Format("15:04"),
			session.TotalTokens,
			session.TotalCost,
			projectsStr))
		util.LogDebug(fmt.Sprintf("Session %s details - StartTime: %s, EndTime: %s, ResetTime: %s, IsActive: %v, IsWindowDetected: %v, Projects: %d",
			session.ID,
			time.Unix(session.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(session.EndTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(session.ResetTime, 0).Format("2006-01-02 15:04:05"),
			session.IsActive,
			session.IsWindowDetected,
			len(session.Projects)))
	}

	util.LogInfo(fmt.Sprintf("Detected %d active sessions", len(m.activeSessions)))
}

func (m *Manager) updateDisplay() {
	m.mu.RLock()
	isLoading := m.isLoading
	loadingMessage := m.loadingMessage
	
	var sessions []*Session
	if !isLoading && len(m.activeSessions) > 0 {
		// Use current active sessions if available and not loading
		sessions = make([]*Session, len(m.activeSessions))
		copy(sessions, m.activeSessions)
	} else if isLoading && len(m.previousSessions) > 0 {
		// Use previous valid sessions during loading to avoid empty display
		sessions = make([]*Session, len(m.previousSessions))
		copy(sessions, m.previousSessions)
	} else {
		// No sessions available
		sessions = make([]*Session, 0)
	}
	m.mu.RUnlock()

	// Apply sorting
	m.sorter.Sort(sessions)

	// Update state with loading information
	state := m.state
	state.IsLoading = isLoading
	state.LoadingMessage = loadingMessage

	// Pass state to display
	m.display.RenderWithState(sessions, state)
}

func (m *Manager) handleKeyboard(event KeyEvent) bool {
	// Handle confirm dialog inputs first
	if m.state.ConfirmDialog != nil {
		switch event.Type {
		case KeyChar:
			switch event.Key {
			case 'y', 'Y':
				if m.state.ConfirmDialog.OnConfirm != nil {
					m.state.ConfirmDialog.OnConfirm()
				}
				// Clear screen after dialog is dismissed
				m.display.ClearScreen()
				return false
			case 'n', 'N', 27: // 'n', 'N', or ESC
				if m.state.ConfirmDialog.OnCancel != nil {
					m.state.ConfirmDialog.OnCancel()
				}
				// Clear screen after dialog is dismissed
				m.display.ClearScreen()
				return false
			}
		case KeyEscape:
			if m.state.ConfirmDialog.OnCancel != nil {
				m.state.ConfirmDialog.OnCancel()
			}
			// Clear screen after dialog is dismissed
			m.display.ClearScreen()
			return false
		}
		return false // Ignore other keys when dialog is open
	}

	switch event.Type {
	case KeyChar:
		switch event.Key {
		case 'q', 'Q', 3: // 'q', 'Q', or Ctrl+C
			// Quit
			return true
		case 'r', 'R':
			// Force refresh
			m.state.ForceRefresh = true
			m.refreshData()
		case 'c', 'C':
			// Clear window history
			m.clearWindowHistory()
		case 'p', 'P':
			// Pause/unpause
			m.state.IsPaused = !m.state.IsPaused
		case 'h', 'H':
			// Toggle help
			m.state.ShowHelp = !m.state.ShowHelp
		case 't', 'T':
			// Cycle through layout styles
			m.state.LayoutStyle = (m.state.LayoutStyle + 1) % 2
		}
	case KeyEscape:
		// If help or details are shown, close them; otherwise quit
		if m.state.ShowHelp {
			m.state.ShowHelp = false
		} else {
			// Quit the program
			return true
		}
	}

	return false
}

// refreshDataAtomic performs atomic data refresh with double buffering
func (m *Manager) refreshDataAtomic() {
	// Acquire refresh mutex to prevent concurrent refreshes
	m.refreshMutex.Lock()
	defer m.refreshMutex.Unlock()

	// Set loading state
	m.mu.Lock()
	m.isLoading = true
	m.loadingMessage = "Refreshing data..."
	// Store previous sessions as backup
	if len(m.activeSessions) > 0 {
		m.previousSessions = make([]*Session, len(m.activeSessions))
		copy(m.previousSessions, m.activeSessions)
	}
	m.mu.Unlock()

	// Rescan recent files and update
	files, err := m.scanRecentFiles()
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to scan recent files: %v", err))
		// Reset loading state on error but keep previous data
		m.mu.Lock()
		m.isLoading = false
		m.loadingMessage = ""
		m.mu.Unlock()
		return
	}

	// Track which files are new or changed
	changedFiles := m.identifyChangedFiles(files)
	
	m.loadFiles(files)
	
	// Prepare new sessions (double buffering)
	var newSessions []*Session
	if m.sessionConfig.EnableIncrementalDetection && len(changedFiles) > 0 {
		newSessions = m.incrementalDetectSessionsAtomic(changedFiles)
	} else {
		newSessions = m.fullDetectSessionsAtomic()
	}

	// Atomically replace sessions
	m.mu.Lock()
	if len(newSessions) > 0 {
		m.activeSessions = newSessions
		m.lastDataUpdate = time.Now().Unix()
	}
	// Clear loading state
	m.isLoading = false
	m.loadingMessage = ""
	m.mu.Unlock()
}

// Legacy method for backward compatibility
func (m *Manager) refreshData() {
	m.refreshDataAtomic()
}

// identifyChangedFiles returns files that are new or have changed since last load
func (m *Manager) identifyChangedFiles(files []string) []string {
	changedFiles := make([]string, 0)
	
	for _, file := range files {
		sessionId := extractSessionId(file)
		
		// Check if file is in memory cache
		if entry, exists := m.memoryCache.Get(sessionId); exists {
			// File exists, check if it's marked as dirty (changed)
			if entry.IsDirty {
				changedFiles = append(changedFiles, sessionId)
			}
		} else {
			// New file
			changedFiles = append(changedFiles, sessionId)
		}
	}
	
	return changedFiles
}

func (m *Manager) clearWindowHistory() {
	// Get confirmation from user
	m.state.ConfirmDialog = &model.ConfirmDialog{
		Title:   "Clear Window History",
		Message: "This will clear all learned window boundaries (preserving limit messages). Continue?",
		OnConfirm: func() {
			// Set loading state for window history clearing
			m.mu.Lock()
			m.isLoading = true
			m.loadingMessage = "Clearing window history..."
			m.mu.Unlock()

			// Get history file path
			homeDir, _ := os.UserHomeDir()
			historyPath := filepath.Join(homeDir, ".go-claude-monitor", "history", "window_history.json")

			// Instead of removing the file, preserve limit_message entries
			if m.detector != nil && m.detector.windowHistory != nil {
				// Get all limit-reached windows from the last 3 days
				limitWindows := m.detector.windowHistory.GetLimitReachedWindows()
				currentTime := time.Now().Unix()
				minTime := currentTime - constants.LimitWindowRetentionSeconds

				// Filter to keep only recent limit windows
				var preservedWindows []WindowRecord
				for _, window := range limitWindows {
					if window.EndTime >= minTime && window.Source == "limit_message" {
						preservedWindows = append(preservedWindows, window)
					}
				}

				util.LogInfo(fmt.Sprintf("Clearing window history, preserving %d limit_message entries", len(preservedWindows)))

				// Create new window history with only preserved entries
				newHistory := NewWindowHistoryManager(m.config.CacheDir)
				for _, window := range preservedWindows {
					newHistory.AddOrUpdateWindow(window)
				}

				// Save the new history
				if err := newHistory.Save(); err != nil {
					util.LogError(fmt.Sprintf("Failed to save cleared window history: %v", err))
					m.state.StatusMessage = "Failed to clear window history"
					
					// Clear loading state on error
					m.mu.Lock()
					m.isLoading = false
					m.loadingMessage = ""
					m.mu.Unlock()
				} else {
					// Replace the detector's window history
					m.detector.windowHistory = newHistory
					util.LogInfo("Window history cleared successfully, limit messages preserved")

					// Reload data atomically (this will clear loading state when done)
					m.refreshDataAtomic()
				}
			} else {
				// Fallback: remove the file if detector is not initialized
				if err := os.Remove(historyPath); err != nil && !os.IsNotExist(err) {
					util.LogError(fmt.Sprintf("Failed to remove window history: %v", err))
					m.state.StatusMessage = "Failed to clear window history"
				} else {
					util.LogInfo("Window history cleared (no detector available)")
				}
				
				// Clear loading state
				m.mu.Lock()
				m.isLoading = false
				m.loadingMessage = ""
				m.mu.Unlock()
			}

			// Clear confirm dialog
			m.state.ConfirmDialog = nil
		},
		OnCancel: func() {
			m.state.ConfirmDialog = nil
		},
	}
}

func (m *Manager) persistCache() {
	dirtyEntries := m.memoryCache.GetDirtyEntries()

	for hash, entry := range dirtyEntries {
		if err := m.fileCache.Set(hash, entry); err != nil {
			util.LogError(fmt.Sprintf("Failed to persist cache: %v", err))
		}
	}

	// Also save window history with cleanup
	if m.detector != nil && m.detector.windowHistory != nil {
		// Clean old windows periodically
		if removedCount := m.detector.windowHistory.CleanOldWindows(); removedCount > 0 {
			util.LogInfo(fmt.Sprintf("Cleaned %d old windows from history", removedCount))
		}
		
		// Merge account-level windows
		m.detector.windowHistory.MergeAccountWindows()
		
		// Save the updated history
		if err := m.detector.windowHistory.Save(); err != nil {
			util.LogError(fmt.Sprintf("Failed to save window history: %v", err))
		}
	}

	m.lastCacheSave = time.Now().Unix()
}

func (m *Manager) startWatcher(ctx context.Context) error {
	// Initialize file watcher
	watcher, err := NewFileWatcher([]string{m.config.DataDir})
	if err != nil {
		return err
	}

	m.watcher = watcher
	return nil
}

func (m *Manager) handleFileChange(event model.FileEvent) {
	// Handle file change event
	util.LogDebug(fmt.Sprintf("File changed: %s (%s)", event.Path, event.Operation))

	// Parse and update the changed file
	sessionId := extractSessionId(event.Path)
	sessionIdMap := map[string]string{event.Path: sessionId}

	m.parseAndCacheFiles([]string{event.Path}, sessionIdMap)
	m.detectSessions()
}

// Close cleans up all resources used by the Manager
func (m *Manager) Close() error {
	// Save window history before closing
	if m.detector != nil && m.detector.windowHistory != nil {
		if err := m.detector.windowHistory.Save(); err != nil {
			util.LogError(fmt.Sprintf("Failed to save window history on close: %v", err))
		}
	}

	// Close file watcher if it exists
	if m.watcher != nil {
		if err := m.watcher.Close(); err != nil {
			return fmt.Errorf("failed to close file watcher: %w", err)
		}
	}

	// Keyboard cleanup is handled by defer in Run()
	// Other resources don't need explicit cleanup

	return nil
}

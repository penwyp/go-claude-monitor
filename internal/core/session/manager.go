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
	config      *TopConfig
	fileCache   cache.Cache            // Reuse existing cache.Cache
	memoryCache *MemoryCache           // Memory cache layer
	scanner     *scanner.FileScanner   // Reuse existing scanner
	parser      *parser.Parser         // Reuse existing parser
	aggregator  *aggregator.Aggregator // Reuse existing aggregator
	detector    *SessionDetector       // Session detector
	calculator  *MetricsCalculator     // Metrics calculator
	display     *TerminalDisplay       // Display component
	watcher     *FileWatcher           // File watcher
	planLimits  pricing.Plan           // Plan limits
	keyboard    *KeyboardReader        // Keyboard input
	sorter      *SessionSorter         // Session sorter

	mu             sync.RWMutex
	activeSessions []*Session
	lastCacheSave  int64
	state          model.InteractionState // Interaction state
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

	return &Manager{
		config:      config,
		fileCache:   fileCache,
		memoryCache: NewMemoryCache(),
		scanner:     scanner.NewFileScanner(config.DataDir),
		parser:      parser.NewParser(config.Concurrency),
		aggregator:  agg,
		detector:    NewSessionDetectorWithAggregator(agg, config.Timezone, config.CacheDir), // Create detector with aggregator instance and cache dir
		calculator:  NewMetricsCalculator(planLimits),
		display:     NewTerminalDisplay(config),
		planLimits:  planLimits,
		sorter:      NewSessionSorter(),
		state:       model.InteractionState{},
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

	// Phase 1: Preload data (reuse cache.Preload)
	if err := m.preload(); err != nil {
		return fmt.Errorf("preload failed: %w", err)
	}

	// Phase 2: Start file monitoring
	if err := m.startWatcher(ctx); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Phase 4: Initialize keyboard for default framework
	keyboard, err := NewKeyboardReader()
	if err != nil {
		return fmt.Errorf("failed to initialize keyboard: %w", err)
	}
	m.keyboard = keyboard
	defer m.keyboard.Close()

	// Enter alternate screen mode
	m.display.EnterAlternateScreen()
	defer m.display.ExitAlternateScreen()

	// Phase 5: Main loop for default framework
	uiTicker := time.NewTicker(time.Duration(1000/m.config.UIRefreshRate) * time.Millisecond)
	defer uiTicker.Stop()

	dataTicker := time.NewTicker(m.config.DataRefreshInterval)
	defer dataTicker.Stop()

	cacheTicker := time.NewTicker(1 * time.Minute)
	defer cacheTicker.Stop()

	// Initial display
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
				m.refreshData()
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

	util.LogInfo(fmt.Sprintf("Found %d recent files (modified within 5 hours)", len(files)))

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

	// Filter files modified within 6 hours to ensure we capture complete 5-hour sessions
	cutoff := time.Now().Add(-6 * time.Hour).Unix()
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

		// Filter logs within 5 hours
		recentLogs := m.filterRecentLogs(result.Logs)
		if len(recentLogs) == 0 {
			continue
		}

		// Aggregate data
		projectName := aggregator.ExtractProjectName(result.File)
		hourlyData := m.aggregator.AggregateByHourAndModel(recentLogs, projectName)

		// Create aggregated data
		sessionId := sessionIdMap[result.File]
		aggregatedData := &aggregator.AggregatedData{
			FileHash:    sessionId, // Using sessionId for now, will rename field later
			FilePath:    result.File,
			ProjectName: projectName,
			HourlyStats: hourlyData,
			SessionId:   sessionId,
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
	cutoff := time.Now().Add(-6 * time.Hour).Unix() // Expand to 6 hours to ensure complete 5-hour sessions
	var recent []model.ConversationLog

	for _, log := range logs {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}

		if ts.Unix() > cutoff {
			recent = append(recent, log)
		}
	}

	return recent
}

func (m *Manager) detectSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

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

	// Get all data within 6 hours from memory cache to ensure complete 5-hour sessions
	hourlyData, rawLogs := m.memoryCache.GetRecentDataWithLogs(6 * 3600)

	// Get cached window info
	cachedWindowInfo := m.memoryCache.GetCachedWindowInfo()

	// Detect sessions with raw logs for limit detection
	input := SessionDetectionInput{
		HourlyData:       hourlyData,
		RawLogs:          rawLogs,
		CachedWindowInfo: cachedWindowInfo,
	}
	m.activeSessions = m.detector.DetectSessionsWithLimits(input)

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
		util.LogInfo(fmt.Sprintf("Session %d: ID=%s, Window=%s (%s), ResetTime=%s, Tokens=%d, Cost=%.2f",
			i+1, session.ID,
			time.Unix(session.StartTime, 0).Format("15:04"),
			session.WindowSource,
			time.Unix(session.ResetTime, 0).Format("15:04"),
			session.TotalTokens,
			session.TotalCost))
		util.LogDebug(fmt.Sprintf("Session %s details - StartTime: %s, EndTime: %s, ResetTime: %s, IsActive: %v, IsWindowDetected: %v",
			session.ID,
			time.Unix(session.StartTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(session.EndTime, 0).Format("2006-01-02 15:04:05"),
			time.Unix(session.ResetTime, 0).Format("2006-01-02 15:04:05"),
			session.IsActive,
			session.IsWindowDetected))
	}

	util.LogInfo(fmt.Sprintf("Detected %d active sessions", len(m.activeSessions)))
}

func (m *Manager) updateDisplay() {
	m.mu.RLock()
	sessions := make([]*Session, len(m.activeSessions))
	copy(sessions, m.activeSessions)
	m.mu.RUnlock()

	// Apply sorting
	m.sorter.Sort(sessions)

	// Pass state to display
	m.display.RenderWithState(sessions, m.state)
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

func (m *Manager) refreshData() {
	// Rescan recent files and update
	files, err := m.scanRecentFiles()
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to scan recent files: %v", err))
		return
	}

	m.loadFiles(files)
	m.detectSessions()
}

func (m *Manager) clearWindowHistory() {
	// Get confirmation from user
	m.state.ConfirmDialog = &model.ConfirmDialog{
		Title:   "Clear Window History",
		Message: "This will clear all learned window boundaries (preserving limit messages). Continue?",
		OnConfirm: func() {
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
				} else {
					// Replace the detector's window history
					m.detector.windowHistory = newHistory
					util.LogInfo("Window history cleared successfully, limit messages preserved")

					// Reload data
					m.refreshData()
				}
			} else {
				// Fallback: remove the file if detector is not initialized
				if err := os.Remove(historyPath); err != nil && !os.IsNotExist(err) {
					util.LogError(fmt.Sprintf("Failed to remove window history: %v", err))
					m.state.StatusMessage = "Failed to clear window history"
				} else {
					util.LogInfo("Window history cleared (no detector available)")
				}
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

	// Also save window history
	if m.detector != nil && m.detector.windowHistory != nil {
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

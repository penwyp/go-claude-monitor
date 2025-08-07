package top

import (
	"context"
	"fmt"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/monitoring"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/presentation/display"
	"github.com/penwyp/go-claude-monitor/internal/presentation/interaction"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// Orchestrator coordinates all components for the top command
type Orchestrator struct {
	config      *TopConfig
	planLimits  pricing.Plan
	
	// Core components
	dataLoader  *DataLoader
	refreshCtrl *RefreshController
	stateManager *StateManager
	
	// Session components
	detector   *session.SessionDetector
	calculator *session.MetricsCalculator
	
	// UI components
	display  *display.TerminalDisplay
	keyboard *interaction.KeyboardReader
	sorter   *interaction.SessionSorter
	
	// Monitoring
	watcher *monitoring.FileWatcher
	
	// Cache management
	lastCacheSave int64
}

// NewOrchestrator creates a new Orchestrator instance
func NewOrchestrator(config *TopConfig) (*Orchestrator, error) {
	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	// Initialize components
	dataLoader, err := NewDataLoader(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create data loader: %w", err)
	}
	
	// Determine plan limits
	planLimits := pricing.GetPlanWithDefault(config.Plan, config.CustomLimitTokens)
	
	// Create session detector with aggregator from data loader
	detector := session.NewSessionDetectorWithAggregator(nil, config.Timezone, config.CacheDir)
	
	// Create metrics calculator
	calculator := session.NewMetricsCalculator(planLimits)
	
	// Create refresh controller
	refreshCtrl := NewRefreshController(dataLoader, detector, calculator)
	
	// Create state manager
	stateManager := NewStateManager()
	
	// Create display
	displayConfig := &display.DisplayConfig{
		Plan:       config.Plan,
		Timezone:   config.Timezone,
		TimeFormat: config.TimeFormat,
	}
	termDisplay := display.NewTerminalDisplay(displayConfig)
	
	// Create sorter
	sorter := interaction.NewSessionSorter()
	
	return &Orchestrator{
		config:       config,
		planLimits:   planLimits,
		dataLoader:   dataLoader,
		refreshCtrl:  refreshCtrl,
		stateManager: stateManager,
		detector:     detector,
		calculator:   calculator,
		display:      termDisplay,
		sorter:       sorter,
	}, nil
}

// Run starts the orchestrator main loop
func (o *Orchestrator) Run(ctx context.Context) error {
	util.LogInfo("Starting Claude Monitor Top...")
	
	// Ensure cleanup on exit
	defer o.Close()
	
	// Initialize global time provider with configured timezone
	if err := util.InitializeTimeProvider(o.config.Timezone); err != nil {
		return fmt.Errorf("failed to initialize timezone: %w", err)
	}
	
	// Phase 1: Initialize keyboard
	keyboard, err := interaction.NewKeyboardReader()
	if err != nil {
		return fmt.Errorf("failed to initialize keyboard: %w", err)
	}
	o.keyboard = keyboard
	defer o.keyboard.Close()
	
	// Enter alternate screen mode
	o.display.EnterAlternateScreen()
	defer o.display.ExitAlternateScreen()
	
	// Set initial loading state
	o.stateManager.SetLoadingState(true, "Initializing and loading data...")
	
	// Show initial loading screen
	o.updateDisplay()
	
	// Phase 2: Preload data
	if err := o.dataLoader.Preload(); err != nil {
		return fmt.Errorf("preload failed: %w", err)
	}
	
	// Perform initial session detection
	sessions, err := o.refreshCtrl.FullDetect()
	if err != nil {
		return fmt.Errorf("initial detection failed: %w", err)
	}
	
	// Update state with detected sessions
	o.stateManager.SetSessions(sessions)
	o.stateManager.SetLoadingState(false, "")
	
	// Phase 3: Start file monitoring
	if err := o.startWatcher(ctx); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	
	// Phase 4: Main event loop
	uiTicker := time.NewTicker(time.Duration(1000/o.config.UIRefreshRate) * time.Millisecond)
	defer uiTicker.Stop()
	
	dataTicker := time.NewTicker(o.config.DataRefreshInterval)
	defer dataTicker.Stop()
	
	cacheTicker := time.NewTicker(1 * time.Minute)
	defer cacheTicker.Stop()
	
	// Initial display with loaded data
	o.updateDisplay()
	
	for {
		select {
		case <-ctx.Done():
			util.LogInfo("Shutting down Claude Monitor Top...")
			return nil
			
		case <-uiTicker.C:
			// UI refresh
			state := o.stateManager.GetInteractionState()
			if !state.IsPaused {
				o.updateDisplay()
			}
			
		case <-dataTicker.C:
			// Data refresh
			state := o.stateManager.GetInteractionState()
			if !state.IsPaused || state.ForceRefresh {
				o.refreshData()
				o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
					s.ForceRefresh = false
				})
			}
			
		case <-cacheTicker.C:
			// Persist cache
			o.persistCache()
			
		case event := <-o.watcher.Events():
			// Handle file changes
			state := o.stateManager.GetInteractionState()
			if !state.IsPaused {
				o.handleFileChange(event)
			}
			
		case keyEvent := <-o.keyboard.Events():
			// Handle keyboard input
			if o.handleKeyboard(keyEvent) {
				return nil // Exit requested
			}
			o.updateDisplay() // Update display after keyboard action
		}
	}
}

// LoadAndAnalyzeData performs the core session detection workflow without UI
func (o *Orchestrator) LoadAndAnalyzeData() ([]*session.Session, error) {
	// Initialize global time provider
	if err := util.InitializeTimeProvider(o.config.Timezone); err != nil {
		return nil, fmt.Errorf("failed to initialize timezone: %w", err)
	}
	
	// Preload data
	if err := o.dataLoader.Preload(); err != nil {
		return nil, fmt.Errorf("preload failed: %w", err)
	}
	
	// Detect sessions
	sessions, err := o.refreshCtrl.FullDetect()
	if err != nil {
		return nil, fmt.Errorf("session detection failed: %w", err)
	}
	
	return sessions, nil
}

// GetAggregatedMetrics calculates aggregated metrics from sessions
func (o *Orchestrator) GetAggregatedMetrics(sessions []*session.Session) *model.AggregatedMetrics {
	displaySessions := convertSessionsForDisplay(sessions)
	return o.display.CalculateAggregatedMetrics(displaySessions)
}

// updateDisplay updates the terminal display
func (o *Orchestrator) updateDisplay() {
	isLoading, loadingMessage := o.stateManager.GetLoadingState()
	sessions := o.stateManager.GetSessionsForDisplay()
	
	// Convert for sorting
	sortingSessions := convertSessionsForSorting(sessions)
	o.sorter.Sort(sortingSessions)
	applySortingToOriginal(sessions, sortingSessions)
	
	// Convert for display
	displaySessions := convertSessionsForDisplay(sessions)
	
	// Update state with loading information
	state := o.stateManager.GetInteractionState()
	state.IsLoading = isLoading
	state.LoadingMessage = loadingMessage
	
	// Pass state to display
	o.display.RenderWithState(displaySessions, state)
}

// refreshData performs data refresh
func (o *Orchestrator) refreshData() {
	// Set loading state
	o.stateManager.SetLoadingState(true, "Refreshing data...")
	
	// Perform refresh
	sessions, err := o.refreshCtrl.RefreshData()
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to refresh data: %v", err))
		o.stateManager.SetLoadingState(false, "")
		return
	}
	
	// Update state with new sessions
	o.stateManager.SetSessions(sessions)
	o.stateManager.SetLoadingState(false, "")
}

// handleKeyboard handles keyboard events
func (o *Orchestrator) handleKeyboard(event interaction.KeyEvent) bool {
	state := o.stateManager.GetInteractionState()
	
	// Handle confirm dialog inputs first
	if state.ConfirmDialog != nil {
		switch event.Type {
		case interaction.KeyChar:
			switch event.Key {
			case 'y', 'Y':
				if state.ConfirmDialog.OnConfirm != nil {
					state.ConfirmDialog.OnConfirm()
				}
				o.display.ClearScreen()
				return false
			case 'n', 'N', 27: // 'n', 'N', or ESC
				if state.ConfirmDialog.OnCancel != nil {
					state.ConfirmDialog.OnCancel()
				}
				o.display.ClearScreen()
				return false
			}
		case interaction.KeyEscape:
			if state.ConfirmDialog.OnCancel != nil {
				state.ConfirmDialog.OnCancel()
			}
			o.display.ClearScreen()
			return false
		}
		return false // Ignore other keys when dialog is open
	}
	
	// Handle normal keyboard input
	switch event.Type {
	case interaction.KeyChar:
		switch event.Key {
		case 'q', 'Q', 3: // 'q', 'Q', or Ctrl+C
			return true // Exit
		case 'r', 'R':
			// Force refresh
			o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
				s.ForceRefresh = true
			})
			o.refreshData()
		case 'c', 'C':
			// Clear window history
			o.clearWindowHistory()
		case 'x', 'X':
			// Clear cache
			o.clearCache()
		case 'p', 'P':
			// Pause/unpause
			o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
				s.IsPaused = !s.IsPaused
			})
		case 'h', 'H':
			// Toggle help
			o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
				s.ShowHelp = !s.ShowHelp
			})
		case 't', 'T':
			// Cycle through layout styles
			o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
				s.LayoutStyle = (s.LayoutStyle + 1) % 2
			})
		}
	case interaction.KeyEscape:
		// If help is shown, close it; otherwise quit
		state := o.stateManager.GetInteractionState()
		if state.ShowHelp {
			o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
				s.ShowHelp = false
			})
		} else {
			return true // Exit
		}
	}
	
	return false
}

// clearWindowHistory clears the window history with confirmation
func (o *Orchestrator) clearWindowHistory() {
	o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
		s.ConfirmDialog = &model.ConfirmDialog{
			Title:   "Clear Window History",
			Message: "This will clear all learned window boundaries (preserving limit messages). Continue?",
			OnConfirm: func() {
				o.stateManager.SetLoadingState(true, "Clearing window history...")
				
				// Clear window history
				if o.detector.GetWindowHistory() != nil {
					// Preserve limit messages logic here
					o.detector.GetWindowHistory().ClearNonLimitWindows()
					o.detector.GetWindowHistory().Save()
				}
				
				// Refresh data
				o.refreshData()
				
				// Clear dialog
				o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
					s.ConfirmDialog = nil
				})
			},
			OnCancel: func() {
				o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
					s.ConfirmDialog = nil
				})
			},
		}
	})
}

// clearCache clears memory cache with confirmation
func (o *Orchestrator) clearCache() {
	o.stateManager.UpdateInteractionState(func(s *model.InteractionState) {
		s.ConfirmDialog = &model.ConfirmDialog{
			Title:   "Clear Memory Cache",
			Message: "This will clear all cached session data and force a full reload. Continue?",
			OnConfirm: func() {
				o.stateManager.SetLoadingState(true, "Clearing memory cache...")
				
				// Clear memory cache through data loader
				if memoryCache := o.dataLoader.GetMemoryCache(); memoryCache != nil {
					memoryCache.Clear()
				}
				
				// Force a full refresh after clearing cache
				go func() {
					// Small delay to show loading message
					time.Sleep(100 * time.Millisecond)
					sessions, err := o.refreshCtrl.FullDetect()
					if err != nil {
						util.LogError(fmt.Sprintf("Failed to refresh after cache clear: %v", err))
						o.stateManager.SetLoadingState(false, "")
						return
					}
					
					// Update state with new sessions
					o.stateManager.SetSessions(sessions)
					o.stateManager.SetLoadingState(false, "")
					
					util.LogInfo("Memory cache cleared and data refreshed")
				}()
			},
			OnCancel: func() {
				// Clear the dialog
			},
		}
	})
}

// persistCache persists dirty cache entries and window history
func (o *Orchestrator) persistCache() {
	// Persist dirty cache entries
	o.dataLoader.PersistDirtyEntries()
	
	// Save window history with cleanup
	if o.detector.GetWindowHistory() != nil {
		// Clean old windows periodically
		if removedCount := o.detector.GetWindowHistory().CleanOldWindows(); removedCount > 0 {
			util.LogInfo(fmt.Sprintf("Cleaned %d old windows from history", removedCount))
		}
		
		// Merge account-level windows
		o.detector.GetWindowHistory().MergeAccountWindows()
		
		// Save the updated history
		if err := o.detector.GetWindowHistory().Save(); err != nil {
			util.LogError(fmt.Sprintf("Failed to save window history: %v", err))
		}
	}
	
	o.lastCacheSave = time.Now().Unix()
}

// startWatcher initializes the file watcher
func (o *Orchestrator) startWatcher(ctx context.Context) error {
	watcher, err := monitoring.NewFileWatcher([]string{o.config.DataDir})
	if err != nil {
		return err
	}
	o.watcher = watcher
	return nil
}

// handleFileChange handles file change events
func (o *Orchestrator) handleFileChange(event model.FileEvent) {
	util.LogDebug(fmt.Sprintf("File changed: %s (%s)", event.Path, event.Operation))
	
	// Parse and update the changed file
	sessionId := extractSessionId(event.Path)
	o.dataLoader.LoadFiles([]string{event.Path})
	
	// Perform incremental detection
	sessions, err := o.refreshCtrl.IncrementalDetect([]string{sessionId})
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to handle file change: %v", err))
		return
	}
	
	// Update state
	o.stateManager.SetSessions(sessions)
}

// Close cleans up all resources
func (o *Orchestrator) Close() error {
	// Save dirty cache entries before closing
	if err := o.dataLoader.PersistDirtyEntries(); err != nil {
		util.LogError(fmt.Sprintf("Failed to persist dirty cache entries on close: %v", err))
	}
	
	// Save window history before closing
	if o.detector.GetWindowHistory() != nil {
		if err := o.detector.GetWindowHistory().Save(); err != nil {
			util.LogError(fmt.Sprintf("Failed to save window history on close: %v", err))
		}
	}
	
	// Close file watcher
	if o.watcher != nil {
		if err := o.watcher.Close(); err != nil {
			return fmt.Errorf("failed to close file watcher: %w", err)
		}
	}
	
	return nil
}


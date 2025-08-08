package top

import (
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
)

// StateManager manages application state in a thread-safe manner
type StateManager struct {
	mu sync.RWMutex
	
	// Session state
	activeSessions   []*session.Session
	previousSessions []*session.Session // Keep previous valid sessions during refresh
	
	// Loading state
	isLoading      bool
	loadingMessage string
	
	// Interaction state
	interactionState model.InteractionState
	
	// Metadata
	lastDataUpdate int64 // Timestamp of last successful data update
	hasInitialData bool  // Flag to track if initial data has been loaded
	lastValidCount int   // Track last valid session count for integrity check
}

// NewStateManager creates a new StateManager instance
func NewStateManager() *StateManager {
	return &StateManager{
		activeSessions:   make([]*session.Session, 0),
		previousSessions: make([]*session.Session, 0),
		interactionState: model.InteractionState{},
		hasInitialData:   false,
		lastValidCount:   0,
	}
}


// SetSessions updates active sessions (thread-safe)
func (sm *StateManager) SetSessions(sessions []*session.Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Enhanced data protection
	if sessions == nil || len(sessions) == 0 {
		// For non-initial loads, strictly reject empty data
		if sm.hasInitialData {
			// Log warning but keep existing data
			currentCount := len(sm.activeSessions)
			if currentCount > 0 {
				// This is likely a transient issue, maintain existing data
				return
			}
			// Even if current is empty, check previous sessions as fallback
			if len(sm.previousSessions) > 0 {
				// Restore from previous sessions
				sm.activeSessions = make([]*session.Session, len(sm.previousSessions))
				copy(sm.activeSessions, sm.previousSessions)
				return
			}
		}
		// For initial load, we may accept empty sessions
		// but mark that we still don't have initial data
		if !sm.hasInitialData && len(sessions) == 0 {
			// Don't mark as having initial data yet
			sm.activeSessions = sessions
			sm.lastDataUpdate = time.Now().Unix()
			return
		}
	}
	
	// Data integrity check: warn if session count drops significantly
	newCount := len(sessions)
	if sm.hasInitialData && sm.lastValidCount > 0 {
		dropPercentage := float64(sm.lastValidCount - newCount) / float64(sm.lastValidCount) * 100
		if dropPercentage > 50 {
			// Significant drop, log warning but still update
			// (This could be legitimate if files were deleted)
		}
	}
	
	// Store current sessions as previous before updating
	if len(sm.activeSessions) > 0 {
		sm.previousSessions = make([]*session.Session, len(sm.activeSessions))
		copy(sm.previousSessions, sm.activeSessions)
	}
	
	// Update active sessions
	sm.activeSessions = sessions
	sm.lastDataUpdate = time.Now().Unix()
	
	// Update tracking flags
	if newCount > 0 {
		sm.hasInitialData = true
		sm.lastValidCount = newCount
	}
}


// GetLoadingState returns current loading state and message
func (sm *StateManager) GetLoadingState() (bool, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return sm.isLoading, sm.loadingMessage
}

// SetLoadingState updates loading state and message (deprecated, use SetDisplayStatus)
func (sm *StateManager) SetLoadingState(isLoading bool, message string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.isLoading = isLoading
	sm.loadingMessage = message
	
	// Also update new DisplayStatus for compatibility
	if isLoading {
		sm.interactionState.DisplayStatus = model.StatusLoading
		sm.interactionState.StatusIndicator = message
	} else {
		sm.interactionState.DisplayStatus = model.StatusNormal
		sm.interactionState.StatusIndicator = ""
	}
}

// SetDisplayStatus sets the display status with an optional message
func (sm *StateManager) SetDisplayStatus(status model.DisplayStatus, message string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.interactionState.DisplayStatus = status
	sm.interactionState.StatusIndicator = message
	
	// Update deprecated fields for compatibility
	switch status {
	case model.StatusLoading:
		sm.isLoading = true
		sm.loadingMessage = message
	case model.StatusRefreshing, model.StatusClearing:
		// These states show data with indicator, not full loading screen
		sm.isLoading = false
		sm.loadingMessage = ""
	default:
		sm.isLoading = false
		sm.loadingMessage = ""
	}
}

// GetInteractionState returns current interaction state
func (sm *StateManager) GetInteractionState() model.InteractionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Return a copy of the state
	return sm.interactionState
}


// UpdateInteractionState updates specific fields of interaction state
func (sm *StateManager) UpdateInteractionState(updateFunc func(*model.InteractionState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	updateFunc(&sm.interactionState)
}


// GetCurrentSessions returns a copy of current active sessions (thread-safe)
func (sm *StateManager) GetCurrentSessions() []*session.Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	sessions := make([]*session.Session, len(sm.activeSessions))
	copy(sessions, sm.activeSessions)
	return sessions
}

// GetSessionsForDisplay returns sessions appropriate for display based on loading state
func (sm *StateManager) GetSessionsForDisplay() []*session.Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Primary: Return active sessions if available
	if len(sm.activeSessions) > 0 {
		sessions := make([]*session.Session, len(sm.activeSessions))
		copy(sessions, sm.activeSessions)
		return sessions
	}
	
	// Fallback: If no active sessions, use previous sessions as fallback
	// This ensures we always show data if we have any historical data
	if len(sm.previousSessions) > 0 {
		sessions := make([]*session.Session, len(sm.previousSessions))
		copy(sessions, sm.previousSessions)
		return sessions
	}
	
	// No sessions available at all
	return make([]*session.Session, 0)
}
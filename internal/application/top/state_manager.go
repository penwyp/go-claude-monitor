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
}

// NewStateManager creates a new StateManager instance
func NewStateManager() *StateManager {
	return &StateManager{
		activeSessions:   make([]*session.Session, 0),
		previousSessions: make([]*session.Session, 0),
		interactionState: model.InteractionState{},
	}
}


// SetSessions updates active sessions (thread-safe)
func (sm *StateManager) SetSessions(sessions []*session.Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Store current sessions as previous before updating
	if len(sm.activeSessions) > 0 {
		sm.previousSessions = make([]*session.Session, len(sm.activeSessions))
		copy(sm.previousSessions, sm.activeSessions)
	}
	
	sm.activeSessions = sessions
	sm.lastDataUpdate = time.Now().Unix()
}


// GetLoadingState returns current loading state and message
func (sm *StateManager) GetLoadingState() (bool, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return sm.isLoading, sm.loadingMessage
}

// SetLoadingState updates loading state and message
func (sm *StateManager) SetLoadingState(isLoading bool, message string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.isLoading = isLoading
	sm.loadingMessage = message
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


// GetSessionsForDisplay returns sessions appropriate for display based on loading state
func (sm *StateManager) GetSessionsForDisplay() []*session.Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// If not loading and have active sessions, return them
	if !sm.isLoading && len(sm.activeSessions) > 0 {
		sessions := make([]*session.Session, len(sm.activeSessions))
		copy(sessions, sm.activeSessions)
		return sessions
	}
	
	// If loading but have previous sessions, return those to avoid empty display
	if sm.isLoading && len(sm.previousSessions) > 0 {
		sessions := make([]*session.Session, len(sm.previousSessions))
		copy(sessions, sm.previousSessions)
		return sessions
	}
	
	// No sessions available
	return make([]*session.Session, 0)
}
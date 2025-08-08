package session

import (
	"fmt"
	"sort"
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/constants"
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session/strategies"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// SessionDetectorWithStrategy is a refactored session detector using the strategy pattern
type SessionDetectorWithStrategy struct {
	// Strategy registry
	registry *strategies.StrategyRegistry
	
	// Core components
	aggregator      *aggregator.Aggregator
	limitParser     *LimitParser
	windowHistory   *WindowHistoryManager
	sessionDuration time.Duration
	location        *time.Location
	cacheDir        string
}

// NewSessionDetectorWithStrategy creates a new session detector with strategy pattern
func NewSessionDetectorWithStrategy(agg *aggregator.Aggregator, timezone string, cacheDir string) *SessionDetectorWithStrategy {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}
	
	detector := &SessionDetectorWithStrategy{
		registry:        strategies.NewStrategyRegistry(),
		aggregator:      agg,
		limitParser:     NewLimitParser(),
		sessionDuration: constants.SessionDuration,
		location:        loc,
		cacheDir:        cacheDir,
	}
	
	// Load window history
	detector.windowHistory = NewWindowHistoryManager(cacheDir)
	if err := detector.windowHistory.Load(); err != nil {
		util.LogWarn(fmt.Sprintf("Failed to load window history: %v", err))
	}
	
	// Register all strategies in priority order
	detector.registerStrategies()
	
	return detector
}

// registerStrategies registers all detection strategies
func (d *SessionDetectorWithStrategy) registerStrategies() {
	// Create adapters for existing components
	parserAdapter := d.createLimitParserAdapter()
	
	// Register strategies in priority order (will be auto-sorted by registry)
	d.registry.Register(strategies.NewHistoryLimitStrategy())
	d.registry.Register(strategies.NewCurrentLimitStrategy(parserAdapter))
	d.registry.Register(strategies.NewContinuousActivityStrategy())
	d.registry.Register(strategies.NewHistoryAccountStrategy())
	d.registry.Register(strategies.NewGapDetectionStrategy())
	d.registry.Register(strategies.NewFirstMessageStrategy())
	
	util.LogInfo("Registered all window detection strategies:")
	util.LogInfo(d.registry.Summary())
}

// createWindowHistoryAdapter creates an adapter for WindowHistoryManager
func (d *SessionDetectorWithStrategy) createWindowHistoryAdapter() strategies.WindowHistoryAccess {
	if d.windowHistory == nil {
		return nil
	}
	
	// Create an adapter that implements WindowHistoryAccess
	return &windowHistoryAccessAdapter{
		manager: d.windowHistory,
	}
}

// createLimitParserAdapter creates an adapter for LimitParser
func (d *SessionDetectorWithStrategy) createLimitParserAdapter() strategies.LimitParser {
	if d.limitParser == nil {
		return nil
	}
	
	// Create an adapter that implements LimitParser interface
	return &limitParserStrategyAdapter{
		parser: d.limitParser,
	}
}

// collectWindowCandidatesWithStrategy uses the strategy pattern to collect candidates
func (d *SessionDetectorWithStrategy) collectWindowCandidatesWithStrategy(input SessionDetectionInput) []strategies.WindowCandidate {
	// Convert input to strategy format
	strategyInput := strategies.DetectionInput{
		GlobalTimeline:  input.GlobalTimeline,
		RawLogs:        d.extractRawLogs(input.GlobalTimeline),
		WindowHistory:   d.createWindowHistoryAdapter(),
		SessionDuration: d.sessionDuration,
		CurrentTime:     time.Now().Unix(),
	}
	
	// Collect candidates from all strategies
	candidates := d.registry.CollectCandidates(strategyInput)
	
	// Update window history if we detected new limit messages
	d.updateWindowHistoryFromCandidates(candidates)
	
	return candidates
}

// extractRawLogs extracts raw conversation logs from timeline
func (d *SessionDetectorWithStrategy) extractRawLogs(timeline []timeline.TimestampedLog) []model.ConversationLog {
	logs := make([]model.ConversationLog, 0, len(timeline))
	for _, tl := range timeline {
		logs = append(logs, tl.Log)
	}
	return logs
}

// updateWindowHistoryFromCandidates updates window history based on detected limit messages
func (d *SessionDetectorWithStrategy) updateWindowHistoryFromCandidates(candidates []strategies.WindowCandidate) {
	if d.windowHistory == nil {
		return
	}
	
	for _, candidate := range candidates {
		// Update history for limit messages
		if candidate.Source == "limit_message" && candidate.IsLimit {
			if resetTimeStr, ok := candidate.Metadata["reset_time"]; ok {
				var resetTime int64
				fmt.Sscanf(resetTimeStr, "%d", &resetTime)
				
				messageTime := time.Now().Unix() // Default to now if not available
				if msgTimeStr, ok := candidate.Metadata["message_time"]; ok {
					fmt.Sscanf(msgTimeStr, "%d", &messageTime)
				}
				
				d.windowHistory.UpdateFromLimitMessage(resetTime, messageTime, candidate.LimitMessage)
			}
		}
	}
}

// selectBestWindowsWithStrategy selects the best non-overlapping windows
func (d *SessionDetectorWithStrategy) selectBestWindowsWithStrategy(candidates []strategies.WindowCandidate) []strategies.WindowCandidate {
	if len(candidates) == 0 {
		return []strategies.WindowCandidate{}
	}
	
	// Sort by priority (descending) then by start time (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].StartTime < candidates[j].StartTime
	})
	
	selected := make([]strategies.WindowCandidate, 0)
	sessionSeconds := int64(d.sessionDuration.Seconds())
	
	for _, candidate := range candidates {
		// Ensure window is exactly 5 hours
		if candidate.EndTime-candidate.StartTime != sessionSeconds {
			candidate.EndTime = candidate.StartTime + sessionSeconds
		}
		
		// Check for overlap with already selected windows
		if !d.hasOverlap(candidate, selected) {
			selected = append(selected, candidate)
		}
	}
	
	// Sort selected windows by start time
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].StartTime < selected[j].StartTime
	})
	
	return selected
}

// hasOverlap checks if a candidate overlaps with any selected windows
func (d *SessionDetectorWithStrategy) hasOverlap(candidate strategies.WindowCandidate, selected []strategies.WindowCandidate) bool {
	for _, sel := range selected {
		// For continuous_activity windows, allow boundary touching
		if candidate.Source == "continuous_activity" || sel.Source == "continuous_activity" {
			// Strict overlap check - boundaries touching is OK
			if candidate.StartTime < sel.EndTime && candidate.EndTime > sel.StartTime {
				// Check if it's just boundary touching
				if candidate.StartTime == sel.EndTime || candidate.EndTime == sel.StartTime {
					// Boundary touching is allowed for continuous_activity windows
					continue
				}
				return true
			}
		} else {
			// Original logic for other window types
			if candidate.StartTime < sel.EndTime && candidate.EndTime > sel.StartTime {
				return true
			}
		}
	}
	return false
}

// Adapter implementations

// windowHistoryAccessAdapter adapts WindowHistoryManager to WindowHistoryAccess interface
type windowHistoryAccessAdapter struct {
	manager *WindowHistoryManager
}

func (a *windowHistoryAccessAdapter) GetAccountLevelWindows() []strategies.HistoricalWindow {
	windows := make([]strategies.HistoricalWindow, 0)
	for _, w := range a.manager.GetAccountLevelWindows() {
		windows = append(windows, strategies.HistoricalWindow{
			SessionID:      w.SessionID,
			Source:         w.Source,
			StartTime:      w.StartTime,
			EndTime:        w.EndTime,
			IsLimitReached: w.IsLimitReached,
			IsAccountLevel: w.IsAccountLevel,
			LimitMessage:   w.LimitMessage,
		})
	}
	return windows
}

func (a *windowHistoryAccessAdapter) GetRecentWindows(duration time.Duration) []strategies.HistoricalWindow {
	windows := make([]strategies.HistoricalWindow, 0)
	for _, w := range a.manager.GetRecentWindows(duration) {
		windows = append(windows, strategies.HistoricalWindow{
			SessionID:      w.SessionID,
			Source:         w.Source,
			StartTime:      w.StartTime,
			EndTime:        w.EndTime,
			IsLimitReached: w.IsLimitReached,
			IsAccountLevel: w.IsAccountLevel,
			LimitMessage:   w.LimitMessage,
		})
	}
	return windows
}

func (a *windowHistoryAccessAdapter) GetLimitReachedWindows() []strategies.HistoricalWindow {
	windows := make([]strategies.HistoricalWindow, 0)
	for _, w := range a.manager.GetLimitReachedWindows() {
		windows = append(windows, strategies.HistoricalWindow{
			SessionID:      w.SessionID,
			Source:         w.Source,
			StartTime:      w.StartTime,
			EndTime:        w.EndTime,
			IsLimitReached: w.IsLimitReached,
			IsAccountLevel: w.IsAccountLevel,
			LimitMessage:   w.LimitMessage,
		})
	}
	return windows
}

// limitParserStrategyAdapter adapts LimitParser to strategies.LimitParser interface
type limitParserStrategyAdapter struct {
	parser *LimitParser
}

func (a *limitParserStrategyAdapter) ParseLogs(logs []interface{}) []strategies.LimitInfo {
	// Convert to ConversationLog
	convLogs := make([]model.ConversationLog, 0, len(logs))
	for _, log := range logs {
		if convLog, ok := log.(model.ConversationLog); ok {
			convLogs = append(convLogs, convLog)
		}
	}
	
	// Parse and convert results
	limits := make([]strategies.LimitInfo, 0)
	for _, l := range a.parser.ParseLogs(convLogs) {
		limits = append(limits, strategies.LimitInfo{
			Content:   l.Content,
			Timestamp: l.Timestamp,
			ResetTime: l.ResetTime,
		})
	}
	return limits
}
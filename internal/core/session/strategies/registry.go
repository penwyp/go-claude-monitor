package strategies

import (
	"fmt"
	"sort"
	"sync"
)

// StrategyRegistry manages all window detection strategies
type StrategyRegistry struct {
	strategies []WindowDetectionStrategy
	mu         sync.RWMutex
	
	// Configuration
	enabledStrategies map[string]bool // Allow enabling/disabling specific strategies
}

// NewStrategyRegistry creates a new strategy registry
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{
		strategies:        make([]WindowDetectionStrategy, 0),
		enabledStrategies: make(map[string]bool),
	}
}

// Register adds a new strategy to the registry
func (r *StrategyRegistry) Register(strategy WindowDetectionStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check if strategy with same name already exists
	for i, existing := range r.strategies {
		if existing.Name() == strategy.Name() {
			// Replace existing strategy
			r.strategies[i] = strategy
			logDebug(fmt.Sprintf("StrategyRegistry: Replaced existing strategy '%s'", strategy.Name()))
			return
		}
	}
	
	// Add new strategy
	r.strategies = append(r.strategies, strategy)
	// Enable by default
	r.enabledStrategies[strategy.Name()] = true
	
	// Keep strategies sorted by priority (descending)
	sort.Slice(r.strategies, func(i, j int) bool {
		return r.strategies[i].Priority() > r.strategies[j].Priority()
	})
	
	logInfo(fmt.Sprintf("StrategyRegistry: Registered strategy '%s' with priority %d", 
		strategy.Name(), strategy.Priority()))
}

// EnableStrategy enables a specific strategy
func (r *StrategyRegistry) EnableStrategy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabledStrategies[name] = true
}

// DisableStrategy disables a specific strategy
func (r *StrategyRegistry) DisableStrategy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabledStrategies[name] = false
}

// IsEnabled checks if a strategy is enabled
func (r *StrategyRegistry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	enabled, exists := r.enabledStrategies[name]
	return exists && enabled
}

// CollectCandidates runs all enabled strategies and collects window candidates
func (r *StrategyRegistry) CollectCandidates(input DetectionInput) []WindowCandidate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	allCandidates := make([]WindowCandidate, 0)
	
	logInfo(fmt.Sprintf("StrategyRegistry: Running %d registered strategies", len(r.strategies)))
	
	// Run each strategy in priority order
	for _, strategy := range r.strategies {
		// Check if strategy is enabled
		if !r.IsEnabled(strategy.Name()) {
			logDebug(fmt.Sprintf("StrategyRegistry: Skipping disabled strategy '%s'", strategy.Name()))
			continue
		}
		
		logDebug(fmt.Sprintf("StrategyRegistry: Running strategy '%s' (priority %d)",
			strategy.Name(), strategy.Priority()))
		
		// Detect windows using this strategy
		candidates := strategy.Detect(input)
		
		if len(candidates) > 0 {
			logInfo(fmt.Sprintf("StrategyRegistry: Strategy '%s' detected %d candidates",
				strategy.Name(), len(candidates)))
			allCandidates = append(allCandidates, candidates...)
		}
	}
	
	logInfo(fmt.Sprintf("StrategyRegistry: Collected total of %d window candidates", len(allCandidates)))
	return allCandidates
}

// GetStrategies returns a list of all registered strategies
func (r *StrategyRegistry) GetStrategies() []WindowDetectionStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent external modification
	strategies := make([]WindowDetectionStrategy, len(r.strategies))
	copy(strategies, r.strategies)
	return strategies
}

// GetStrategyByName returns a specific strategy by name
func (r *StrategyRegistry) GetStrategyByName(name string) (WindowDetectionStrategy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, strategy := range r.strategies {
		if strategy.Name() == name {
			return strategy, true
		}
	}
	return nil, false
}

// Clear removes all registered strategies
func (r *StrategyRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.strategies = make([]WindowDetectionStrategy, 0)
	r.enabledStrategies = make(map[string]bool)
}

// Summary returns a summary of registered strategies
func (r *StrategyRegistry) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	summary := fmt.Sprintf("StrategyRegistry: %d strategies registered\n", len(r.strategies))
	for _, strategy := range r.strategies {
		enabled := "enabled"
		if !r.IsEnabled(strategy.Name()) {
			enabled = "disabled"
		}
		summary += fmt.Sprintf("  - %s (priority %d): %s [%s]\n",
			strategy.Name(),
			strategy.Priority(),
			strategy.Description(),
			enabled)
	}
	return summary
}
# Window Detection Strategy Pattern Architecture

## Overview

The window detection system has been refactored to use the **Strategy Pattern**, providing a clean, extensible, and maintainable architecture for detecting session windows. This document describes the new architecture and how to use it.

## Architecture

### Core Components

```
internal/core/session/strategies/
├── interface.go           # Core interfaces and base types
├── registry.go            # Strategy registry for managing strategies
├── adapters.go            # Adapters for existing components
├── history_limit.go       # Priority 10: Historical limit windows
├── current_limit.go       # Priority 9: Current limit messages
├── continuous_activity.go # Priority 8: Strict 5-hour windows
├── history_account.go     # Priority 7: Account-level history
├── gap_detection.go       # Priority 5: Gap-based detection
├── first_message.go       # Priority 3: First message fallback
└── strategies_test.go     # Unit tests
```

## Design Pattern Benefits

### Before (Monolithic Approach)
```go
func collectWindowCandidates(input SessionDetectionInput) []WindowCandidate {
    candidates := make([]WindowCandidate, 0)
    
    // Priority 1: Account-level limit windows from history
    if d.windowHistory != nil {
        // 20+ lines of code...
    }
    
    // Priority 2: Current limit messages
    if len(rawLogs) > 0 {
        // 15+ lines of code...
    }
    
    // Priority 3.5: Continuous activity
    if len(input.GlobalTimeline) > 0 {
        // 30+ lines of code...
    }
    
    // ... more hardcoded priorities
    
    return candidates
}
```

### After (Strategy Pattern)
```go
func collectWindowCandidates(input SessionDetectionInput) []WindowCandidate {
    return registry.CollectCandidates(input)
}
```

## Strategy Interface

Each strategy implements the following interface:

```go
type WindowDetectionStrategy interface {
    // Name returns the unique name of this strategy
    Name() string
    
    // Priority returns the priority (higher is better)
    Priority() int
    
    // Detect analyzes input and returns window candidates
    Detect(input DetectionInput) []WindowCandidate
    
    // Description returns human-readable description
    Description() string
}
```

## Strategy Registry

The registry manages all strategies and provides centralized control:

```go
type StrategyRegistry struct {
    strategies []WindowDetectionStrategy
    enabledStrategies map[string]bool
}

// Key methods
func (r *StrategyRegistry) Register(strategy WindowDetectionStrategy)
func (r *StrategyRegistry) CollectCandidates(input DetectionInput) []WindowCandidate
func (r *StrategyRegistry) EnableStrategy(name string)
func (r *StrategyRegistry) DisableStrategy(name string)
```

## Available Strategies

| Priority | Name | Purpose |
|----------|------|---------|
| 10 | `history_limit` | Historical limit windows from rate limit messages |
| 9 | `limit_message` | Current limit messages with reset times |
| 8 | `continuous_activity` | Strict 5-hour windows for continuous activity |
| 7 | `history_account` | Account-level windows from history |
| 5 | `gap` | Windows after 5+ hour gaps |
| 3 | `first_message` | Window from first activity |

## Usage Examples

### Basic Usage

```go
// Create detector with strategies
detector := NewSessionDetectorWithStrategy(aggregator, "Asia/Shanghai", cacheDir)

// Strategies are automatically registered in priority order
// The detector will use all enabled strategies to detect windows
sessions := detector.DetectSessions(input)
```

### Custom Strategy Implementation

```go
// Implement a custom strategy
type CustomStrategy struct {
    strategies.BaseStrategy
}

func NewCustomStrategy() *CustomStrategy {
    return &CustomStrategy{
        BaseStrategy: strategies.NewBaseStrategy(
            "custom",      // name
            6,            // priority
            "Custom detection logic", // description
        ),
    }
}

func (s *CustomStrategy) Detect(input DetectionInput) []WindowCandidate {
    // Custom detection logic
    candidates := make([]WindowCandidate, 0)
    // ... detection implementation
    return candidates
}

// Register the custom strategy
detector.registry.Register(NewCustomStrategy())
```

### Enabling/Disabling Strategies

```go
// Disable gap detection temporarily
detector.registry.DisableStrategy("gap")

// Re-enable it later
detector.registry.EnableStrategy("gap")

// Check if a strategy is enabled
if detector.registry.IsEnabled("continuous_activity") {
    // Strategy is active
}
```

### Strategy Configuration

```go
// Get all registered strategies
strategies := detector.registry.GetStrategies()
for _, strategy := range strategies {
    fmt.Printf("%s (priority %d): %s\n", 
        strategy.Name(), 
        strategy.Priority(), 
        strategy.Description())
}

// Get a specific strategy
if strategy, found := detector.registry.GetStrategyByName("gap"); found {
    // Use or configure the strategy
}
```

## Advantages of the New Architecture

### 1. **Extensibility**
- Adding new detection strategies requires no changes to core code
- Simply implement the interface and register the strategy

### 2. **Testability**
- Each strategy can be tested in isolation
- Mock strategies can be easily created for testing

### 3. **Maintainability**
- Each strategy is self-contained with single responsibility
- Changes to one strategy don't affect others

### 4. **Configurability**
- Strategies can be enabled/disabled at runtime
- Priority system ensures correct precedence

### 5. **Clarity**
- Main detection flow is simplified
- Strategy logic is clearly organized

### 6. **Reusability**
- Strategies can be shared across different detectors
- Common logic can be extracted to base classes

## Migration Guide

### For Existing Code

1. The original `SessionDetector` still works unchanged
2. To use the new pattern, replace with `SessionDetectorWithStrategy`
3. All existing tests should continue to pass

### For New Features

1. Create a new strategy implementing `WindowDetectionStrategy`
2. Register it with appropriate priority
3. No changes needed to core detection logic

## Testing

### Unit Testing Individual Strategies

```go
func TestMyStrategy(t *testing.T) {
    strategy := NewMyStrategy()
    
    input := strategies.DetectionInput{
        GlobalTimeline: createTestTimeline(),
        SessionDuration: 5 * time.Hour,
    }
    
    candidates := strategy.Detect(input)
    
    // Assert expected candidates
    assert.Equal(t, expectedCount, len(candidates))
}
```

### Integration Testing

```go
func TestStrategyIntegration(t *testing.T) {
    registry := strategies.NewStrategyRegistry()
    registry.Register(NewStrategy1())
    registry.Register(NewStrategy2())
    
    candidates := registry.CollectCandidates(input)
    
    // Verify combined results
}
```

## Performance Considerations

- Strategies run sequentially in priority order
- Consider implementing parallel execution for independent strategies
- Cache results when strategies share expensive computations
- Use early termination when appropriate

## Future Enhancements

1. **Parallel Strategy Execution**: Run independent strategies concurrently
2. **Strategy Composition**: Combine multiple strategies into composite strategies
3. **Configuration File Support**: Load strategy configuration from YAML/JSON
4. **Plugin System**: Load strategies dynamically as plugins
5. **Performance Metrics**: Track strategy execution time and hit rates
6. **Machine Learning**: Use ML to optimize strategy priorities based on historical data

## Conclusion

The Strategy Pattern refactoring provides a solid foundation for the window detection system, making it more maintainable, testable, and extensible while preserving all existing functionality.
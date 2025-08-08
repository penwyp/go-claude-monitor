# Strict 5-Hour Window Division Strategy

## Overview

As of August 2025, go-claude-monitor implements a **strict 5-hour window division strategy** for session detection. This ensures that all Claude usage is divided into precise 5-hour windows, even when activity is continuous across window boundaries.

## Key Principles

### 1. Strict 5-Hour Boundaries
- Every session window is exactly 5 hours (18,000 seconds)
- Windows start at hour boundaries (e.g., 08:00, 13:00, 18:00, 23:00)
- No exceptions, even for continuous activity

### 2. Continuous Activity Handling
- **Old Behavior**: Continuous activity without 5-hour gaps was treated as a single session
- **New Behavior**: Continuous activity is split at 5-hour boundaries into multiple sessions

### 3. Window Priority System

Windows are detected and prioritized in the following order:

| Priority | Source | Description |
|----------|--------|-------------|
| 10 | `history_limit` | Historical limit windows from previous rate limit messages |
| 9 | `limit_message` | Current limit messages with reset times |
| 8 | `continuous_activity` | **NEW: Strict 5-hour windows for continuous activity** |
| 7 | `history_account` | Account-level windows from history |
| 5 | `gap` | Windows created after 5+ hour gaps |
| 3 | `first_message` | Window starting from first activity |

## Implementation Details

### Window Generation Algorithm

```go
// For continuous activity, generate strict 5-hour windows
currentWindowStart := TruncateToHour(firstActivity)

for currentWindowStart <= lastActivity {
    windowEnd := currentWindowStart + 5_hours
    
    // Check if window has activity
    if hasActivityInWindow(currentWindowStart, windowEnd) {
        createWindow(currentWindowStart, windowEnd, "continuous_activity")
    }
    
    // Move to next 5-hour boundary
    currentWindowStart = windowEnd
}
```

### Boundary Handling

Activities at exact window boundaries follow these rules:
- Activity at `timestamp >= windowStart && timestamp < windowEnd` belongs to that window
- Activity at exactly `windowEnd` belongs to the **next** window

Example:
- Window 1: 17:00:00 - 22:00:00
- Window 2: 22:00:00 - 03:00:00
- Activity at 21:59:59 → Window 1
- Activity at 22:00:00 → Window 2

## Real-World Example

### Scenario: Continuous Usage from 17:40 to 00:08 (next day)

**Old Behavior**:
- Single session from 17:00 to 00:08 (7+ hours)

**New Behavior**:
- Session 1: 17:00-22:00 (includes activity from 17:40-21:59)
- Session 2: 22:00-03:00 (includes activity from 22:00-00:08)

### Timeline Visualization

```
Time (CST): 17:00----18:00----19:00----20:00----21:00----22:00----23:00----00:00----01:00----02:00----03:00
Activity:         |------------------continuous activity------------------|
Windows:    |------------ Window 1 (17:00-22:00) ------------|------------ Window 2 (22:00-03:00) ------------|
```

## Benefits

1. **Predictable Rate Limit Management**: Each 5-hour window has clear boundaries
2. **Better Usage Tracking**: Usage is cleanly divided into manageable chunks
3. **Accurate Cost Attribution**: Costs are properly allocated to their respective windows
4. **Compliance with Claude's Rate Limits**: Aligns with Claude's 5-hour session windows

## Configuration

Currently, the strict window division is always enabled. Future versions may add configuration options:

```bash
# Potential future configuration
export WINDOW_STRATEGY=strict_interval  # Default: strict 5-hour windows
export WINDOW_STRATEGY=gap_based        # Legacy: only create new windows after gaps
```

## Migration Guide

### For Existing Users

When upgrading to the version with strict window division:

1. **Window History**: Existing window history is preserved but new windows follow strict rules
2. **Historical Data**: Past sessions remain unchanged; only new detections use strict division
3. **Reports**: Usage reports will show more granular session breakdown

### For Developers

When working with session detection:

1. Always expect multiple sessions for continuous activity > 5 hours
2. Use `WindowSource` field to identify how windows were created
3. `continuous_activity` source indicates strict 5-hour enforcement

## Testing

The implementation includes comprehensive tests:

- `TestStrictFiveHourWindows`: Validates basic strict window division
- `TestContinuousActivityAcrossMultipleWindows`: Tests activity spanning 3+ windows
- `TestBoundaryActivityAssignment`: Ensures correct boundary handling

Run tests with:
```bash
go test -v -run TestStrict ./internal/core/session
go test -v -run TestContinuous ./internal/core/session
go test -v -run TestBoundary ./internal/core/session
```

## Debugging

To see detailed window detection in action:

```bash
# Enable debug logging
LOG_LEVEL=debug go-claude-monitor detect

# Look for these log messages:
# "Generating strict 5-hour windows from..."
# "Added continuous_activity window..."
# "Continuous activity window X-Y: N logs assigned"
```

## FAQ

### Q: Why strict 5-hour windows?
A: Claude's rate limits reset every 5 hours. Strict windows ensure accurate tracking and prevent confusion about when limits reset.

### Q: What happens to sessions with gaps?
A: Gap detection still works. If there's a 5+ hour gap, a new window starts after the gap. Strict windows only apply to continuous activity.

### Q: Can I disable strict window division?
A: Currently no. This is the default and recommended behavior. Future versions may add configuration options.

### Q: How are limit messages handled?
A: Limit messages have the highest priority (9-10) and override continuous activity windows. They provide the most accurate window boundaries.

## Related Documentation

- [Session Detection Patterns](./session-design-patterns.md)
- [Window History Management](./window-history.md)
- [Rate Limit Handling](./rate-limits.md)
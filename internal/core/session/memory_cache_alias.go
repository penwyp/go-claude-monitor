package session

import (
	"github.com/penwyp/go-claude-monitor/internal/core/cache"
	"github.com/penwyp/go-claude-monitor/internal/core/timeline"
)

// Type aliases for backward compatibility during refactoring
type WindowDetectionInfo = cache.WindowDetectionInfo
type MemoryCacheEntry = cache.MemoryCacheEntry
type MemoryCache = cache.MemoryCache

var NewMemoryCache = cache.NewMemoryCache

// TimestampedLog is now in timeline package
type TimestampedLog = timeline.TimestampedLog
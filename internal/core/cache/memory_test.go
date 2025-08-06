package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
)

// int64Ptr returns a pointer to an int64 value
func int64Ptr(v int64) *int64 {
	return &v
}

func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache()
	if cache == nil {
		t.Fatal("NewMemoryCache returned nil")
	}
	if cache.entries == nil {
		t.Error("Expected entries map to be initialized")
	}
	if len(cache.entries) != 0 {
		t.Errorf("Expected empty cache, got %d entries", len(cache.entries))
	}
}

func TestMemoryCacheSetAndGet(t *testing.T) {
	cache := NewMemoryCache()
	sessionId := "session-123"
	
	// Create test entry
	entry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{
				{
					Hour:         time.Now().Unix(),
					TotalTokens:  1000,
					MessageCount: 5,
					ProjectName:  "test-project",
				},
			},
		},
		RawLogs: []model.ConversationLog{
			{
				Type:      "user",
				Timestamp: time.Now().Format(time.RFC3339),
				SessionId: sessionId,
			},
		},
	}
	
	// Test Set
	cache.Set(sessionId, entry)
	
	// Verify entry was set correctly
	retrieved, ok := cache.Get(sessionId)
	if !ok {
		t.Error("Expected to find entry in cache")
	}
	if retrieved == nil {
		t.Fatal("Retrieved entry is nil")
	}
	if len(retrieved.HourlyStats) == 0 || retrieved.HourlyStats[0].TotalTokens != 1000 {
		t.Errorf("Expected TotalTokens 1000, got %d", retrieved.HourlyStats[0].TotalTokens)
	}
	if !retrieved.IsDirty {
		t.Error("Expected entry to be marked as dirty")
	}
	if retrieved.LastAccessed == 0 {
		t.Error("Expected LastAccessed to be set")
	}
	if len(retrieved.RawLogs) != 1 {
		t.Errorf("Expected 1 raw log, got %d", len(retrieved.RawLogs))
	}
}

func TestMemoryCacheGetNonExistent(t *testing.T) {
	cache := NewMemoryCache()
	
	entry, ok := cache.Get("non-existent")
	if ok {
		t.Error("Expected to not find non-existent entry")
	}
	if entry != nil {
		t.Error("Expected nil entry for non-existent key")
	}
}

func TestMemoryCacheGetDirtyEntries(t *testing.T) {
	cache := NewMemoryCache()
	
	// Add clean entry
	cleanEntry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{{TotalTokens: 100}},
		},
		IsDirty: false,
	}
	cache.entries["clean"] = cleanEntry
	
	// Add dirty entries
	dirtyEntry1 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{{TotalTokens: 200}},
		},
		IsDirty: true,
	}
	cache.entries["dirty1"] = dirtyEntry1
	
	dirtyEntry2 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{{TotalTokens: 300}},
		},
		IsDirty: true,
	}
	cache.entries["dirty2"] = dirtyEntry2
	
	// Get dirty entries
	dirtyEntries := cache.GetDirtyEntries()
	
	// Verify results
	if len(dirtyEntries) != 2 {
		t.Errorf("Expected 2 dirty entries, got %d", len(dirtyEntries))
	}
	
	if _, ok := dirtyEntries["clean"]; ok {
		t.Error("Clean entry should not be in dirty entries")
	}
	
	if entry, ok := dirtyEntries["dirty1"]; !ok {
		t.Error("Expected dirty1 to be in dirty entries")
	} else if len(entry.HourlyStats) == 0 || entry.HourlyStats[0].TotalTokens != 200 {
		t.Errorf("Expected dirty1 TotalTokens 200, got %d", entry.HourlyStats[0].TotalTokens)
	}
	
	if entry, ok := dirtyEntries["dirty2"]; !ok {
		t.Error("Expected dirty2 to be in dirty entries")
	} else if len(entry.HourlyStats) == 0 || entry.HourlyStats[0].TotalTokens != 300 {
		t.Errorf("Expected dirty2 TotalTokens 300, got %d", entry.HourlyStats[0].TotalTokens)
	}
	
	// Verify dirty flags were cleared
	if cache.entries["dirty1"].IsDirty {
		t.Error("Expected dirty1 to be marked as clean after GetDirtyEntries")
	}
	if cache.entries["dirty2"].IsDirty {
		t.Error("Expected dirty2 to be marked as clean after GetDirtyEntries")
	}
}

func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache()
	
	// Add some entries
	cache.entries["entry1"] = &MemoryCacheEntry{AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 100}}}}
	cache.entries["entry2"] = &MemoryCacheEntry{AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 200}}}}
	
	if len(cache.entries) != 2 {
		t.Errorf("Expected 2 entries before clear, got %d", len(cache.entries))
	}
	
	// Clear cache
	cache.Clear()
	
	// Verify cache is empty
	if len(cache.entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(cache.entries))
	}
	
	// Verify we can still use the cache
	newEntry := &MemoryCacheEntry{AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 300}}}}
	cache.Set("new", newEntry)
	
	if len(cache.entries) != 1 {
		t.Errorf("Expected 1 entry after adding to cleared cache, got %d", len(cache.entries))
	}
}

func TestMemoryCacheGetCachedWindowInfo(t *testing.T) {
	cache := NewMemoryCache()
	
	// Add entry without window info
	entry1 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 100}}},
		WindowInfo:     nil,
	}
	cache.entries["session1"] = entry1
	
	// Add entry with window info
	windowInfo := &WindowDetectionInfo{
		WindowStartTime:  int64Ptr(1704106800),
		IsWindowDetected: true,
		WindowSource:     "limit_message",
		DetectedAt:       time.Now().Unix(),
		FirstEntryTime:   1704106800,
	}
	entry2 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 200}}},
		WindowInfo:     windowInfo,
	}
	cache.entries["session2"] = entry2
	
	// Get cached window info
	winInfo := cache.GetCachedWindowInfo()
	
	// Verify results
	if len(winInfo) != 1 {
		t.Errorf("Expected 1 window info entry, got %d", len(winInfo))
	}
	
	if _, ok := winInfo["session1"]; ok {
		t.Error("session1 should not have window info")
	}
	
	if info, ok := winInfo["session2"]; !ok {
		t.Error("Expected session2 to have window info")
	} else {
		if !info.IsWindowDetected {
			t.Error("Expected IsWindowDetected to be true")
		}
		if info.WindowSource != "limit_message" {
			t.Errorf("Expected WindowSource 'limit_message', got %s", info.WindowSource)
		}
		if info.WindowStartTime == nil || *info.WindowStartTime != 1704106800 {
			t.Errorf("Expected WindowStartTime 1704106800, got %v", info.WindowStartTime)
		}
		if info.FirstEntryTime != 1704106800 {
			t.Errorf("Expected FirstEntryTime 1704106800, got %d", info.FirstEntryTime)
		}
	}
}

func TestMemoryCacheGetRecentDataWithLogs(t *testing.T) {
	cache := NewMemoryCache()
	now := time.Now()
	duration := int64(3600) // 1 hour
	
	// Add entry with recent data
	recentHourly := aggregator.HourlyData{
		Hour:         now.Add(-30 * time.Minute).Unix(), // 30 minutes ago
		TotalTokens:  100,
		MessageCount: 5,
		ProjectName:  "recent-project",
	}
	
	// Add entry with old data
	oldHourly := aggregator.HourlyData{
		Hour:         now.Add(-2 * time.Hour).Unix(), // 2 hours ago (outside duration)
		TotalTokens:  200,
		MessageCount: 10,
		ProjectName:  "old-project",
	}
	
	recentLog := model.ConversationLog{
		Type:      "user",
		Timestamp: now.Add(-15 * time.Minute).Format(time.RFC3339), // 15 minutes ago
		SessionId: "session1",
	}
	
	oldLog := model.ConversationLog{
		Type:      "user",
		Timestamp: now.Add(-90 * time.Minute).Format(time.RFC3339), // 90 minutes ago (outside duration)
		SessionId: "session1",
	}
	
	entry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{recentHourly, oldHourly},
		},
		RawLogs: []model.ConversationLog{recentLog, oldLog},
	}
	cache.entries["session1"] = entry
	
	// Get recent data
	hourlyData, rawLogs := cache.GetRecentDataWithLogs(duration)
	
	// Verify hourly data
	if len(hourlyData) != 1 {
		t.Errorf("Expected 1 recent hourly data entry, got %d", len(hourlyData))
	} else {
		if hourlyData[0].ProjectName != "recent-project" {
			t.Errorf("Expected recent-project, got %s", hourlyData[0].ProjectName)
		}
		if hourlyData[0].TotalTokens != 100 {
			t.Errorf("Expected 100 tokens, got %d", hourlyData[0].TotalTokens)
		}
	}
	
	// Verify raw logs
	if len(rawLogs) != 1 {
		t.Errorf("Expected 1 recent raw log, got %d", len(rawLogs))
	} else {
		if rawLogs[0].SessionId != "session1" {
			t.Errorf("Expected session1, got %s", rawLogs[0].SessionId)
		}
	}
}

func TestMemoryCacheGetRecentDataWithInvalidTimestamps(t *testing.T) {
	cache := NewMemoryCache()
	now := time.Now()
	duration := int64(3600) // 1 hour
	
	// Add entry with invalid timestamp log
	invalidLog := model.ConversationLog{
		Type:      "user",
		Timestamp: "invalid-timestamp",
		SessionId: "session1",
	}
	
	validLog := model.ConversationLog{
		Type:      "user",
		Timestamp: now.Add(-15 * time.Minute).Format(time.RFC3339),
		SessionId: "session1",
	}
	
	entry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{
			HourlyStats: []aggregator.HourlyData{},
		},
		RawLogs: []model.ConversationLog{invalidLog, validLog},
	}
	cache.entries["session1"] = entry
	
	// Get recent data
	hourlyData, rawLogs := cache.GetRecentDataWithLogs(duration)
	
	// Should only get the valid log
	if len(rawLogs) != 1 {
		t.Errorf("Expected 1 valid raw log, got %d", len(rawLogs))
	} else {
		if rawLogs[0].Timestamp == "invalid-timestamp" {
			t.Error("Should not include log with invalid timestamp")
		}
	}
	
	if len(hourlyData) != 0 {
		t.Errorf("Expected 0 hourly data entries, got %d", len(hourlyData))
	}
}

func TestMemoryCacheUpdateWindowInfo(t *testing.T) {
	cache := NewMemoryCache()
	sessionId := "session-123"
	
	// Add initial entry
	entry := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 100}}},
		IsDirty:        false,
		WindowInfo:     nil,
	}
	cache.entries[sessionId] = entry
	
	// Update window info
	windowInfo := &WindowDetectionInfo{
		WindowStartTime:  int64Ptr(1704106800),
		IsWindowDetected: true,
		WindowSource:     "gap",
		DetectedAt:       time.Now().Unix(),
		FirstEntryTime:   1704106800,
	}
	
	cache.UpdateWindowInfo(sessionId, windowInfo)
	
	// Verify update
	updatedEntry, ok := cache.Get(sessionId)
	if !ok {
		t.Fatal("Expected to find updated entry")
	}
	
	if updatedEntry.WindowInfo == nil {
		t.Fatal("Expected WindowInfo to be set")
	}
	
	if !updatedEntry.WindowInfo.IsWindowDetected {
		t.Error("Expected IsWindowDetected to be true")
	}
	
	if updatedEntry.WindowInfo.WindowSource != "gap" {
		t.Errorf("Expected WindowSource 'gap', got %s", updatedEntry.WindowInfo.WindowSource)
	}
	
	if !updatedEntry.IsDirty {
		t.Error("Expected entry to be marked as dirty after window info update")
	}
}

func TestMemoryCacheUpdateWindowInfoNonExistentSession(t *testing.T) {
	cache := NewMemoryCache()
	
	// Try to update window info for non-existent session
	windowInfo := &WindowDetectionInfo{
		WindowStartTime:  int64Ptr(1704106800),
		IsWindowDetected: true,
		WindowSource:     "limit_message",
	}
	
	// Should not panic
	cache.UpdateWindowInfo("non-existent", windowInfo)
	
	// Verify no entry was created
	if len(cache.entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(cache.entries))
	}
}

func TestMemoryCacheGetHistoricalLogs(t *testing.T) {
	cache := NewMemoryCache()
	now := time.Now()
	duration := int64(7200) // 2 hours
	
	// Add entries with logs at different times
	recentLog := model.ConversationLog{
		Type:      "user",
		Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339), // 30 minutes ago
		Content:   "Recent message",
	}
	
	oldLog := model.ConversationLog{
		Type:      "user",
		Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), // 3 hours ago (outside duration)
		Content:   "Old message",
	}
	
	invalidLog := model.ConversationLog{
		Type:      "user",
		Timestamp: "invalid-timestamp",
		Content:   "Invalid timestamp message",
	}
	
	entry1 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{},
		RawLogs:        []model.ConversationLog{recentLog, oldLog},
	}
	cache.entries["session1"] = entry1
	
	entry2 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{},
		RawLogs:        []model.ConversationLog{invalidLog},
	}
	cache.entries["session2"] = entry2
	
	entry3 := &MemoryCacheEntry{
		AggregatedData: &aggregator.AggregatedData{},
		RawLogs:        nil, // No logs
	}
	cache.entries["session3"] = entry3
	
	// Get historical logs
	historicalLogs := cache.GetHistoricalLogs(duration)
	
	// Should only get the recent log
	if len(historicalLogs) != 1 {
		t.Errorf("Expected 1 historical log, got %d", len(historicalLogs))
	} else {
		if historicalLogs[0].Content != "Recent message" {
			t.Errorf("Expected 'Recent message', got %s", historicalLogs[0].Content)
		}
	}
}

// Test concurrent access
func TestMemoryCacheConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache()
	numGoroutines := 10
	numOperations := 100
	
	// Channel to coordinate goroutines
	done := make(chan bool, numGoroutines)
	
	// Start multiple goroutines performing different operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			for j := 0; j < numOperations; j++ {
				sessionId := fmt.Sprintf("session-%d-%d", id, j)
				
				// Set entry
				entry := &MemoryCacheEntry{
					AggregatedData: &aggregator.AggregatedData{
						HourlyStats: []aggregator.HourlyData{{TotalTokens: id*1000 + j}},
					},
				}
				cache.Set(sessionId, entry)
				
				// Get entry
				retrieved, ok := cache.Get(sessionId)
				if !ok {
					t.Errorf("Failed to retrieve entry %s", sessionId)
					continue
				}
				
				expectedTokens := id*1000 + j
				if len(retrieved.AggregatedData.HourlyStats) == 0 || retrieved.AggregatedData.HourlyStats[0].TotalTokens != expectedTokens {
					if len(retrieved.AggregatedData.HourlyStats) > 0 {
						t.Errorf("Expected %d tokens, got %d", expectedTokens, retrieved.AggregatedData.HourlyStats[0].TotalTokens)
					} else {
						t.Error("No hourly stats found")
					}
				}
				
				// Update window info
				windowInfo := &WindowDetectionInfo{
					WindowSource: fmt.Sprintf("source-%d", j),
				}
				cache.UpdateWindowInfo(sessionId, windowInfo)
			}
			
			// Get dirty entries
			cache.GetDirtyEntries()
			
			// Get cached window info
			cache.GetCachedWindowInfo()
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations to complete")
		}
	}
	
	// Verify final state
	expectedEntries := numGoroutines * numOperations
	if len(cache.entries) != expectedEntries {
		t.Errorf("Expected %d entries after concurrent operations, got %d", expectedEntries, len(cache.entries))
	}
}

// Test edge cases
func TestMemoryCacheEdgeCases(t *testing.T) {
	t.Run("nil_entry", func(t *testing.T) {
		cache := NewMemoryCache()
		// This should not panic
		cache.Set("test", nil)
		
		entry, ok := cache.Get("test")
		if !ok {
			t.Error("Expected to find nil entry")
		}
		if entry != nil {
			t.Error("Expected nil entry")
		}
	})
	
	t.Run("empty_session_id", func(t *testing.T) {
		cache := NewMemoryCache()
		entry := &MemoryCacheEntry{
			AggregatedData: &aggregator.AggregatedData{HourlyStats: []aggregator.HourlyData{{TotalTokens: 100}}},
		}
		
		cache.Set("", entry)
		retrieved, ok := cache.Get("")
		if !ok {
			t.Error("Expected to find entry with empty key")
		}
		if len(retrieved.AggregatedData.HourlyStats) == 0 || retrieved.AggregatedData.HourlyStats[0].TotalTokens != 100 {
			if len(retrieved.AggregatedData.HourlyStats) > 0 {
				t.Errorf("Expected 100 tokens, got %d", retrieved.AggregatedData.HourlyStats[0].TotalTokens)
			} else {
				t.Error("No hourly stats found")
			}
		}
	})
	
	t.Run("zero_duration", func(t *testing.T) {
		cache := NewMemoryCache()
		now := time.Now()
		
		entry := &MemoryCacheEntry{
			AggregatedData: &aggregator.AggregatedData{
				HourlyStats: []aggregator.HourlyData{
					{Hour: now.Unix(), TotalTokens: 100},
				},
			},
			RawLogs: []model.ConversationLog{
				{Timestamp: now.Format(time.RFC3339)},
			},
		}
		cache.entries["test"] = entry
		
		// With zero duration, should get all data
		hourlyData, rawLogs := cache.GetRecentDataWithLogs(0)
		if len(hourlyData) != 1 {
			t.Errorf("Expected 1 hourly data with zero duration, got %d", len(hourlyData))
		}
		if len(rawLogs) != 1 {
			t.Errorf("Expected 1 raw log with zero duration, got %d", len(rawLogs))
		}
	})
	
	t.Run("negative_duration", func(t *testing.T) {
		cache := NewMemoryCache()
		now := time.Now()
		
		entry := &MemoryCacheEntry{
			AggregatedData: &aggregator.AggregatedData{
				HourlyStats: []aggregator.HourlyData{
					{Hour: now.Unix(), TotalTokens: 100},
				},
			},
			RawLogs: []model.ConversationLog{
				{Timestamp: now.Format(time.RFC3339)},
			},
		}
		cache.entries["test"] = entry
		
		// With negative duration, should get all data
		hourlyData, rawLogs := cache.GetRecentDataWithLogs(-3600)
		if len(hourlyData) != 1 {
			t.Errorf("Expected 1 hourly data with negative duration, got %d", len(hourlyData))
		}
		if len(rawLogs) != 1 {
			t.Errorf("Expected 1 raw log with negative duration, got %d", len(rawLogs))
		}
	})
}

// Helper function

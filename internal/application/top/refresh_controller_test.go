package top

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncrementalDetection(t *testing.T) {
	// Create test config
	config := &TopConfig{
		DataDir:             "/tmp/test_data",
		CacheDir:            "/tmp/test_cache",
		Plan:                "max5",
		CustomLimitTokens:   0,
		Timezone:            "UTC",
		TimeFormat:          "15:04:05",
		DataRefreshInterval: 30 * time.Second,
		UIRefreshRate:       2.0,
		Concurrency:         4,
		PricingSource:       "default",
		PricingOfflineMode:  true,
	}

	// Create data loader
	dataLoader, err := NewDataLoader(config)
	require.NoError(t, err)

	// Create detector and calculator
	detector := session.NewSessionDetectorWithAggregator(nil, config.Timezone, config.CacheDir)
	planLimits := pricing.GetPlanWithDefault(config.Plan, config.CustomLimitTokens)
	calculator := session.NewMetricsCalculator(planLimits)

	// Create state manager
	stateManager := NewStateManager()

	// Create refresh controller
	refreshCtrl := NewRefreshController(dataLoader, detector, calculator, stateManager)

	t.Run("incremental_with_no_changes", func(t *testing.T) {
		// Test that incremental detection with no changed files falls back to full detection
		sessions, err := refreshCtrl.IncrementalDetect([]string{})
		assert.NoError(t, err)
		assert.NotNil(t, sessions)
	})

	t.Run("incremental_with_existing_windows", func(t *testing.T) {
		// Create some test sessions
		testSessions := []*session.Session{
			{
				ID:               "session1",
				StartTime:        time.Now().Add(-10 * time.Hour).Unix(),
				EndTime:          time.Now().Add(-5 * time.Hour).Unix(),
				IsWindowDetected: true,
				WindowSource:     "gap",
				Projects:         make(map[string]*session.ProjectStats),
				ModelDistribution: make(map[string]*model.ModelStats),
				PerModelStats:    make(map[string]map[string]interface{}),
				HourlyMetrics:    make([]*model.HourlyMetric, 0),
				LimitMessages:    make([]map[string]interface{}, 0),
				ProjectionData:   make(map[string]interface{}),
			},
			{
				ID:               "session2",
				StartTime:        time.Now().Add(-5 * time.Hour).Unix(),
				EndTime:          time.Now().Unix(),
				IsWindowDetected: true,
				WindowSource:     "gap",
				Projects:         make(map[string]*session.ProjectStats),
				ModelDistribution: make(map[string]*model.ModelStats),
				PerModelStats:    make(map[string]map[string]interface{}),
				HourlyMetrics:    make([]*model.HourlyMetric, 0),
				LimitMessages:    make([]map[string]interface{}, 0),
				ProjectionData:   make(map[string]interface{}),
			},
		}

		// Set current sessions in state manager
		stateManager.SetSessions(testSessions)

		// Test incremental detection with changed files
		// Since we don't have actual files, this will test the logic path
		sessions, err := refreshCtrl.IncrementalDetect([]string{"test_file"})
		assert.NoError(t, err)
		assert.NotNil(t, sessions)
	})
}
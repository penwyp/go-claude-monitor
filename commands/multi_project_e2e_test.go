//go:build e2e
// +build e2e

package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/e2e"
	"github.com/penwyp/go-claude-monitor/internal/testing/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiProjectBasicAnalysis tests basic multi-project analysis functionality
func TestMultiProjectBasicAnalysis(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create diverse set of projects with different characteristics
	now := time.Now()
	projects := []struct {
		name      string
		startTime time.Time
		generator func(string, time.Time) error
	}{
		{"web-frontend", now.Add(-4 * time.Hour), generator.GenerateSimpleSession},
		{"api-backend", now.Add(-3 * time.Hour), generator.GenerateMultiModelSession},
		{"data-analysis", now.Add(-6 * time.Hour), generator.GenerateContinuousActivity},
		{"mobile-app", now.Add(-2 * time.Hour), generator.GenerateSimpleSession},
		{"ml-pipeline", now.Add(-8 * time.Hour), generator.GenerateSessionWithLimit},
	}
	
	for _, proj := range projects {
		err := proj.generator(proj.name, proj.startTime)
		require.NoError(t, err, "Failed to generate project %s", proj.name)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multi-project analysis with JSON output for verification
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project analysis should succeed: %s", string(output))
	
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Should produce valid JSON output")
	
	// Verify all projects are present
	projectsData := result["projects"].(map[string]interface{})
	assert.Equal(t, len(projects), len(projectsData), "Should have all projects in output")
	
	for _, proj := range projects {
		assert.Contains(t, projectsData, proj.name, "Should include project %s", proj.name)
		
		// Verify project has expected data structure
		projectData := projectsData[proj.name].(map[string]interface{})
		assert.Contains(t, projectData, "total_cost", "Project %s should have total_cost", proj.name)
		assert.Contains(t, projectData, "total_tokens", "Project %s should have total_tokens", proj.name)
		assert.Contains(t, projectData, "models", "Project %s should have models", proj.name)
	}
	
	// Verify summary information
	if summary, hasSummary := result["summary"]; hasSummary {
		summaryData := summary.(map[string]interface{})
		assert.Contains(t, summaryData, "total_projects", "Should have total project count")
		
		totalProjects := summaryData["total_projects"].(float64)
		assert.Equal(t, float64(len(projects)), totalProjects, "Total project count should match")
	}
}

// TestMultiProjectAggregation tests aggregation across multiple projects
func TestMultiProjectAggregation(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create projects with known token amounts for aggregation testing
	now := time.Now()
	projectData := []struct {
		name   string
		tokens int
		models []string
	}{
		{"project-alpha", 1000, []string{"claude-3.5-sonnet"}},
		{"project-beta", 2000, []string{"claude-3-opus", "claude-3-haiku"}},
		{"project-gamma", 1500, []string{"claude-3.5-sonnet", "claude-3-opus"}},
	}
	
	expectedTotalTokens := 0
	for _, proj := range projectData {
		expectedTotalTokens += proj.tokens
		
		// Generate session based on whether it's single or multi-model
		if len(proj.models) == 1 {
			err := generator.GenerateSimpleSession(proj.name, now.Add(-2*time.Hour))
			require.NoError(t, err)
		} else {
			err := generator.GenerateMultiModelSession(proj.name, now.Add(-2*time.Hour))
			require.NoError(t, err)
		}
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test aggregation with breakdown
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--breakdown", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project breakdown should succeed: %s", string(output))
	
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Should produce valid JSON with breakdown")
	
	// Verify individual project aggregation
	projectsData := result["projects"].(map[string]interface{})
	
	totalCostAcrossProjects := 0.0
	totalTokensAcrossProjects := 0.0
	
	for _, proj := range projectData {
		projectResult := projectsData[proj.name].(map[string]interface{})
		
		projectCost := projectResult["total_cost"].(float64)
		projectTokens := projectResult["total_tokens"].(float64)
		
		assert.Greater(t, projectCost, 0.0, "Project %s should have non-zero cost", proj.name)
		assert.Greater(t, projectTokens, 0.0, "Project %s should have non-zero tokens", proj.name)
		
		totalCostAcrossProjects += projectCost
		totalTokensAcrossProjects += projectTokens
	}
	
	// Verify totals are reasonable
	assert.Greater(t, totalCostAcrossProjects, 0.0, "Total cost should be positive")
	assert.Greater(t, totalTokensAcrossProjects, 0.0, "Total tokens should be positive")
}

// TestMultiProjectTimeGrouping tests time-based grouping with multiple projects
func TestMultiProjectTimeGrouping(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create projects across different time periods
	now := time.Now()
	timeBasedProjects := []struct {
		name string
		time time.Time
	}{
		{"yesterday-morning", now.Add(-30 * time.Hour)},
		{"yesterday-evening", now.Add(-18 * time.Hour)},
		{"today-morning", now.Add(-8 * time.Hour)},
		{"today-afternoon", now.Add(-4 * time.Hour)},
		{"recent", now.Add(-1 * time.Hour)},
	}
	
	for _, proj := range timeBasedProjects {
		err := generator.GenerateSimpleSession(proj.name, proj.time)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test different grouping strategies
	testCases := []struct {
		groupBy     string
		duration    string
		description string
	}{
		{"hour", "24h", "hourly grouping for last 24h"},
		{"day", "48h", "daily grouping for last 48h"},
		{"week", "168h", "weekly grouping for last week"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--group-by", tc.groupBy, "--duration", tc.duration, "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Time grouping should succeed: %s", string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON for time grouping")
			
			// Should have grouped data
			projectsData := result["projects"].(map[string]interface{})
			assert.Greater(t, len(projectsData), 0, "Should have grouped projects for %s", tc.description)
			
			// Verify time-based grouping worked
			foundProjects := 0
			for projectName := range projectsData {
				for _, proj := range timeBasedProjects {
					if strings.Contains(projectName, proj.name) {
						foundProjects++
						break
					}
				}
			}
			assert.Greater(t, foundProjects, 0, "Should find time-grouped projects for %s", tc.description)
		})
	}
}

// TestMultiProjectTopCommand tests top command with multiple projects
func TestMultiProjectTopCommand(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create multiple active projects
	now := time.Now()
	activeProjects := []string{
		"top-project-1",
		"top-project-2", 
		"top-project-3",
		"top-project-4",
		"top-project-5",
	}
	
	for i, proj := range activeProjects {
		// Stagger project times slightly
		startTime := now.Add(time.Duration(-2-i) * time.Hour)
		err := generator.GenerateMultiModelSession(proj, startTime)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	config := &e2e.TUITestConfig{
		Command: binaryPath,
		Args:    []string{"--dir", tempDir, "top"},
		Timeout: 10 * time.Second,
		Rows:    30,
		Cols:    120,
	}

	session, err := e2e.NewTUITestSession(config)
	require.NoError(t, err)
	defer session.ForceStop()

	// Wait for data to load
	time.Sleep(2 * time.Second)

	// Should display multiple projects
	topOutput := session.GetCleanOutput()
	
	projectsFound := 0
	for _, proj := range activeProjects {
		if strings.Contains(topOutput, proj) {
			projectsFound++
		}
	}
	
	assert.Greater(t, projectsFound, 2, "Should display multiple projects in top view")
	
	// Test sorting with multiple projects
	err = session.SendKey('c') // Sort by cost
	require.NoError(t, err)
	
	time.Sleep(500 * time.Millisecond)
	
	sortedOutput := session.GetCleanOutput()
	assert.NotEqual(t, topOutput, sortedOutput, "Sorting should change the display")
	
	// Should still show projects after sorting
	sortedProjectsFound := 0
	for _, proj := range activeProjects {
		if strings.Contains(sortedOutput, proj) {
			sortedProjectsFound++
		}
	}
	
	assert.Greater(t, sortedProjectsFound, 2, "Should still show multiple projects after sorting")

	err = session.Stop()
	assert.NoError(t, err)
}

// TestMultiProjectDetection tests session detection across multiple projects
func TestMultiProjectDetection(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create projects with different session characteristics
	now := time.Now()
	detectionProjects := []struct {
		name      string
		generator func(string, time.Time) error
		time      time.Time
	}{
		{"simple-detection", generator.GenerateSimpleSession, now.Add(-2 * time.Hour)},
		{"continuous-detection", generator.GenerateContinuousActivity, now.Add(-5 * time.Hour)},
		{"limited-detection", generator.GenerateSessionWithLimit, now.Add(-8 * time.Hour)},
		{"multi-model-detection", generator.GenerateMultiModelSession, now.Add(-3 * time.Hour)},
	}
	
	for _, proj := range detectionProjects {
		err := proj.generator(proj.name, proj.time)
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multi-project session detection
	cmd := exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project detection should succeed: %s", string(output))
	outputStr := string(output)
	
	// Should detect all projects
	for _, proj := range detectionProjects {
		assert.Contains(t, outputStr, proj.name, "Should detect project %s", proj.name)
	}
	
	// Should show comprehensive analysis
	assert.Contains(t, outputStr, "Session Detection Analysis", "Should show detection analysis")
	assert.Contains(t, outputStr, "Window Detection", "Should show window detection statistics")
	assert.Contains(t, outputStr, "Model Distribution", "Should show model distribution")
	
	// Should show detection method indicators
	detectionIcons := []string{"🎯", "⏳", "📍"}
	hasDetectionIcon := false
	for _, icon := range detectionIcons {
		if strings.Contains(outputStr, icon) {
			hasDetectionIcon = true
			break
		}
	}
	assert.True(t, hasDetectionIcon, "Should show detection method icons")
}

// TestMultiProjectScalability tests scalability with many projects
func TestMultiProjectScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create many small projects
	numProjects := 50
	now := time.Now()
	
	for i := 0; i < numProjects; i++ {
		projectName := fmt.Sprintf("scale-project-%02d", i)
		startTime := now.Add(time.Duration(-i-1) * time.Hour)
		
		// Alternate between different session types
		if i%3 == 0 {
			err := generator.GenerateSimpleSession(projectName, startTime)
			require.NoError(t, err)
		} else if i%3 == 1 {
			err := generator.GenerateMultiModelSession(projectName, startTime)
			require.NoError(t, err)
		} else {
			err := generator.GenerateContinuousActivity(projectName, startTime)
			require.NoError(t, err)
		}
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test performance with many projects
	testCases := []struct {
		command     []string
		maxDuration time.Duration
		description string
	}{
		{
			command:     []string{"--dir", tempDir, "--duration", "72h", "--output", "json"},
			maxDuration: 10 * time.Second,
			description: "root command with many projects",
		},
		{
			command:     []string{"--dir", tempDir, "detect"},
			maxDuration: 15 * time.Second,
			description: "detect command with many projects", 
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			startTime := time.Now()
			
			cmd := exec.Command(binaryPath, tc.command...)
			output, err := cmd.CombinedOutput()
			
			elapsed := time.Since(startTime)
			
			assert.NoError(t, err, "%s should succeed: %s", tc.description, string(output))
			assert.Less(t, elapsed, tc.maxDuration, 
				"%s should complete within %v, took %v", tc.description, tc.maxDuration, elapsed)
			
			if strings.Contains(tc.description, "root command") {
				// Verify JSON output for root command
				var result map[string]interface{}
				err = json.Unmarshal(output, &result)
				assert.NoError(t, err, "Should produce valid JSON with many projects")
				
				projectsData := result["projects"].(map[string]interface{})
				assert.Greater(t, len(projectsData), numProjects/2, 
					"Should handle large number of projects")
			} else {
				// Verify detect output
				outputStr := string(output)
				assert.Contains(t, outputStr, "Sessions Found", "Should show session analysis")
				
				// Should contain some of the projects
				projectsFound := 0
				for i := 0; i < 10; i++ { // Check first 10 projects
					projectName := fmt.Sprintf("scale-project-%02d", i)
					if strings.Contains(outputStr, projectName) {
						projectsFound++
					}
				}
				assert.Greater(t, projectsFound, 3, "Should find multiple projects in detect output")
			}
		})
	}
}

// TestMultiProjectConcurrentAccess tests concurrent access patterns
func TestMultiProjectConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}
	
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create moderate number of projects for concurrent testing
	numProjects := 10
	now := time.Now()
	
	for i := 0; i < numProjects; i++ {
		projectName := fmt.Sprintf("concurrent-project-%d", i)
		err := generator.GenerateMultiModelSession(projectName, now.Add(time.Duration(-i-1)*time.Hour))
		require.NoError(t, err)
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multiple top command sessions running concurrently
	numSessions := 3
	sessions := make([]*e2e.TUITestSession, numSessions)

	for i := 0; i < numSessions; i++ {
		config := &e2e.TUITestConfig{
			Command: binaryPath,
			Args:    []string{"--dir", tempDir, "top", "--refresh-per-second", "2"},
			Timeout: 10 * time.Second,
		}

		session, err := e2e.NewTUITestSession(config)
		require.NoError(t, err, "Concurrent session %d should start", i)
		sessions[i] = session
		defer session.ForceStop()
		
		// Stagger startup
		time.Sleep(200 * time.Millisecond)
	}

	// Let sessions run concurrently
	time.Sleep(3 * time.Second)

	// Verify all sessions are functional
	for i, session := range sessions {
		// Test interaction
		err = session.SendKey('c') // Sort by cost
		require.NoError(t, err, "Session %d should accept input", i)
		
		time.Sleep(300 * time.Millisecond)
		
		output := session.GetCleanOutput()
		assert.Contains(t, output, "concurrent-project", "Session %d should show project data", i)
		
		// Test additional sorting
		err = session.SendKey('t') // Sort by tokens
		require.NoError(t, err, "Session %d should handle multiple sorts", i)
		
		time.Sleep(200 * time.Millisecond)
	}

	// Clean shutdown of all sessions
	for i, session := range sessions {
		err = session.Stop()
		assert.NoError(t, err, "Session %d should shutdown cleanly", i)
	}
}

// TestMultiProjectErrorRecovery tests error recovery with multiple projects
func TestMultiProjectErrorRecovery(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)
	
	// Create mix of valid and potentially problematic projects
	now := time.Now()
	
	// Valid projects
	err := generator.GenerateSimpleSession("valid-project-1", now.Add(-2*time.Hour))
	require.NoError(t, err)
	
	err = generator.GenerateMultiModelSession("valid-project-2", now.Add(-3*time.Hour))
	require.NoError(t, err)
	
	// Create empty project (edge case)
	err = generator.CreateEmptyProject("empty-project")
	require.NoError(t, err)
	
	// Create project with rate limit (complex case)
	err = generator.GenerateSessionWithLimit("complex-project", now.Add(-5*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test that analysis succeeds despite mixed project types
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project analysis should handle mixed project types: %s", string(output))
	
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Should produce valid JSON despite mixed projects")
	
	// Should include valid projects
	projectsData := result["projects"].(map[string]interface{})
	assert.Contains(t, projectsData, "valid-project-1", "Should include valid project 1")
	assert.Contains(t, projectsData, "valid-project-2", "Should include valid project 2")
	assert.Contains(t, projectsData, "complex-project", "Should include complex project")
	
	// Empty project may or may not be included depending on implementation
	
	// Test detect command error recovery
	cmd = exec.Command(binaryPath, "--dir", tempDir, "detect")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Detect should handle mixed project types: %s", string(output))
	outputStr := string(output)
	
	// Should show analysis despite mixed project types
	assert.Contains(t, outputStr, "Session Detection", "Should show session detection analysis")
	assert.Contains(t, outputStr, "valid-project-1", "Should detect valid project 1")
	assert.Contains(t, outputStr, "valid-project-2", "Should detect valid project 2")
}


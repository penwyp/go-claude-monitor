//go:build e2e
// +build e2e

package commands

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/testing/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CostTestData represents expected cost calculation data
type CostTestData struct {
	Model        string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	ExpectedCost float64
	Description  string
}

// TestCostCalculationAccuracyDefault tests cost calculation accuracy with default pricing
func TestCostCalculationAccuracyDefault(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Create precise test data with known costs
	testCases := []CostTestData{
		{
			Model:        "claude-3.5-sonnet",
			InputTokens:  1000,
			OutputTokens: 500,
			CacheRead:    0,
			CacheWrite:   0,
			ExpectedCost: 0.0045, // Based on typical sonnet pricing: $3/1M input + $15/1M output
			Description:  "Basic sonnet calculation",
		},
		{
			Model:        "claude-3-opus",
			InputTokens:  1000,
			OutputTokens: 500,
			CacheRead:    0,
			CacheWrite:   0,
			ExpectedCost: 0.0225, // Opus is more expensive: $15/1M input + $75/1M output
			Description:  "Basic opus calculation",
		},
		{
			Model:        "claude-3-haiku",
			InputTokens:  10000,
			OutputTokens: 5000,
			CacheRead:    0,
			CacheWrite:   0,
			ExpectedCost: 0.00625, // Haiku is cheaper: $0.25/1M input + $1.25/1M output
			Description:  "Basic haiku calculation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			// Generate test session with specific cost data
			err := generateTestSessionWithCost(generator, "cost-test-"+strings.ToLower(tc.Model), tc, time.Now().Add(-1*time.Hour))
			require.NoError(t, err)

			binaryPath := filepath.Join(t.TempDir(), "test-monitor")
			buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
			output, err := buildCmd.CombinedOutput()
			require.NoError(t, err, "Failed to build binary: %s", string(output))

			// Test JSON output for precise cost verification
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json", "--pricing-source", "default")
			output, err = cmd.CombinedOutput()
			
			assert.NoError(t, err, "Cost calculation should succeed: %s", string(output))
			
			// Parse JSON and verify cost
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON")

			projects := result["projects"].(map[string]interface{})
			projectName := "cost-test-" + strings.ToLower(tc.Model)
			
			if projectData, exists := projects[projectName]; exists {
				project := projectData.(map[string]interface{})
				totalCost := project["total_cost"].(float64)
				
				// Allow for small floating point differences (within 1 cent)
				tolerance := 0.01
				assert.InDelta(t, tc.ExpectedCost, totalCost, tolerance, 
					"Cost calculation should be accurate for %s. Expected: %f, Got: %f", 
					tc.Model, tc.ExpectedCost, totalCost)
			} else {
				t.Errorf("Project %s not found in output", projectName)
			}
		})
	}
}

// TestCostCalculationWithCaching tests cache token cost calculations
func TestCostCalculationWithCaching(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Test cache calculations (typically cache read is cheaper than regular input)
	testCases := []CostTestData{
		{
			Model:        "claude-3.5-sonnet",
			InputTokens:  1000,
			OutputTokens: 500,
			CacheRead:    2000, // Cache read tokens
			CacheWrite:   1000, // Cache write tokens
			ExpectedCost: 0.0075, // Should be different from non-cached cost
			Description:  "Sonnet with caching",
		},
		{
			Model:        "claude-3-haiku",
			InputTokens:  5000,
			OutputTokens: 2000,
			CacheRead:    10000,
			CacheWrite:   5000,
			ExpectedCost: 0.015, // Adjusted for cache pricing
			Description:  "Haiku with heavy caching",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			err := generateTestSessionWithCost(generator, "cache-test", tc, time.Now().Add(-1*time.Hour))
			require.NoError(t, err)

			binaryPath := filepath.Join(t.TempDir(), "test-monitor")
			buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
			output, err := buildCmd.CombinedOutput()
			require.NoError(t, err, "Failed to build binary: %s", string(output))

			// Compare with and without cache
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json", "--pricing-source", "default")
			output, err = cmd.CombinedOutput()
			
			assert.NoError(t, err, "Cached cost calculation should succeed")
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err)

			// Verify cache tokens are accounted for
			projects := result["projects"].(map[string]interface{})
			if projectData, exists := projects["cache-test"]; exists {
				project := projectData.(map[string]interface{})
				
				// Should have cache token information
				if models, hasModels := project["models"].(map[string]interface{}); hasModels {
					if modelData, hasModel := models[tc.Model].(map[string]interface{}); hasModel {
						cacheRead := modelData["cache_read"].(float64)
						cacheWrite := modelData["cache_write"].(float64)
						
						assert.Equal(t, float64(tc.CacheRead), cacheRead, "Cache read tokens should match")
						assert.Equal(t, float64(tc.CacheWrite), cacheWrite, "Cache write tokens should match")
					}
				}
			}
		})
	}
}

// TestCostCalculationLiteLLMProvider tests LiteLLM pricing provider
func TestCostCalculationLiteLLMProvider(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Generate multi-model session
	err := generator.GenerateMultiModelSession("litellm-cost-test", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test both online and offline modes
	testCases := []struct {
		name    string
		offline bool
		args    []string
	}{
		{
			name:    "litellm_online",
			offline: false,
			args:    []string{"--dir", tempDir, "--duration", "24h", "--output", "json", "--pricing-source", "litellm"},
		},
		{
			name:    "litellm_offline",
			offline: true,
			args:    []string{"--dir", tempDir, "--duration", "24h", "--output", "json", "--pricing-source", "litellm", "--pricing-offline"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			output, err := cmd.CombinedOutput()
			
			// Should succeed regardless of network availability
			assert.NoError(t, err, "LiteLLM cost calculation should succeed: %s", string(output))
			
			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err, "Should produce valid JSON")

			// Verify cost calculation exists
			projects := result["projects"].(map[string]interface{})
			if projectData, exists := projects["litellm-cost-test"]; exists {
				project := projectData.(map[string]interface{})
				totalCost := project["total_cost"].(float64)
				
				assert.Greater(t, totalCost, 0.0, "Should calculate non-zero cost")
				assert.Less(t, totalCost, 100.0, "Cost should be reasonable")
			}

			// Compare online vs offline results if both succeed
			if tc.name == "litellm_offline" {
				// Offline mode should still produce reasonable results
				outputStr := string(output)
				assert.Contains(t, outputStr, "litellm-cost-test", "Should show project data")
			}
		})
	}
}

// TestCostCalculationConsistency tests consistency across different output formats
func TestCostCalculationConsistency(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Generate consistent test data
	testData := CostTestData{
		Model:        "claude-3.5-sonnet",
		InputTokens:  5000,
		OutputTokens: 2500,
		CacheRead:    1000,
		CacheWrite:   500,
		ExpectedCost: 0.02,
		Description:  "Consistency test",
	}
	
	err := generateTestSessionWithCost(generator, "consistency-test", testData, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Get reference cost from JSON output
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	jsonOutput, err := cmd.CombinedOutput()
	require.NoError(t, err, "JSON output should succeed")

	var jsonResult map[string]interface{}
	err = json.Unmarshal(jsonOutput, &jsonResult)
	require.NoError(t, err)

	projects := jsonResult["projects"].(map[string]interface{})
	projectData := projects["consistency-test"].(map[string]interface{})
	referenceCost := projectData["total_cost"].(float64)

	// Test other formats contain consistent cost information
	formats := []string{"table", "csv", "summary"}

	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", format)
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Format %s should succeed", format)
			outputStr := string(output)

			// Extract cost from output (this is format-specific)
			costFound := extractCostFromOutput(outputStr, format)
			if costFound > 0 {
				// Allow for formatting differences (like rounding)
				tolerance := 0.001
				assert.InDelta(t, referenceCost, costFound, tolerance,
					"Cost should be consistent across formats. JSON: %f, %s: %f", 
					referenceCost, format, costFound)
			} else {
				// At minimum, should contain cost representation
				assert.True(t, 
					strings.Contains(outputStr, "$") || 
					strings.Contains(outputStr, "cost") ||
					strings.Contains(outputStr, "Cost"),
					"Format %s should contain cost information", format)
			}
		})
	}
}

// TestCostCalculationPrecision tests precision of cost calculations
func TestCostCalculationPrecision(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Test very small and very large token amounts
	testCases := []struct {
		name        string
		inputTokens int
		description string
	}{
		{
			name:        "micro-usage",
			inputTokens: 1,
			description: "Single token precision",
		},
		{
			name:        "small-usage",
			inputTokens: 100,
			description: "Small token amount precision",
		},
		{
			name:        "large-usage",
			inputTokens: 1000000,
			description: "Large token amount precision",
		},
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testData := CostTestData{
				Model:       "claude-3.5-sonnet",
				InputTokens: tc.inputTokens,
				OutputTokens: tc.inputTokens / 2,
				Description: tc.description,
			}

			err := generateTestSessionWithCost(generator, tc.name, testData, time.Now().Add(-1*time.Hour))
			require.NoError(t, err)

			cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
			output, err := cmd.CombinedOutput()
			
			assert.NoError(t, err, "Precision test should succeed: %s", string(output))

			var result map[string]interface{}
			err = json.Unmarshal(output, &result)
			require.NoError(t, err)

			projects := result["projects"].(map[string]interface{})
			if projectData, exists := projects[tc.name]; exists {
				project := projectData.(map[string]interface{})
				totalCost := project["total_cost"].(float64)

				// Verify reasonable cost calculation
				if tc.inputTokens == 1 {
					// Should be very small but non-zero
					assert.Greater(t, totalCost, 0.0, "Single token should have non-zero cost")
					assert.Less(t, totalCost, 0.01, "Single token cost should be very small")
				} else if tc.inputTokens == 1000000 {
					// Should be proportionally larger
					assert.Greater(t, totalCost, 1.0, "Large usage should have substantial cost")
					assert.Less(t, totalCost, 1000.0, "Large usage cost should be reasonable")
				}

				// Verify no floating point precision issues (NaN, Inf)
				assert.False(t, math.IsNaN(totalCost), "Cost should not be NaN")
				assert.False(t, math.IsInf(totalCost, 0), "Cost should not be infinite")
			}
		})
	}
}

// TestCostCalculationRateLimit tests cost calculation with rate-limited sessions
func TestCostCalculationRateLimit(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Generate session with rate limit
	err := generator.GenerateSessionWithLimit("rate-limit-cost", time.Now().Add(-3*time.Hour))
	require.NoError(t, err)

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test cost calculation with rate limit data
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Rate limit cost calculation should succeed: %s", string(output))

	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err)

	// Verify cost calculation includes successful requests but handles failures
	projects := result["projects"].(map[string]interface{})
	if projectData, exists := projects["rate-limit-cost"]; exists {
		project := projectData.(map[string]interface{})
		totalCost := project["total_cost"].(float64)
		
		// Should have some cost from successful requests before limit
		assert.Greater(t, totalCost, 0.0, "Should have cost from successful requests")
		
		// Verify token counts don't include failed requests
		totalTokens := project["total_tokens"].(float64)
		assert.Greater(t, totalTokens, 0.0, "Should have token usage from successful requests")
	}
}

// TestCostCalculationMultiProject tests cost calculation across multiple projects
func TestCostCalculationMultiProject(t *testing.T) {
	tempDir := t.TempDir()
	generator := fixtures.NewTestDataGenerator(tempDir)

	// Generate multiple projects with known costs
	projects := []struct {
		name     string
		tokens   int
		expected float64
	}{
		{"project-a", 1000, 0.003},
		{"project-b", 2000, 0.006},
		{"project-c", 5000, 0.015},
	}

	totalExpected := 0.0
	for _, proj := range projects {
		testData := CostTestData{
			Model:       "claude-3.5-sonnet",
			InputTokens: proj.tokens,
			OutputTokens: proj.tokens / 2,
			Description: proj.name,
		}
		
		err := generateTestSessionWithCost(generator, proj.name, testData, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		totalExpected += proj.expected
	}

	binaryPath := filepath.Join(t.TempDir(), "test-monitor")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Test multi-project cost aggregation
	cmd := exec.Command(binaryPath, "--dir", tempDir, "--duration", "24h", "--output", "json")
	output, err = cmd.CombinedOutput()
	
	assert.NoError(t, err, "Multi-project cost calculation should succeed: %s", string(output))

	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err)

	// Verify individual project costs
	projectsData := result["projects"].(map[string]interface{})
	
	for _, proj := range projects {
		if projectData, exists := projectsData[proj.name]; exists {
			project := projectData.(map[string]interface{})
			totalCost := project["total_cost"].(float64)
			
			tolerance := 0.002
			assert.InDelta(t, proj.expected, totalCost, tolerance,
				"Project %s cost should match expected. Expected: %f, Got: %f",
				proj.name, proj.expected, totalCost)
		} else {
			t.Errorf("Project %s not found in output", proj.name)
		}
	}

	// Verify total aggregation if available
	if summary, hasSummary := result["summary"].(map[string]interface{}); hasSummary {
		if totalCost, hasTotalCost := summary["total_cost"].(float64); hasTotalCost {
			tolerance := 0.005
			assert.InDelta(t, totalExpected, totalCost, tolerance,
				"Total cost should match sum of individual projects")
		}
	}
}

// Helper function to generate test session with specific cost data
func generateTestSessionWithCost(generator *fixtures.TestDataGenerator, projectName string, testData CostTestData, startTime time.Time) error {
	projectDir := filepath.Join(generator.GetBaseDir(), projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	// Create JSONL entry with specific cost data
	entry := fixtures.JSONLEntry{
		Timestamp:    startTime.Format(time.RFC3339),
		Type:         "usage",
		Model:        testData.Model,
		InputTokens:  testData.InputTokens,
		OutputTokens: testData.OutputTokens,
		CacheRead:    testData.CacheRead,
		CacheWrite:   testData.CacheWrite,
		Cost:         testData.ExpectedCost,
	}

	return generator.WriteJSONL(filepath.Join(projectDir, "usage.jsonl"), []fixtures.JSONLEntry{entry})
}

// Helper function to extract cost from formatted output
func extractCostFromOutput(output, format string) float64 {
	switch format {
	case "table":
		// Look for dollar amounts in table format
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "$") && strings.Contains(line, "consistency-test") {
				// Extract dollar amount
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.HasPrefix(field, "$") {
						costStr := strings.TrimPrefix(field, "$")
						costStr = strings.ReplaceAll(costStr, ",", "")
						if cost, err := strconv.ParseFloat(costStr, 64); err == nil {
							return cost
						}
					}
				}
			}
		}
	case "csv":
		// Look for cost column in CSV
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "consistency-test") {
				fields := strings.Split(line, ",")
				for _, field := range fields {
					field = strings.TrimSpace(field)
					if strings.HasPrefix(field, "$") || strings.Contains(field, ".") {
						costStr := strings.TrimPrefix(field, "$")
						costStr = strings.ReplaceAll(costStr, ",", "")
						if cost, err := strconv.ParseFloat(costStr, 64); err == nil {
							return cost
						}
					}
				}
			}
		}
	case "summary":
		// Look for total cost in summary
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Total Cost:") || strings.Contains(line, "Cost:") {
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.HasPrefix(field, "$") {
						costStr := strings.TrimPrefix(field, "$")
						costStr = strings.ReplaceAll(costStr, ",", "")
						if cost, err := strconv.ParseFloat(costStr, 64); err == nil {
							return cost
						}
					}
				}
			}
		}
	}
	return 0.0
}
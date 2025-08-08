package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JSONLEntry represents a single JSONL log entry
type JSONLEntry struct {
	Timestamp   string `json:"timestamp"`
	Type        string `json:"type"`
	Model       string `json:"model,omitempty"`
	InputTokens int    `json:"input_tokens,omitempty"`
	OutputTokens int   `json:"output_tokens,omitempty"`
	CacheRead   int    `json:"cache_read,omitempty"`
	CacheWrite  int    `json:"cache_write,omitempty"`
	Cost        float64 `json:"cost,omitempty"`
	Message     string `json:"message,omitempty"`
	ResetTime   string `json:"reset_time,omitempty"`
}

// TestDataGenerator generates test JSONL data
type TestDataGenerator struct {
	baseDir string
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(baseDir string) *TestDataGenerator {
	return &TestDataGenerator{
		baseDir: baseDir,
	}
}

// GenerateSimpleSession generates a simple session with regular activity
func (g *TestDataGenerator) GenerateSimpleSession(projectName string, startTime time.Time) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	entries := []JSONLEntry{
		{
			Timestamp:   startTime.Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 1000,
			OutputTokens: 500,
			CacheRead:   100,
			CacheWrite:  50,
			Cost:        0.015,
		},
		{
			Timestamp:   startTime.Add(30 * time.Minute).Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 2000,
			OutputTokens: 1000,
			CacheRead:   200,
			CacheWrite:  100,
			Cost:        0.030,
		},
		{
			Timestamp:   startTime.Add(1 * time.Hour).Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 1500,
			OutputTokens: 750,
			CacheRead:   150,
			CacheWrite:  75,
			Cost:        0.023,
		},
	}

	return g.writeJSONL(filepath.Join(projectDir, "usage.jsonl"), entries)
}

// GenerateSessionWithLimit generates a session that hits the rate limit
func (g *TestDataGenerator) GenerateSessionWithLimit(projectName string, startTime time.Time) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	resetTime := startTime.Add(5 * time.Hour)
	entries := []JSONLEntry{
		{
			Timestamp:   startTime.Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 50000,
			OutputTokens: 25000,
			CacheRead:   5000,
			CacheWrite:  2500,
			Cost:        0.750,
		},
		{
			Timestamp:   startTime.Add(1 * time.Hour).Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 100000,
			OutputTokens: 50000,
			CacheRead:   10000,
			CacheWrite:  5000,
			Cost:        1.500,
		},
		{
			Timestamp: startTime.Add(2 * time.Hour).Format(time.RFC3339),
			Type:      "limit",
			Message:   fmt.Sprintf("Rate limit exceeded. Please try again after %s", resetTime.Format(time.RFC3339)),
			ResetTime: resetTime.Format(time.RFC3339),
		},
		{
			Timestamp:   startTime.Add(2*time.Hour + 30*time.Minute).Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 0,
			OutputTokens: 0,
			Message:     "Request failed due to rate limit",
		},
	}

	return g.writeJSONL(filepath.Join(projectDir, "usage.jsonl"), entries)
}

// GenerateMultiModelSession generates a session with multiple models
func (g *TestDataGenerator) GenerateMultiModelSession(projectName string, startTime time.Time) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	models := []string{"claude-3.5-sonnet", "claude-3-opus", "claude-3-haiku"}
	var entries []JSONLEntry

	for i := 0; i < 10; i++ {
		model := models[i%len(models)]
		timestamp := startTime.Add(time.Duration(i*15) * time.Minute)
		
		entries = append(entries, JSONLEntry{
			Timestamp:   timestamp.Format(time.RFC3339),
			Type:        "usage",
			Model:       model,
			InputTokens: 1000 + i*100,
			OutputTokens: 500 + i*50,
			CacheRead:   100 + i*10,
			CacheWrite:  50 + i*5,
			Cost:        float64(15+i*2) / 1000,
		})
	}

	return g.writeJSONL(filepath.Join(projectDir, "usage.jsonl"), entries)
}

// GenerateLargeDataset generates a large dataset for performance testing
func (g *TestDataGenerator) GenerateLargeDataset(projectName string, startTime time.Time, numEntries int) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	var entries []JSONLEntry
	models := []string{"claude-3.5-sonnet", "claude-3-opus", "claude-3-haiku"}

	for i := 0; i < numEntries; i++ {
		timestamp := startTime.Add(time.Duration(i) * time.Minute)
		model := models[i%len(models)]
		
		entries = append(entries, JSONLEntry{
			Timestamp:   timestamp.Format(time.RFC3339),
			Type:        "usage",
			Model:       model,
			InputTokens: 500 + (i%1000)*2,
			OutputTokens: 250 + (i%500),
			CacheRead:   50 + (i%100),
			CacheWrite:  25 + (i%50),
			Cost:        float64(10+(i%100)) / 1000,
		})

		// Add a rate limit message every 100 entries
		if i > 0 && i%100 == 0 {
			resetTime := timestamp.Add(5 * time.Hour)
			entries = append(entries, JSONLEntry{
				Timestamp: timestamp.Add(30 * time.Second).Format(time.RFC3339),
				Type:      "limit",
				Message:   fmt.Sprintf("Rate limit exceeded. Reset at %s", resetTime.Format(time.RFC3339)),
				ResetTime: resetTime.Format(time.RFC3339),
			})
		}
	}

	return g.writeJSONL(filepath.Join(projectDir, "usage.jsonl"), entries)
}

// GenerateContinuousActivity generates continuous activity across a 5-hour window
func (g *TestDataGenerator) GenerateContinuousActivity(projectName string, startTime time.Time) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	var entries []JSONLEntry
	
	// Generate activity every 5 minutes for 5 hours
	for i := 0; i < 60; i++ {
		timestamp := startTime.Add(time.Duration(i*5) * time.Minute)
		
		entries = append(entries, JSONLEntry{
			Timestamp:   timestamp.Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 100 + i*10,
			OutputTokens: 50 + i*5,
			CacheRead:   10 + i,
			CacheWrite:  5 + i/2,
			Cost:        float64(2+i) / 1000,
		})
	}

	return g.writeJSONL(filepath.Join(projectDir, "usage.jsonl"), entries)
}

// writeJSONL writes entries to a JSONL file
func (g *TestDataGenerator) writeJSONL(filename string, entries []JSONLEntry) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}

	return nil
}

// CleanupTestData removes all generated test data
func (g *TestDataGenerator) CleanupTestData() error {
	return os.RemoveAll(g.baseDir)
}

// CreateEmptyProject creates an empty project directory
func (g *TestDataGenerator) CreateEmptyProject(projectName string) error {
	projectDir := filepath.Join(g.baseDir, projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}
	
	// Create an empty usage.jsonl file
	file, err := os.Create(filepath.Join(projectDir, "usage.jsonl"))
	if err != nil {
		return err
	}
	return file.Close()
}
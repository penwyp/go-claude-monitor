package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JSONLEntry represents a single JSONL log entry in Claude Code format
type JSONLEntry struct {
	Timestamp    string  `json:"timestamp"`
	Type         string  `json:"type"`
	Uuid         string  `json:"uuid"`
	SessionId    string  `json:"sessionId"`
	UserType     string  `json:"userType"`
	Version      string  `json:"version"`
	Message      Message `json:"message"`
	// Additional fields for simplified test data
	Model        string  `json:"model,omitempty"`
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	CacheRead    int     `json:"cacheRead,omitempty"`
	CacheWrite   int     `json:"cacheWrite,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
	ResetTime    string  `json:"resetTime,omitempty"`
}

// Message represents the message structure in Claude Code logs
type Message struct {
	Role    string `json:"role"`
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Model   string `json:"model,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
}

// Usage represents token usage in Claude Code logs
type Usage struct {
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
	ServiceTier              string `json:"service_tier"`
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

	sessionId := "session-" + projectName
	entries := []JSONLEntry{
		// User message
		{
			Timestamp: startTime.Format(time.RFC3339),
			Type:      "message",
			Uuid:      "uuid-user-1",
			SessionId: sessionId,
			UserType:  "human",
			Version:   "1.0",
			Message: Message{
				Role:    "user",
				Type:    "text",
				Content: "Test user message",
			},
		},
		// Assistant response with usage
		{
			Timestamp: startTime.Add(5 * time.Second).Format(time.RFC3339),
			Type:      "message",
			Uuid:      "uuid-assistant-1",
			SessionId: sessionId,
			UserType:  "human",
			Version:   "1.0",
			Message: Message{
				Role:    "assistant",
				Type:    "text",
				Content: "Test assistant response",
				Model:   "claude-3.5-sonnet",
				Usage: &Usage{
					InputTokens:              1000,
					OutputTokens:             500,
					CacheCreationInputTokens: 50,
					CacheReadInputTokens:     100,
					ServiceTier:              "default",
				},
			},
		},
		// Another user message
		{
			Timestamp: startTime.Add(30 * time.Minute).Format(time.RFC3339),
			Type:      "message",
			Uuid:      "uuid-user-2",
			SessionId: sessionId,
			UserType:  "human",
			Version:   "1.0",
			Message: Message{
				Role:    "user",
				Type:    "text",
				Content: "Another test message",
			},
		},
		// Another assistant response
		{
			Timestamp: startTime.Add(30*time.Minute + 5*time.Second).Format(time.RFC3339),
			Type:      "message",
			Uuid:      "uuid-assistant-2",
			SessionId: sessionId,
			UserType:  "human",
			Version:   "1.0",
			Message: Message{
				Role:    "assistant",
				Type:    "text",
				Content: "Another assistant response",
				Model:   "claude-3.5-sonnet",
				Usage: &Usage{
					InputTokens:              2000,
					OutputTokens:             1000,
					CacheCreationInputTokens: 100,
					CacheReadInputTokens:     200,
					ServiceTier:              "default",
				},
			},
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
			Message: Message{
				Role:    "system",
				Type:    "error",
				Content: fmt.Sprintf("Rate limit exceeded. Please try again after %s", resetTime.Format(time.RFC3339)),
			},
			ResetTime: resetTime.Format(time.RFC3339),
		},
		{
			Timestamp:   startTime.Add(2*time.Hour + 30*time.Minute).Format(time.RFC3339),
			Type:        "usage",
			Model:       "claude-3.5-sonnet",
			InputTokens: 0,
			OutputTokens: 0,
			Message: Message{
				Role:    "system",
				Type:    "error",
				Content: "Request failed due to rate limit",
			},
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
				Message: Message{
					Role:    "system",
					Type:    "error",
					Content: fmt.Sprintf("Rate limit exceeded. Reset at %s", resetTime.Format(time.RFC3339)),
				},
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

// GetBaseDir returns the base directory for test data
func (g *TestDataGenerator) GetBaseDir() string {
	return g.baseDir
}

// WriteJSONL writes entries to a JSONL file (public method for test helpers)
func (g *TestDataGenerator) WriteJSONL(filename string, entries []JSONLEntry) error {
	return g.writeJSONL(filename, entries)
}
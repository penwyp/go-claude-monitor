package aggregator

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// Aggregator is responsible for aggregating conversation logs by hour and model.
type Aggregator struct {
	pricing  pricing.PricingProvider
	timezone string
}

// HourlyData holds aggregated statistics for a specific hour and model.
type HourlyData struct {
	Hour           int64  `json:"hour"` // Unix timestamp (truncated to hour)
	Model          string `json:"model"`
	ProjectName    string `json:"projectName"`
	InputTokens    int    `json:"inputTokens"`
	OutputTokens   int    `json:"outputTokens"`
	CacheCreation  int    `json:"cacheCreation"`
	CacheRead      int    `json:"cacheRead"`
	TotalTokens    int    `json:"totalTokens"`
	MessageCount   int    `json:"messageCount"`
	FirstEntryTime int64  `json:"firstEntryTime"` // Unix timestamp of first entry in this hour
	LastEntryTime  int64  `json:"lastEntryTime"`  // Unix timestamp of last entry in this hour
}

// CachedLimitInfo contains essential limit message information for caching
type CachedLimitInfo struct {
	Type      string `json:"type"`      // "opus_limit", "general_limit", "system_limit"
	Timestamp int64  `json:"timestamp"` // Unix timestamp when limit was detected
	ResetTime *int64 `json:"resetTime"` // Unix timestamp when limit will reset
	Content   string `json:"content"`   // Original limit message content
	Model     string `json:"model"`     // Model that hit the limit
}

// AggregatedData represents the aggregation result for a file.
type AggregatedData struct {
	FileHash           string       `json:"fileHash"`
	FilePath           string       `json:"filePath"`
	SessionId          string       `json:"sessionId"` // Session ID extracted from filename
	ProjectName        string       `json:"projectName"`
	HourlyStats        []HourlyData `json:"hourlyStats"`
	LastModified       int64        `json:"lastModified"`
	FileSize           int64        `json:"fileSize"`
	Inode              uint64       `json:"inode"`                         // File inode
	ContentFingerprint string       `json:"content_fingerprint,omitempty"` // Content fingerprint for change detection
	LimitMessages      []CachedLimitInfo  `json:"limitMessages,omitempty"`       // Detected limit messages for window detection
}

// NewAggregatorWithTimezone creates a new Aggregator with a specified timezone.
func NewAggregatorWithTimezone(timezone string) *Aggregator {
	return &Aggregator{
		pricing:  pricing.NewDefaultProvider(),
		timezone: timezone,
	}
}

// NewAggregatorWithConfig creates a new Aggregator with pricing and timezone configuration.
func NewAggregatorWithConfig(pricingSource string, pricingOfflineMode bool, cacheDir, timezone string) (*Aggregator, error) {
	util.LogDebug(fmt.Sprintf("Creating aggregator with pricing config: source=%s, offline=%t, timezone=%s",
		pricingSource, pricingOfflineMode, timezone))

	// Create pricing configuration
	pricingConfig := &pricing.SourceConfig{
		PricingSource:      pricingSource,
		PricingOfflineMode: pricingOfflineMode,
	}

	// Create pricing provider using factory
	pricingProvider, err := pricing.CreatePricingProvider(pricingConfig, cacheDir)
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to create pricing provider: %v", err))
		return nil, err
	}

	util.LogDebug(fmt.Sprintf("Successfully created aggregator with %s pricing provider",
		pricingProvider.GetProviderName()))

	return &Aggregator{
		pricing:  pricingProvider,
		timezone: timezone,
	}, nil
}

// calculateCost computes the cost for the given HourlyData and pricing.
// This method is now used for real-time cost calculation, not for storing cost during aggregation.
func (a *Aggregator) calculateCost(data *HourlyData, pricing pricing.ModelPricing) float64 {
	cost := float64(data.InputTokens) / 1_000_000 * pricing.Input
	cost += float64(data.OutputTokens) / 1_000_000 * pricing.Output
	cost += float64(data.CacheCreation) / 1_000_000 * pricing.CacheCreation
	cost += float64(data.CacheRead) / 1_000_000 * pricing.CacheRead
	return cost
}

// CalculateCost provides a public interface for real-time cost calculation.
func (a *Aggregator) CalculateCost(data *HourlyData) (float64, error) {
	modelPricing, err := a.pricing.GetPricing(context.Background(), data.Model)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to get pricing for model %s: %v", data.Model, err))
		// Use default pricing as fallback
		modelPricing = pricing.ModelPricing{
			Input:         3.0, // Default pricing per million tokens
			Output:        15.0,
			CacheCreation: 3.75,
			CacheRead:     0.3,
		}
	}
	return a.calculateCost(data, modelPricing), nil
}

// ExtractProjectName extracts the project name from the file path.
func ExtractProjectName(filePath string) string {
	dir := filepath.Dir(filePath)
	projectName := filepath.Base(dir)

	if isUUID(projectName) {
		parentDir := filepath.Dir(dir)
		parentName := filepath.Base(parentDir)
		if parentName != "projects" && parentName != "." {
			projectName = parentName + "/" + projectName
		}
	}

	return projectName
}

// isUUID checks if the given string is a UUID.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}

	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}

	return len(parts[0]) == 8 && len(parts[1]) == 4 &&
		len(parts[2]) == 4 && len(parts[3]) == 4 &&
		len(parts[4]) == 12
}

// TokenCounts holds extracted token information matching Python reference.
type TokenCounts struct {
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	TotalTokens   int
}

// extractTokens implements comprehensive token extraction matching Python reference.
func extractTokens(log model.ConversationLog) TokenCounts {
	tokens := TokenCounts{
		InputTokens:   0,
		OutputTokens:  0,
		CacheCreation: 0,
		CacheRead:     0,
		TotalTokens:   0,
	}

	// Use message.usage as primary source (matching Python's approach)
	if log.Type == model.EntryMessage || log.Type == model.EntryAssistant {
		usage := log.Message.Usage

		// Extract tokens directly from usage
		input := usage.InputTokens
		output := usage.OutputTokens
		cacheCreation := usage.CacheCreationInputTokens
		cacheRead := usage.CacheReadInputTokens

		// Only set values if we have actual tokens
		if input > 0 || output > 0 || cacheCreation > 0 || cacheRead > 0 {
			tokens.InputTokens = input
			tokens.OutputTokens = output
			tokens.CacheCreation = cacheCreation
			tokens.CacheRead = cacheRead
			tokens.TotalTokens = input + output + cacheCreation + cacheRead
		}
	}

	return tokens
}

// Helper function to truncate timestamp to hour in UTC.
func truncateToHourUTC(timestamp int64) int64 {
	return (timestamp / 3600) * 3600
}

// Helper function to parse RFC3339 timestamp to Unix timestamp.
func parseToUnixTimestamp(timestampStr string) (int64, error) {
	t, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// normalizeModelName returns a normalized model name, defaulting to "unknown" if empty.
func normalizeModelName(model string) string {
	if model == "" {
		return "unknown"
	}
	return model
}

// AggregateByHourAndModel aggregates conversation logs using Unix timestamps internally.
// This version works entirely in UTC to avoid timezone confusion.
func (a *Aggregator) AggregateByHourAndModel(logs []model.ConversationLog, projectName string) []HourlyData {
	// First pass: Find the first occurrence hour for each requestId.
	requestIdFirstHour := make(map[string]int64)
	for _, log := range logs {
		if log.Type != model.EntryMessage && log.Type != model.EntryAssistant {
			continue
		}
		if log.RequestId == "" {
			continue
		}
		if log.Message.Id == "" {
			continue
		}

		timestamp, err := parseToUnixTimestamp(log.Timestamp)
		if err != nil {
			util.LogDebug(fmt.Sprintf("Failed to parse timestamp: %s - %v", log.Timestamp, err))
			continue
		}

		// Truncate to hour in UTC.
		hourTimestamp := truncateToHourUTC(timestamp)

		if _, exists := requestIdFirstHour[log.RequestId]; !exists {
			requestIdFirstHour[log.RequestId] = hourTimestamp
		}
	}

	// Structure to hold tokens by requestId.
	type RequestIdTokens struct {
		Hour           int64 // Unix timestamp (truncated to hour)
		Model          string
		InputTokens    int
		OutputTokens   int
		CacheCreation  int
		CacheRead      int
		MessageCount   int
		FirstEntryTime int64 // Unix timestamp
		LastEntryTime  int64 // Unix timestamp
	}
	requestIdTokensMap := make(map[string]*RequestIdTokens)

	// Second pass: Aggregate tokens by requestId.
	for _, log := range logs {
		if log.Type != model.EntryMessage && log.Type != model.EntryAssistant {
			continue
		}
		if log.RequestId == "" {
			continue
		}
		if log.Message.Id == "" {
			continue
		}

		timestamp, err := parseToUnixTimestamp(log.Timestamp)
		if err != nil {
			continue
		}

		tokens := extractTokens(log)
		model := normalizeModelName(log.Message.Model)

		firstHour := requestIdFirstHour[log.RequestId]
		key := fmt.Sprintf("%d|%s|%s", firstHour, model, log.RequestId)

		if _, exists := requestIdTokensMap[key]; !exists {
			requestIdTokensMap[key] = &RequestIdTokens{
				Hour:           firstHour,
				Model:          model,
				FirstEntryTime: timestamp,
				LastEntryTime:  timestamp,
				MessageCount:   1,
			}
		}

		reqTokens := requestIdTokensMap[key]
		// Update first/last entry times.
		if timestamp < reqTokens.FirstEntryTime {
			reqTokens.FirstEntryTime = timestamp
		}
		if timestamp > reqTokens.LastEntryTime {
			reqTokens.LastEntryTime = timestamp
		}
		// For input tokens, use max value.
		if tokens.InputTokens > reqTokens.InputTokens {
			reqTokens.InputTokens = tokens.InputTokens
		}
		if tokens.CacheCreation > reqTokens.CacheCreation {
			reqTokens.CacheCreation = tokens.CacheCreation
		}
		if tokens.CacheRead > reqTokens.CacheRead {
			reqTokens.CacheRead = tokens.CacheRead
		}
		if tokens.OutputTokens > reqTokens.OutputTokens {
			reqTokens.OutputTokens = tokens.OutputTokens
		}

	}

	// Third pass: Aggregate requestId data into hourly data.
	hourlyMap := make(map[string]*HourlyData)
	for _, reqTokens := range requestIdTokensMap {
		key := fmt.Sprintf("%d|%s", reqTokens.Hour, reqTokens.Model)

		if _, exists := hourlyMap[key]; !exists {
			hourlyMap[key] = &HourlyData{
				Hour:           reqTokens.Hour,
				Model:          reqTokens.Model,
				ProjectName:    projectName,
				InputTokens:    0,
				OutputTokens:   0,
				CacheCreation:  0,
				CacheRead:      0,
				TotalTokens:    0,
				MessageCount:   0,
				FirstEntryTime: reqTokens.FirstEntryTime,
				LastEntryTime:  reqTokens.LastEntryTime,
			}
		}

		hourly := hourlyMap[key]
		hourly.InputTokens += reqTokens.InputTokens
		hourly.OutputTokens += reqTokens.OutputTokens
		hourly.CacheCreation += reqTokens.CacheCreation
		hourly.CacheRead += reqTokens.CacheRead
		hourly.MessageCount += reqTokens.MessageCount

		// Update first/last entry times.
		if reqTokens.FirstEntryTime < hourly.FirstEntryTime {
			hourly.FirstEntryTime = reqTokens.FirstEntryTime
		}
		if reqTokens.LastEntryTime > hourly.LastEntryTime {
			hourly.LastEntryTime = reqTokens.LastEntryTime
		}
	}

	// Convert map to slice and calculate totals.
	result := make([]HourlyData, 0, len(hourlyMap))
	for _, hourly := range hourlyMap {
		hourly.TotalTokens = hourly.InputTokens + hourly.OutputTokens +
			hourly.CacheCreation + hourly.CacheRead
		result = append(result, *hourly)
	}

	// Sort by hour timestamp.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Hour < result[j].Hour
	})

	return result
}

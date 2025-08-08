package internal

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// LimitParseStrategy defines the interface for parsing limit messages
type LimitParseStrategy interface {
	// CanParse checks if this strategy can parse the given content
	CanParse(content string) bool
	// Parse extracts limit information from the content
	Parse(content string, timestamp int64, metadata map[string]string) *LimitResult
	// Priority returns the priority of this strategy (higher = more specific)
	Priority() int
}

// LimitResult contains parsed limit information
type LimitResult struct {
	Type        string
	ResetTime   *int64
	WaitMinutes *int
	Confidence  float64 // 0.0 to 1.0, how confident we are in this parse
}

// OpusLimitStrategy handles Opus-specific rate limits
type OpusLimitStrategy struct {
	opusPattern *regexp.Regexp
	waitPattern *regexp.Regexp
}

func NewOpusLimitStrategy() *OpusLimitStrategy {
	return &OpusLimitStrategy{
		opusPattern: regexp.MustCompile(`(?i)(opus).*(rate\s*limit|limit\s*exceeded|limit\s*reached|limit\s*hit)`),
		waitPattern: regexp.MustCompile(`(?i)wait\s+(\d+)\s+minutes?`),
	}
}

func (s *OpusLimitStrategy) CanParse(content string) bool {
	contentLower := strings.ToLower(content)
	return s.opusPattern.MatchString(contentLower)
}

func (s *OpusLimitStrategy) Parse(content string, timestamp int64, metadata map[string]string) *LimitResult {
	result := &LimitResult{
		Type:       "opus_limit",
		Confidence: 0.9,
	}
	
	// Extract wait time
	if matches := s.waitPattern.FindStringSubmatch(content); len(matches) > 1 {
		waitMinutes := 0
		fmt.Sscanf(matches[1], "%d", &waitMinutes)
		result.WaitMinutes = &waitMinutes
		
		// Calculate reset time
		resetTime := timestamp + int64(waitMinutes*60)
		result.ResetTime = &resetTime
		result.Confidence = 1.0 // Very confident when we have wait time
	}
	
	return result
}

func (s *OpusLimitStrategy) Priority() int {
	return 10 // High priority for specific model limits
}

// ResetTimestampStrategy handles "limit reached|timestamp" format
type ResetTimestampStrategy struct {
	resetPattern *regexp.Regexp
}

func NewResetTimestampStrategy() *ResetTimestampStrategy {
	return &ResetTimestampStrategy{
		resetPattern: regexp.MustCompile(`(?i)limit\s+reached\|(\d+)`),
	}
}

func (s *ResetTimestampStrategy) CanParse(content string) bool {
	return s.resetPattern.MatchString(content)
}

func (s *ResetTimestampStrategy) Parse(content string, timestamp int64, metadata map[string]string) *LimitResult {
	matches := s.resetPattern.FindStringSubmatch(content)
	if len(matches) <= 1 {
		return nil
	}
	
	var resetTimestamp int64
	fmt.Sscanf(matches[1], "%d", &resetTimestamp)
	
	// Convert milliseconds to seconds if needed
	if resetTimestamp > 1e12 {
		resetTimestamp = resetTimestamp / 1000
	}
	
	return &LimitResult{
		Type:       "general_limit",
		ResetTime:  &resetTimestamp,
		Confidence: 1.0, // Very confident with explicit timestamp
	}
}

func (s *ResetTimestampStrategy) Priority() int {
	return 15 // Highest priority for explicit timestamps
}

// ClaudeAILimitStrategy handles "Claude AI usage limit reached" messages
type ClaudeAILimitStrategy struct {
	pattern *regexp.Regexp
}

func NewClaudeAILimitStrategy() *ClaudeAILimitStrategy {
	return &ClaudeAILimitStrategy{
		pattern: regexp.MustCompile(`(?i)claude\s+ai\s+usage\s+limit\s+reached`),
	}
}

func (s *ClaudeAILimitStrategy) CanParse(content string) bool {
	return s.pattern.MatchString(content)
}

func (s *ClaudeAILimitStrategy) Parse(content string, timestamp int64, metadata map[string]string) *LimitResult {
	// First check if there's a timestamp in the content
	resetStrategy := NewResetTimestampStrategy()
	if resetStrategy.CanParse(content) {
		return resetStrategy.Parse(content, timestamp, metadata)
	}
	
	// Otherwise return basic limit info
	return &LimitResult{
		Type:       "api_error_limit",
		Confidence: 0.8,
	}
}

func (s *ClaudeAILimitStrategy) Priority() int {
	return 8
}

// GeneralLimitStrategy handles general rate limit patterns
type GeneralLimitStrategy struct {
	generalPattern *regexp.Regexp
}

func NewGeneralLimitStrategy() *GeneralLimitStrategy {
	return &GeneralLimitStrategy{
		generalPattern: regexp.MustCompile(`(?i)(rate\s*limit|limit\s*exceeded|limit\s*reached|you've\s*reached|quota\s*exceeded)`),
	}
}

func (s *GeneralLimitStrategy) CanParse(content string) bool {
	contentLower := strings.ToLower(content)
	// Quick check for limit-related keywords
	if !strings.Contains(contentLower, "limit") && !strings.Contains(contentLower, "rate") && !strings.Contains(contentLower, "quota") {
		return false
	}
	return s.generalPattern.MatchString(contentLower)
}

func (s *GeneralLimitStrategy) Parse(content string, timestamp int64, metadata map[string]string) *LimitResult {
	return &LimitResult{
		Type:       "system_limit",
		Confidence: 0.6, // Lower confidence for general patterns
	}
}

func (s *GeneralLimitStrategy) Priority() int {
	return 5 // Lowest priority for general patterns
}

// LimitStrategyRegistry manages and applies limit parsing strategies
type LimitStrategyRegistry struct {
	strategies []LimitParseStrategy
}

// NewLimitStrategyRegistry creates a new registry with default strategies
func NewLimitStrategyRegistry() *LimitStrategyRegistry {
	registry := &LimitStrategyRegistry{
		strategies: []LimitParseStrategy{
			NewResetTimestampStrategy(), // Priority 15
			NewOpusLimitStrategy(),       // Priority 10
			NewClaudeAILimitStrategy(),   // Priority 8
			NewGeneralLimitStrategy(),    // Priority 5
		},
	}
	
	// Sort strategies by priority (highest first)
	registry.sortStrategies()
	return registry
}

// sortStrategies sorts strategies by priority (highest first)
func (r *LimitStrategyRegistry) sortStrategies() {
	// Simple bubble sort for small number of strategies
	n := len(r.strategies)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if r.strategies[j].Priority() < r.strategies[j+1].Priority() {
				r.strategies[j], r.strategies[j+1] = r.strategies[j+1], r.strategies[j]
			}
		}
	}
}

// AddStrategy adds a new strategy to the registry
func (r *LimitStrategyRegistry) AddStrategy(strategy LimitParseStrategy) {
	r.strategies = append(r.strategies, strategy)
	r.sortStrategies()
}

// Parse attempts to parse content using the registered strategies
func (r *LimitStrategyRegistry) Parse(content string, timestamp int64, metadata map[string]string) *LimitResult {
	// Try strategies in priority order
	for _, strategy := range r.strategies {
		if strategy.CanParse(content) {
			if result := strategy.Parse(content, timestamp, metadata); result != nil {
				util.LogDebug(fmt.Sprintf("LimitStrategy: %T parsed successfully with confidence %.2f",
					strategy, result.Confidence))
				return result
			}
		}
	}
	return nil
}

// ParseLog attempts to parse a log entry for limit information
func (r *LimitStrategyRegistry) ParseLog(log model.ConversationLog) *LimitResult {
	timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to parse timestamp: %s", log.Timestamp))
		return nil
	}
	
	metadata := map[string]string{
		"request_id": log.RequestId,
		"session_id": log.SessionId,
		"type":       log.Type,
	}
	
	// Extract model if available
	if log.Message.Model != "" {
		metadata["model"] = log.Message.Model
	}
	
	// Try to parse based on log type
	switch log.Type {
	case "system":
		return r.Parse(log.Content, timestamp.Unix(), metadata)
		
	case "user", "assistant":
		// Check content items
		for _, item := range log.Message.Content {
			var content string
			
			// Extract content based on item type
			switch item.Type {
			case "text":
				content = item.Text
			case "tool_result":
				content = ExtractToolResultContent(item)
			}
			
			if content != "" {
				if result := r.Parse(content, timestamp.Unix(), metadata); result != nil {
					return result
				}
			}
		}
	}
	
	return nil
}

// ExtractToolResultContent extracts text content from tool result (exported for use)
func ExtractToolResultContent(item model.ContentItem) string {
	if item.Content == nil {
		return ""
	}
	
	switch v := item.Content.(type) {
	case string:
		return v
	case []interface{}:
		// Handle array of content items
		var content strings.Builder
		for _, subItem := range v {
			if subMap, ok := subItem.(map[string]interface{}); ok {
				if text, ok := subMap["text"].(string); ok {
					content.WriteString(text)
					content.WriteString(" ")
				}
			}
		}
		return content.String()
	}
	
	return ""
}
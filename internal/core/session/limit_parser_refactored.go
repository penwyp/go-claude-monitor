package session

import (
	"fmt"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/session/internal"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// LimitInfoV2 contains refactored information about a detected rate limit
type LimitInfoV2 struct {
	// Core fields
	Type      string // "opus_limit", "general_limit", "system_limit", "api_error_limit"
	Timestamp int64  // Unix timestamp when limit was detected
	Content   string // Original limit message content
	
	// Optional reset information
	ResetTime   *int64 // Unix timestamp when limit will reset
	WaitMinutes *int   // Minutes to wait (for Opus limits)
	
	// Metadata
	RequestID  string  // Request ID associated with the limit
	MessageID  string  // Message ID associated with the limit
	Model      string  // Model that hit the limit
	SessionID  string  // Session ID
	Confidence float64 // Confidence in the parse (0.0 to 1.0)
}

// LimitParserV2 refactored parser using strategy pattern
type LimitParserV2 struct {
	registry *internal.LimitStrategyRegistry
}

// NewLimitParserV2 creates a new refactored limit parser
func NewLimitParserV2() *LimitParserV2 {
	return &LimitParserV2{
		registry: internal.NewLimitStrategyRegistry(),
	}
}

// ParseLogs parses conversation logs and returns detected limit information
func (p *LimitParserV2) ParseLogs(logs []model.ConversationLog) []LimitInfoV2 {
	util.LogInfo(fmt.Sprintf("LimitParserV2: Parsing %d logs for limit messages", len(logs)))
	
	var limits []LimitInfoV2
	
	for _, log := range logs {
		if limit := p.parseLog(log); limit != nil {
			limits = append(limits, *limit)
			p.logLimitFound(log.Type, limit)
		}
	}
	
	util.LogInfo(fmt.Sprintf("LimitParserV2: Found %d limit messages total", len(limits)))
	return limits
}

// parseLog parses a single log entry for limit information
func (p *LimitParserV2) parseLog(log model.ConversationLog) *LimitInfoV2 {
	// Use the strategy registry to parse the log
	result := p.registry.ParseLog(log)
	if result == nil {
		return nil
	}
	
	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to parse timestamp: %s", log.Timestamp))
		return nil
	}
	
	// Build LimitInfoV2 from result and log
	limit := &LimitInfoV2{
		Type:        result.Type,
		Timestamp:   timestamp.Unix(),
		Content:     extractContent(log),
		ResetTime:   result.ResetTime,
		WaitMinutes: result.WaitMinutes,
		RequestID:   log.RequestId,
		SessionID:   log.SessionId,
		Confidence:  result.Confidence,
	}
	
	// Extract additional metadata
	if log.Message.Id != "" {
		limit.MessageID = log.Message.Id
	}
	if log.Message.Model != "" {
		limit.Model = log.Message.Model
	}
	
	return limit
}

// extractContent extracts the relevant content from a log entry
func extractContent(log model.ConversationLog) string {
	// For system messages, use the content directly
	if log.Type == "system" {
		return log.Content
	}
	
	// For user/assistant messages, extract from content items
	for _, item := range log.Message.Content {
		switch item.Type {
		case "text":
			if item.Text != "" {
				return item.Text
			}
		case "tool_result":
			if content := internal.ExtractToolResultContent(item); content != "" {
				return content
			}
		}
	}
	
	return ""
}

// logLimitFound logs information about a found limit
func (p *LimitParserV2) logLimitFound(logType string, limit *LimitInfoV2) {
	resetTimeStr := "nil"
	if limit.ResetTime != nil {
		resetTimeStr = time.Unix(*limit.ResetTime, 0).Format("2006-01-02 15:04:05")
	}
	
	util.LogInfo(fmt.Sprintf("Found %s limit: type=%s, confidence=%.2f, reset=%s",
		logType, limit.Type, limit.Confidence, resetTimeStr))
	
	util.LogDebug(fmt.Sprintf("Limit details - Type: %s, Timestamp: %s, ResetTime: %s, Model: %s, Confidence: %.2f",
		limit.Type,
		time.Unix(limit.Timestamp, 0).Format("2006-01-02 15:04:05"),
		resetTimeStr,
		limit.Model,
		limit.Confidence))
}

// AddCustomStrategy adds a custom parsing strategy to the parser
func (p *LimitParserV2) AddCustomStrategy(strategy internal.LimitParseStrategy) {
	p.registry.AddStrategy(strategy)
}

// GetHighConfidenceLimits returns only limits with confidence >= threshold
func (p *LimitParserV2) GetHighConfidenceLimits(limits []LimitInfoV2, threshold float64) []LimitInfoV2 {
	var highConfidence []LimitInfoV2
	for _, limit := range limits {
		if limit.Confidence >= threshold {
			highConfidence = append(highConfidence, limit)
		}
	}
	return highConfidence
}

// ConvertFromLegacy converts legacy LimitInfo to LimitInfoV2
func ConvertLimitFromLegacy(legacy LimitInfo) LimitInfoV2 {
	return LimitInfoV2{
		Type:        legacy.Type,
		Timestamp:   legacy.Timestamp,
		Content:     legacy.Content,
		ResetTime:   legacy.ResetTime,
		WaitMinutes: legacy.WaitMinutes,
		RequestID:   legacy.RequestID,
		MessageID:   legacy.MessageID,
		Model:       legacy.Model,
		SessionID:   legacy.SessionID,
		Confidence:  0.8, // Default confidence for legacy
	}
}

// ConvertLimitToLegacy converts LimitInfoV2 to legacy LimitInfo
func ConvertLimitToLegacy(v2 LimitInfoV2) LimitInfo {
	return LimitInfo{
		Type:        v2.Type,
		Timestamp:   v2.Timestamp,
		ResetTime:   v2.ResetTime,
		WaitMinutes: v2.WaitMinutes,
		Content:     v2.Content,
		RequestID:   v2.RequestID,
		MessageID:   v2.MessageID,
		Model:       v2.Model,
		SessionID:   v2.SessionID,
	}
}

// LimitParserConfig configuration for the parser
type LimitParserConfig struct {
	MinConfidence float64 // Minimum confidence threshold
	EnableDebug   bool    // Enable debug logging
}

// NewLimitParserWithConfig creates a parser with custom configuration
func NewLimitParserWithConfig(config LimitParserConfig) *LimitParserV2 {
	parser := NewLimitParserV2()
	// Could add configuration logic here if needed
	return parser
}


package session

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// LimitInfo contains information about a detected rate limit
type LimitInfo struct {
	Type        string // "opus_limit", "general_limit", "system_limit"
	Timestamp   int64  // Unix timestamp when limit was detected
	ResetTime   *int64 // Unix timestamp when limit will reset
	WaitMinutes *int   // Minutes to wait (for Opus limits)
	Content     string // Original limit message content
	RequestID   string // Request ID associated with the limit
	MessageID   string // Message ID associated with the limit
	Model       string // Model that hit the limit
	SessionID   string // Session ID
}

// LimitParser parses conversation logs to detect rate limit messages
type LimitParser struct {
	// Patterns for different types of limit messages
	opusPattern    *regexp.Regexp
	waitPattern    *regexp.Regexp
	resetPattern   *regexp.Regexp
	generalPattern *regexp.Regexp
}

// NewLimitParser creates a new limit message parser
func NewLimitParser() *LimitParser {
	return &LimitParser{
		// Matches Opus-specific rate limits
		opusPattern: regexp.MustCompile(`(?i)(opus).*(rate\s*limit|limit\s*exceeded|limit\s*reached|limit\s*hit)`),
		// Matches wait time instructions
		waitPattern: regexp.MustCompile(`(?i)wait\s+(\d+)\s+minutes?`),
		// Matches reset timestamp in "limit reached|timestamp" format
		resetPattern: regexp.MustCompile(`(?i)limit\s+reached\|(\d+)`),
		// General limit patterns
		generalPattern: regexp.MustCompile(`(?i)(rate\s*limit|limit\s*exceeded|limit\s*reached|you've\s*reached|quota\s*exceeded)`),
	}
}

// ParseLogs parses conversation logs and returns detected limit information
func (p *LimitParser) ParseLogs(logs []model.ConversationLog) []LimitInfo {
	var limits []LimitInfo

	util.LogInfo(fmt.Sprintf("LimitParser: Parsing %d logs for limit messages", len(logs)))
	util.LogDebug(fmt.Sprintf("LimitParser: Starting with patterns - opus: %v, wait: %v, reset: %v, general: %v",
		p.opusPattern != nil, p.waitPattern != nil, p.resetPattern != nil, p.generalPattern != nil))

	for _, log := range logs {
		// Parse different log types
		switch log.Type {
		case "system":
			if limit := p.parseSystemMessage(log); limit != nil {
				limits = append(limits, *limit)
				util.LogInfo(fmt.Sprintf("Found system limit message: %s", limit.Type))
				util.LogDebug(fmt.Sprintf("System limit details - Type: %s, Timestamp: %s, ResetTime: %v, Content: %.100s",
					limit.Type,
					time.Unix(limit.Timestamp, 0).Format("2006-01-02 15:04:05"),
					limit.ResetTime,
					limit.Content))
			}
		case "user", "assistant":
			if limit := p.parseUserAssistantMessage(log); limit != nil {
				limits = append(limits, *limit)
				resetTimeStr := "nil"
				if limit.ResetTime != nil {
					resetTimeStr = time.Unix(*limit.ResetTime, 0).Format("2006-01-02 15:04:05")
				}
				util.LogInfo(fmt.Sprintf("Found %s limit message: %s, reset time: %v",
					log.Type, limit.Type, limit.ResetTime))
				util.LogDebug(fmt.Sprintf("%s limit details - Type: %s, Timestamp: %s, ResetTime: %s, Model: %s",
					log.Type,
					limit.Type,
					time.Unix(limit.Timestamp, 0).Format("2006-01-02 15:04:05"),
					resetTimeStr,
					limit.Model))
			}
		}
	}

	util.LogInfo(fmt.Sprintf("LimitParser: Found %d limit messages total", len(limits)))
	return limits
}

// parseSystemMessage parses system messages for limit information
func (p *LimitParser) parseSystemMessage(log model.ConversationLog) *LimitInfo {
	content := strings.ToLower(log.Content)

	// Quick check for limit-related keywords
	if !strings.Contains(content, "limit") && !strings.Contains(content, "rate") {
		return nil
	}

	timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to parse timestamp: %s", log.Timestamp))
		return nil
	}

	limit := &LimitInfo{
		Timestamp: timestamp.Unix(),
		Content:   log.Content,
		RequestID: log.RequestId,
		SessionID: log.SessionId,
	}

	// Check for Opus-specific limit
	if p.opusPattern.MatchString(content) {
		limit.Type = "opus_limit"

		// Extract wait time
		if matches := p.waitPattern.FindStringSubmatch(log.Content); len(matches) > 1 {
			waitMinutes := 0
			fmt.Sscanf(matches[1], "%d", &waitMinutes)
			limit.WaitMinutes = &waitMinutes

			// Calculate reset time
			resetTime := timestamp.Add(time.Duration(waitMinutes) * time.Minute).Unix()
			limit.ResetTime = &resetTime
		}

		return limit
	}

	// Check for general limit
	if p.generalPattern.MatchString(content) {
		limit.Type = "system_limit"
		return limit
	}

	return nil
}

// parseUserAssistantMessage parses user/assistant messages for limit information
func (p *LimitParser) parseUserAssistantMessage(log model.ConversationLog) *LimitInfo {
	// Extract model from message
	model := ""
	if log.Message.Model != "" {
		model = log.Message.Model
	}

	// Check content items for limit messages
	for _, item := range log.Message.Content {
		// Check text content (e.g., API error messages)
		if item.Type == "text" && item.Text != "" {
			if limit := p.parseTextContent(item.Text, log, model); limit != nil {
				return limit
			}
		}

		// Check tool results
		if item.Type == "tool_result" && item.Content != nil {
			// Parse tool result content for limit messages
			if limit := p.parseToolResult(item, log, model); limit != nil {
				return limit
			}
		}
	}

	return nil
}

// parseToolResult parses tool result content for limit messages
func (p *LimitParser) parseToolResult(item model.ContentItem, log model.ConversationLog, modelName string) *LimitInfo {
	// Convert content to string if possible
	contentStr := ""
	switch v := item.Content.(type) {
	case string:
		contentStr = v
	case []interface{}:
		// Handle array of content items
		for _, subItem := range v {
			if subMap, ok := subItem.(map[string]interface{}); ok {
				if text, ok := subMap["text"].(string); ok {
					contentStr += text + " "
				}
			}
		}
	}

	if contentStr == "" {
		return nil
	}

	contentLower := strings.ToLower(contentStr)
	if !strings.Contains(contentLower, "limit reached") {
		return nil
	}

	timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		return nil
	}

	limit := &LimitInfo{
		Type:      "general_limit",
		Timestamp: timestamp.Unix(),
		Content:   contentStr,
		RequestID: log.RequestId,
		SessionID: log.SessionId,
		Model:     modelName,
	}

	// Extract reset timestamp from "limit reached|timestamp" format
	if matches := p.resetPattern.FindStringSubmatch(contentStr); len(matches) > 1 {
		var resetTimestamp int64
		fmt.Sscanf(matches[1], "%d", &resetTimestamp)
		// Convert milliseconds to seconds if needed
		if resetTimestamp > 1e12 {
			resetTimestamp = resetTimestamp / 1000
		}
		limit.ResetTime = &resetTimestamp
	}

	// Set message ID if available
	if log.Message.Id != "" {
		limit.MessageID = log.Message.Id
	}

	return limit
}

// parseTextContent parses text content for limit messages (e.g., from API error messages)
func (p *LimitParser) parseTextContent(text string, log model.ConversationLog, modelName string) *LimitInfo {
	textLower := strings.ToLower(text)

	// Check if this is a limit message
	if !strings.Contains(textLower, "limit reached") &&
		!strings.Contains(textLower, "rate limit") &&
		!strings.Contains(textLower, "limit exceeded") {
		return nil
	}

	timestamp, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		return nil
	}

	limit := &LimitInfo{
		Type:      "api_error_limit",
		Timestamp: timestamp.Unix(),
		Content:   text,
		RequestID: log.RequestId,
		SessionID: log.SessionId,
		Model:     modelName,
	}

	// Extract reset timestamp from "limit reached|timestamp" format
	if matches := p.resetPattern.FindStringSubmatch(text); len(matches) > 1 {
		var resetTimestamp int64
		fmt.Sscanf(matches[1], "%d", &resetTimestamp)
		// Convert milliseconds to seconds if needed
		if resetTimestamp > 1e12 {
			resetTimestamp = resetTimestamp / 1000
		}
		limit.ResetTime = &resetTimestamp

		util.LogInfo(fmt.Sprintf("Parsed limit message with reset time: %d from text: %s",
			resetTimestamp, text))
	}

	// Set message ID if available
	if log.Message.Id != "" {
		limit.MessageID = log.Message.Id
	}

	return limit
}

// DetectWindowFromLimits analyzes limit messages to determine window start times
func (p *LimitParser) DetectWindowFromLimits(limits []LimitInfo) (windowStart *int64, source string) {
	util.LogDebug(fmt.Sprintf("DetectWindowFromLimits: Analyzing %d limits", len(limits)))
	
	if len(limits) == 0 {
		return nil, ""
	}

	// Sort limits by timestamp (most recent first)
	// Find the most recent limit with reset time
	var bestLimit *LimitInfo
	for i := range limits {
		limit := &limits[i]
		if limit.ResetTime != nil {
			util.LogDebug(fmt.Sprintf("Limit candidate %d - Type: %s, Timestamp: %s, ResetTime: %s",
				i,
				limit.Type,
				time.Unix(limit.Timestamp, 0).Format("2006-01-02 15:04:05"),
				time.Unix(*limit.ResetTime, 0).Format("2006-01-02 15:04:05")))
			
			if bestLimit == nil || limit.Timestamp > bestLimit.Timestamp {
				bestLimit = limit
			}
		}
	}

	if bestLimit == nil {
		util.LogDebug("No limits found with reset time")
		return nil, ""
	}

	// Calculate window start from reset time
	// Window starts 5 hours before reset
	windowStartTime := *bestLimit.ResetTime - (5 * 60 * 60)

	util.LogInfo(fmt.Sprintf("Detected window from limit message: start=%s, reset=%s, type=%s",
		time.Unix(windowStartTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(*bestLimit.ResetTime, 0).Format("2006-01-02 15:04:05"),
		bestLimit.Type))
	util.LogDebug(fmt.Sprintf("Best limit details - Type: %s, Timestamp: %s, Content: %.100s",
		bestLimit.Type,
		time.Unix(bestLimit.Timestamp, 0).Format("2006-01-02 15:04:05"),
		bestLimit.Content))

	return &windowStartTime, "limit_message"
}

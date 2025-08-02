package model

import (
	"fmt"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"time"
)

// HourlyMetric contains metrics for a specific hour
type HourlyMetric struct {
	Hour         time.Time
	Tokens       int
	Cost         float64
	InputTokens  int
	OutputTokens int
}

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Operation string
}

// InteractionState represents the current UI interaction state
type InteractionState struct {
	IsPaused      bool
	ShowHelp      bool
	ForceRefresh  bool
	LayoutStyle   int    // 0: Full Dashboard, 1: Minimal
	StatusMessage string // Status message to display
	ConfirmDialog *ConfirmDialog
}

// ConfirmDialog represents a confirmation dialog
type ConfirmDialog struct {
	Title     string
	Message   string
	OnConfirm func()
	OnCancel  func()
}

// AggregatedMetrics represents combined metrics from all sessions
type AggregatedMetrics struct {
	TotalCost           float64
	TotalTokens         int
	TotalMessages       int
	ActiveSessions      int
	TotalSessions       int
	AverageBurnRate     float64
	CostBurnRate        float64
	TokenBurnRate       float64
	MessageBurnRate     float64
	TimeRemaining       time.Duration
	ModelDistribution   map[string]*ModelStats
	CostLimit           float64
	TokenLimit          int
	MessageLimit        int
	LimitExceeded       bool
	LimitExceededReason string
	ResetTime           int64 // Unix timestamp
	PredictedEndTime    int64 // Unix timestamp
	CostPerMinute       float64

	// Sliding window information
	WindowSource     string // Source of window detection: "limit_message", "gap", "first_message", "rounded_hour"
	IsWindowDetected bool   // Whether window timing was explicitly detected

	// Session status
	HasActiveSession bool // Whether there is at least one active (non-expired) session
}

func (aggregated AggregatedMetrics) GetTokenPercentage() float64 {
	if aggregated.TokenLimit < 0 {
		return 0
	}

	percentage := (float64(aggregated.TotalTokens) / float64(aggregated.TokenLimit)) * 100
	if percentage > 100 {
		percentage = 100
	}
	return percentage
}

func (aggregated AggregatedMetrics) GetTokensRunOut(param LayoutParam) string {
	tp := util.GetTimeProvider()
	tokensRunOut := "Unknown"
	if aggregated.PredictedEndTime != 0 {
		predictedTime := time.Unix(aggregated.PredictedEndTime, 0).UTC()
		predictedTimeLocal := tp.In(predictedTime)
		tokensRunOut = predictedTimeLocal.Format("15:04")
		if param.TimeFormat == "12h" {
			tokensRunOut = predictedTimeLocal.Format("3:04 PM")
		}
	}
	return tokensRunOut
}

func (aggregated AggregatedMetrics) GetMessagePercentage() float64 {
	if aggregated.MessageLimit < 0 {
		return 0
	}

	percentage := (float64(aggregated.TotalMessages) / float64(aggregated.MessageLimit)) * 100
	if percentage > 100 {
		percentage = 100
	}
	return percentage
}

func (aggregated AggregatedMetrics) FormatResetTime(param LayoutParam) string {
	resetTime := aggregated.ResetTime
	if resetTime == 0 {
		return "Unknown"
	}

	tp := util.GetTimeProvider()
	resetTimeObj := time.Unix(resetTime, 0).UTC()
	resetTimeLocal := tp.In(resetTimeObj)

	util.LogDebug(fmt.Sprintf("FormatResetTime - Input: %d (%s), UTC: %s, Local: %s, TimeFormat: %s, WindowSource: %s",
		resetTime,
		time.Unix(resetTime, 0).Format("2006-01-02 15:04:05"),
		resetTimeObj.Format("2006-01-02 15:04:05"),
		resetTimeLocal.Format("2006-01-02 15:04:05"),
		param.TimeFormat,
		aggregated.WindowSource))

	if param.TimeFormat == "12h" {
		return resetTimeLocal.Format("3:04 PM")
	}
	return resetTimeLocal.Format("15:04")
}

func (aggregated AggregatedMetrics) AppendWindowIndicator(resetTimeStr string) string {
	// Add window detection indicator if applicable
	if aggregated.IsWindowDetected && aggregated.WindowSource != "" {
		windowIndicator := ""
		switch aggregated.WindowSource {
		case "limit_message":
			windowIndicator = " 🎯"
		case "gap":
			windowIndicator = " ⏳"
		case "first_message":
			windowIndicator = " 📍"
		default:
			windowIndicator = ""
		}
		resetTimeStr += windowIndicator
	}
	return resetTimeStr
}

func (aggregated AggregatedMetrics) GetCostPercentage() float64 {
	if aggregated.CostLimit < 0 {
		return 0
	}

	percentage := (float64(aggregated.TotalCost) / float64(aggregated.CostLimit)) * 100
	if percentage > 100 {
		percentage = 100
	}
	return percentage
}

// FormatRemainingTime calculates and formats the time remaining until reset
func (aggregated AggregatedMetrics) FormatRemainingTime() string {
	if aggregated.ResetTime == 0 {
		return "No active session"
	}
	
	now := util.GetTimeProvider().Now()
	resetTime := time.Unix(aggregated.ResetTime, 0)
	remaining := resetTime.Sub(now)
	
	if remaining <= 0 {
		return "Expired"
	}
	
	return util.FormatDuration(remaining)
}

// ModelStats contains statistics for a specific model
type ModelStats struct {
	Model  string
	Tokens int
	Cost   float64
	Count  int
}

// BurnRate represents the token/cost consumption rate
type BurnRate struct {
	TokensPerMinute float64
	CostPerHour     float64
	CostPerMinute   float64
}

type LayoutParam struct {
	Timezone   string
	TimeFormat string
	Plan       string
}

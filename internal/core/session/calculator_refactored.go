package session

import (
	"sort"
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"github.com/penwyp/go-claude-monitor/internal/core/session/internal"
)

// MetricsCalculatorV2 is a refactored version of the metrics calculator
type MetricsCalculatorV2 struct {
	planLimits      pricing.Plan
	limitCalc       *internal.LimitCalculator
	projectionCalc  *internal.ProjectionCalculator
	utilizationCalc *internal.UtilizationCalculator
	metricsCalc     *internal.MetricsCalculations
}

// NewMetricsCalculatorV2 creates a new refactored metrics calculator
func NewMetricsCalculatorV2(limits pricing.Plan) *MetricsCalculatorV2 {
	const sessionDurationHours = 5.0
	
	return &MetricsCalculatorV2{
		planLimits:      limits,
		limitCalc:       internal.NewLimitCalculator(limits),
		projectionCalc:  internal.NewProjectionCalculator(sessionDurationHours),
		utilizationCalc: internal.NewUtilizationCalculator(sessionDurationHours),
		metricsCalc:     internal.NewMetricsCalculations(),
	}
}

// Calculate performs all metric calculations for a session
func (c *MetricsCalculatorV2) Calculate(session *Session) {
	if session == nil {
		return
	}
	
	// Sort hourly metrics by time
	c.sortHourlyMetrics(session)
	
	// Calculate all metrics
	c.calculateUtilizationRate(session)
	c.calculateTimeToLimit(session)
	c.calculateProjections(session)
}

// sortHourlyMetrics sorts the hourly metrics by time
func (c *MetricsCalculatorV2) sortHourlyMetrics(session *Session) {
	sort.Slice(session.HourlyMetrics, func(i, j int) bool {
		return session.HourlyMetrics[i].Hour.Before(session.HourlyMetrics[j].Hour)
	})
}

// calculateUtilizationRate calculates the utilization rate based on plan limits
func (c *MetricsCalculatorV2) calculateUtilizationRate(session *Session) {
	// Check if we have enough data
	elapsedMinutes := c.metricsCalc.CalculateElapsedMinutes(session.StartTime)
	if elapsedMinutes <= 0 {
		return
	}
	
	var utilizationRate float64
	
	// Prefer token-based calculation if available
	if c.limitCalc.HasTokenLimit() && session.TokensPerMinute > 0 {
		utilizationRate = c.utilizationCalc.CalculateTokenUtilizationRate(
			session.TokensPerMinute,
			c.limitCalc.GetTokenLimit(),
		)
	} else if c.limitCalc.HasCostLimit() && session.CostPerHour > 0 {
		utilizationRate = c.utilizationCalc.CalculateCostUtilizationRate(
			session.CostPerHour,
			c.limitCalc.GetCostLimit(),
		)
	}
	
	// Update burn rate based on utilization
	if utilizationRate > 0 {
		session.BurnRate = c.metricsCalc.CalculateAdjustedBurnRate(
			session.TokensPerMinute,
			utilizationRate,
		)
	}
}

// calculateTimeToLimit calculates when limits will be reached
func (c *MetricsCalculatorV2) calculateTimeToLimit(session *Session) {
	// Skip if no rates available
	if !c.hasValidRates(session) {
		return
	}
	
	nowTimestamp := time.Now().Unix()
	predictedEndTime := c.getPredictedEndTime(session, nowTimestamp)
	
	// Update session with predicted end time
	if predictedEndTime > 0 && predictedEndTime < session.ResetTime {
		session.PredictedEndTime = predictedEndTime
		session.TimeRemaining = c.projectionCalc.CalculateTimeRemaining(
			nowTimestamp,
			predictedEndTime,
		)
	}
}

// hasValidRates checks if the session has valid rate data
func (c *MetricsCalculatorV2) hasValidRates(session *Session) bool {
	return session.TokensPerMinute > 0 || session.CostPerMinute > 0
}

// getPredictedEndTime calculates when limits will be exhausted
func (c *MetricsCalculatorV2) getPredictedEndTime(session *Session, nowTimestamp int64) int64 {
	var tokenEndTime, costEndTime int64
	
	// Calculate token-based end time
	if c.limitCalc.HasTokenLimit() && session.TokensPerMinute > 0 {
		remainingTokens := c.limitCalc.CalculateRemainingTokens(session.TotalTokens)
		minutesToTokenLimit := c.metricsCalc.CalculateMinutesToLimit(
			remainingTokens,
			session.TokensPerMinute,
		)
		tokenEndTime = c.metricsCalc.CalculatePredictedEndTime(
			nowTimestamp,
			minutesToTokenLimit,
		)
	}
	
	// Calculate cost-based end time
	if c.limitCalc.HasCostLimit() && session.CostPerMinute > 0 {
		remainingCost := c.limitCalc.CalculateRemainingCost(session.TotalCost)
		minutesToCostLimit := c.metricsCalc.CalculateMinutesToLimit(
			remainingCost,
			session.CostPerMinute,
		)
		costEndTime = c.metricsCalc.CalculatePredictedEndTime(
			nowTimestamp,
			minutesToCostLimit,
		)
	}
	
	// Return the earliest end time (whichever limit hits first)
	return c.getEarliestEndTime(tokenEndTime, costEndTime)
}

// getEarliestEndTime returns the earliest non-zero end time
func (c *MetricsCalculatorV2) getEarliestEndTime(time1, time2 int64) int64 {
	if time1 == 0 {
		return time2
	}
	if time2 == 0 {
		return time1
	}
	if time1 < time2 {
		return time1
	}
	return time2
}

// calculateProjections calculates and caps projected usage
func (c *MetricsCalculatorV2) calculateProjections(session *Session) {
	// Cap token projection
	if c.limitCalc.HasTokenLimit() {
		session.ProjectedTokens = int(c.metricsCalc.CapProjection(
			float64(session.ProjectedTokens),
			c.limitCalc.GetTokenLimit(),
		))
	}
	
	// Cap cost projection
	if c.limitCalc.HasCostLimit() {
		session.ProjectedCost = c.metricsCalc.CapProjection(
			session.ProjectedCost,
			c.limitCalc.GetCostLimit(),
		)
	}
}

// CalculateWithConfig performs calculations with custom configuration
type CalculatorConfig struct {
	SessionDurationHours float64
	EnableDebugLogging   bool
}

// MetricsCalculatorWithConfig creates a calculator with custom config
func NewMetricsCalculatorWithConfig(limits pricing.Plan, config CalculatorConfig) *MetricsCalculatorV2 {
	duration := config.SessionDurationHours
	if duration <= 0 {
		duration = 5.0 // Default
	}
	
	return &MetricsCalculatorV2{
		planLimits:      limits,
		limitCalc:       internal.NewLimitCalculator(limits),
		projectionCalc:  internal.NewProjectionCalculator(duration),
		utilizationCalc: internal.NewUtilizationCalculator(duration),
		metricsCalc:     internal.NewMetricsCalculations(),
	}
}

// GetUtilizationRate returns the current utilization rate for a session
func (c *MetricsCalculatorV2) GetUtilizationRate(session *Session) float64 {
	if c.limitCalc.HasTokenLimit() && session.TokensPerMinute > 0 {
		return c.utilizationCalc.CalculateTokenUtilizationRate(
			session.TokensPerMinute,
			c.limitCalc.GetTokenLimit(),
		)
	}
	
	if c.limitCalc.HasCostLimit() && session.CostPerHour > 0 {
		return c.utilizationCalc.CalculateCostUtilizationRate(
			session.CostPerHour,
			c.limitCalc.GetCostLimit(),
		)
	}
	
	return 0
}

// GetRemainingCapacity returns remaining tokens and cost before limits
func (c *MetricsCalculatorV2) GetRemainingCapacity(session *Session) (remainingTokens float64, remainingCost float64) {
	remainingTokens = c.limitCalc.CalculateRemainingTokens(session.TotalTokens)
	remainingCost = c.limitCalc.CalculateRemainingCost(session.TotalCost)
	return
}
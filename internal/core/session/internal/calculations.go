package internal

import (
	"math"
	"time"
	
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
)

// MetricsCalculations provides calculation utilities for session metrics
type MetricsCalculations struct{}

// NewMetricsCalculations creates a new calculations helper
func NewMetricsCalculations() *MetricsCalculations {
	return &MetricsCalculations{}
}

// CalculateElapsedMinutes calculates elapsed time in minutes from a start time
func (m *MetricsCalculations) CalculateElapsedMinutes(startTime int64) float64 {
	start := time.Unix(startTime, 0)
	elapsed := time.Since(start)
	return elapsed.Minutes()
}

// CalculateTokenUtilization calculates token utilization rate
func (m *MetricsCalculations) CalculateTokenUtilization(actualRate, tokenLimit float64, durationHours float64) float64 {
	if tokenLimit <= 0 || durationHours <= 0 {
		return 0
	}
	
	expectedRate := tokenLimit / (durationHours * 60) // Convert to per-minute rate
	if expectedRate <= 0 {
		return 0
	}
	
	return (actualRate / expectedRate) * 100
}

// CalculateCostUtilization calculates cost utilization rate
func (m *MetricsCalculations) CalculateCostUtilization(actualRate, costLimit float64, durationHours float64) float64 {
	if costLimit <= 0 || durationHours <= 0 {
		return 0
	}
	
	expectedRate := costLimit / durationHours // Per-hour rate
	if expectedRate <= 0 {
		return 0
	}
	
	return (actualRate / expectedRate) * 100
}

// CalculateAdjustedBurnRate calculates burn rate adjusted by utilization
func (m *MetricsCalculations) CalculateAdjustedBurnRate(baseRate, utilizationPercent float64) float64 {
	if utilizationPercent <= 0 {
		return 0
	}
	return baseRate * (utilizationPercent / 100)
}

// CalculateMinutesToLimit calculates minutes until a limit is reached
func (m *MetricsCalculations) CalculateMinutesToLimit(remaining, rate float64) float64 {
	if rate <= 0 || remaining <= 0 {
		return math.MaxFloat64 // Never reach limit
	}
	return remaining / rate
}

// CalculatePredictedEndTime calculates when limits will be exhausted
func (m *MetricsCalculations) CalculatePredictedEndTime(nowTimestamp int64, minutesToLimit float64) int64 {
	if minutesToLimit == math.MaxFloat64 {
		return 0 // No predicted end
	}
	return nowTimestamp + int64(minutesToLimit*60)
}

// CapProjection ensures a projection doesn't exceed a limit
func (m *MetricsCalculations) CapProjection(projection, limit float64) float64 {
	if limit <= 0 {
		return projection // No limit
	}
	if projection > limit {
		return limit
	}
	return projection
}

// LimitCalculator handles limit-based calculations
type LimitCalculator struct {
	tokenLimit float64
	costLimit  float64
}

// NewLimitCalculator creates a new limit calculator
func NewLimitCalculator(plan pricing.Plan) *LimitCalculator {
	return &LimitCalculator{
		tokenLimit: float64(plan.TokenLimit),
		costLimit:  plan.CostLimit,
	}
}

// HasTokenLimit checks if token limit is set
func (l *LimitCalculator) HasTokenLimit() bool {
	return l.tokenLimit > 0
}

// HasCostLimit checks if cost limit is set
func (l *LimitCalculator) HasCostLimit() bool {
	return l.costLimit > 0
}

// GetTokenLimit returns the token limit
func (l *LimitCalculator) GetTokenLimit() float64 {
	return l.tokenLimit
}

// GetCostLimit returns the cost limit
func (l *LimitCalculator) GetCostLimit() float64 {
	return l.costLimit
}

// CalculateRemainingTokens calculates remaining tokens before limit
func (l *LimitCalculator) CalculateRemainingTokens(used int) float64 {
	if !l.HasTokenLimit() {
		return math.MaxFloat64
	}
	remaining := l.tokenLimit - float64(used)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CalculateRemainingCost calculates remaining cost before limit
func (l *LimitCalculator) CalculateRemainingCost(used float64) float64 {
	if !l.HasCostLimit() {
		return math.MaxFloat64
	}
	remaining := l.costLimit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ProjectionCalculator handles projection calculations
type ProjectionCalculator struct {
	sessionDuration float64 // Duration in hours
}

// NewProjectionCalculator creates a new projection calculator
func NewProjectionCalculator(durationHours float64) *ProjectionCalculator {
	return &ProjectionCalculator{
		sessionDuration: durationHours,
	}
}

// ProjectTokens projects token usage for the session duration
func (p *ProjectionCalculator) ProjectTokens(tokensPerMinute float64) float64 {
	if tokensPerMinute <= 0 {
		return 0
	}
	return tokensPerMinute * p.sessionDuration * 60
}

// ProjectCost projects cost for the session duration
func (p *ProjectionCalculator) ProjectCost(costPerHour float64) float64 {
	if costPerHour <= 0 {
		return 0
	}
	return costPerHour * p.sessionDuration
}

// CalculateTimeRemaining calculates time remaining in a session
func (p *ProjectionCalculator) CalculateTimeRemaining(nowTimestamp, resetTime int64) time.Duration {
	if resetTime <= nowTimestamp {
		return 0
	}
	return time.Duration((resetTime - nowTimestamp) * int64(time.Second))
}

// UtilizationCalculator handles utilization calculations
type UtilizationCalculator struct {
	sessionDurationHours float64
}

// NewUtilizationCalculator creates a new utilization calculator
func NewUtilizationCalculator(durationHours float64) *UtilizationCalculator {
	return &UtilizationCalculator{
		sessionDurationHours: durationHours,
	}
}

// CalculateTokenUtilizationRate calculates token utilization as a percentage
func (u *UtilizationCalculator) CalculateTokenUtilizationRate(tokensPerMinute float64, tokenLimit float64) float64 {
	if tokenLimit <= 0 || u.sessionDurationHours <= 0 {
		return 0
	}
	
	expectedTokensPerMinute := tokenLimit / (u.sessionDurationHours * 60)
	if expectedTokensPerMinute <= 0 {
		return 0
	}
	
	return (tokensPerMinute / expectedTokensPerMinute) * 100
}

// CalculateCostUtilizationRate calculates cost utilization as a percentage
func (u *UtilizationCalculator) CalculateCostUtilizationRate(costPerHour, costLimit float64) float64 {
	if costLimit <= 0 || u.sessionDurationHours <= 0 {
		return 0
	}
	
	expectedCostPerHour := costLimit / u.sessionDurationHours
	if expectedCostPerHour <= 0 {
		return 0
	}
	
	return (costPerHour / expectedCostPerHour) * 100
}
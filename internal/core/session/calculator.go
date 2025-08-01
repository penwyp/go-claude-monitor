package session

import (
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
	"sort"
	"time"
)

type MetricsCalculator struct {
	planLimits pricing.Plan
}

func NewMetricsCalculator(limits pricing.Plan) *MetricsCalculator {
	return &MetricsCalculator{
		planLimits: limits,
	}
}

func (c *MetricsCalculator) Calculate(session *Session) {
	// Sort hourly metrics by time
	sort.Slice(session.HourlyMetrics, func(i, j int) bool {
		return session.HourlyMetrics[i].Hour.Before(session.HourlyMetrics[j].Hour)
	})

	// Calculate additional metrics based on plan limits
	c.calculateUtilizationRate(session)
	c.calculateTimeToLimit(session)
}

func (c *MetricsCalculator) calculateUtilizationRate(session *Session) {
	// Calculate utilization rate based on elapsed time
	startTime := time.Unix(session.StartTime, 0)
	elapsed := time.Since(startTime)
	if elapsed.Minutes() <= 0 {
		return
	}

	// Calculate actual vs expected usage
	if c.planLimits.TokenLimit > 0 {
		// Expected tokens per minute for full utilization
		expectedTokensPerMinute := float64(c.planLimits.TokenLimit) / (5 * 60) // 5 hours
		utilizationRate := session.TokensPerMinute / expectedTokensPerMinute * 100

		// Adjust burn rate based on utilization
		session.BurnRate = session.TokensPerMinute * (utilizationRate / 100)
	} else if c.planLimits.CostLimit > 0 {
		// Expected cost per hour for full utilization
		expectedCostPerHour := c.planLimits.CostLimit / 5 // 5 hours
		utilizationRate := session.CostPerHour / expectedCostPerHour * 100

		// Adjust burn rate based on utilization
		session.BurnRate = session.TokensPerMinute * (utilizationRate / 100)
	}
}

func (c *MetricsCalculator) calculateTimeToLimit(session *Session) {
	// Calculate time remaining until hitting limits
	if session.TokensPerMinute <= 0 && session.CostPerMinute <= 0 {
		return
	}

	nowTimestamp := time.Now().Unix()
	var predictedEndTimestamp int64

	// Prioritize cost limit calculation for cost-based plans
	if c.planLimits.CostLimit > 0 && session.CostPerMinute > 0 {
		// Calculate based on cost limit
		remainingCost := c.planLimits.CostLimit - session.TotalCost
		if remainingCost > 0 {
			minutesToLimit := remainingCost / session.CostPerMinute
			costEndTimestamp := nowTimestamp + int64(minutesToLimit*60)
			predictedEndTimestamp = costEndTimestamp
		}
	} else if c.planLimits.TokenLimit > 0 && session.TokensPerMinute > 0 {
		// Fallback to token limit if no cost limit
		remainingTokens := c.planLimits.TokenLimit - session.TotalTokens
		if remainingTokens > 0 {
			minutesToLimit := float64(remainingTokens) / session.TokensPerMinute
			tokenEndTimestamp := nowTimestamp + int64(minutesToLimit*60)
			predictedEndTimestamp = tokenEndTimestamp
		}
	}

	// Update predicted end time (tokens/cost will run out)
	if predictedEndTimestamp != 0 && predictedEndTimestamp < session.ResetTime {
		session.PredictedEndTime = predictedEndTimestamp
		// Update time remaining to reflect when limits will be hit
		session.TimeRemaining = time.Duration((predictedEndTimestamp - nowTimestamp) * int64(time.Second))
	}

	// Adjust projections based on limits
	c.adjustProjections(session)
}

func (c *MetricsCalculator) adjustProjections(session *Session) {
	// Cap projections at plan limits
	if c.planLimits.TokenLimit > 0 {
		if session.ProjectedTokens > c.planLimits.TokenLimit {
			session.ProjectedTokens = c.planLimits.TokenLimit
		}
	}

	if c.planLimits.CostLimit > 0 {
		if session.ProjectedCost > c.planLimits.CostLimit {
			session.ProjectedCost = c.planLimits.CostLimit
		}
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

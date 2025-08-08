package internal

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
)

// CalculateTotalTokens calculates the total tokens from a usage object
func CalculateTotalTokens(usage model.Usage) int {
	total := usage.InputTokens + usage.OutputTokens
	if usage.CacheCreationInputTokens > 0 {
		total += usage.CacheCreationInputTokens
	}
	if usage.CacheReadInputTokens > 0 {
		total += usage.CacheReadInputTokens
	}
	return total
}

// SumTokens calculates the sum of tokens from multiple usage entries
func SumTokens(entries []model.Usage) int {
	total := 0
	for _, entry := range entries {
		total += CalculateTotalTokens(entry)
	}
	return total
}

// CalculateAverage calculates the average of a slice of float64 values
func CalculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// UpdateModelStats updates model statistics with new token and cost data
func UpdateModelStats(stats *model.ModelStats, tokens int, cost float64) {
	if stats == nil {
		return
	}
	stats.Tokens += tokens
	stats.Cost += cost
	stats.Count++
}

// MergeModelStats merges two model stats objects
func MergeModelStats(stats1, stats2 *model.ModelStats) *model.ModelStats {
	if stats1 == nil && stats2 == nil {
		return &model.ModelStats{}
	}
	if stats1 == nil {
		return &model.ModelStats{
			Tokens: stats2.Tokens,
			Cost:   stats2.Cost,
			Count:  stats2.Count,
		}
	}
	if stats2 == nil {
		return &model.ModelStats{
			Tokens: stats1.Tokens,
			Cost:   stats1.Cost,
			Count:  stats1.Count,
		}
	}
	
	return &model.ModelStats{
		Tokens: stats1.Tokens + stats2.Tokens,
		Cost:   stats1.Cost + stats2.Cost,
		Count:  stats1.Count + stats2.Count,
	}
}

// CreateModelStats creates a new ModelStats object with the given values
func CreateModelStats(tokens int, cost float64, count int) *model.ModelStats {
	return &model.ModelStats{
		Tokens: tokens,
		Cost:   cost,
		Count:  count,
	}
}

// CalculateRate calculates a rate given a value and time period in seconds
func CalculateRate(value float64, periodSeconds float64) float64 {
	if periodSeconds <= 0 {
		return 0
	}
	return value / periodSeconds
}

// CalculateTokensPerMinute calculates tokens per minute rate
func CalculateTokensPerMinute(totalTokens int, elapsedSeconds float64) float64 {
	if elapsedSeconds <= 0 {
		return 0
	}
	elapsedMinutes := elapsedSeconds / 60.0
	return float64(totalTokens) / elapsedMinutes
}

// CalculateCostPerHour calculates cost per hour rate
func CalculateCostPerHour(totalCost float64, elapsedSeconds float64) float64 {
	if elapsedSeconds <= 0 {
		return 0
	}
	elapsedHours := elapsedSeconds / 3600.0
	return totalCost / elapsedHours
}

// CalculateUtilizationRate calculates the utilization rate as a percentage
func CalculateUtilizationRate(actual, expected float64) float64 {
	if expected <= 0 {
		return 0
	}
	return (actual / expected) * 100
}

// PredictTimeToLimit predicts the time remaining until a limit is reached
func PredictTimeToLimit(current, rate, limit float64) float64 {
	if rate <= 0 || current >= limit {
		return 0
	}
	remaining := limit - current
	return remaining / rate
}

// CapProjection ensures a projected value doesn't exceed a limit
func CapProjection(projected, limit float64) float64 {
	if limit > 0 && projected > limit {
		return limit
	}
	return projected
}

// SumModelStats sums up statistics from multiple ModelStats objects
func SumModelStats(statsMap map[string]*model.ModelStats) (totalTokens int, totalCost float64, totalCount int) {
	for _, stats := range statsMap {
		if stats != nil {
			totalTokens += stats.Tokens
			totalCost += stats.Cost
			totalCount += stats.Count
		}
	}
	return totalTokens, totalCost, totalCount
}
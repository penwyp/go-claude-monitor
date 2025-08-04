package session

import (
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/core/pricing"
)

func TestNewMetricsCalculator(t *testing.T) {
	tests := []struct {
		name   string
		limits pricing.Plan
	}{
		{
			name: "pro_plan",
			limits: pricing.Plan{
				Name:       "pro",
				TokenLimit: 250000,
				CostLimit:  25.0,
			},
		},
		{
			name: "max5_plan",
			limits: pricing.Plan{
				Name:       "max5",
				TokenLimit: 500000,
				CostLimit:  100.0,
			},
		},
		{
			name: "zero_limits",
			limits: pricing.Plan{
				Name:       "custom",
				TokenLimit: 0,
				CostLimit:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator(tt.limits)
			if calc == nil {
				t.Fatal("NewMetricsCalculator returned nil")
			}
			if calc.planLimits.Name != tt.limits.Name {
				t.Errorf("Expected plan name %s, got %s", tt.limits.Name, calc.planLimits.Name)
			}
			if calc.planLimits.TokenLimit != tt.limits.TokenLimit {
				t.Errorf("Expected token limit %d, got %d", tt.limits.TokenLimit, calc.planLimits.TokenLimit)
			}
			if calc.planLimits.CostLimit != tt.limits.CostLimit {
				t.Errorf("Expected cost limit %f, got %f", tt.limits.CostLimit, calc.planLimits.CostLimit)
			}
		})
	}
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		name    string
		limits  pricing.Plan
		session *Session
	}{
		{
			name: "calculate_with_hourly_metrics",
			limits: pricing.Plan{
				Name:       "pro",
				TokenLimit: 250000,
				CostLimit:  25.0,
			},
			session: &Session{
				ID:        "session1",
				StartTime: time.Now().Add(-1 * time.Hour).Unix(),
				HourlyMetrics: []*model.HourlyMetric{
					{
						Hour:        time.Now().Add(-2 * time.Hour),
						Tokens:      1000,
						Cost:        5.0,
					},
					{
						Hour:        time.Now().Add(-1 * time.Hour),
						Tokens:      2000,
						Cost:        10.0,
					},
				},
				TotalTokens:     3000,
				TotalCost:       15.0,
				TokensPerMinute: 50.0,
				CostPerMinute:   0.25,
				ResetTime:       time.Now().Add(4 * time.Hour).Unix(),
			},
		},
		{
			name: "calculate_empty_session",
			limits: pricing.Plan{
				Name:       "pro",
				TokenLimit: 250000,
				CostLimit:  25.0,
			},
			session: &Session{
				ID:              "session2",
				StartTime:       time.Now().Unix(),
				HourlyMetrics:   []*model.HourlyMetric{},
				TotalTokens:     0,
				TotalCost:       0,
				TokensPerMinute: 0,
				CostPerMinute:   0,
				ResetTime:       time.Now().Add(5 * time.Hour).Unix(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator(tt.limits)
			
			// Store original values to check if they were modified
			originalTokens := tt.session.TotalTokens
			originalCost := tt.session.TotalCost
			originalMetricsLen := len(tt.session.HourlyMetrics)
			
			calc.Calculate(tt.session)
			
			// Verify hourly metrics are sorted
			for i := 1; i < len(tt.session.HourlyMetrics); i++ {
				if tt.session.HourlyMetrics[i-1].Hour.After(tt.session.HourlyMetrics[i].Hour) {
					t.Errorf("Hourly metrics not sorted: metric %d (%v) is after metric %d (%v)",
						i-1, tt.session.HourlyMetrics[i-1].Hour,
						i, tt.session.HourlyMetrics[i].Hour)
				}
			}
			
			// Verify basic data integrity
			if tt.session.TotalTokens != originalTokens {
				t.Errorf("Calculate should not modify TotalTokens, was %d, now %d", originalTokens, tt.session.TotalTokens)
			}
			if tt.session.TotalCost != originalCost {
				t.Errorf("Calculate should not modify TotalCost, was %f, now %f", originalCost, tt.session.TotalCost)
			}
			if len(tt.session.HourlyMetrics) != originalMetricsLen {
				t.Errorf("Calculate should not modify HourlyMetrics length, was %d, now %d", originalMetricsLen, len(tt.session.HourlyMetrics))
			}
		})
	}
}

func TestCalculateUtilizationRate(t *testing.T) {
	tests := []struct {
		name             string
		limits           pricing.Plan
		session          *Session
		expectBurnRate   bool
		expectedMinBurn  float64
		expectedMaxBurn  float64
	}{
		{
			name: "token_based_calculation",
			limits: pricing.Plan{
				TokenLimit: 300000, // 5 hours = 1000 tokens per minute for full utilization
				CostLimit:  0,
			},
			session: &Session{
				StartTime:       time.Now().Add(-30 * time.Minute).Unix(),
				TokensPerMinute: 500.0, // 50% utilization
				BurnRate:        0,     // Will be calculated
			},
			expectBurnRate:  true,
			expectedMinBurn: 200.0, // Should be around 250 (500 * 0.5)
			expectedMaxBurn: 300.0,
		},
		{
			name: "cost_based_calculation",
			limits: pricing.Plan{
				TokenLimit: 0,
				CostLimit:  25.0, // 5 hours = $5 per hour for full utilization
			},
			session: &Session{
				StartTime:       time.Now().Add(-60 * time.Minute).Unix(),
				TokensPerMinute: 1000.0,
				CostPerHour:     2.5, // 50% utilization
				BurnRate:        0,   // Will be calculated
			},
			expectBurnRate:  true,
			expectedMinBurn: 400.0, // Should be around 500 (1000 * 0.5)
			expectedMaxBurn: 600.0,
		},
		{
			name: "zero_elapsed_time",
			limits: pricing.Plan{
				TokenLimit: 300000,
				CostLimit:  0,
			},
			session: &Session{
				StartTime:       time.Now().Add(1 * time.Minute).Unix(), // Future time
				TokensPerMinute: 500.0,
				BurnRate:        0,
			},
			expectBurnRate: false, // Should not calculate for future/zero elapsed time
		},
		{
			name: "zero_limits",
			limits: pricing.Plan{
				TokenLimit: 0,
				CostLimit:  0,
			},
			session: &Session{
				StartTime:       time.Now().Add(-30 * time.Minute).Unix(),
				TokensPerMinute: 500.0,
				BurnRate:        0,
			},
			expectBurnRate: false, // Should not calculate with zero limits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator(tt.limits)
			originalBurnRate := tt.session.BurnRate
			
			calc.calculateUtilizationRate(tt.session)
			
			if tt.expectBurnRate {
				if tt.session.BurnRate == originalBurnRate {
					t.Errorf("Expected burn rate to be calculated, but it remained %f", originalBurnRate)
				}
				if tt.session.BurnRate < tt.expectedMinBurn || tt.session.BurnRate > tt.expectedMaxBurn {
					t.Errorf("Expected burn rate between %f and %f, got %f", tt.expectedMinBurn, tt.expectedMaxBurn, tt.session.BurnRate)
				}
			} else {
				if tt.session.BurnRate != originalBurnRate {
					t.Errorf("Expected burn rate to remain unchanged (%f), but got %f", originalBurnRate, tt.session.BurnRate)
				}
			}
		})
	}
}

func TestCalculateTimeToLimit(t *testing.T) {
	tests := []struct {
		name                     string
		limits                   pricing.Plan
		session                  *Session
		expectPredictedEndTime   bool
		expectTimeRemainingSet   bool
	}{
		{
			name: "cost_limit_reached_first",
			limits: pricing.Plan{
				TokenLimit: 500000,
				CostLimit:  25.0,
			},
			session: &Session{
				TotalCost:     20.0, // $5 remaining
				TotalTokens:   50000, // 450K tokens remaining
				CostPerMinute: 0.5,   // Will reach cost limit in 10 minutes
				TokensPerMinute: 1000, // Would reach token limit in 450 minutes
				ResetTime:     time.Now().Add(3 * time.Hour).Unix(),
			},
			expectPredictedEndTime: true,
			expectTimeRemainingSet: true,
		},
		{
			name: "token_limit_only",
			limits: pricing.Plan{
				TokenLimit: 100000,
				CostLimit:  0, // No cost limit
			},
			session: &Session{
				TotalTokens:     90000, // 10K tokens remaining
				TokensPerMinute: 1000,  // Will reach limit in 10 minutes
				ResetTime:       time.Now().Add(3 * time.Hour).Unix(),
			},
			expectPredictedEndTime: true,
			expectTimeRemainingSet: true,
		},
		{
			name: "zero_burn_rates",
			limits: pricing.Plan{
				TokenLimit: 100000,
				CostLimit:  25.0,
			},
			session: &Session{
				TotalTokens:     50000,
				TotalCost:       10.0,
				TokensPerMinute: 0, // No usage
				CostPerMinute:   0, // No usage
				ResetTime:       time.Now().Add(3 * time.Hour).Unix(),
			},
			expectPredictedEndTime: false,
			expectTimeRemainingSet: false,
		},
		{
			name: "already_at_limits",
			limits: pricing.Plan{
				TokenLimit: 100000,
				CostLimit:  25.0,
			},
			session: &Session{
				TotalTokens:     100000, // Already at token limit
				TotalCost:       25.0,   // Already at cost limit
				TokensPerMinute: 1000,
				CostPerMinute:   0.5,
				ResetTime:       time.Now().Add(3 * time.Hour).Unix(),
			},
			expectPredictedEndTime: false, // No remaining capacity
			expectTimeRemainingSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator(tt.limits)
			originalPredictedEndTime := tt.session.PredictedEndTime
			originalTimeRemaining := tt.session.TimeRemaining
			
			calc.calculateTimeToLimit(tt.session)
			
			if tt.expectPredictedEndTime {
				if tt.session.PredictedEndTime == originalPredictedEndTime {
					t.Errorf("Expected PredictedEndTime to be set, but it remained %d", originalPredictedEndTime)
				}
				if tt.session.PredictedEndTime <= time.Now().Unix() {
					t.Errorf("Expected PredictedEndTime to be in the future, got %d", tt.session.PredictedEndTime)
				}
				if tt.session.PredictedEndTime >= tt.session.ResetTime {
					t.Errorf("Expected PredictedEndTime (%d) to be before ResetTime (%d)", tt.session.PredictedEndTime, tt.session.ResetTime)
				}
			} else {
				if tt.session.PredictedEndTime != originalPredictedEndTime {
					t.Errorf("Expected PredictedEndTime to remain unchanged (%d), but got %d", originalPredictedEndTime, tt.session.PredictedEndTime)
				}
			}
			
			if tt.expectTimeRemainingSet {
				if tt.session.TimeRemaining == originalTimeRemaining {
					t.Errorf("Expected TimeRemaining to be set, but it remained %v", originalTimeRemaining)
				}
				if tt.session.TimeRemaining <= 0 {
					t.Errorf("Expected TimeRemaining to be positive, got %v", tt.session.TimeRemaining)
				}
			} else {
				if tt.session.TimeRemaining != originalTimeRemaining {
					t.Errorf("Expected TimeRemaining to remain unchanged (%v), but got %v", originalTimeRemaining, tt.session.TimeRemaining)
				}
			}
		})
	}
}

func TestAdjustProjections(t *testing.T) {
	tests := []struct {
		name                    string
		limits                  pricing.Plan
		session                 *Session
		expectedProjectedTokens int
		expectedProjectedCost   float64
	}{
		{
			name: "cap_at_token_limit",
			limits: pricing.Plan{
				TokenLimit: 100000,
				CostLimit:  0,
			},
			session: &Session{
				ProjectedTokens: 150000, // Exceeds limit
				ProjectedCost:   50.0,
			},
			expectedProjectedTokens: 100000, // Should be capped
			expectedProjectedCost:   50.0,   // Should remain unchanged
		},
		{
			name: "cap_at_cost_limit",
			limits: pricing.Plan{
				TokenLimit: 0,
				CostLimit:  25.0,
			},
			session: &Session{
				ProjectedTokens: 50000,
				ProjectedCost:   30.0, // Exceeds limit
			},
			expectedProjectedTokens: 50000, // Should remain unchanged
			expectedProjectedCost:   25.0,  // Should be capped
		},
		{
			name: "within_both_limits",
			limits: pricing.Plan{
				TokenLimit: 100000,
				CostLimit:  25.0,
			},
			session: &Session{
				ProjectedTokens: 80000, // Within limit
				ProjectedCost:   20.0,  // Within limit
			},
			expectedProjectedTokens: 80000, // Should remain unchanged
			expectedProjectedCost:   20.0,  // Should remain unchanged
		},
		{
			name: "zero_limits",
			limits: pricing.Plan{
				TokenLimit: 0,
				CostLimit:  0,
			},
			session: &Session{
				ProjectedTokens: 150000,
				ProjectedCost:   50.0,
			},
			expectedProjectedTokens: 150000, // Should remain unchanged with zero limits
			expectedProjectedCost:   50.0,   // Should remain unchanged with zero limits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator(tt.limits)
			
			calc.adjustProjections(tt.session)
			
			if tt.session.ProjectedTokens != tt.expectedProjectedTokens {
				t.Errorf("Expected ProjectedTokens %d, got %d", tt.expectedProjectedTokens, tt.session.ProjectedTokens)
			}
			if tt.session.ProjectedCost != tt.expectedProjectedCost {
				t.Errorf("Expected ProjectedCost %f, got %f", tt.expectedProjectedCost, tt.session.ProjectedCost)
			}
		})
	}
}

// Test edge cases and error conditions
func TestCalculatorEdgeCases(t *testing.T) {
	t.Run("nil_session", func(t *testing.T) {
		calc := NewMetricsCalculator(pricing.Plan{TokenLimit: 100000})
		// This should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Calculate panicked with nil session: %v", r)
			}
		}()
		calc.Calculate(nil)
	})

	t.Run("nil_hourly_metrics", func(t *testing.T) {
		calc := NewMetricsCalculator(pricing.Plan{TokenLimit: 100000})
		session := &Session{
			ID:            "test",
			StartTime:     time.Now().Unix(),
			HourlyMetrics: nil,
		}
		// This should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Calculate panicked with nil HourlyMetrics: %v", r)
			}
		}()
		calc.Calculate(session)
	})

	t.Run("extreme_values", func(t *testing.T) {
		calc := NewMetricsCalculator(pricing.Plan{TokenLimit: 1000000000, CostLimit: 10000.0})
		session := &Session{
			StartTime:       time.Now().Add(-24 * time.Hour).Unix(),
			TotalTokens:     999999999,
			TotalCost:       9999.99,
			TokensPerMinute: 1000000,
			CostPerMinute:   1000.0,
			ProjectedTokens: 2000000000, // Will be capped
			ProjectedCost:   20000.0,    // Will be capped
			ResetTime:       time.Now().Add(1 * time.Hour).Unix(),
		}
		
		calc.Calculate(session)
		
		// Verify projections are capped
		if session.ProjectedTokens > calc.planLimits.TokenLimit {
			t.Errorf("ProjectedTokens not capped: %d > %d", session.ProjectedTokens, calc.planLimits.TokenLimit)
		}
		if session.ProjectedCost > calc.planLimits.CostLimit {
			t.Errorf("ProjectedCost not capped: %f > %f", session.ProjectedCost, calc.planLimits.CostLimit)
		}
	})

	t.Run("negative_values", func(t *testing.T) {
		calc := NewMetricsCalculator(pricing.Plan{TokenLimit: 100000, CostLimit: 25.0})
		session := &Session{
			StartTime:       time.Now().Unix(),
			TotalTokens:     -1000,  // Negative (shouldn't happen in real usage)
			TotalCost:       -10.0,  // Negative (shouldn't happen in real usage)
			TokensPerMinute: -100.0, // Negative (shouldn't happen in real usage)
			CostPerMinute:   -1.0,   // Negative (shouldn't happen in real usage)
			ResetTime:       time.Now().Add(5 * time.Hour).Unix(),
		}
		
		// Should not panic with negative values
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Calculate panicked with negative values: %v", r)
			}
		}()
		calc.Calculate(session)
	})
}

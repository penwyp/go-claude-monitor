package top

import (
	"github.com/penwyp/go-claude-monitor/internal/core/session"
	"github.com/penwyp/go-claude-monitor/internal/presentation/display"
	"github.com/penwyp/go-claude-monitor/internal/presentation/interaction"
)

// convertSessionsForDisplay converts session.Session to display.Session
func convertSessionsForDisplay(sessions []*session.Session) []*display.Session {
	result := make([]*display.Session, len(sessions))
	for i, s := range sessions {
		result[i] = &display.Session{
			ID:               s.ID,
			StartTime:        s.StartTime,
			StartHour:        s.StartHour,
			EndTime:          s.EndTime,
			ActualEndTime:    s.ActualEndTime,
			IsActive:         s.IsActive,
			IsGap:            s.IsGap,
			ProjectName:      s.ProjectName,
			SentMessageCount: s.SentMessageCount,
			WindowStartTime:  s.WindowStartTime,
			IsWindowDetected: s.IsWindowDetected,
			WindowSource:     s.WindowSource,
			FirstEntryTime:   s.FirstEntryTime,
			TotalTokens:      s.TotalTokens,
			TotalCost:        s.TotalCost,
			ProjectTokens:    s.ProjectTokens,
			ProjectCost:      s.ProjectCost,
			ModelsUsed:       s.ModelsUsed,
			EntriesCount:     s.EntriesCount,
			WindowPriority:   s.WindowPriority,
			// Real-time metrics
			ResetTime:         s.ResetTime,
			BurnRate:          s.BurnRate,
			ModelDistribution: s.ModelDistribution,
			MessageCount:      s.MessageCount,
			CostPerHour:       s.CostPerHour,
			CostPerMinute:     s.CostPerMinute,
			TokensPerMinute:   s.TokensPerMinute,
			PredictedEndTime:  s.PredictedEndTime,
		}
		// Copy projects map
		if s.Projects != nil {
			result[i].Projects = make(map[string]*display.ProjectStats)
			for k, v := range s.Projects {
				result[i].Projects[k] = &display.ProjectStats{
					TokenCount:   v.TotalTokens,
					Cost:         v.TotalCost,
					MessageCount: v.MessageCount,
					ModelsUsed:   make(map[string]int),
				}
				// Convert model distribution
				for model, stats := range v.ModelDistribution {
					result[i].Projects[k].ModelsUsed[model] = stats.Tokens
				}
			}
		}
	}
	return result
}

// convertSessionsForSorting converts session.Session to interaction.Session
func convertSessionsForSorting(sessions []*session.Session) []*interaction.Session {
	result := make([]*interaction.Session, len(sessions))
	for i, s := range sessions {
		result[i] = &interaction.Session{
			StartTime:   s.StartTime,
			TotalCost:   s.TotalCost,
			TotalTokens: s.TotalTokens,
		}
	}
	return result
}

// applySortingToOriginal applies the sorting from interaction sessions back to original sessions
func applySortingToOriginal(original []*session.Session, sorted []*interaction.Session) {
	// Create a map from sorted sessions to their indices
	sortedMap := make(map[int64]int)
	for i, s := range sorted {
		sortedMap[s.StartTime] = i
	}
	
	// Create a copy of original sessions
	temp := make([]*session.Session, len(original))
	copy(temp, original)
	
	// Reorder original based on sorted indices
	for i, s := range temp {
		if idx, ok := sortedMap[s.StartTime]; ok {
			original[idx] = s
		} else {
			// Fallback in case of mismatch
			original[i] = s
		}
	}
}
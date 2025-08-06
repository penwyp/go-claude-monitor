package interaction

import (
	"sort"
	"github.com/penwyp/go-claude-monitor/internal/core/session"
)

// SortField represents the field to sort sessions by
type SortField int

const (
	SortByTime SortField = iota
	SortByCost
	SortByTokens
)

// SortOrder represents the sort order
type SortOrder int

const (
	SortAscending SortOrder = iota
	SortDescending
)

// SessionSorter handles sorting of sessions
type SessionSorter struct {
	field SortField
	order SortOrder
}

// NewSessionSorter creates a new session sorter
func NewSessionSorter() *SessionSorter {
	return &SessionSorter{
		field: SortByTime,
		order: SortDescending,
	}
}

// Sort sorts the sessions based on current settings
func (s *SessionSorter) Sort(sessions []*session.Session) {
	sort.Slice(sessions, func(i, j int) bool {
		var less bool

		switch s.field {
		case SortByTime:
			less = sessions[i].StartTime < sessions[j].StartTime
		case SortByCost:
			less = sessions[i].TotalCost < sessions[j].TotalCost
		case SortByTokens:
			less = sessions[i].TotalTokens < sessions[j].TotalTokens
		}

		if s.order == SortDescending {
			return !less
		}
		return less
	})
}

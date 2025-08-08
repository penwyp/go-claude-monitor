package interaction

import (
	"testing"
	"time"
)

func TestKeyboardReader(t *testing.T) {
	// Test key event parsing
	kr := &KeyboardReader{
		input: make(chan KeyEvent, 10),
		stop:  make(chan struct{}),
	}

	tests := []struct {
		name     string
		input    []byte
		expected *KeyEvent
	}{
		{
			name:     "Regular char",
			input:    []byte{'a'},
			expected: &KeyEvent{Key: 'a', Type: KeyChar},
		},
		{
			name:     "Escape",
			input:    []byte{27},
			expected: &KeyEvent{Key: 27, Type: KeyEscape},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := kr.parseInput(tt.input)
			if tt.expected == nil {
				if event != nil {
					t.Errorf("Expected nil, got %+v", event)
				}
			} else {
				if event == nil {
					t.Errorf("Expected %+v, got nil", tt.expected)
				} else if event.Type != tt.expected.Type || event.Key != tt.expected.Key {
					t.Errorf("Expected %+v, got %+v", tt.expected, event)
				}
			}
		})
	}
}

func TestSessionSorter(t *testing.T) {
	// Create test sessions
	now := time.Now()
	session1Time := now.Add(-2 * time.Hour).Unix()
	session2Time := now.Add(-1 * time.Hour).Unix() // Most recent
	session3Time := now.Add(-3 * time.Hour).Unix() // Oldest
	
	sessions := []*Session{
		{
			StartTime:   session1Time,
			TotalTokens: 1000,
			TotalCost:   10.0,
		},
		{
			StartTime:   session2Time,
			TotalTokens: 2000,
			TotalCost:   5.0,
		},
		{
			StartTime:   session3Time,
			TotalTokens: 500,
			TotalCost:   15.0,
		},
	}

	sorter := NewSessionSorter()

	// Test time sorting (default)
	sorter.Sort(sessions)
	if sessions[0].StartTime != session2Time {
		t.Errorf("Expected most recent session first for time sort descending, got %d", sessions[0].StartTime)
	}
}

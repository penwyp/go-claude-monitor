package session

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
	sessions := []*Session{
		{
			ID:          "session1",
			StartTime:   now.Add(-2 * time.Hour).Unix(),
			TotalTokens: 1000,
			TotalCost:   10.0,
		},
		{
			ID:          "session2",
			StartTime:   now.Add(-1 * time.Hour).Unix(),
			TotalTokens: 2000,
			TotalCost:   5.0,
		},
		{
			ID:          "session3",
			StartTime:   now.Add(-3 * time.Hour).Unix(),
			TotalTokens: 500,
			TotalCost:   15.0,
		},
	}

	sorter := NewSessionSorter()

	// Test time sorting (default)
	sorter.Sort(sessions)
	if sessions[0].ID != "session2" {
		t.Errorf("Expected session2 first for time sort descending, got %s", sessions[0].ID)
	}
}

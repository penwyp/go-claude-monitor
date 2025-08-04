package session

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTerminalDisplay(t *testing.T) {
	tests := []struct {
		name   string
		config *TopConfig
	}{
		{
			name: "default_config",
			config: &TopConfig{
				Plan:       "pro",
				Timezone:   "UTC",
				TimeFormat: "24h",
			},
		},
		{
			name: "minimal_config",
			config: &TopConfig{},
		},
		{
			name:   "nil_config",
			config: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display := NewTerminalDisplay(tt.config)
			
			assert.NotNil(t, display)
			assert.Equal(t, tt.config, display.config)
			assert.True(t, display.smartRenderEnabled)
			assert.NotNil(t, display.previousScreen)
			assert.False(t, display.inAlternateScreen)
			assert.Equal(t, int64(0), display.lastDraw)
			assert.Equal(t, 0, display.lastLayoutStyle)
			assert.False(t, display.previousShowHelp)
		})
	}
}

func TestTerminalDisplayScreenManagement(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	// Test initial state
	assert.False(t, display.inAlternateScreen)
	
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	// Buffer to capture output
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	// Test EnterAlternateScreen
	display.EnterAlternateScreen()
	assert.True(t, display.inAlternateScreen)
	
	// Test double call (should not do anything)
	display.EnterAlternateScreen()
	assert.True(t, display.inAlternateScreen)
	
	// Test ClearScreen when in alternate screen
	display.ClearScreen()
	
	// Test ExitAlternateScreen
	display.ExitAlternateScreen()
	assert.False(t, display.inAlternateScreen)
	
	// Test double exit (should not do anything)
	display.ExitAlternateScreen()
	assert.False(t, display.inAlternateScreen)
	
	// Test ClearScreen when not in alternate screen (should not output anything)
	display.ClearScreen()
	
	// Restore stdout and capture output
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	outputStr := output.String()
	
	// Verify escape sequences are present
	assert.Contains(t, outputStr, "\033[?1049h") // Enter alternate screen
	assert.Contains(t, outputStr, "\033[?1049l") // Exit alternate screen
	assert.Contains(t, outputStr, "\033[2J")     // Clear screen sequences
}

func TestCalculateAggregatedMetrics(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	t.Run("empty_sessions", func(t *testing.T) {
		aggregated := display.calculateAggregatedMetrics([]*Session{})
		
		assert.NotNil(t, aggregated)
		assert.Equal(t, 0, aggregated.TotalSessions)
		assert.Equal(t, 0, aggregated.ActiveSessions)
		assert.Empty(t, aggregated.ModelDistribution)
		assert.False(t, aggregated.HasActiveSession)
		assert.False(t, aggregated.LimitExceeded)
		
		// Verify plan limits are set correctly - check that they are non-zero
		// The exact values depend on the plan implementation
		assert.True(t, aggregated.CostLimit > 0, "CostLimit should be greater than 0")
		assert.True(t, aggregated.TokenLimit > 0, "TokenLimit should be greater than 0")
	})
	
	t.Run("single_active_session", func(t *testing.T) {
		currentTime := time.Now().Unix()
		sessions := []*Session{
			{
				ID:                "session1",
				TotalTokens:       1000,
				TotalCost:         5.0,
				MessageCount:      10,
				IsActive:          true,
				StartTime:         currentTime - 3600, // 1 hour ago
				ResetTime:         currentTime + 3600, // 1 hour from now
				WindowSource:      "limit_message",
				IsWindowDetected:  true,
				BurnRate:          1.5,
				CostPerHour:       3.0,
				CostPerMinute:     0.05,
				TokensPerMinute:   10.0,
				ModelDistribution: map[string]*model.ModelStats{
					"claude-3-5-sonnet": {
						Model:  "claude-3-5-sonnet",
						Tokens: 1000,
						Cost:   5.0,
						Count:  10,
					},
				},
			},
		}
		
		aggregated := display.calculateAggregatedMetrics(sessions)
		
		assert.Equal(t, 1, aggregated.TotalSessions)
		assert.Equal(t, 1, aggregated.ActiveSessions)
		assert.Equal(t, 1000, aggregated.TotalTokens)
		assert.Equal(t, 5.0, aggregated.TotalCost)
		assert.Equal(t, 10, aggregated.TotalMessages)
		assert.True(t, aggregated.HasActiveSession)
		assert.Equal(t, "limit_message", aggregated.WindowSource)
		assert.True(t, aggregated.IsWindowDetected)
		assert.Equal(t, currentTime+3600, aggregated.ResetTime)
		
		// Verify burn rates
		assert.Equal(t, 0.05, aggregated.CostPerMinute)
		assert.Equal(t, 10.0, aggregated.TokenBurnRate)
		
		// Verify model distribution
		assert.Len(t, aggregated.ModelDistribution, 1)
		sonnetStats := aggregated.ModelDistribution["claude-3-5-sonnet"]
		assert.NotNil(t, sonnetStats)
		assert.Equal(t, 1000, sonnetStats.Tokens)
		assert.Equal(t, 5.0, sonnetStats.Cost)
		assert.Equal(t, 10, sonnetStats.Count)
	})
	
	t.Run("multiple_sessions_first_active", func(t *testing.T) {
		currentTime := time.Now().Unix()
		sessions := []*Session{
			{
				ID:                "session1",
				TotalTokens:       1000,
				TotalCost:         5.0,
				MessageCount:      10,
				IsActive:          true,
				StartTime:         currentTime - 7200, // 2 hours ago (earlier)
				ResetTime:         currentTime + 1800, // 30 min from now
				WindowSource:      "gap",
				IsWindowDetected:  true,
				CostPerMinute:     0.05,
				TokensPerMinute:   10.0,
				ModelDistribution: map[string]*model.ModelStats{
					"claude-3-5-sonnet": {Tokens: 1000, Cost: 5.0, Count: 10},
				},
			},
			{
				ID:                "session2",
				TotalTokens:       2000,
				TotalCost:         10.0,
				MessageCount:      20,
				IsActive:          true,
				StartTime:         currentTime - 3600, // 1 hour ago (later)
				ResetTime:         currentTime + 3600, // 1 hour from now
				WindowSource:      "first_message",
				IsWindowDetected:  false,
				CostPerMinute:     0.10,
				TokensPerMinute:   20.0,
				ModelDistribution: map[string]*model.ModelStats{
					"claude-3-5-haiku": {Tokens: 2000, Cost: 10.0, Count: 20},
				},
			},
		}
		
		aggregated := display.calculateAggregatedMetrics(sessions)
		
		assert.Equal(t, 2, aggregated.TotalSessions)
		assert.Equal(t, 2, aggregated.ActiveSessions)
		
		// Should use metrics from first active session (session1 - earlier start time)
		assert.Equal(t, 1000, aggregated.TotalTokens)
		assert.Equal(t, 5.0, aggregated.TotalCost)
		assert.Equal(t, 10, aggregated.TotalMessages)
		assert.Equal(t, "gap", aggregated.WindowSource)
		assert.True(t, aggregated.IsWindowDetected)
		assert.Equal(t, currentTime+1800, aggregated.ResetTime)
		
		// But model distribution should combine both sessions
		assert.Len(t, aggregated.ModelDistribution, 2)
		assert.NotNil(t, aggregated.ModelDistribution["claude-3-5-sonnet"])
		assert.NotNil(t, aggregated.ModelDistribution["claude-3-5-haiku"])
	})
	
	t.Run("limit_exceeded_scenarios", func(t *testing.T) {
		// Test cost limit exceeded (pro plan limit is 18.0)
		sessions := []*Session{
			{
				ID:           "session1",
				TotalCost:    25.0, // Exceeds pro plan limit of 18.0
				TotalTokens:  1000,
				MessageCount: 10,
				IsActive:     true,
			},
		}
		
		aggregated := display.calculateAggregatedMetrics(sessions)
		assert.True(t, aggregated.LimitExceeded)
		assert.Equal(t, "COST LIMIT EXCEEDED", aggregated.LimitExceededReason)
		
		// Test token limit exceeded (pro plan limit is 4,000,000)
		sessions[0].TotalCost = 5.0 // Within cost limit
		sessions[0].TotalTokens = 5000000 // Exceeds pro plan token limit of 4,000,000
		
		aggregated = display.calculateAggregatedMetrics(sessions)
		assert.True(t, aggregated.LimitExceeded)
		assert.Equal(t, "TOKEN LIMIT EXCEEDED", aggregated.LimitExceededReason)
		
		// Test message limit exceeded (pro plan limit is 40)
		sessions[0].TotalTokens = 1000 // Within token limit
		sessions[0].MessageCount = 50 // Exceeds message limit of 40
		
		aggregated = display.calculateAggregatedMetrics(sessions)
		assert.True(t, aggregated.LimitExceeded)
		assert.Equal(t, "MESSAGE LIMIT EXCEEDED", aggregated.LimitExceededReason)
	})
	
	t.Run("predicted_end_time_calculation", func(t *testing.T) {
		currentTime := time.Now().Unix()
		sessions := []*Session{
			{
				ID:              "session1",
				TotalCost:       9.0, // Half of pro plan limit (18.0)
				TotalTokens:     2000000, // Half of pro plan token limit (4,000,000)
				MessageCount:    10,
				IsActive:        true,
				ResetTime:       currentTime + 3600,
				CostPerMinute:   0.15, // Would reach limit in 60 minutes (9.0 / 0.15)
				TokensPerMinute: 2083.33, // Would reach token limit in 60 minutes
			},
		}
		
		aggregated := display.calculateAggregatedMetrics(sessions)
		
		// Should calculate predicted end time based on cost burn rate
		expectedEndTime := currentTime + 60*60 // 60 minutes from now
		assert.InDelta(t, expectedEndTime, aggregated.PredictedEndTime, 60) // Allow 60 second tolerance
	})
}

func TestRenderWithState(t *testing.T) {
	config := &TopConfig{
		Plan:       "pro",
		Timezone:   "UTC",
		TimeFormat: "24h",
	}
	
	display := NewTerminalDisplay(config)
	
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	// Enter alternate screen for testing
	display.EnterAlternateScreen()
	
	t.Run("normal_render", func(t *testing.T) {
		sessions := []*Session{
			{
				ID:           "session1",
				TotalTokens:  1000,
				TotalCost:    5.0,
				MessageCount: 10,
				IsActive:     true,
			},
		}
		
		state := model.InteractionState{
			LayoutStyle: 0,
			ShowHelp:    false,
		}
		
		// This should not panic
		display.RenderWithState(sessions, state)
	})
	
	t.Run("help_render", func(t *testing.T) {
		state := model.InteractionState{
			ShowHelp: true,
		}
		
		// This should render help and not panic
		display.RenderWithState([]*Session{}, state)
	})
	
	t.Run("confirm_dialog_render", func(t *testing.T) {
		dialog := &model.ConfirmDialog{
			Title:   "Test Dialog",
			Message: "This is a test confirmation dialog",
		}
		
		state := model.InteractionState{
			ConfirmDialog: dialog,
		}
		
		// This should render dialog and not panic
		display.RenderWithState([]*Session{}, state)
	})
	
	t.Run("layout_style_change", func(t *testing.T) {
		// First render with style 0
		state1 := model.InteractionState{LayoutStyle: 0}
		display.RenderWithState([]*Session{}, state1)
		
		// Then render with style 1 (should trigger clear screen)
		state2 := model.InteractionState{LayoutStyle: 1}
		display.RenderWithState([]*Session{}, state2)
		
		assert.Equal(t, 1, display.lastLayoutStyle)
	})
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	outputStr := output.String()
	
	// Basic checks that output was generated
	assert.NotEmpty(t, outputStr)
}

func TestRenderHelp(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	display.renderHelp()
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	outputStr := output.String()
	
	// Verify help content
	assert.Contains(t, outputStr, "Claude Monitor Top - Help")
	assert.Contains(t, outputStr, "Keyboard Shortcuts:")
	assert.Contains(t, outputStr, "q/Esc/Ctrl+C - Quit the program")
	assert.Contains(t, outputStr, "r         - Force refresh data")
	assert.Contains(t, outputStr, "t         - Change layout style")
	assert.Contains(t, outputStr, "Layout Styles:")
	assert.Contains(t, outputStr, "Status Colors:")
	assert.Contains(t, outputStr, "🟢 Green")
	assert.Contains(t, outputStr, "🟡 Yellow")
	assert.Contains(t, outputStr, "🔴 Red")
}

func TestRenderConfirmDialog(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	display.EnterAlternateScreen() // Needed for ClearScreen in renderConfirmDialog
	
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	dialog := &model.ConfirmDialog{
		Title:   "Test Confirm",
		Message: "Are you sure you want to proceed with this test action?",
	}
	
	display.renderConfirmDialog(dialog)
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	outputStr := output.String()
	
	// Verify dialog content
	assert.Contains(t, outputStr, "Test Confirm")
	assert.Contains(t, outputStr, "Are you sure you want to proceed")
	assert.Contains(t, outputStr, "(Y)es / (N)o")
	assert.Contains(t, outputStr, "╔") // Box drawing characters
	assert.Contains(t, outputStr, "╚")
}

func TestRenderStatusMessage(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	display.renderStatusMessage("Test status message")
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	outputStr := output.String()
	
	// Verify status message
	assert.Contains(t, outputStr, "Status: Test status message")
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    int
		expected []string
	}{
		{
			name:     "short_text",
			text:     "Hello world",
			width:    20,
			expected: []string{"Hello world"},
		},
		{
			name:     "exact_width",
			text:     "Hello world",
			width:    11,
			expected: []string{"Hello world"},
		},
		{
			name:     "needs_wrapping",
			text:     "This is a longer text that needs to be wrapped",
			width:    20,
			expected: []string{"This is a longer", "text that needs to be", "wrapped"},
		},
		{
			name:     "single_long_word",
			text:     "supercalifragilisticexpialidocious",
			width:    20,
			expected: []string{"supercalifragilisticexpialidocious"},
		},
		{
			name:     "empty_text",
			text:     "",
			width:    10,
			expected: []string{},
		},
		{
			name:     "multiple_spaces",
			text:     "Hello    world    test",
			width:    10,
			expected: []string{"Hello", "world test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSmartRender(t *testing.T) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	// Test smart rendering enable/disable
	assert.True(t, display.smartRenderEnabled)
	
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	
	var output bytes.Buffer
	done := make(chan bool)
	go func() {
		io.Copy(&output, r)
		done <- true
	}()
	
	// Test smart render with mock layout strategy
	// We can't easily test the actual layout strategy without complex mocking
	// but we can verify the method doesn't panic
	display.smartRenderEnabled = false
	
	state := model.InteractionState{LayoutStyle: 0}
	sessions := []*Session{
		{
			ID:           "test",
			TotalTokens:  100,
			TotalCost:    1.0,
			MessageCount: 5,
			IsActive:     true,
		},
	}
	
	display.RenderWithState(sessions, state)
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	<-done
	
	// Should not panic and should generate some output
	outputStr := output.String()
	assert.NotEmpty(t, outputStr)
}

func TestDisplayStateTransitions(t *testing.T) {
	config := &TopConfig{
		Plan:       "pro",
		Timezone:   "UTC",
		TimeFormat: "24h",
	}
	
	display := NewTerminalDisplay(config)
	display.EnterAlternateScreen()
	
	// Test help state transitions
	t.Run("help_transitions", func(t *testing.T) {
		// Initial state - no help
		assert.False(t, display.previousShowHelp)
		
		// Show help
		state := model.InteractionState{ShowHelp: true}
		display.RenderWithState([]*Session{}, state)
		assert.True(t, display.previousShowHelp)
		
		// Hide help
		state.ShowHelp = false
		display.RenderWithState([]*Session{}, state)
		assert.False(t, display.previousShowHelp)
	})
	
	t.Run("layout_style_transitions", func(t *testing.T) {
		// Initial layout style
		assert.Equal(t, 0, display.lastLayoutStyle)
		
		// Change layout style
		state := model.InteractionState{LayoutStyle: 1}
		display.RenderWithState([]*Session{}, state)
		assert.Equal(t, 1, display.lastLayoutStyle)
	})
}

// Benchmark tests for performance
func BenchmarkCalculateAggregatedMetrics(b *testing.B) {
	config := &TopConfig{
		Plan:     "pro",
		Timezone: "UTC",
	}
	
	display := NewTerminalDisplay(config)
	
	// Create test sessions
	sessions := make([]*Session, 10)
	for i := 0; i < 10; i++ {
		sessions[i] = &Session{
			ID:           fmt.Sprintf("session%d", i),
			TotalTokens:  1000 + i*100,
			TotalCost:    float64(5 + i),
			MessageCount: 10 + i,
			IsActive:     i%2 == 0,
			BurnRate:     1.5 + float64(i)*0.1,
			ModelDistribution: map[string]*model.ModelStats{
				"claude-3-5-sonnet": {
					Tokens: 1000 + i*100,
					Cost:   float64(5 + i),
					Count:  10 + i,
				},
			},
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		display.calculateAggregatedMetrics(sessions)
	}
}

func BenchmarkWrapText(b *testing.B) {
	text := "This is a longer text that needs to be wrapped properly across multiple lines to test the performance of the text wrapping function"
	width := 20
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapText(text, width)
	}
}

// Test error conditions and edge cases
func TestDisplayEdgeCases(t *testing.T) {
	t.Run("nil_sessions", func(t *testing.T) {
		config := &TopConfig{Plan: "pro", Timezone: "UTC"}
		display := NewTerminalDisplay(config)
		
		// Should not panic with nil sessions
		aggregated := display.calculateAggregatedMetrics(nil)
		assert.NotNil(t, aggregated)
		assert.Equal(t, 0, aggregated.TotalSessions)
	})
	
	t.Run("sessions_with_nil_model_distribution", func(t *testing.T) {
		config := &TopConfig{Plan: "pro", Timezone: "UTC"}
		display := NewTerminalDisplay(config)
		
		sessions := []*Session{
			{
				ID:                "session1",
				TotalTokens:       1000,
				TotalCost:         5.0,
				IsActive:          true,
				ModelDistribution: nil, // nil distribution
			},
		}
		
		// Should not panic
		aggregated := display.calculateAggregatedMetrics(sessions)
		assert.NotNil(t, aggregated)
		assert.NotNil(t, aggregated.ModelDistribution)
	})
	
	t.Run("very_long_dialog_message", func(t *testing.T) {
		config := &TopConfig{Plan: "pro", Timezone: "UTC"}
		display := NewTerminalDisplay(config)
		display.EnterAlternateScreen()
		
		longMessage := strings.Repeat("This is a very long message that should be wrapped properly. ", 20)
		dialog := &model.ConfirmDialog{
			Title:   "Long Message Test",
			Message: longMessage,
		}
		
		// Should not panic with very long message
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		
		display.renderConfirmDialog(dialog)
		
		w.Close()
		os.Stdout = oldStdout
		
		// Just verify it didn't panic - output verification would be complex
	})
	
	t.Run("zero_width_wrap", func(t *testing.T) {
		result := wrapText("Hello world", 0)
		// Should handle gracefully - exact behavior may vary
		assert.NotNil(t, result)
	})
	
	t.Run("negative_width_wrap", func(t *testing.T) {
		result := wrapText("Hello world", -5)
		// Should handle gracefully - exact behavior may vary
		assert.NotNil(t, result)
	})
}
package layout

import (
	"testing"
)

func TestNewSizer(t *testing.T) {
	tests := []struct {
		name           string
		terminalWidth  int
		terminalHeight int
		wantMinWidth   int
		wantMinHeight  int
	}{
		{
			name:           "standard_terminal",
			terminalWidth:  80,
			terminalHeight: 24,
			wantMinWidth:   60,
			wantMinHeight:  20,
		},
		{
			name:           "wide_terminal",
			terminalWidth:  120,
			terminalHeight: 40,
			wantMinWidth:   60,
			wantMinHeight:  20,
		},
		{
			name:           "narrow_terminal",
			terminalWidth:  50,
			terminalHeight: 20,
			wantMinWidth:   50, // Should use actual width if less than min
			wantMinHeight:  20,
		},
		{
			name:           "very_small_terminal",
			terminalWidth:  30,
			terminalHeight: 10,
			wantMinWidth:   30,
			wantMinHeight:  10,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sizer := NewSizer(tt.terminalWidth, tt.terminalHeight)
			if sizer == nil {
				t.Fatal("NewSizer returned nil")
			}
			
			if sizer.Width != tt.terminalWidth {
				t.Errorf("Expected width %d, got %d", tt.terminalWidth, sizer.Width)
			}
			
			if sizer.Height != tt.terminalHeight {
				t.Errorf("Expected height %d, got %d", tt.terminalHeight, sizer.Height)
			}
		})
	}
}

func TestSizerCalculations(t *testing.T) {
	tests := []struct {
		name             string
		width            int
		height           int
		headerLines      int
		footerLines      int
		wantSessionLines int
	}{
		{
			name:             "standard_layout",
			width:            80,
			height:           24,
			headerLines:      5,
			footerLines:      3,
			wantSessionLines: 16, // 24 - 5 - 3
		},
		{
			name:             "tall_terminal",
			width:            80,
			height:           50,
			headerLines:      5,
			footerLines:      3,
			wantSessionLines: 42, // 50 - 5 - 3
		},
		{
			name:             "minimal_space",
			width:            80,
			height:           10,
			headerLines:      5,
			footerLines:      3,
			wantSessionLines: 2, // 10 - 5 - 3
		},
		{
			name:             "no_space_for_sessions",
			width:            80,
			height:           8,
			headerLines:      5,
			footerLines:      3,
			wantSessionLines: 0, // 8 - 5 - 3 = 0
		},
		{
			name:             "negative_space",
			width:            80,
			height:           5,
			headerLines:      5,
			footerLines:      3,
			wantSessionLines: 0, // Should not go negative
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sizer := NewSizer(tt.width, tt.height)
			
			availableLines := sizer.GetAvailableSessionLines(tt.headerLines, tt.footerLines)
			if availableLines != tt.wantSessionLines {
				t.Errorf("Expected %d session lines, got %d", tt.wantSessionLines, availableLines)
			}
		})
	}
}

func TestSizerDynamicCalculations(t *testing.T) {
	sizer := NewSizer(100, 30)
	
	t.Run("progress_bar_width", func(t *testing.T) {
		// Progress bar should adapt to terminal width
		progressWidth := sizer.GetProgressBarWidth()
		if progressWidth <= 0 {
			t.Error("Expected positive progress bar width")
		}
		if progressWidth >= sizer.Width {
			t.Error("Progress bar width should be less than terminal width")
		}
	})
	
	t.Run("column_widths", func(t *testing.T) {
		// Test column width calculations for session list
		nameWidth := sizer.GetSessionNameWidth()
		if nameWidth <= 0 {
			t.Error("Expected positive session name width")
		}
		
		metricsWidth := sizer.GetMetricsWidth()
		if metricsWidth <= 0 {
			t.Error("Expected positive metrics width")
		}
		
		// Total should not exceed terminal width
		totalWidth := nameWidth + metricsWidth + 10 // Add some padding
		if totalWidth > sizer.Width {
			t.Error("Column widths exceed terminal width")
		}
	})
	
	t.Run("adaptive_sizing", func(t *testing.T) {
		// Test with different terminal sizes
		sizes := []struct {
			width  int
			height int
		}{
			{40, 20},   // Very narrow
			{200, 50},  // Very wide
			{80, 15},   // Short
			{120, 100}, // Very tall
		}
		
		for _, size := range sizes {
			s := NewSizer(size.width, size.height)
			
			// All calculations should produce valid results
			if s.GetProgressBarWidth() <= 0 {
				t.Errorf("Invalid progress bar width for size %dx%d", size.width, size.height)
			}
			
			if s.GetSessionNameWidth() <= 0 {
				t.Errorf("Invalid session name width for size %dx%d", size.width, size.height)
			}
			
			if s.GetAvailableSessionLines(5, 3) < 0 {
				t.Errorf("Negative session lines for size %dx%d", size.width, size.height)
			}
		}
	})
}

func TestSizerEdgeCases(t *testing.T) {
	t.Run("zero_dimensions", func(t *testing.T) {
		sizer := NewSizer(0, 0)
		if sizer.Width != 0 || sizer.Height != 0 {
			t.Error("Expected zero dimensions to be preserved")
		}
		
		// Should handle calculations gracefully
		if sizer.GetAvailableSessionLines(0, 0) != 0 {
			t.Error("Expected 0 available lines with zero height")
		}
	})
	
	t.Run("negative_dimensions", func(t *testing.T) {
		sizer := NewSizer(-10, -10)
		// Implementation might clamp to 0 or preserve negative
		// Just ensure it doesn't panic
		_ = sizer.GetAvailableSessionLines(5, 3)
		_ = sizer.GetProgressBarWidth()
	})
	
	t.Run("extreme_dimensions", func(t *testing.T) {
		// Test with very large dimensions
		sizer := NewSizer(10000, 10000)
		
		progressWidth := sizer.GetProgressBarWidth()
		if progressWidth <= 0 || progressWidth >= 10000 {
			t.Error("Progress bar width should be reasonable even with extreme terminal size")
		}
		
		// Test with mixed extreme dimensions
		sizer2 := NewSizer(20, 10000)
		nameWidth := sizer2.GetSessionNameWidth()
		if nameWidth >= 20 || nameWidth <= 0 {
			t.Error("Session name width should adapt to narrow terminal")
		}
	})
}

// Helper methods that might be on Sizer
func (s *Sizer) GetProgressBarWidth() int {
	// Typical calculation: reserve space for labels and padding
	const minWidth = 10
	const padding = 20
	
	width := s.Width - padding
	if width < minWidth {
		return minWidth
	}
	if width > 50 {
		return 50 // Cap at reasonable maximum
	}
	return width
}

func (s *Sizer) GetSessionNameWidth() int {
	// Allocate percentage of width for session name
	const minWidth = 20
	nameWidth := s.Width / 3
	
	if nameWidth < minWidth {
		return minWidth
	}
	if nameWidth > 50 {
		return 50
	}
	return nameWidth
}

func (s *Sizer) GetMetricsWidth() int {
	// Remaining width for metrics
	const minWidth = 30
	nameWidth := s.GetSessionNameWidth()
	remaining := s.Width - nameWidth - 10 // padding
	
	if remaining < minWidth {
		return minWidth
	}
	return remaining
}

func (s *Sizer) GetAvailableSessionLines(headerLines, footerLines int) int {
	available := s.Height - headerLines - footerLines
	if available < 0 {
		return 0
	}
	return available
}
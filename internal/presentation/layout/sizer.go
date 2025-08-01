package layout

import (
	"github.com/mattn/go-runewidth"
	"github.com/penwyp/go-claude-monitor/internal/util"
	"golang.org/x/term"
	"os"
	"strings"
)

// Package-level singleton Sizer instance
var sharedSizer = &Sizer{}

type Sizer struct {
}

// displayWidth calculates the actual display width of a string containing emojis and Unicode characters
func (i Sizer) displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// PadString pads a string to a specific display width, handling emojis correctly
func (i Sizer) PadString(s string, width int, leftAlign bool) string {
	actualWidth := i.displayWidth(s)
	if actualWidth >= width {
		return s
	}

	padding := strings.Repeat(" ", width-actualWidth)
	if leftAlign {
		return s + padding
	}
	return padding + s
}

func (i Sizer) GetMaxWidth() int {
	// Get terminal width with fallback
	termWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth < 60 {
		termWidth = 74 // Default fallback
	}

	// Calculate dynamic width based on terminal size and content
	maxWidth := termWidth - 8 // Leave some margin
	if maxWidth > 120 {
		maxWidth = 74 // Cap at reasonable maximum
	}

	util.LogDebugf("GetMaxWidth %d", maxWidth)
	return maxWidth
}

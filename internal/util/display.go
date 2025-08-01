package util

import (
	"fmt"
	"github.com/mattn/go-runewidth"
	"strings"
)

// Terminal control sequences
const (
	ColorReset   = "\033[0m"
	ColorBlue    = "\033[34m"
	ColorCyan    = "\033[36m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorRed     = "\033[31m"
	ColorMagenta = "\033[35m"
	ColorBold    = "\033[1m"

	// Terminal control sequences
	ClearScreen         = "\033[2J"     // Clear entire screen
	ClearLine           = "\033[2K"     // Clear entire line
	ClearLineFromCursor = "\033[0K"     // Clear from cursor to end of line
	ClearScrollback     = "\033[3J"     // Clear scrollback buffer
	ResetScrollRegion   = "\033[r"      // Reset scroll region
	DisableScrollback   = "\033[?1007h" // Disable scrollback
	EnableScrollback    = "\033[?1007l" // Enable scrollback
	MoveCursorHome      = "\033[H"      // Move cursor to home position
	SaveCursor          = "\033[s"      // Save cursor position
	RestoreCursor       = "\033[u"      // Restore cursor position
	HideCursor          = "\033[?25l"   // Hide cursor
	ShowCursor          = "\033[?25h"   // Show cursor
)

// GetDisplayWidth calculates the actual display width of a string, accounting for emojis
func GetDisplayWidth(text string) int {
	return runewidth.StringWidth(text)
}

// CreateProgressBar creates a progress bar with the given percentage and width
func CreateProgressBar(percentage float64, width int) string {
	if width < 10 {
		width = 12
	}
	barWidth := width - 12
	if barWidth < 0 {
		barWidth = 0
	}
	filled := int((percentage / 100) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled) + "]"
	return bar
}

// GetPercentageEmoji returns an emoji based on the percentage value
func GetPercentageEmoji(percentage float64) string {
	if percentage >= 90 {
		return "🔴"
	}
	if percentage >= 60 {
		return "🟡"
	}
	return "🟢"
}

// FormatHeaderTitle formats main header titles (Magenta + Bold)
func FormatHeaderTitle(title string) string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorMagenta, title, ColorReset)
}

// FormatDiagnosticTitle formats diagnostic/analysis titles (Yellow + Bold)
func FormatDiagnosticTitle(title string) string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorYellow, title, ColorReset)
}

// FormatOverviewTitle formats overview/summary titles (Cyan + Bold)
func FormatOverviewTitle(title string) string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorCyan, title, ColorReset)
}

// FormatDataTitle formats data section titles (Green + Bold)
func FormatDataTitle(title string) string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorGreen, title, ColorReset)
}

// FormatSectionSeparator creates a visual separator line
func FormatSectionSeparator() string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorCyan, "────────────────────────────────────────────────────────────────────────────────", ColorReset)
}

// MoveCursor returns ANSI sequence to move cursor to specific position
func MoveCursor(row, col int) string {
	return fmt.Sprintf("\033[%d;%dH", row, col)
}

// CenterText centers text within the given width
func CenterText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	padding := (width - len(text)) / 2
	return fmt.Sprintf("%s%s%s", strings.Repeat(" ", padding), text, strings.Repeat(" ", width-padding-len(text)))
}

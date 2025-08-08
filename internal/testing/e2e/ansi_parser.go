package e2e

import (
	"regexp"
	"strings"
)

// ANSI escape code patterns
var (
	ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	ansiCursor = regexp.MustCompile(`\x1b\[([0-9]+);([0-9]+)[Hf]`)
	ansiClear  = regexp.MustCompile(`\x1b\[([0-9]*)[JK]`)
)

// TerminalScreen represents a virtual terminal screen
type TerminalScreen struct {
	rows   int
	cols   int
	buffer [][]rune
	cursorX int
	cursorY int
}

// NewTerminalScreen creates a new virtual terminal screen
func NewTerminalScreen(rows, cols int) *TerminalScreen {
	buffer := make([][]rune, rows)
	for i := range buffer {
		buffer[i] = make([]rune, cols)
		for j := range buffer[i] {
			buffer[i][j] = ' '
		}
	}
	return &TerminalScreen{
		rows:   rows,
		cols:   cols,
		buffer: buffer,
	}
}

// StripANSI removes all ANSI escape codes from a string
func StripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// ParseTerminalOutput parses terminal output with ANSI codes into a screen
func ParseTerminalOutput(output string) *TerminalScreen {
	screen := NewTerminalScreen(24, 80) // Default size
	
	// Process the output character by character
	i := 0
	runes := []rune(output)
	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Handle ANSI escape sequence
			i = screen.handleANSISequence(runes, i)
		} else if runes[i] == '\r' {
			// Carriage return
			screen.cursorX = 0
			i++
		} else if runes[i] == '\n' {
			// Line feed
			screen.cursorY++
			screen.cursorX = 0
			if screen.cursorY >= screen.rows {
				screen.scrollUp()
			}
			i++
		} else if runes[i] == '\b' {
			// Backspace
			if screen.cursorX > 0 {
				screen.cursorX--
			}
			i++
		} else {
			// Regular character
			screen.putChar(runes[i])
			i++
		}
	}
	
	return screen
}

// handleANSISequence processes an ANSI escape sequence
func (s *TerminalScreen) handleANSISequence(runes []rune, start int) int {
	i := start + 2 // Skip \x1b[
	
	// Find the end of the sequence
	params := []int{}
	current := 0
	for i < len(runes) {
		if runes[i] >= '0' && runes[i] <= '9' {
			current = current*10 + int(runes[i]-'0')
		} else if runes[i] == ';' {
			params = append(params, current)
			current = 0
		} else {
			params = append(params, current)
			// Handle the command
			s.handleANSICommand(runes[i], params)
			return i + 1
		}
		i++
	}
	return i
}

// handleANSICommand processes a specific ANSI command
func (s *TerminalScreen) handleANSICommand(cmd rune, params []int) {
	switch cmd {
	case 'H', 'f': // Cursor position
		row, col := 1, 1
		if len(params) > 0 && params[0] > 0 {
			row = params[0]
		}
		if len(params) > 1 && params[1] > 0 {
			col = params[1]
		}
		s.cursorY = row - 1
		s.cursorX = col - 1
		
	case 'J': // Clear screen
		mode := 0
		if len(params) > 0 {
			mode = params[0]
		}
		switch mode {
		case 0: // Clear from cursor to end
			s.clearFromCursor()
		case 1: // Clear from start to cursor
			s.clearToCursor()
		case 2: // Clear entire screen
			s.clear()
		}
		
	case 'K': // Clear line
		mode := 0
		if len(params) > 0 {
			mode = params[0]
		}
		switch mode {
		case 0: // Clear from cursor to end of line
			s.clearLineFromCursor()
		case 1: // Clear from start to cursor
			s.clearLineToCursor()
		case 2: // Clear entire line
			s.clearLine()
		}
		
	case 'A': // Cursor up
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		s.cursorY = max(0, s.cursorY-n)
		
	case 'B': // Cursor down
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		s.cursorY = min(s.rows-1, s.cursorY+n)
		
	case 'C': // Cursor forward
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		s.cursorX = min(s.cols-1, s.cursorX+n)
		
	case 'D': // Cursor backward
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		s.cursorX = max(0, s.cursorX-n)
	}
}

// putChar places a character at the current cursor position
func (s *TerminalScreen) putChar(ch rune) {
	if s.cursorY >= 0 && s.cursorY < s.rows && s.cursorX >= 0 && s.cursorX < s.cols {
		s.buffer[s.cursorY][s.cursorX] = ch
		s.cursorX++
		if s.cursorX >= s.cols {
			s.cursorX = 0
			s.cursorY++
			if s.cursorY >= s.rows {
				s.scrollUp()
			}
		}
	}
}

// clear clears the entire screen
func (s *TerminalScreen) clear() {
	for i := range s.buffer {
		for j := range s.buffer[i] {
			s.buffer[i][j] = ' '
		}
	}
}

// clearFromCursor clears from cursor to end of screen
func (s *TerminalScreen) clearFromCursor() {
	// Clear rest of current line
	for j := s.cursorX; j < s.cols; j++ {
		s.buffer[s.cursorY][j] = ' '
	}
	// Clear all lines below
	for i := s.cursorY + 1; i < s.rows; i++ {
		for j := 0; j < s.cols; j++ {
			s.buffer[i][j] = ' '
		}
	}
}

// clearToCursor clears from start of screen to cursor
func (s *TerminalScreen) clearToCursor() {
	// Clear all lines above
	for i := 0; i < s.cursorY; i++ {
		for j := 0; j < s.cols; j++ {
			s.buffer[i][j] = ' '
		}
	}
	// Clear start of current line to cursor
	for j := 0; j <= s.cursorX; j++ {
		s.buffer[s.cursorY][j] = ' '
	}
}

// clearLine clears the current line
func (s *TerminalScreen) clearLine() {
	for j := 0; j < s.cols; j++ {
		s.buffer[s.cursorY][j] = ' '
	}
}

// clearLineFromCursor clears from cursor to end of line
func (s *TerminalScreen) clearLineFromCursor() {
	for j := s.cursorX; j < s.cols; j++ {
		s.buffer[s.cursorY][j] = ' '
	}
}

// clearLineToCursor clears from start of line to cursor
func (s *TerminalScreen) clearLineToCursor() {
	for j := 0; j <= s.cursorX; j++ {
		s.buffer[s.cursorY][j] = ' '
	}
}

// scrollUp scrolls the screen up by one line
func (s *TerminalScreen) scrollUp() {
	// Move all lines up by one
	for i := 0; i < s.rows-1; i++ {
		s.buffer[i] = s.buffer[i+1]
	}
	// Clear the last line
	s.buffer[s.rows-1] = make([]rune, s.cols)
	for j := range s.buffer[s.rows-1] {
		s.buffer[s.rows-1][j] = ' '
	}
	s.cursorY = s.rows - 1
}

// Render returns the screen content as a string
func (s *TerminalScreen) Render() string {
	var result strings.Builder
	for i, row := range s.buffer {
		result.WriteString(strings.TrimRight(string(row), " "))
		if i < len(s.buffer)-1 {
			result.WriteRune('\n')
		}
	}
	return result.String()
}

// GetLine returns a specific line from the screen
func (s *TerminalScreen) GetLine(line int) string {
	if line >= 0 && line < s.rows {
		return strings.TrimRight(string(s.buffer[line]), " ")
	}
	return ""
}

// ContainsText checks if the screen contains specific text
func (s *TerminalScreen) ContainsText(text string) bool {
	screenText := s.Render()
	return strings.Contains(screenText, text)
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
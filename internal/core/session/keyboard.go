package session

import (
	"golang.org/x/sys/unix"
	"os"
)

// KeyboardReader handles keyboard input in raw mode
type KeyboardReader struct {
	oldState *unix.Termios
	input    chan KeyEvent
	stop     chan struct{}
}

// KeyEvent represents a keyboard event
type KeyEvent struct {
	Key  rune
	Type KeyType
}

// KeyType represents the type of key pressed
type KeyType int

const (
	KeyChar KeyType = iota
	KeyEscape
)

// NewKeyboardReader creates a new keyboard reader
func NewKeyboardReader() (*KeyboardReader, error) {
	kr := &KeyboardReader{
		input: make(chan KeyEvent, 10),
		stop:  make(chan struct{}),
	}

	// Set terminal to raw mode
	if err := kr.enableRawMode(); err != nil {
		return nil, err
	}

	// Start reading keyboard input
	go kr.readInput()

	return kr, nil
}

// enableRawMode sets the terminal to raw mode
func (kr *KeyboardReader) enableRawMode() error {
	fd := int(os.Stdin.Fd())

	// Get current terminal state
	oldState, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	kr.oldState = oldState

	// Create new state for raw mode
	newState := *oldState
	newState.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN
	// Keep ISIG enabled to allow Ctrl+C handling
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	newState.Cflag |= unix.CS8
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	// Apply new state
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &newState); err != nil {
		return err
	}

	return nil
}

// disableRawMode restores the terminal to normal mode
func (kr *KeyboardReader) disableRawMode() error {
	if kr.oldState == nil {
		return nil
	}

	fd := int(os.Stdin.Fd())
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, kr.oldState)
}

// readInput reads keyboard input in a goroutine
func (kr *KeyboardReader) readInput() {
	buf := make([]byte, 3)

	for {
		select {
		case <-kr.stop:
			return
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil {
				continue
			}

			if n == 0 {
				continue
			}

			// Parse the input
			event := kr.parseInput(buf[:n])
			if event != nil {
				select {
				case kr.input <- *event:
				case <-kr.stop:
					return
				}
			}
		}
	}
}

// parseInput parses raw keyboard input
func (kr *KeyboardReader) parseInput(buf []byte) *KeyEvent {
	if len(buf) == 0 {
		return nil
	}

	// Handle Ctrl+C
	if buf[0] == 3 { // Ctrl+C
		return &KeyEvent{Key: 3, Type: KeyChar}
	}

	// Handle escape sequences
	if buf[0] == 27 { // ESC
		if len(buf) == 1 {
			return &KeyEvent{Key: 27, Type: KeyEscape}
		}
		if len(buf) >= 3 && buf[1] == '[' {
		}
		return nil
	}

	// Handle regular characters
	return &KeyEvent{Key: rune(buf[0]), Type: KeyChar}
}

// Events returns the keyboard event channel
func (kr *KeyboardReader) Events() <-chan KeyEvent {
	return kr.input
}

// Close stops the keyboard reader and restores terminal
func (kr *KeyboardReader) Close() error {
	close(kr.stop)
	return kr.disableRawMode()
}

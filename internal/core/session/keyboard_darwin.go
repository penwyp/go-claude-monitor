//go:build darwin

package session

import (
	"golang.org/x/sys/unix"
	"os"
)

// enableRawMode sets the terminal to raw mode on Darwin/macOS
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

// disableRawMode restores the terminal to normal mode on Darwin/macOS
func (kr *KeyboardReader) disableRawMode() error {
	if kr.oldState == nil {
		return nil
	}

	fd := int(os.Stdin.Fd())
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, kr.oldState)
}
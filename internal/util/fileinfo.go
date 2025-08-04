package util

import (
	"fmt"
	"os"
	"syscall"
)

// FileInfo contains extended file information, including modification time, size, and inode number.
type FileInfo struct {
	ModTime int64  // Last modification time of the file
	Size    int64  // File size in bytes
	Inode   uint64 // Inode number (unique file identifier on Unix-like systems)
}

// GetFileInfo retrieves detailed file information, including inode number.
// Supported on Linux and macOS.
func GetFileInfo(filepath string) (*FileInfo, error) {
	stat, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}

	// Retrieve system-specific stat information (for inode, etc.)
	sysStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get file system information: %s", filepath)
	}

	return &FileInfo{
		ModTime: stat.ModTime().Unix(),
		Size:    stat.Size(),
		Inode:   sysStat.Ino,
	}, nil
}

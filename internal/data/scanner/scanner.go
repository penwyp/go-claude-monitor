package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/util"
)

// FileScanner scans files in the specified directory
type FileScanner struct {
	baseDir    string
	pattern    string
	concurrent int
}

// ScanResult represents the result of a scan
type ScanResult struct {
	Files []string
	Error error
}

// NewFileScanner creates a new FileScanner instance
func NewFileScanner(baseDir string) *FileScanner {
	return &FileScanner{
		baseDir:    baseDir,
		pattern:    "*.jsonl",
		concurrent: 10,
	}
}

// Scan scans all files in the directory and returns all .jsonl file paths
func (s *FileScanner) Scan() ([]string, error) {
	start := time.Now()
	var files []string
	dirCount := 0
	totalCount := 0

	// Log: Start scanning directory
	util.LogDebug(fmt.Sprintf("Start scanning directory: %s", s.baseDir))

	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log: Skip file due to error
			util.LogDebug(fmt.Sprintf("Skip file (error): %s - %v", path, err))
			return nil
		}

		if info.IsDir() {
			dirCount++
			return nil
		}

		totalCount++
		if strings.HasSuffix(strings.ToLower(path), ".jsonl") {
			files = append(files, path)
		}

		return nil
	})

	duration := time.Since(start)
	// Log: File scan completed
	util.LogDebug(fmt.Sprintf("File scan completed: duration %v, scanned %d directories, %d files, found %d JSONL files",
		duration, dirCount, totalCount, len(files)))

	return files, err
}

package util

import (
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"os"
	"strings"
	"sync"
)

// ConsoleOutput writes logs to console
type ConsoleOutput struct {
	writer io.Writer
	format LogFormat
	mu     sync.Mutex
}

// NewConsoleOutput creates a new console output
func NewConsoleOutput(writer io.Writer, format LogFormat) Output {
	return &ConsoleOutput{
		writer: writer,
		format: format,
	}
}

// Write writes a log entry to console
func (c *ConsoleOutput) Write(entry LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var output string
	if c.format == FormatJSON {
		data, err := sonic.Marshal(entry)
		if err != nil {
			return err
		}
		output = string(data)
	} else {
		// Text format
		timestamp := entry.Timestamp.Format("2006/01/02 15:04:05")
		output = fmt.Sprintf("%s [%s] %s", timestamp, entry.Level, entry.Message)

		// Add fields if any
		if len(entry.Fields) > 0 {
			fieldStrs := make([]string, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
			}
			output += " " + strings.Join(fieldStrs, " ")
		}
	}

	_, err := fmt.Fprintln(c.writer, output)
	return err
}

// Close closes the console output
func (c *ConsoleOutput) Close() error {
	return nil
}

// FileOutput writes logs to a file
type FileOutput struct {
	file   *os.File
	format LogFormat
	mu     sync.Mutex
}

// NewFileOutput creates a new file output
func NewFileOutput(path string, format LogFormat) (Output, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileOutput{
		file:   file,
		format: format,
	}, nil
}

// Write writes a log entry to file
func (f *FileOutput) Write(entry LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var output string
	if f.format == FormatJSON {
		data, err := sonic.Marshal(entry)
		if err != nil {
			return err
		}
		output = string(data)
	} else {
		// Text format with caller info
		timestamp := entry.Timestamp.Format("2006/01/02 15:04:05")
		output = fmt.Sprintf("%s [%s] %s", timestamp, entry.Level, entry.Message)

		// Add fields if any
		if len(entry.Fields) > 0 {
			fieldStrs := make([]string, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
			}
			output += " " + strings.Join(fieldStrs, " ")
		}
	}

	_, err := fmt.Fprintln(f.file, output)
	return err
}

// Close closes the file
func (f *FileOutput) Close() error {
	return f.file.Close()
}

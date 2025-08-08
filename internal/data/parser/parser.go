package parser

import (
	"bufio"
	"fmt"
	"github.com/bytedance/sonic"
	"os"
	"sync"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

// Parser is a struct for parsing conversation log files.
type Parser struct {
	concurrency int
	mu          sync.Mutex
	cache       map[string][]model.ConversationLog
}

// ParseResult represents the result of parsing a single file.
type ParseResult struct {
	File  string
	Logs  []model.ConversationLog
	Error error
}

// NewParser creates a new Parser instance.
func NewParser(concurrency int) *Parser {
	return &Parser{
		concurrency: concurrency,
		cache:       make(map[string][]model.ConversationLog),
	}
}

// ParseFile parses the log file at the specified path and returns a slice of ConversationLog and an error if any.
func (p *Parser) ParseFile(filepath string) ([]model.ConversationLog, error) {
	p.mu.Lock()
	if cached, ok := p.cache[filepath]; ok {
		p.mu.Unlock()
		return cached, nil
	}
	p.mu.Unlock()

	util.LogDebug(fmt.Sprintf("Start parsing file: %s", filepath))

	file, err := os.Open(filepath)
	if err != nil {
		util.LogDebug(fmt.Sprintf("Failed to open file: %s - %v", filepath, err))
		return nil, err
	}
	defer file.Close()

	var logs []model.ConversationLog
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lineCount := 0
	validLogs := 0
	for scanner.Scan() {
		lineCount++
		var log model.ConversationLog
		if err := sonic.Unmarshal(scanner.Bytes(), &log); err != nil {
			util.LogDebug(fmt.Sprintf("Skip invalid JSON line %s:%d - %v", filepath, lineCount, err))
			continue
		}
		logs = append(logs, log)
		validLogs++
	}

	if err := scanner.Err(); err != nil {
		util.LogDebug(fmt.Sprintf("Error scanning file: %s - %v", filepath, err))
		return nil, err
	}

	p.mu.Lock()
	p.cache[filepath] = logs
	p.mu.Unlock()

	return logs, nil
}

// ParseFiles parses multiple files concurrently and returns a channel of ParseResult.
func (p *Parser) ParseFiles(files []string) <-chan ParseResult {
	start := time.Now()
	results := make(chan ParseResult, len(files))
	var wg sync.WaitGroup

	util.LogDebug(fmt.Sprintf("Start concurrent parsing of %d files, concurrency: %d", len(files), p.concurrency))

	semaphore := make(chan struct{}, p.concurrency)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fileStart := time.Now()
			logs, err := p.ParseFile(f)
			fileDuration := time.Since(fileStart)

			if err != nil {
				util.LogDebug(fmt.Sprintf("File parsing failed: %s, duration %v - %v", f, fileDuration, err))
			}

			results <- ParseResult{
				File:  f,
				Logs:  logs,
				Error: err,
			}
		}(file)
	}

	go func() {
		wg.Wait()
		close(results)

		totalDuration := time.Since(start)
		util.LogDebug(fmt.Sprintf("Concurrent parsing finished, total duration: %v", totalDuration))
	}()

	return results
}

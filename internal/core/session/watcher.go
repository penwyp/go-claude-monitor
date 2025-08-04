package session

import (
	"github.com/penwyp/go-claude-monitor/internal/core/model"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

type FileWatcher struct {
	watcher *fsnotify.Watcher
	paths   []string
	events  chan model.FileEvent
}

func NewFileWatcher(paths []string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher: watcher,
		paths:   paths,
		events:  make(chan model.FileEvent, 100),
	}

	// Add monitoring paths
	for _, path := range paths {
		if err := fw.addPath(path); err != nil {
			return nil, err
		}
	}

	// Start event processing
	go fw.processEvents()

	return fw, nil
}

func (fw *FileWatcher) addPath(path string) error {
	// Recursively add directories
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return fw.watcher.Add(p)
		}

		return nil
	})
}

func (fw *FileWatcher) processEvents() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Only process JSONL files
			if filepath.Ext(event.Name) == ".jsonl" {
				fw.events <- model.FileEvent{
					Path:      event.Name,
					Operation: event.Op.String(),
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue running
			util.LogError("File monitoring error: " + err.Error())
		}
	}
}

func (fw *FileWatcher) Events() <-chan model.FileEvent {
	return fw.events
}

func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}

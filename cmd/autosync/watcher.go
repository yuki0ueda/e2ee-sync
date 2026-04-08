package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors sync_dir for file changes and triggers sync after debounce.
type Watcher struct {
	fsw        *fsnotify.Watcher
	syncDir    string
	debounce   time.Duration
	TriggerCh  chan struct{}
	stopCh     chan struct{}
}

func NewWatcher(syncDir string, debounceSec int) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsw:       fsw,
		syncDir:   syncDir,
		debounce:  time.Duration(debounceSec) * time.Second,
		TriggerCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	// Recursively add all directories
	if err := w.addRecursive(syncDir); err != nil {
		fsw.Close()
		return nil, err
	}

	go w.loop()
	return w, nil
}

func (w *Watcher) Close() {
	close(w.stopCh)
	w.fsw.Close()
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if info.IsDir() {
			// Skip hidden directories
			if len(info.Name()) > 1 && info.Name()[0] == '.' {
				return filepath.SkipDir
			}
			if err := w.fsw.Add(path); err != nil {
				log.Printf("Watch add failed for %s: %v", path, err)
			}
		}
		return nil
	})
}

func (w *Watcher) loop() {
	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			// Skip rclone's own temp files
			if isIgnored(event.Name) {
				continue
			}

			// If a new directory was created, watch it
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.fsw.Add(event.Name)
				}
			}

			// Reset debounce timer
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.debounce)
			timerCh = timer.C

		case <-timerCh:
			// Debounce expired — trigger sync
			select {
			case w.TriggerCh <- struct{}{}:
			default:
			}
			timerCh = nil

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		case <-w.stopCh:
			return
		}
	}
}

func isIgnored(name string) bool {
	base := filepath.Base(name)
	// rclone temp files and OS junk
	switch base {
	case ".DS_Store", "Thumbs.db", "desktop.ini":
		return true
	}
	if len(base) > 0 && base[0] == '~' {
		return true
	}
	ext := filepath.Ext(base)
	switch ext {
	case ".tmp", ".swp", ".swo", ".partial":
		return true
	}
	return false
}

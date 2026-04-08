//go:build darwin

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// startApp runs in daemon mode on macOS (systray requires CGO which is
// unavailable in cross-compiled builds).
func startApp(cfg *Config) {
	syncer := NewSyncer(cfg)

	quitCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received %s, shutting down...", sig)
		close(quitCh)
	}()

	// Drain status channel (no tray to update)
	go func() {
		for range syncer.StatusCh {
		}
	}()

	// Initial sync
	log.Println("Running initial sync...")
	syncer.RunBisync()

	// Start file watcher
	watcher, err := NewWatcher(cfg.SyncDir, cfg.DebounceSec)
	if err != nil {
		log.Printf("Warning: file watcher failed to start: %v", err)
	} else {
		defer watcher.Close()
	}

	pollTicker := time.NewTicker(time.Duration(cfg.PollIntervalSec) * time.Second)
	defer pollTicker.Stop()

	var watchCh <-chan struct{}
	if watcher != nil {
		watchCh = watcher.TriggerCh
	}

	for {
		select {
		case <-watchCh:
			log.Println("File change detected, syncing...")
			syncer.RunBisync()
		case <-pollTicker.C:
			log.Println("Poll interval reached, syncing...")
			syncer.RunBisync()
		case <-quitCh:
			log.Println("Shutting down...")
			return
		}
	}
}

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/yuki0ueda/e2ee-sync/internal/version"
)

// safeGo runs fn in a goroutine with panic recovery.
// On panic it logs the recovered value and a full stack trace; the
// process keeps running. Goroutines are not restarted — a dead loop
// is visible in the log for diagnosis rather than silently masked.
func safeGo(name string, fn func()) {
	go func() {
		log.Printf("goroutine %s: started", name)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("goroutine %s: PANIC: %v\n%s", name, r, debug.Stack())
				return
			}
			log.Printf("goroutine %s: exited", name)
		}()
		fn()
	}()
}

func runDaemon() {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	if *configPath == "" {
		log.Fatal("Usage: e2ee-sync daemon --config <path/to/config.json>")
	}

	// Detach from console on Windows (no window in daemon mode)
	detachConsole()

	// Set up file logging next to config.json
	logPath := filepath.Join(filepath.Dir(*configPath), "e2ee-sync.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("FATAL: daemon panic: %v\n%s", r, debug.Stack())
		}
	}()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg.LogPath = logPath

	log.Printf("e2ee-sync daemon %s starting", version.String())
	log.Printf("Log file: %s", logPath)
	log.Printf("Sync dir: %s", cfg.SyncDir)
	log.Printf("Primary remote: %s", cfg.PrimaryRemote)
	if cfg.FallbackRemote != "" {
		log.Printf("Fallback remote: %s", cfg.FallbackRemote)
	}
	log.Printf("Poll interval: %ds, Debounce: %ds", cfg.PollIntervalSec, cfg.DebounceSec)

	startApp(cfg)
}

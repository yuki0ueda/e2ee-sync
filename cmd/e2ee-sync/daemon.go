package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/yuki0ueda/e2ee-sync/internal/version"
)

func runDaemon() {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json")
	fs.Parse(os.Args[2:])

	if *configPath == "" {
		log.Fatal("Usage: e2ee-sync daemon --config <path/to/config.json>")
	}

	// Detach from console on Windows (no window in daemon mode)
	detachConsole()

	// Set up file logging next to config.json
	logPath := filepath.Join(filepath.Dir(*configPath), "autosync.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

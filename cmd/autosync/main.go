package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/yuki0ueda/e2ee-sync/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	configPath := flag.String("config", "", "Path to config.json")
	flag.Parse()

	if *showVersion || (len(os.Args) > 1 && os.Args[1] == "version") {
		fmt.Printf("autosync %s\n", version.String())
		return
	}

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: autosync --config <path/to/config.json>")
		os.Exit(1)
	}

	// Set up file logging next to config.json
	logPath := filepath.Join(filepath.Dir(*configPath), "autosync.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("autosync %s starting", version.String())
	log.Printf("Log file: %s", logPath)
	log.Printf("Sync dir: %s", cfg.SyncDir)
	log.Printf("Primary remote: %s", cfg.PrimaryRemote)
	if cfg.FallbackRemote != "" {
		log.Printf("Fallback remote: %s", cfg.FallbackRemote)
	}
	log.Printf("Poll interval: %ds, Debounce: %ds", cfg.PollIntervalSec, cfg.DebounceSec)

	startApp(cfg)
}

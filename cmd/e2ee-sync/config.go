package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SyncDir         string
	TrashDir        string // backup dir for deleted/overwritten files
	PrimaryRemote   string
	FallbackRemote  string // empty if no hub
	RclonePath      string
	FilterFile      string
	LogPath         string // set at runtime by daemon, not from config file
	DebounceSec     int
	PollIntervalSec int
	TrashRetainDays int // auto-cleanup after this many days (default 30)
	HubTimeoutSec      int    // timeout for hub reachability check (default 5)
	MaxTransferPerSync string // max bytes per sync, e.g. "1G" (default: unlimited)
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	kv := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		SyncDir:        kv["sync_dir"],
		TrashDir:       kv["trash_dir"],
		PrimaryRemote:  kv["primary_remote"],
		FallbackRemote: kv["fallback_remote"],
		RclonePath:     kv["rclone_path"],
		FilterFile:     kv["filter_file"],
	}

	if cfg.RclonePath == "" {
		cfg.RclonePath = "rclone"
	}

	if v, ok := kv["trash_retain_days"]; ok {
		cfg.TrashRetainDays, _ = strconv.Atoi(v)
	}
	if cfg.TrashRetainDays <= 0 {
		cfg.TrashRetainDays = 30
	}

	if v, ok := kv["hub_timeout_sec"]; ok {
		cfg.HubTimeoutSec, _ = strconv.Atoi(v)
	}
	if cfg.HubTimeoutSec <= 0 {
		cfg.HubTimeoutSec = 5
	}

	if v, ok := kv["debounce_sec"]; ok {
		cfg.DebounceSec, _ = strconv.Atoi(v)
	}
	if cfg.DebounceSec <= 0 {
		cfg.DebounceSec = 5
	}

	if v, ok := kv["poll_interval_sec"]; ok {
		cfg.PollIntervalSec, _ = strconv.Atoi(v)
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = 300
	}

	cfg.MaxTransferPerSync = kv["max_transfer_per_sync"] // e.g. "1G", "500M"

	if cfg.SyncDir == "" {
		return nil, fmt.Errorf("sync_dir is required")
	}
	if cfg.PrimaryRemote == "" {
		return nil, fmt.Errorf("primary_remote is required")
	}

	return cfg, nil
}

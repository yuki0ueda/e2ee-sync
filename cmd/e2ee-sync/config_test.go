package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`sync_dir: /home/user/sync
primary_remote: cloud-crypt:
fallback_remote: hub-crypt:
rclone_path: /usr/bin/rclone
filter_file: /home/user/.config/rclone/filter-rules.txt
debounce_sec: 3
poll_interval_sec: 120
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.SyncDir != "/home/user/sync" {
		t.Errorf("SyncDir = %q, want /home/user/sync", cfg.SyncDir)
	}
	if cfg.PrimaryRemote != "cloud-crypt:" {
		t.Errorf("PrimaryRemote = %q, want cloud-crypt:", cfg.PrimaryRemote)
	}
	if cfg.FallbackRemote != "hub-crypt:" {
		t.Errorf("FallbackRemote = %q, want hub-crypt:", cfg.FallbackRemote)
	}
	if cfg.RclonePath != "/usr/bin/rclone" {
		t.Errorf("RclonePath = %q, want /usr/bin/rclone", cfg.RclonePath)
	}
	if cfg.DebounceSec != 3 {
		t.Errorf("DebounceSec = %d, want 3", cfg.DebounceSec)
	}
	if cfg.PollIntervalSec != 120 {
		t.Errorf("PollIntervalSec = %d, want 120", cfg.PollIntervalSec)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`sync_dir: /tmp/sync
primary_remote: cloud-crypt:
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.RclonePath != "rclone" {
		t.Errorf("RclonePath default = %q, want rclone", cfg.RclonePath)
	}
	if cfg.DebounceSec != 5 {
		t.Errorf("DebounceSec default = %d, want 5", cfg.DebounceSec)
	}
	if cfg.PollIntervalSec != 300 {
		t.Errorf("PollIntervalSec default = %d, want 300", cfg.PollIntervalSec)
	}
	if cfg.FallbackRemote != "" {
		t.Errorf("FallbackRemote = %q, want empty", cfg.FallbackRemote)
	}
}

func TestLoadConfig_MissingSyncDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`primary_remote: cloud-crypt:
`), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("Expected error for missing sync_dir")
	}
}

func TestLoadConfig_MissingPrimaryRemote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`sync_dir: /tmp/sync
`), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("Expected error for missing primary_remote")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Fatal("Expected error for missing file")
	}
}

func TestLoadConfig_Comments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`# This is a comment
sync_dir: /tmp/sync
# Another comment
primary_remote: cloud-crypt:
`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.SyncDir != "/tmp/sync" {
		t.Errorf("SyncDir = %q, want /tmp/sync", cfg.SyncDir)
	}
}

package template

import (
	"strings"
	"testing"
)

func TestRenderDaemonConfig_WithHub(t *testing.T) {
	data := DaemonConfigData{
		UseHub:         true,
		SyncDir:        "/home/user/sync",
		TrashDir:       "/home/user/sync/.trash",
		FilterFilePath: "/home/user/.config/rclone/filter-rules.txt",
	}
	got, err := RenderDaemonConfig(data)
	if err != nil {
		t.Fatalf("RenderDaemonConfig failed: %v", err)
	}
	if !strings.Contains(got, "primary_remote: hub-crypt:") {
		t.Error("Expected primary_remote: hub-crypt:")
	}
	if !strings.Contains(got, "fallback_remote: cloud-crypt:") {
		t.Error("Expected fallback_remote: cloud-crypt:")
	}
	if !strings.Contains(got, "sync_dir: /home/user/sync") {
		t.Error("Expected sync_dir")
	}
	if !strings.Contains(got, "trash_dir: /home/user/sync/.trash") {
		t.Error("Expected trash_dir")
	}
}

func TestRenderDaemonConfig_WithoutHub(t *testing.T) {
	data := DaemonConfigData{
		UseHub:         false,
		SyncDir:        "/home/user/sync",
		FilterFilePath: "/home/user/.config/rclone/filter-rules.txt",
	}
	got, err := RenderDaemonConfig(data)
	if err != nil {
		t.Fatalf("RenderDaemonConfig failed: %v", err)
	}
	if !strings.Contains(got, "primary_remote: cloud-crypt:") {
		t.Error("Expected primary_remote: cloud-crypt:")
	}
	if strings.Contains(got, "fallback_remote") {
		t.Error("Unexpected fallback_remote in R2-only mode")
	}
}

func TestFilterRules(t *testing.T) {
	rules := FilterRules()
	expected := []string{".DS_Store", "Thumbs.db", "*.tmp", "*.swp", ".rclone-test", ".trash/**"}
	for _, s := range expected {
		if !strings.Contains(rules, s) {
			t.Errorf("FilterRules() missing %q", s)
		}
	}
}

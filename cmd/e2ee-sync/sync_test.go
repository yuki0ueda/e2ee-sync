package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResyncDetection(t *testing.T) {
	tests := []struct {
		errMsg    string
		isResync  bool
		isForce   bool
	}{
		{"exit status 7: Bisync critical error: filters file md5 hash not found (must run --resync)", true, false},
		{"exit status 7: Bisync aborted. Must run --resync to recover.", true, false},
		{"exit status 1: Safety abort: all files were changed on Path2", false, true},
		{"exit status 1: some other error", false, false},
		{"connection refused", false, false},
	}

	for _, tt := range tests {
		err := errors.New(tt.errMsg)
		if got := needsResync(err); got != tt.isResync {
			t.Errorf("needsResync(%q) = %v, want %v", tt.errMsg, got, tt.isResync)
		}
		if got := needsForce(err); got != tt.isForce {
			t.Errorf("needsForce(%q) = %v, want %v", tt.errMsg, got, tt.isForce)
		}
	}
}

func TestIsHubRemote(t *testing.T) {
	tests := []struct {
		remote string
		isHub  bool
	}{
		{"hub-crypt:", true},
		{"hub-webdav:", true},
		{"cloud-crypt:", false},
		{"cloud-direct:", false},
		{"r2-crypt:", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHubRemote(tt.remote); got != tt.isHub {
			t.Errorf("isHubRemote(%q) = %v, want %v", tt.remote, got, tt.isHub)
		}
	}
}

func TestCleanOldTrash(t *testing.T) {
	trashDir := t.TempDir()

	// Create date-named directories: old (40 days ago) and recent (5 days ago)
	oldDate := "2020-01-01"
	recentDate := "2099-12-31"
	os.Mkdir(filepath.Join(trashDir, oldDate), 0700)
	os.Mkdir(filepath.Join(trashDir, recentDate), 0700)
	// Create non-date directory (should be ignored)
	os.Mkdir(filepath.Join(trashDir, "not-a-date"), 0700)
	// Create a file (should be ignored)
	os.WriteFile(filepath.Join(trashDir, "stray-file.txt"), []byte("x"), 0644)

	syncer := &Syncer{
		cfg: &Config{
			TrashDir:        trashDir,
			TrashRetainDays: 30,
		},
	}
	syncer.cleanOldTrash()

	// Old directory should be removed
	if _, err := os.Stat(filepath.Join(trashDir, oldDate)); !os.IsNotExist(err) {
		t.Errorf("Old trash dir %s should have been removed", oldDate)
	}
	// Recent directory should remain
	if _, err := os.Stat(filepath.Join(trashDir, recentDate)); err != nil {
		t.Errorf("Recent trash dir %s should still exist", recentDate)
	}
	// Non-date directory should remain
	if _, err := os.Stat(filepath.Join(trashDir, "not-a-date")); err != nil {
		t.Error("Non-date directory should still exist")
	}
	// File should remain
	if _, err := os.Stat(filepath.Join(trashDir, "stray-file.txt")); err != nil {
		t.Error("Stray file should still exist")
	}
}

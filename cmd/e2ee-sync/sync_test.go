package main

import (
	"errors"
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

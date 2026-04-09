package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SyncState int

const (
	StateIdle SyncState = iota
	StateSyncing
	StateError
)

type Status struct {
	State    SyncState
	Message  string
	LastSync time.Time
}

type Syncer struct {
	cfg          *Config
	mu           sync.Mutex
	status       Status
	paused       bool
	firstSync    bool // true until the first successful sync

	// Channels for tray communication
	StatusCh chan Status
}

func NewSyncer(cfg *Config) *Syncer {
	return &Syncer{
		cfg:       cfg,
		status:    Status{State: StateIdle, Message: "Starting..."},
		firstSync: true,
		StatusCh:  make(chan Status, 10),
	}
}

func (s *Syncer) SetPaused(paused bool) {
	s.mu.Lock()
	s.paused = paused
	s.mu.Unlock()
	if paused {
		s.updateStatus(StateIdle, "Paused")
	} else {
		s.updateStatus(StateIdle, "Resumed")
	}
}

func (s *Syncer) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

func (s *Syncer) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// RunBisync executes rclone bisync with failover.
// Returns true if sync succeeded.
func (s *Syncer) RunBisync() bool {
	if s.IsPaused() {
		return true
	}

	// Prevent concurrent syncs — check and set state atomically
	s.mu.Lock()
	if s.status.State == StateSyncing {
		s.mu.Unlock()
		return true
	}
	s.status.State = StateSyncing
	s.status.Message = "Syncing..."
	s.mu.Unlock()
	s.sendStatus(Status{State: StateSyncing, Message: "Syncing..."})

	// Check sync directory exists
	if _, err := os.Stat(s.cfg.SyncDir); os.IsNotExist(err) {
		log.Printf("ERROR: sync directory does not exist: %s", s.cfg.SyncDir)
		s.updateStatus(StateError, "Sync dir missing")
		return false
	}

	// Quick reachability check for hub remotes before attempting bisync.
	// Avoids a 10-minute bisync timeout when hub is down.
	if s.cfg.FallbackRemote != "" && isHubRemote(s.cfg.PrimaryRemote) {
		if !s.checkRemoteReachable(s.cfg.PrimaryRemote) {
			log.Printf("Hub unreachable, skipping to fallback...")
			if err := s.bisync(s.cfg.FallbackRemote); err == nil {
				s.updateStatus(StateIdle, "Synced (fallback)")
				s.mu.Lock()
				s.status.LastSync = time.Now()
				s.mu.Unlock()
				st := s.GetStatus()
				s.sendStatus(st)
				return true
			} else {
				log.Printf("Fallback sync also failed: %v", err)
			}
			s.updateStatus(StateError, "Sync failed")
			return false
		}
	}

	// Try primary remote
	if err := s.bisync(s.cfg.PrimaryRemote); err == nil {
		s.updateStatus(StateIdle, "Synced")
		s.mu.Lock()
		s.status.LastSync = time.Now()
		s.mu.Unlock()
		st := s.GetStatus()
		s.sendStatus(st)
		s.cleanOldTrash()
		return true
	} else {
		log.Printf("Primary sync failed (%s): %v", s.cfg.PrimaryRemote, err)
	}

	// Try fallback if available
	if s.cfg.FallbackRemote != "" {
		log.Printf("Falling back to %s", s.cfg.FallbackRemote)
		if err := s.bisync(s.cfg.FallbackRemote); err == nil {
			s.updateStatus(StateIdle, "Synced (fallback)")
			s.mu.Lock()
			s.status.LastSync = time.Now()
			s.mu.Unlock()
			st := s.GetStatus()
			s.sendStatus(st)
			return true
		} else {
			log.Printf("Fallback sync failed (%s): %v", s.cfg.FallbackRemote, err)
		}
	}

	s.updateStatus(StateError, "Sync failed")
	return false
}

func needsResync(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "must run --resync")
}

func needsForce(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "all files were changed")
}

func (s *Syncer) bisync(remote string) error {
	err := s.runBisyncCmd(remote, false, false)
	if err == nil {
		s.mu.Lock()
		s.firstSync = false
		s.mu.Unlock()
		return nil
	}
	if needsResync(err) {
		s.mu.Lock()
		isFirst := s.firstSync
		s.mu.Unlock()
		if isFirst {
			// Auto-resync only on first sync after startup (safe: establishing baseline)
			log.Printf("Resync required (first sync), retrying with --resync...")
			err2 := s.runBisyncCmd(remote, true, false)
			if err2 == nil {
				s.mu.Lock()
				s.firstSync = false
				s.mu.Unlock()
			}
			return err2
		}
		log.Printf("WARNING: Resync required but auto-resync disabled after first sync. Run 'rclone bisync --resync' manually.")
		return err
	}
	if needsForce(err) {
		// Never auto-force. Log warning and let user decide.
		log.Printf("WARNING: Safety abort — all files changed on one side. Manual intervention required.")
		log.Printf("WARNING: Run 'rclone bisync %s %s --force' if you are sure this is correct.", s.cfg.SyncDir, remote)
		return err
	}
	return err
}

func (s *Syncer) runBisyncCmd(remote string, resync bool, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	args := []string{"bisync", s.cfg.SyncDir, remote, "--checksum", "--resilient", "--recover"}
	if s.cfg.FilterFile != "" {
		args = append(args, "--filters-file", s.cfg.FilterFile)
	}
	if s.cfg.TrashDir != "" {
		// Move deleted/overwritten files to a dated trash folder instead of permanent deletion
		today := time.Now().Format("2006-01-02")
		backupDir := filepath.Join(s.cfg.TrashDir, today)
		os.MkdirAll(backupDir, 0700)
		args = append(args, "--backup-dir1", backupDir)
	}
	if s.cfg.MaxTransferPerSync != "" {
		args = append(args, "--max-transfer", s.cfg.MaxTransferPerSync, "--cutoff-mode", "SOFT")
	}
	if resync {
		args = append(args, "--resync")
	}
	if force {
		args = append(args, "--force")
	}

	cmd := exec.CommandContext(ctx, s.cfg.RclonePath, args...)
	hideChildWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

// cleanOldTrash removes trash directories older than TrashRetainDays.
func (s *Syncer) cleanOldTrash() {
	if s.cfg.TrashDir == "" || s.cfg.TrashRetainDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -s.cfg.TrashRetainDays)
	entries, err := os.ReadDir(s.cfg.TrashDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := time.Parse("2006-01-02", e.Name())
		if err != nil {
			continue // skip non-date directories
		}
		if t.Before(cutoff) {
			path := filepath.Join(s.cfg.TrashDir, e.Name())
			if err := os.RemoveAll(path); err == nil {
				log.Printf("Cleaned old trash: %s", path)
			}
		}
	}
}

func isHubRemote(remote string) bool {
	return strings.HasPrefix(remote, "hub-")
}

// checkRemoteReachable does a quick lsd to check if a remote is reachable.
func (s *Syncer) checkRemoteReachable(remote string) bool {
	timeout := time.Duration(s.cfg.HubTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.RclonePath, "lsd", remote)
	hideChildWindow(cmd)
	return cmd.Run() == nil
}

func (s *Syncer) updateStatus(state SyncState, msg string) {
	s.mu.Lock()
	s.status.State = state
	s.status.Message = msg
	st := s.status
	s.mu.Unlock()
	s.sendStatus(st)
}

func (s *Syncer) sendStatus(st Status) {
	select {
	case s.StatusCh <- st:
	default:
		// Don't block if no one is listening
	}
}

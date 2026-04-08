package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
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
	cfg    *Config
	mu     sync.Mutex
	status Status
	paused bool

	// Channels for tray communication
	StatusCh chan Status
}

func NewSyncer(cfg *Config) *Syncer {
	return &Syncer{
		cfg:      cfg,
		status:   Status{State: StateIdle, Message: "Starting..."},
		StatusCh: make(chan Status, 10),
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

	// Prevent concurrent syncs
	if !s.mu.TryLock() {
		return true // already syncing
	}
	syncing := s.status.State == StateSyncing
	s.mu.Unlock()
	if syncing {
		return true
	}

	s.updateStatus(StateSyncing, "Syncing...")

	// Try primary remote
	if err := s.bisync(s.cfg.PrimaryRemote); err == nil {
		s.updateStatus(StateIdle, "Synced")
		s.mu.Lock()
		s.status.LastSync = time.Now()
		s.mu.Unlock()
		// Send updated status with LastSync
		st := s.GetStatus()
		s.sendStatus(st)
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

func (s *Syncer) bisync(remote string) error {
	err := s.runBisyncCmd(remote, false, false)
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "must run --resync") {
		log.Printf("Resync required, retrying with --resync...")
		return s.runBisyncCmd(remote, true, false)
	}
	if strings.Contains(errMsg, "all files were changed") {
		log.Printf("Safety abort detected, retrying with --force...")
		return s.runBisyncCmd(remote, false, true)
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

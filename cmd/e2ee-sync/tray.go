//go:build windows || linux

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fyne.io/systray"
)

var (
	iconIdle    []byte
	iconSyncing []byte
	iconError   []byte
)

func init() {
	iconIdle = generateIcon(color.RGBA{76, 175, 80, 255})    // green
	iconSyncing = generateIcon(color.RGBA{33, 150, 243, 255}) // blue
	iconError = generateIcon(color.RGBA{244, 67, 54, 255})    // red
}

func generateIcon(c color.RGBA) []byte {
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			dx := float64(x) - float64(size)/2 + 0.5
			dy := float64(y) - float64(size)/2 + 0.5
			if dx*dx+dy*dy <= float64(size*size)/4 {
				img.Set(x, y, c)
			}
		}
	}

	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)

	if runtime.GOOS == "windows" {
		return pngToICO(pngBuf.Bytes(), img)
	}
	return pngBuf.Bytes()
}

// pngToICO wraps a PNG image in an ICO container for Windows systray.
func pngToICO(pngData []byte, img *image.RGBA) []byte {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var buf bytes.Buffer
	// ICO header: reserved(2) + type(2) + count(2)
	binary.Write(&buf, binary.LittleEndian, uint16(0))    // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // ICO type
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // 1 image

	// ICO directory entry
	buf.WriteByte(byte(w))           // width (0 = 256)
	buf.WriteByte(byte(h))           // height
	buf.WriteByte(0)                 // color palette
	buf.WriteByte(0)                 // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // color planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))   // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // data size
	binary.Write(&buf, binary.LittleEndian, uint32(22))   // data offset (6 + 16)

	// PNG data
	buf.Write(pngData)
	return buf.Bytes()
}

func startApp(cfg *Config) {
	syncer := NewSyncer(cfg)
	systray.Run(func() { onReady(cfg, syncer) }, func() {})
}

func onReady(cfg *Config, syncer *Syncer) {
	systray.SetIcon(iconIdle)
	systray.SetTitle("e2ee-sync")
	systray.SetTooltip("e2ee-sync: Idle")

	mStatus := systray.AddMenuItem("Status: Starting...", "")
	mStatus.Disable()
	mLastSync := systray.AddMenuItem("Last sync: never", "")
	mLastSync.Disable()
	systray.AddSeparator()
	mSyncNow := systray.AddMenuItem("Sync Now", "Trigger immediate sync")
	mPause := systray.AddMenuItem("Pause", "Pause/resume sync")
	systray.AddSeparator()
	mOpenSync := systray.AddMenuItem("Open Sync Folder", cfg.SyncDir)
	mOpenConfig := systray.AddMenuItem("Open Config Folder", filepath.Dir(cfg.FilterFile))
	mOpenLog := systray.AddMenuItem("View Log", "Open daemon log file")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop e2ee-sync")

	quitCh := make(chan struct{})
	syncNowCh := make(chan struct{}, 1)

	// Handle OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(quitCh)
		systray.Quit()
	}()

	// Tray menu event loop
	go func() {
		paused := false
		for {
			select {
			case <-mSyncNow.ClickedCh:
				select {
				case syncNowCh <- struct{}{}:
				default:
				}
			case <-mOpenSync.ClickedCh:
				openPath(cfg.SyncDir)
			case <-mOpenConfig.ClickedCh:
				openPath(filepath.Dir(cfg.FilterFile))
			case <-mOpenLog.ClickedCh:
				openPath(cfg.LogPath)
			case <-mPause.ClickedCh:
				paused = !paused
				syncer.SetPaused(paused)
				if paused {
					mPause.SetTitle("Resume")
				} else {
					mPause.SetTitle("Pause")
				}
			case <-mQuit.ClickedCh:
				close(quitCh)
				systray.Quit()
				return
			}
		}
	}()

	// Status update loop
	go func() {
		for st := range syncer.StatusCh {
			switch st.State {
			case StateIdle:
				systray.SetIcon(iconIdle)
				tooltip := "e2ee-sync: " + st.Message
				if strings.Contains(st.Message, "fallback") {
					tooltip = "e2ee-sync: Hub unreachable, syncing via cloud"
				}
				systray.SetTooltip(tooltip)
				mStatus.SetTitle("Status: " + st.Message)
			case StateSyncing:
				systray.SetIcon(iconSyncing)
				systray.SetTooltip("e2ee-sync: Syncing...")
				mStatus.SetTitle("Status: Syncing...")
			case StateError:
				systray.SetIcon(iconError)
				systray.SetTooltip("e2ee-sync: Error")
				mStatus.SetTitle("Status: " + st.Message)
			}
			if !st.LastSync.IsZero() {
				mLastSync.SetTitle(fmt.Sprintf("Last sync: %s", st.LastSync.Format("15:04:05")))
			}
		}
	}()

	// Run sync loop in background
	go runSyncLoop(cfg, syncer, syncNowCh, quitCh)
}

func runSyncLoop(cfg *Config, syncer *Syncer, syncNowCh <-chan struct{}, quitCh <-chan struct{}) {
	// Initial sync
	log.Println("Running initial sync...")
	syncer.RunBisync()

	// Start file watcher
	watcher, err := NewWatcher(cfg.SyncDir, cfg.DebounceSec)
	if err != nil {
		log.Printf("Warning: file watcher failed to start: %v", err)
		log.Println("Falling back to poll-only mode")
	} else {
		defer watcher.Close()
	}

	pollTicker := time.NewTicker(time.Duration(cfg.PollIntervalSec) * time.Second)
	defer pollTicker.Stop()

	var watchCh <-chan struct{}
	if watcher != nil {
		watchCh = watcher.TriggerCh
	}

	for {
		select {
		case <-watchCh:
			log.Println("File change detected, syncing...")
			syncer.RunBisync()
		case <-pollTicker.C:
			log.Println("Poll interval reached, syncing...")
			syncer.RunBisync()
		case <-syncNowCh:
			log.Println("Manual sync requested...")
			syncer.RunBisync()
		case <-quitCh:
			log.Println("Shutting down...")
			return
		}
	}
}

// openPath opens a file or directory in the OS default application.
func openPath(path string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", path)
	} else {
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Start()
}

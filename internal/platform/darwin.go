//go:build darwin

package platform

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type darwinPlatform struct {
	home string
}

func detect() Platform {
	home, _ := os.UserHomeDir()
	return &darwinPlatform{home: home}
}

func (p *darwinPlatform) RcloneConfigDir() string {
	return filepath.Join(p.home, ".config", "rclone")
}

func (p *darwinPlatform) SyncDir() string {
	return filepath.Join(p.home, "sync")
}

func (p *darwinPlatform) BinDir() string {
	return "/usr/local/bin"
}

func (p *darwinPlatform) ConfigDir() string {
	return filepath.Join(p.home, ".config", "e2ee-sync")
}

func (p *darwinPlatform) CheckRclone() error {
	_, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone not found in PATH")
	}
	return exec.Command("rclone", "version").Run()
}

func (p *darwinPlatform) CheckTailscale() error {
	paths := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"tailscale",
	}
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			if exec.Command(path, "status", "--json").Run() == nil {
				return nil
			}
			return fmt.Errorf("tailscale is not connected")
		}
	}
	return fmt.Errorf("tailscale not found")
}

func (p *darwinPlatform) RcloneInstallHint() string {
	return "Install rclone:\n  brew install rclone\n  or: curl https://rclone.org/install.sh | sudo bash"
}

func (p *darwinPlatform) TailscaleInstallHint() string {
	return "Install Tailscale:\n  Download from https://tailscale.com/download/mac"
}

func (p *darwinPlatform) RegisterDaemon(binPath, configPath string) error {
	agentDir := filepath.Join(p.home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	logPath := filepath.Join(p.home, ".config", "e2ee-sync", "e2ee-sync.log")
	errPath := filepath.Join(p.home, ".config", "e2ee-sync", "e2ee-sync.err")

	// Escape paths for XML safety (handles &, <, >, etc.)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.e2ee-sync</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
        <string>--config</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, html.EscapeString(binPath), html.EscapeString(configPath),
		html.EscapeString(logPath), html.EscapeString(errPath))

	plistPath := filepath.Join(agentDir, "com.e2ee-sync.plist")
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w", err)
	}
	return nil
}

func (p *darwinPlatform) UnregisterDaemon() error {
	plistPath := filepath.Join(p.home, "Library", "LaunchAgents", "com.e2ee-sync.plist")
	// Best-effort unload — agent may not be loaded
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}
	return nil
}

func (p *darwinPlatform) RegisterDaemonHint(binPath, configPath string) string {
	plistPath := filepath.Join(p.home, "Library", "LaunchAgents", "com.e2ee-sync.plist")
	return fmt.Sprintf("Manual registration:\n"+
		"  1. Create %s with appropriate plist content\n"+
		"  2. launchctl load %s", plistPath, plistPath)
}

// DaemonStatus returns the current state of the e2ee-sync LaunchAgent.
// Command failure is not an error — it means the agent is not loaded.
func (p *darwinPlatform) DaemonStatus() (string, error) {
	cmd := exec.Command("launchctl", "list", "com.e2ee-sync")
	out, err := cmd.Output()
	if err != nil {
		return "not-installed", nil
	}
	if strings.Contains(string(out), "\"PID\"") {
		return "running", nil
	}
	return "stopped", nil
}

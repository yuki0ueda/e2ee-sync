//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type linuxPlatform struct {
	home string
}

func detect() Platform {
	home, _ := os.UserHomeDir()
	return &linuxPlatform{home: home}
}

func (p *linuxPlatform) RcloneConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rclone")
	}
	return filepath.Join(p.home, ".config", "rclone")
}

func (p *linuxPlatform) SyncDir() string {
	return filepath.Join(p.home, "sync")
}

func (p *linuxPlatform) AutosyncBinDir() string {
	return filepath.Join(p.home, ".local", "bin")
}

func (p *linuxPlatform) AutosyncConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "autosync")
	}
	return filepath.Join(p.home, ".config", "autosync")
}

func (p *linuxPlatform) CheckRclone() error {
	_, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone not found in PATH")
	}
	cmd := exec.Command("rclone", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone version check failed: %w", err)
	}
	return nil
}

func (p *linuxPlatform) CheckTailscale() error {
	_, err := exec.LookPath("tailscale")
	if err != nil {
		return fmt.Errorf("tailscale not found in PATH")
	}
	cmd := exec.Command("tailscale", "status", "--json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tailscale is not connected: %w", err)
	}
	return nil
}

func (p *linuxPlatform) RcloneInstallHint() string {
	return "Install rclone:\n  sudo apt install rclone\n  or: curl https://rclone.org/install.sh | sudo bash"
}

func (p *linuxPlatform) TailscaleInstallHint() string {
	return "Install Tailscale:\n  curl -fsSL https://tailscale.com/install.sh | sh\n  sudo tailscale up"
}

func (p *linuxPlatform) RegisterDaemon(binPath, configPath string) error {
	unitDir := filepath.Join(p.home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user dir: %w", err)
	}

	unit := fmt.Sprintf(`[Unit]
Description=E2EE File Sync Daemon
After=network-online.target

[Service]
Type=simple
ExecStart=%s --config %s
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, binPath, configPath)

	unitPath := filepath.Join(unitDir, "autosync.service")
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "autosync.service"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("failed to run %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func (p *linuxPlatform) UnregisterDaemon() error {
	// Best-effort disable — service may not be registered yet
	_ = exec.Command("systemctl", "--user", "disable", "--now", "autosync.service").Run()
	unitPath := filepath.Join(p.home, ".config", "systemd", "user", "autosync.service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}
	// Best-effort reload — not critical if it fails
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func (p *linuxPlatform) RegisterDaemonHint(binPath, configPath string) string {
	return fmt.Sprintf("Manual registration:\n"+
		"  1. Create %s with appropriate unit content\n"+
		"  2. systemctl --user daemon-reload\n"+
		"  3. systemctl --user enable --now autosync.service",
		filepath.Join(p.home, ".config", "systemd", "user", "autosync.service"))
}

// DaemonStatus returns the current state of the autosync service.
// Returns the status string and nil error — command failure is not an error
// condition here, it simply means the service is inactive or not installed.
func (p *linuxPlatform) DaemonStatus() (string, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", "autosync.service")
	out, _ := cmd.Output()
	status := strings.TrimSpace(string(out))
	if status == "" {
		return "not-installed", nil
	}
	return status, nil
}

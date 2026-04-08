//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsPlatform struct {
	home string
}

func detect() Platform {
	home, _ := os.UserHomeDir()
	return &windowsPlatform{home: home}
}

func (p *windowsPlatform) RcloneConfigDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "rclone")
	}
	return filepath.Join(p.home, "AppData", "Roaming", "rclone")
}

func (p *windowsPlatform) SyncDir() string {
	return filepath.Join(p.home, "sync")
}

func (p *windowsPlatform) BinDir() string {
	return filepath.Join(p.home, ".local", "bin")
}

func (p *windowsPlatform) ConfigDir() string {
	return filepath.Join(p.home, ".config", "e2ee-sync")
}

func (p *windowsPlatform) CheckRclone() error {
	_, err := exec.LookPath("rclone.exe")
	if err != nil {
		return fmt.Errorf("rclone not found in PATH")
	}
	return exec.Command("rclone", "version").Run()
}

func (p *windowsPlatform) CheckTailscale() error {
	paths := []string{
		"tailscale.exe",
		filepath.Join(os.Getenv("ProgramFiles"), "Tailscale", "tailscale.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Tailscale", "tailscale.exe"),
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

func (p *windowsPlatform) RcloneInstallHint() string {
	return "Install rclone:\n  winget install Rclone.Rclone\n  or download from https://rclone.org/downloads/"
}

func (p *windowsPlatform) TailscaleInstallHint() string {
	return "Install Tailscale:\n  winget install Tailscale.Tailscale\n  or download from https://tailscale.com/download/windows"
}

func (p *windowsPlatform) registerBatPath() string {
	return filepath.Join(p.BinDir(), "register-daemon.bat")
}

func (p *windowsPlatform) unregisterBatPath() string {
	return filepath.Join(p.BinDir(), "unregister-daemon.bat")
}

// RegisterDaemon generates a register-daemon.bat file instead of running schtasks directly.
// On Windows, schtasks /SC ONLOGON requires administrator privileges, but requesting
// UAC elevation via manifest breaks stdin. The .bat file can be right-click → Run as administrator.
func (p *windowsPlatform) RegisterDaemon(binPath, configPath string) error {
	batContent := fmt.Sprintf("@echo off\r\n"+
		"echo Registering E2EE-Sync daemon...\r\n"+
		"schtasks /Create /TN \"E2EE-Sync\" /TR \"\\\"%s\\\" daemon --config \\\"%s\\\"\" /SC ONLOGON /F\r\n"+
		"if %%errorlevel%% neq 0 (\r\n"+
		"    echo Failed. Please run this file as administrator.\r\n"+
		"    pause\r\n"+
		"    exit /b 1\r\n"+
		")\r\n"+
		"schtasks /Run /TN \"E2EE-Sync\"\r\n"+
		"echo Done. e2ee-sync daemon will start at every logon.\r\n"+
		"pause\r\n",
		binPath, configPath)

	batPath := p.registerBatPath()
	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		return fmt.Errorf("failed to write register-daemon.bat: %w", err)
	}
	return nil
}

// UnregisterDaemon generates unregister-daemon.bat that must be run as administrator.
// It handles both current (E2EE-Sync) and legacy (E2EE-Autosync) task names.
func (p *windowsPlatform) UnregisterDaemon() error {
	binPath := filepath.Join(p.BinDir(), "e2ee-sync.exe")
	batContent := fmt.Sprintf("@echo off\r\n"+
		"echo Stopping E2EE-Sync daemon...\r\n"+
		"taskkill /IM e2ee-sync.exe /F >nul 2>&1\r\n"+
		"schtasks /End /TN \"E2EE-Sync\" >nul 2>&1\r\n"+
		"schtasks /Delete /TN \"E2EE-Sync\" /F >nul 2>&1\r\n"+
		"schtasks /End /TN \"E2EE-Autosync\" >nul 2>&1\r\n"+
		"schtasks /Delete /TN \"E2EE-Autosync\" /F >nul 2>&1\r\n"+
		"echo Removing files...\r\n"+
		"del /f \"%s\" >nul 2>&1\r\n"+
		"del /f \"%s\" >nul 2>&1\r\n"+
		"echo Done. You can delete this file and the .local\\bin folder.\r\n"+
		"pause\r\n",
		binPath, p.registerBatPath())

	batPath := p.unregisterBatPath()
	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		return fmt.Errorf("failed to write unregister-daemon.bat: %w", err)
	}
	return nil
}

func (p *windowsPlatform) RegisterDaemonHint(binPath, configPath string) string {
	return fmt.Sprintf("Run as administrator:\n  %s", p.registerBatPath())
}

// DaemonStatus checks if the scheduled task is registered and running.
// Checks both current (E2EE-Sync) and legacy (E2EE-Autosync) task names.
func (p *windowsPlatform) DaemonStatus() (string, error) {
	for _, tn := range []string{"E2EE-Sync", "E2EE-Autosync"} {
		cmd := exec.Command("schtasks", "/Query", "/TN", tn, "/FO", "CSV", "/NH")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		output := string(out)
		if strings.Contains(output, "Running") {
			return "running", nil
		}
		return "stopped", nil
	}
	return "not-installed", nil
}

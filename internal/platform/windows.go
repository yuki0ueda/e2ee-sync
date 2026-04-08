//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
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

func (p *windowsPlatform) AutosyncBinDir() string {
	return filepath.Join(p.home, ".local", "bin")
}

func (p *windowsPlatform) AutosyncConfigDir() string {
	return filepath.Join(p.home, ".config", "autosync")
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
	return filepath.Join(p.AutosyncBinDir(), "register-daemon.bat")
}

// RegisterDaemon generates a register-daemon.bat file instead of running schtasks directly.
// On Windows, schtasks /SC ONLOGON requires administrator privileges, but requesting
// UAC elevation via manifest breaks stdin. The .bat file can be right-click → Run as administrator.
func (p *windowsPlatform) RegisterDaemon(binPath, configPath string) error {
	batContent := fmt.Sprintf("@echo off\r\n"+
		"echo Registering E2EE-Autosync daemon...\r\n"+
		"schtasks /Create /TN \"E2EE-Autosync\" /TR \"\\\"%s\\\" --config \\\"%s\\\"\" /SC ONLOGON /F\r\n"+
		"if %%errorlevel%% neq 0 (\r\n"+
		"    echo Failed. Please run this file as administrator.\r\n"+
		"    pause\r\n"+
		"    exit /b 1\r\n"+
		")\r\n"+
		"schtasks /Run /TN \"E2EE-Autosync\"\r\n"+
		"echo Done. Autosync will start at every logon.\r\n"+
		"pause\r\n",
		binPath, configPath)

	batPath := p.registerBatPath()
	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		return fmt.Errorf("failed to write register-daemon.bat: %w", err)
	}
	return nil
}

func (p *windowsPlatform) UnregisterDaemon() error {
	// Best-effort stop and delete scheduled task
	_ = exec.Command("schtasks", "/End", "/TN", "E2EE-Autosync").Run()
	_ = exec.Command("schtasks", "/Delete", "/TN", "E2EE-Autosync", "/F").Run()
	// Remove .bat file
	_ = os.Remove(p.registerBatPath())
	return nil
}

func (p *windowsPlatform) RegisterDaemonHint(binPath, configPath string) string {
	return fmt.Sprintf("Run as administrator:\n  %s", p.registerBatPath())
}

// DaemonStatus checks if the scheduled task is registered and running.
// Command failure is not an error — it means the task is not registered.
func (p *windowsPlatform) DaemonStatus() (string, error) {
	cmd := exec.Command("schtasks", "/Query", "/TN", "E2EE-Autosync", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return "not-installed", nil
	}
	output := string(out)
	if strings.Contains(output, "Running") {
		return "running", nil
	}
	return "stopped", nil
}

// IsLaunchedFromExplorer detects if the process was started by double-clicking
// in Windows Explorer (parent process is explorer.exe).
func IsLaunchedFromExplorer() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleProcessList := kernel32.NewProc("GetConsoleProcessList")
	pids := make([]uint32, 2)
	ret, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(len(pids)),
	)
	// If only 1 process attached to console, it was likely double-clicked
	return ret == 1
}

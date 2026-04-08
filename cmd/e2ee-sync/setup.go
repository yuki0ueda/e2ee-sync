package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/yuki0ueda/e2ee-sync/internal/credential"
	"github.com/yuki0ueda/e2ee-sync/internal/platform"
	"github.com/yuki0ueda/e2ee-sync/internal/rclone"
	tmpl "github.com/yuki0ueda/e2ee-sync/internal/template"
	"github.com/yuki0ueda/e2ee-sync/internal/version"
)

func runSetup() {
	plat := platform.Detect()
	rc := rclone.NewClient("")

	// Step 1: Prerequisites + hub mode selection
	step(1, 9, "Checking prerequisites")
	if err := plat.CheckRclone(); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, plat.RcloneInstallHint())
		fatalf("rclone not available: %v", err)
	}
	if err := plat.CheckTailscale(); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, plat.TailscaleInstallHint())
		fatalf("tailscale not available: %v", err)
	}

	fmt.Println("  e2ee-sync-hub is an optional relay server (Proxmox LXC).")
	fmt.Println("  It enables faster sync via Tailscale direct connection.")
	fmt.Println("  If you haven't set up a hub, select N.")
	useHub, _ := credential.Confirm("Do you have an e2ee-sync-hub?")
	if useHub {
		if err := checkHubReachability(); err != nil {
			fatalf("%v", err)
		}
		ok("Prerequisites OK (hub mode: sync via hub + cloud fallback)")
	} else {
		ok("Prerequisites OK (direct mode: sync via cloud storage)")
	}

	// Step 2: Credential input
	step(2, 9, "Collecting credentials")
	backend, err := credential.SelectBackend()
	if err != nil {
		fatalf("Backend selection failed: %v", err)
	}
	var creds *credential.Credentials
	creds, err = credential.Collect(useHub, backend)
	if err != nil {
		fatalf("Credential collection failed: %v", err)
	}
	ok("Credentials collected (%s)", backend.Name)

	// Step 3: Generate rclone.conf via rclone config create
	step(3, 9, "Generating rclone.conf")
	configDir := plat.RcloneConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fatalf("Failed to create config dir: %v", err)
	}
	confPath := filepath.Join(configDir, "rclone.conf")
	backupRcloneConf(confPath)
	if err := createRcloneRemotes(rc, creds, useHub); err != nil {
		fatalf("Failed to create rclone remotes: %v", err)
	}
	creds.WebDAVPassword = ""
	creds.EncryptionPassword = ""
	creds.EncryptionSalt = ""
	creds.S3SecretAccessKey = ""
	ok("rclone.conf written to %s", confPath)

	// Step 4: Filter rules
	step(4, 9, "Writing filter-rules.txt")
	filterPath := filepath.Join(configDir, "filter-rules.txt")
	if err := os.WriteFile(filterPath, []byte(tmpl.FilterRules()), 0644); err != nil {
		fatalf("Failed to write filter-rules.txt: %v", err)
	}
	ok("filter-rules.txt written")

	// Step 5: Sync directory
	step(5, 9, "Creating sync directory")
	syncDir := plat.SyncDir()
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		fatalf("Failed to create sync dir: %v", err)
	}
	testFile := filepath.Join(syncDir, "RCLONE_TEST")
	if err := os.WriteFile(testFile, []byte("rclone test file\n"), 0644); err != nil {
		fatalf("Failed to write RCLONE_TEST: %v", err)
	}
	ok("Sync directory: %s", syncDir)

	// Step 6: Connection tests (with 401 re-entry loop for hub mode)
	step(6, 9, "Testing connections")
	syncRemote := "cloud-crypt:"
	if useHub {
		syncRemote = "hub-crypt:"
	}
	const maxAuthRetries = 3
	for authAttempt := 0; ; authAttempt++ {
		var testRemotes []struct {
			name   string
			remote string
		}
		if useHub {
			testRemotes = []struct {
				name   string
				remote string
			}{
				{"hub-webdav", "hub-webdav:"},
				{"hub-crypt", "hub-crypt:"},
				{"cloud-crypt", "cloud-crypt:"},
			}
		} else {
			testRemotes = []struct {
				name   string
				remote string
			}{
				{"cloud-crypt", "cloud-crypt:"},
			}
		}
		authFailed := false
		for _, t := range testRemotes {
			if err := retryConnectionTest(rc, t.name, t.remote); err != nil {
				if authAttempt < maxAuthRetries-1 {
					warnf("Connection test failed for %s: %v", t.name, err)
					yes, _ := credential.Confirm("  Re-enter credentials?")
					if yes {
						authFailed = true
						break
					}
				}
				warnf("Connection test failed for %s: %v", t.name, err)
				warnf("You can re-run 'e2ee-sync verify' after fixing the issue.")
			} else {
				ok("  %s: OK", t.name)
			}
		}
		if !authFailed {
			break
		}
		fmt.Printf("\n  Re-enter credentials (attempt %d/%d):\n", authAttempt+2, maxAuthRetries)
		creds, err = credential.Collect(useHub, backend)
		if err != nil {
			fatalf("Credential collection failed: %v", err)
		}
		if err := createRcloneRemotes(rc, creds, useHub); err != nil {
			fatalf("Failed to update rclone remotes: %v", err)
		}
		creds.WebDAVPassword = ""
		creds.EncryptionPassword = ""
		creds.EncryptionSalt = ""
		creds.S3SecretAccessKey = ""
		ok("rclone.conf updated")
	}

	// Step 7: Initial resync
	step(7, 9, "Running initial bisync (resync)")
	if err := retryResync(rc, syncDir, syncRemote); err != nil {
		warnf("Initial resync failed: %v", err)
		warnf("You can run manually: rclone bisync %s %s --resync", syncDir, syncRemote)
	} else {
		ok("Initial resync completed")
	}

	// Step 8: Deploy binary + config
	step(8, 9, "Deploying e2ee-sync")
	binDst, err := deploySelf(plat)
	if err != nil {
		warnf("Failed to deploy: %v", err)
	} else {
		ok("e2ee-sync deployed to %s", binDst)
	}
	autosyncConfigDir := plat.AutosyncConfigDir()
	if err := os.MkdirAll(autosyncConfigDir, 0755); err != nil {
		fatalf("Failed to create config dir: %v", err)
	}
	configContent, err := tmpl.RenderAutosyncConfig(tmpl.AutosyncConfigData{
		UseHub:         useHub,
		SyncDir:        syncDir,
		FilterFilePath: filterPath,
	})
	if err != nil {
		fatalf("Failed to render config: %v", err)
	}
	autosyncConfigPath := filepath.Join(autosyncConfigDir, "config.json")
	if err := os.WriteFile(autosyncConfigPath, []byte(configContent), 0600); err != nil {
		fatalf("Failed to write config.json: %v", err)
	}
	ok("config.json written to %s", autosyncConfigPath)

	// Step 9: Register daemon
	step(9, 9, "Registering daemon")
	if binDst != "" {
		if err := plat.RegisterDaemon(binDst, autosyncConfigPath); err != nil {
			warnf("Daemon registration failed: %v", err)
			fmt.Fprintln(os.Stderr, plat.RegisterDaemonHint(binDst, autosyncConfigPath))
		} else {
			if runtime.GOOS == "windows" {
				ok("register-daemon.bat created")
				fmt.Println()
				fmt.Println("  *** ACTION REQUIRED ***")
				fmt.Println("  Sync will NOT start until you register the daemon:")
				fmt.Println("  1. Open this folder in Explorer:")
				fmt.Printf("     %s\n", plat.AutosyncBinDir())
				fmt.Println("  2. Right-click register-daemon.bat → Run as administrator")
			} else {
				ok("Daemon registered and started")
			}
		}
	} else {
		warnf("Skipping daemon registration (binary not deployed)")
	}

	// Summary
	fmt.Println()
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Printf("  Sync directory:  %s\n", syncDir)
	fmt.Printf("  rclone.conf:     %s\n", confPath)
	fmt.Printf("  Filter rules:    %s\n", filterPath)
	fmt.Printf("  Config:          %s\n", autosyncConfigPath)
	if binDst != "" {
		fmt.Printf("  Binary:          %s\n", binDst)
	}
	fmt.Println()
	fmt.Println("Your files in ~/sync will now be synchronized automatically.")
}

// --- Upgrade ---

func runUpgrade() {
	plat := platform.Detect()

	binPath := filepath.Join(plat.AutosyncBinDir(), binaryName())
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		fatalf("e2ee-sync not found at %s. Run 'e2ee-sync setup' first.", binPath)
	}

	// Check version
	cmd := exec.Command(binPath, "version")
	out, err := cmd.Output()
	if err != nil {
		fatalf("Failed to get version: %v", err)
	}
	currentVersion := strings.TrimSpace(string(out))
	fmt.Printf("Current: %s\n", currentVersion)
	fmt.Printf("New:     e2ee-sync %s\n", version.String())

	if strings.Contains(currentVersion, version.Version) {
		fmt.Println("Already up to date.")
		return
	}

	// Stop daemon
	fmt.Println("Stopping daemon...")
	_ = plat.UnregisterDaemon()

	// Replace binary
	fmt.Println("Replacing binary...")
	backupPath := binPath + ".bak"
	if err := os.Rename(binPath, backupPath); err != nil {
		fatalf("Failed to backup old binary: %v", err)
	}
	if _, err := deploySelf(plat); err != nil {
		_ = os.Rename(backupPath, binPath)
		fatalf("Failed to deploy new binary: %v", err)
	}

	// Restart daemon
	fmt.Println("Restarting daemon...")
	configPath := filepath.Join(plat.AutosyncConfigDir(), "config.json")
	if err := plat.RegisterDaemon(binPath, configPath); err != nil {
		warnf("Daemon restart failed: %v", err)
	}

	// Verify
	cmd = exec.Command(binPath, "version")
	out, err = cmd.Output()
	if err != nil {
		warnf("Version verification failed: %v", err)
	} else {
		fmt.Printf("Updated: %s\n", strings.TrimSpace(string(out)))
	}
	ok("Upgrade complete")
}

// --- Verify ---

func runVerify() {
	plat := platform.Detect()
	rc := rclone.NewClient("")
	allOk := true

	fmt.Print("\n=== Verifying E2EE File Sync ===\n\n")

	fmt.Println("[Prerequisites]")
	if err := plat.CheckRclone(); err != nil {
		warnf("  rclone: %v", err)
		allOk = false
	} else {
		ok("  rclone: OK")
	}
	if err := plat.CheckTailscale(); err != nil {
		warnf("  tailscale: %v", err)
		allOk = false
	} else {
		ok("  tailscale: OK")
	}

	fmt.Println("\n[Configuration Files]")
	configDir := plat.RcloneConfigDir()
	files := []string{
		filepath.Join(configDir, "rclone.conf"),
		filepath.Join(configDir, "filter-rules.txt"),
		filepath.Join(plat.AutosyncConfigDir(), "config.json"),
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			warnf("  %s: MISSING", f)
			allOk = false
		} else {
			ok("  %s: OK", f)
		}
	}

	fmt.Println("\n[Sync Directory]")
	syncDir := plat.SyncDir()
	if _, err := os.Stat(syncDir); err != nil {
		warnf("  %s: MISSING", syncDir)
		allOk = false
	} else {
		ok("  %s: OK", syncDir)
	}

	confPath := filepath.Join(configDir, "rclone.conf")
	confBytes, _ := os.ReadFile(confPath)
	hubConfigured := strings.Contains(string(confBytes), "[hub-webdav]")

	fmt.Println("\n[Connection Tests]")
	var remotes []string
	if hubConfigured {
		remotes = []string{"hub-webdav:", "hub-crypt:", "cloud-crypt:"}
	} else {
		remotes = []string{"cloud-crypt:"}
	}
	for _, remote := range remotes {
		if _, err := rc.ListDir(remote); err != nil {
			warnf("  %s FAILED: %v", remote, err)
			allOk = false
		} else {
			ok("  %s OK", remote)
		}
	}

	fmt.Println("\n[Daemon]")
	status, _ := plat.DaemonStatus()
	if status == "running" || status == "active" {
		ok("  daemon: %s", status)
	} else {
		warnf("  daemon: %s", status)
		if runtime.GOOS == "windows" {
			fmt.Fprintln(os.Stderr, "    Hint: Run register-daemon.bat as administrator")
			fmt.Fprintf(os.Stderr, "    Location: %s\n", filepath.Join(plat.AutosyncBinDir(), "register-daemon.bat"))
		} else {
			fmt.Fprintln(os.Stderr, "    Hint: Run 'e2ee-sync setup' to register the daemon")
		}
		allOk = false
	}

	fmt.Println()
	if allOk {
		ok("All checks passed")
	} else {
		warnf("Some checks failed — review the output above")
		os.Exit(1)
	}
}

// --- Uninstall ---

func runUninstall() {
	plat := platform.Detect()

	fmt.Print("\n=== Uninstall E2EE File Sync ===\n\n")
	fmt.Println("This will:")
	fmt.Println("  - Stop and unregister the daemon")
	fmt.Println("  - Remove e2ee-sync binary and configuration")
	fmt.Println("  - NOT remove rclone.conf (may contain other remotes)")
	fmt.Println("  - NOT remove ~/sync directory")
	fmt.Println()

	yes, err := credential.Confirm("Continue?")
	if err != nil {
		fatalf("Failed to read input: %v", err)
	}
	if !yes {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println("Stopping daemon...")
	if err := plat.UnregisterDaemon(); err != nil {
		warnf("Daemon removal: %v", err)
	} else {
		ok("Daemon removed")
	}

	binPath := filepath.Join(plat.AutosyncBinDir(), binaryName())
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		warnf("Remove binary: %v", err)
	} else {
		ok("Binary removed: %s", binPath)
	}

	configDir := plat.AutosyncConfigDir()
	if err := os.RemoveAll(configDir); err != nil {
		warnf("Remove config dir: %v", err)
	} else {
		ok("Config removed: %s", configDir)
	}

	fmt.Println("\nUninstall complete.")
	fmt.Printf("rclone.conf and %s were left intact.\n", plat.SyncDir())
}

// --- Helpers ---

func checkHubReachability() error {
	cmd := exec.Command("tailscale", "ping", "--c", "1", "--timeout", "5s", "e2ee-sync-hub")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr)
		warnf("Cannot reach e2ee-sync-hub via Tailscale")
		fmt.Fprintln(os.Stderr, "\n  Tailscale status:")
		statusCmd := exec.Command("tailscale", "status")
		statusCmd.Stdout = os.Stderr
		statusCmd.Stderr = os.Stderr
		_ = statusCmd.Run()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  Possible causes:")
		fmt.Fprintln(os.Stderr, "    - e2ee-sync-hub is offline or not connected to tailnet")
		fmt.Fprintln(os.Stderr, "    - The hostname 'e2ee-sync-hub' is not in your tailnet")
		fmt.Fprintln(os.Stderr, "    - Tailscale ACLs are blocking the connection")
		return fmt.Errorf("e2ee-sync-hub not reachable")
	}
	return nil
}

func createRcloneRemotes(rc *rclone.Client, creds *credential.Credentials, useHub bool) error {
	if useHub {
		if err := rc.ConfigCreate("hub-webdav", "webdav",
			"url", "http://e2ee-sync-hub:8080",
			"vendor", "other",
			"user", "rclone",
			"pass", creds.WebDAVPassword,
		); err != nil {
			return fmt.Errorf("create hub-webdav: %w", err)
		}
		if err := rc.ConfigCreate("hub-crypt", "crypt",
			"remote", "hub-webdav:",
			"password", creds.EncryptionPassword,
			"password2", creds.EncryptionSalt,
			"filename_encryption", "standard",
			"directory_name_encryption", "true",
		); err != nil {
			return fmt.Errorf("create hub-crypt: %w", err)
		}
	}

	// S3-compatible remote (works with R2, AWS S3, B2, etc.)
	s3Params := []string{
		"provider", creds.Backend.Provider,
		"access_key_id", creds.S3AccessKeyID,
		"secret_access_key", creds.S3SecretAccessKey,
		"acl", "private",
	}
	if creds.S3Endpoint != "" {
		s3Params = append(s3Params, "endpoint", creds.S3Endpoint)
	}
	if creds.S3Region != "" {
		s3Params = append(s3Params, "region", creds.S3Region)
	}
	if err := rc.ConfigCreate("cloud-direct", "s3", s3Params...); err != nil {
		return fmt.Errorf("create cloud-direct: %w", err)
	}

	if err := rc.ConfigCreate("cloud-crypt", "crypt",
		"remote", "cloud-direct:e2ee-sync",
		"password", creds.EncryptionPassword,
		"password2", creds.EncryptionSalt,
		"filename_encryption", "standard",
		"directory_name_encryption", "true",
	); err != nil {
		return fmt.Errorf("create cloud-crypt: %w", err)
	}

	return nil
}

func backupRcloneConf(confPath string) {
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return
	}
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s.bak", confPath, timestamp)
	data, err := os.ReadFile(confPath)
	if err != nil {
		warnf("Could not read existing rclone.conf for backup: %v", err)
		return
	}
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		warnf("Could not backup rclone.conf: %v", err)
		return
	}
	fmt.Printf("  Backed up to: %s\n", backupPath)
}

type connectionError struct {
	err    error
	stderr string
}

func (e *connectionError) Error() string { return e.err.Error() }
func (e *connectionError) Unwrap() error { return e.err }

func isUnauthorized(err error) bool {
	var ce *connectionError
	if errors.As(err, &ce) {
		return strings.Contains(ce.stderr, "401") || strings.Contains(strings.ToLower(ce.stderr), "unauthorized")
	}
	return false
}

func retryConnectionTest(rc *rclone.Client, name, remote string) error {
	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if stderr, err := rc.ListDir(remote); err != nil {
			lastErr = &connectionError{err: err, stderr: stderr}
			if i < maxRetries-1 {
				warnf("  %s: attempt %d/%d failed, retrying...", name, i+1, maxRetries)
				time.Sleep(3 * time.Second)
			}
			continue
		}
		return nil
	}
	return lastErr
}

func retryResync(rc *rclone.Client, syncDir, remote string) error {
	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := rc.Bisync(syncDir, remote, true); err != nil {
			lastErr = err
			if i < maxRetries-1 {
				yes, _ := credential.Confirm(fmt.Sprintf("  Resync attempt %d/%d failed. Retry?", i+1, maxRetries))
				if !yes {
					return lastErr
				}
			}
			continue
		}
		return nil
	}
	return lastErr
}

// deploySelf copies the current executable to the platform's bin directory.
func deploySelf(plat platform.Platform) (string, error) {
	binDir := plat.AutosyncBinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}
	dstPath := filepath.Join(binDir, binaryName())

	selfPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine own path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink: %w", err)
	}

	// Skip if already in the target location
	if filepath.Clean(selfPath) == filepath.Clean(dstPath) {
		return dstPath, nil
	}

	data, err := os.ReadFile(selfPath)
	if err != nil {
		return "", fmt.Errorf("read self: %w", err)
	}
	if err := os.WriteFile(dstPath, data, 0755); err != nil {
		return "", fmt.Errorf("write binary: %w", err)
	}
	return dstPath, nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "e2ee-sync.exe"
	}
	return "e2ee-sync"
}

func step(n, total int, msg string) {
	fmt.Printf("\n[%d/%d] %s...\n", n, total, msg)
}

func ok(format string, args ...any) {
	fmt.Printf("  ✓ "+format+"\n", args...)
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  ⚠ "+format+"\n", args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  ✗ "+format+"\n", args...)
	os.Exit(1)
}

func waitIfDoubleClicked() {
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Print("\nPress Enter to exit...")
	fmt.Scanln()
}

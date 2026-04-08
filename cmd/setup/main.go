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

func main() {
	if len(os.Args) < 2 {
		interactiveMenu()
		return
	}
	switch os.Args[1] {
	case "setup":
		runSetup()
	case "upgrade":
		runUpgrade()
	case "verify":
		runVerify()
	case "uninstall":
		runUninstall()
	case "version":
		fmt.Printf("e2ee-sync-setup %s\n", version.String())
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `e2ee-sync-setup %s

Usage:
  e2ee-sync-setup <command>

Commands:
  setup       Full device setup (interactive)
  upgrade     Update autosync binary
  verify      Verify existing configuration
  uninstall   Remove daemon and configuration
  version     Show version
`, version.String())
}

// --- Interactive Menu ---

func interactiveMenu() {
	fmt.Printf("\n=== E2EE File Sync Setup %s ===\n\n", version.String())
	fmt.Println("  1) Setup    — New device setup")
	fmt.Println("  2) Upgrade  — Update autosync to latest version")
	fmt.Println("  3) Verify   — Verify connection and configuration")
	fmt.Println("  4) Quit")
	fmt.Println()

	choice, err := credential.ReadLine("Select [1-4]: ")
	if err != nil {
		fatalf("Failed to read input: %v", err)
	}

	switch strings.TrimSpace(choice) {
	case "1":
		runSetup()
	case "2":
		runUpgrade()
	case "3":
		runVerify()
	case "4":
		return
	default:
		fmt.Fprintln(os.Stderr, "Invalid selection.")
		os.Exit(1)
	}

	waitIfDoubleClicked()
}

// --- Setup ---

func runSetup() {
	plat := platform.Detect()
	rc := rclone.NewClient("")

	// Step 1: Prerequisites + hub mode selection
	step(1, 10, "Checking prerequisites")
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

	useHub, _ := credential.Confirm("Do you have an e2ee-sync-hub?")
	if useHub {
		if err := checkHubReachability(); err != nil {
			fatalf("%v", err)
		}
		ok("Prerequisites OK (hub mode)")
	} else {
		ok("Prerequisites OK (R2-only mode)")
	}

	// Step 2: Credential input
	step(2, 10, "Collecting credentials")
	var creds *credential.Credentials
	creds, err := credential.Collect(useHub)
	if err != nil {
		fatalf("Credential collection failed: %v", err)
	}
	ok("Credentials collected")

	// Step 3: Generate rclone.conf via rclone config create
	step(3, 10, "Generating rclone.conf")
	configDir := plat.RcloneConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fatalf("Failed to create config dir: %v", err)
	}
	confPath := filepath.Join(configDir, "rclone.conf")
	backupRcloneConf(confPath)
	if err := createRcloneRemotes(rc, creds, useHub); err != nil {
		fatalf("Failed to create rclone remotes: %v", err)
	}
	// Clear plaintext from memory
	creds.WebDAVPassword = ""
	creds.EncryptionPassword = ""
	creds.EncryptionSalt = ""
	creds.R2SecretAccessKey = ""
	ok("rclone.conf written to %s", confPath)

	// Step 4: Filter rules
	step(4, 10, "Writing filter-rules.txt")
	filterPath := filepath.Join(configDir, "filter-rules.txt")
	if err := os.WriteFile(filterPath, []byte(tmpl.FilterRules()), 0644); err != nil {
		fatalf("Failed to write filter-rules.txt: %v", err)
	}
	ok("filter-rules.txt written")

	// Step 5: Sync directory
	step(5, 10, "Creating sync directory")
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
	step(6, 10, "Testing connections")
	syncRemote := "r2-crypt:"
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
				{"r2-crypt", "r2-crypt:"},
			}
		} else {
			testRemotes = []struct {
				name   string
				remote string
			}{
				{"r2-crypt", "r2-crypt:"},
			}
		}
		authFailed := false
		for _, t := range testRemotes {
			if err := retryConnectionTest(rc, t.name, t.remote); err != nil {
				if useHub && isUnauthorized(err) && t.name == "hub-webdav" && authAttempt < maxAuthRetries-1 {
					warnf("Authentication failed for %s (401 Unauthorized)", t.name)
					warnf("Please re-enter credentials.")
					authFailed = true
					break
				}
				warnf("Connection test failed for %s: %v", t.name, err)
				warnf("You can re-run 'e2ee-sync-setup verify' after fixing the issue.")
			} else {
				ok("  %s: OK", t.name)
			}
		}
		if !authFailed {
			break
		}
		// Re-collect credentials and regenerate rclone.conf
		fmt.Printf("\n  Re-enter credentials (attempt %d/%d):\n", authAttempt+2, maxAuthRetries)
		creds, err = credential.Collect(useHub)
		if err != nil {
			fatalf("Credential collection failed: %v", err)
		}
		if err := createRcloneRemotes(rc, creds, useHub); err != nil {
			fatalf("Failed to update rclone remotes: %v", err)
		}
		creds.WebDAVPassword = ""
		creds.EncryptionPassword = ""
		creds.EncryptionSalt = ""
		creds.R2SecretAccessKey = ""
		ok("rclone.conf updated")
	}

	// Step 7: Initial resync
	step(7, 10, "Running initial bisync (resync)")
	if err := retryResync(rc, syncDir, syncRemote); err != nil {
		warnf("Initial resync failed: %v", err)
		warnf("You can run manually: rclone bisync %s %s --resync", syncDir, syncRemote)
	} else {
		ok("Initial resync completed")
	}

	// Step 8: Deploy autosync binary + config
	step(8, 10, "Deploying autosync")
	autosyncDst, err := deployAutosync(plat)
	if err != nil {
		warnf("Failed to deploy autosync: %v", err)
		warnf("You can manually copy the autosync binary to %s", plat.AutosyncBinDir())
	} else {
		ok("autosync deployed to %s", autosyncDst)
	}
	autosyncConfigDir := plat.AutosyncConfigDir()
	if err := os.MkdirAll(autosyncConfigDir, 0755); err != nil {
		fatalf("Failed to create autosync config dir: %v", err)
	}
	configContent, err := tmpl.RenderAutosyncConfig(tmpl.AutosyncConfigData{
		UseHub:         useHub,
		SyncDir:        syncDir,
		FilterFilePath: filterPath,
	})
	if err != nil {
		fatalf("Failed to render autosync config: %v", err)
	}
	autosyncConfigPath := filepath.Join(autosyncConfigDir, "config.json")
	if err := os.WriteFile(autosyncConfigPath, []byte(configContent), 0644); err != nil {
		fatalf("Failed to write config.json: %v", err)
	}
	ok("config.json written to %s", autosyncConfigPath)

	// Step 9: Register daemon
	step(9, 10, "Registering daemon")
	if autosyncDst != "" {
		if err := plat.RegisterDaemon(autosyncDst, autosyncConfigPath); err != nil {
			warnf("Daemon registration failed: %v", err)
			fmt.Fprintln(os.Stderr, plat.RegisterDaemonHint(autosyncDst, autosyncConfigPath))
		} else {
			if runtime.GOOS == "windows" {
				// On Windows, RegisterDaemon generates a .bat file instead of registering directly
				ok("register-daemon.bat created")
				fmt.Println("  To complete daemon setup, right-click register-daemon.bat → Run as administrator")
				fmt.Printf("  Location: %s\n", filepath.Join(plat.AutosyncBinDir(), "register-daemon.bat"))
			} else {
				ok("Daemon registered and started")
			}
		}
	} else {
		warnf("Skipping daemon registration (autosync binary not deployed)")
	}

	// Step 10: Completion summary
	step(10, 10, "Complete")
	fmt.Println()
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Printf("  Sync directory:  %s\n", syncDir)
	fmt.Printf("  rclone.conf:     %s\n", confPath)
	fmt.Printf("  Filter rules:    %s\n", filterPath)
	fmt.Printf("  Autosync config: %s\n", autosyncConfigPath)
	if autosyncDst != "" {
		fmt.Printf("  Autosync binary: %s\n", autosyncDst)
	}
	fmt.Println()
	fmt.Println("Your files in ~/sync will now be synchronized automatically.")
}

// --- Upgrade ---

func runUpgrade() {
	plat := platform.Detect()

	// Find current autosync
	autosyncPath := filepath.Join(plat.AutosyncBinDir(), autosyncBinaryName())
	if _, err := os.Stat(autosyncPath); os.IsNotExist(err) {
		fatalf("autosync not found at %s. Run 'e2ee-sync-setup setup' first.", autosyncPath)
	}

	// Check version
	cmd := exec.Command(autosyncPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		fatalf("Failed to get autosync version: %v", err)
	}
	currentVersion := strings.TrimSpace(string(out))
	fmt.Printf("Current: %s\n", currentVersion)
	fmt.Printf("New:     autosync %s\n", version.String())

	if strings.Contains(currentVersion, version.Version) {
		fmt.Println("Already up to date.")
		return
	}

	// Stop daemon
	fmt.Println("Stopping daemon...")
	_ = plat.UnregisterDaemon()

	// Replace binary
	fmt.Println("Replacing binary...")
	backupPath := autosyncPath + ".bak"
	if err := os.Rename(autosyncPath, backupPath); err != nil {
		fatalf("Failed to backup old binary: %v", err)
	}
	if err := deployAutosyncTo(autosyncPath); err != nil {
		// Restore backup
		_ = os.Rename(backupPath, autosyncPath)
		fatalf("Failed to deploy new binary: %v", err)
	}

	// Restart daemon
	fmt.Println("Restarting daemon...")
	configPath := filepath.Join(plat.AutosyncConfigDir(), "config.json")
	if err := plat.RegisterDaemon(autosyncPath, configPath); err != nil {
		warnf("Daemon restart failed: %v", err)
	}

	// Verify
	cmd = exec.Command(autosyncPath, "--version")
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

	// Check prerequisites
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

	// Check files
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

	// Check sync dir
	fmt.Println("\n[Sync Directory]")
	syncDir := plat.SyncDir()
	if _, err := os.Stat(syncDir); err != nil {
		warnf("  %s: MISSING", syncDir)
		allOk = false
	} else {
		ok("  %s: OK", syncDir)
	}

	// Detect hub mode from rclone.conf
	confPath := filepath.Join(configDir, "rclone.conf")
	confBytes, _ := os.ReadFile(confPath)
	hubConfigured := strings.Contains(string(confBytes), "[hub-webdav]")

	// Connection tests
	fmt.Println("\n[Connection Tests]")
	var remotes []string
	if hubConfigured {
		remotes = []string{"hub-webdav:", "hub-crypt:", "r2-crypt:"}
	} else {
		remotes = []string{"r2-crypt:"}
	}
	for _, remote := range remotes {
		if _, err := rc.ListDir(remote); err != nil {
			warnf("  %s FAILED: %v", remote, err)
			allOk = false
		} else {
			ok("  %s OK", remote)
		}
	}

	// Daemon status
	fmt.Println("\n[Daemon]")
	status, _ := plat.DaemonStatus()
	if status == "running" || status == "active" {
		ok("  autosync: %s", status)
	} else {
		warnf("  autosync: %s", status)
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
	fmt.Println("  - Stop and unregister the autosync daemon")
	fmt.Println("  - Remove autosync binary and configuration")
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

	// Stop daemon
	fmt.Println("Stopping daemon...")
	if err := plat.UnregisterDaemon(); err != nil {
		warnf("Daemon removal: %v", err)
	} else {
		ok("Daemon removed")
	}

	// Remove autosync binary
	binPath := filepath.Join(plat.AutosyncBinDir(), autosyncBinaryName())
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		warnf("Remove binary: %v", err)
	} else {
		ok("Binary removed: %s", binPath)
	}

	// Remove autosync config
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
	// Try to reach e2ee-sync-hub via tailscale ping (faster than rclone lsd, works without rclone.conf)
	cmd := exec.Command("tailscale", "ping", "--c", "1", "--timeout", "5s", "e2ee-sync-hub")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr)
		warnf("Cannot reach e2ee-sync-hub via Tailscale")
		// Show tailscale status for diagnostics
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

// createRcloneRemotes uses "rclone config create" to set up all remotes.
// This lets rclone handle password obscuring internally, avoiding
// command-line argument mangling and base64 auto-detection issues.
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

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", creds.R2AccountID)
	if err := rc.ConfigCreate("r2-direct", "s3",
		"provider", "Cloudflare",
		"access_key_id", creds.R2AccessKeyID,
		"secret_access_key", creds.R2SecretAccessKey,
		"endpoint", endpoint,
		"region", "auto",
		"acl", "private",
	); err != nil {
		return fmt.Errorf("create r2-direct: %w", err)
	}

	if err := rc.ConfigCreate("r2-crypt", "crypt",
		"remote", "r2-direct:e2ee-sync",
		"password", creds.EncryptionPassword,
		"password2", creds.EncryptionSalt,
		"filename_encryption", "standard",
		"directory_name_encryption", "true",
	); err != nil {
		return fmt.Errorf("create r2-crypt: %w", err)
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

// connectionError wraps a connection test failure with the stderr output
// so callers can inspect the failure reason (e.g. 401 Unauthorized).
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

func deployAutosync(plat platform.Platform) (string, error) {
	binDir := plat.AutosyncBinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}
	dstPath := filepath.Join(binDir, autosyncBinaryName())
	if err := deployAutosyncTo(dstPath); err != nil {
		return "", err
	}
	return dstPath, nil
}

func deployAutosyncTo(dstPath string) error {
	setupBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine setup binary path: %w", err)
	}
	setupDir := filepath.Dir(setupBin)

	srcName := fmt.Sprintf("autosync-%s-%s", goosToLabel(runtime.GOOS), goarchToLabel(runtime.GOARCH))
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join(setupDir, srcName)

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		// Also try the simple name
		simpleName := "autosync"
		if runtime.GOOS == "windows" {
			simpleName += ".exe"
		}
		srcPath = filepath.Join(setupDir, simpleName)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			return fmt.Errorf("autosync binary not found in %s (tried %s and %s)", setupDir, srcName, simpleName)
		}
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read autosync binary: %w", err)
	}
	if err := os.WriteFile(dstPath, data, 0755); err != nil {
		return fmt.Errorf("write autosync binary: %w", err)
	}
	return nil
}

func autosyncBinaryName() string {
	if runtime.GOOS == "windows" {
		return "autosync.exe"
	}
	return "autosync"
}

func goosToLabel(goos string) string {
	switch goos {
	case "windows":
		return "win"
	case "darwin":
		return "mac"
	default:
		return goos
	}
}

func goarchToLabel(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
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

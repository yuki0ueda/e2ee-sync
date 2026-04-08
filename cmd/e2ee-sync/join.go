package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"path/filepath"
	"runtime"
	"time"

	"github.com/yuki0ueda/e2ee-sync/internal/credential"
	"github.com/yuki0ueda/e2ee-sync/internal/platform"
	"github.com/yuki0ueda/e2ee-sync/internal/rclone"
	tmpl "github.com/yuki0ueda/e2ee-sync/internal/template"
)

func runJoin() {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	code := fs.String("code", "", "One-time code (required if share used --code)")
	fs.Parse(os.Args[2:])

	addr := fs.Arg(0) // positional: ip:port
	if addr == "" {
		fmt.Fprintln(os.Stderr, "Usage: e2ee-sync join <ip:port> [--code <code>]")
		os.Exit(1)
	}

	fmt.Print("\n=== Join E2EE File Sync ===\n\n")

	// Fetch config from sharing device
	fmt.Println("Connecting...")
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s/config", addr)
	if *code != "" {
		url += "?code=" + *code
	}
	resp, err := client.Get(url)
	if err != nil {
		fatalf("Cannot connect to sharing device: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		fatalf("Invalid code. Check the code and try again.")
	}
	if resp.StatusCode != http.StatusOK {
		fatalf("Unexpected response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("Failed to read response: %v", err)
	}

	var payload TransferPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		fatalf("Invalid config data: %v", err)
	}
	// Validate required fields
	if payload.EncPassword == "" || payload.EncSalt == "" {
		fatalf("Incomplete config: missing encryption keys")
	}
	if payload.S3AccessKeyID == "" || payload.S3SecretAccessKey == "" {
		fatalf("Incomplete config: missing cloud storage credentials")
	}
	if payload.BackendProvider == "" {
		fatalf("Incomplete config: missing backend provider")
	}
	if payload.UseHub && payload.HubEndpoint == "" {
		warnf("Hub mode enabled but no hub endpoint received, using default e2ee-sync-hub:8080")
	}
	ok("Configuration received from existing device")

	// Run automated setup using received config
	plat := platform.Detect()
	rc := rclone.NewClient("")

	// Step 1: Prerequisites
	step(1, 7, "Checking prerequisites")
	if err := plat.CheckRclone(); err != nil {
		fmt.Fprintln(os.Stderr, plat.RcloneInstallHint())
		fatalf("rclone not available: %v", err)
	}
	if err := plat.CheckTailscale(); err != nil {
		fmt.Fprintln(os.Stderr, plat.TailscaleInstallHint())
		fatalf("tailscale not available: %v", err)
	}
	if payload.UseHub {
		hubHost := payload.HubEndpoint
		if hubHost == "" {
			hubHost = "e2ee-sync-hub:8080"
		}
		if idx := strings.Index(hubHost, ":"); idx != -1 {
			hubHost = hubHost[:idx]
		}
		if err := checkHubReachability(hubHost); err != nil {
			warnf("Hub not reachable, will use cloud fallback")
		}
	}
	ok("Prerequisites OK")

	// Step 2: Create rclone remotes
	step(2, 7, "Generating rclone.conf")
	configDir := plat.RcloneConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fatalf("Failed to create config dir: %v", err)
	}
	confPath := filepath.Join(configDir, "rclone.conf")
	backupRcloneConf(confPath)

	creds := &credential.Credentials{
		WebDAVPassword:     payload.WebDAVPassword,
		EncryptionPassword: payload.EncPassword,
		EncryptionSalt:     payload.EncSalt,
		S3AccessKeyID:      payload.S3AccessKeyID,
		S3SecretAccessKey:  payload.S3SecretAccessKey,
		S3Endpoint:         payload.S3Endpoint,
		S3Region:           payload.S3Region,
		Backend: credential.Backend{
			Name:     payload.BackendName,
			Provider: payload.BackendProvider,
		},
	}
	hubEndpoint := payload.HubEndpoint
	if hubEndpoint == "" {
		hubEndpoint = "e2ee-sync-hub:8080"
	}
	if err := createRcloneRemotes(rc, creds, payload.UseHub, hubEndpoint); err != nil {
		fatalf("Failed to create rclone remotes: %v", err)
	}
	creds.EncryptionPassword = ""
	creds.EncryptionSalt = ""
	creds.S3SecretAccessKey = ""
	creds.WebDAVPassword = ""
	ok("rclone.conf written to %s", confPath)

	// Step 3: Filter rules
	step(3, 7, "Writing filter-rules.txt")
	filterPath := filepath.Join(configDir, "filter-rules.txt")
	if err := os.WriteFile(filterPath, []byte(tmpl.FilterRules()), 0644); err != nil {
		fatalf("Failed to write filter-rules.txt: %v", err)
	}
	ok("filter-rules.txt written")

	// Step 4: Sync directory
	step(4, 7, "Creating sync directory")
	syncDir := plat.SyncDir()
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		fatalf("Failed to create sync dir: %v", err)
	}
	ok("Sync directory: %s", syncDir)

	// Step 5: Initial resync
	step(5, 7, "Running initial bisync (resync)")
	syncRemote := "cloud-crypt:"
	if payload.UseHub {
		syncRemote = "hub-crypt:"
	}
	if err := rc.Bisync(syncDir, syncRemote, true); err != nil {
		warnf("Initial resync failed: %v", err)
		warnf("You can run manually: rclone bisync %s %s --resync", syncDir, syncRemote)
	} else {
		ok("Initial resync completed")
	}

	// Step 6: Deploy binary + config
	step(6, 7, "Deploying e2ee-sync")
	binDst, err := deploySelf(plat)
	if err != nil {
		warnf("Failed to deploy: %v", err)
	} else {
		ok("e2ee-sync deployed to %s", binDst)
	}
	daemonConfigDir := plat.ConfigDir()
	if err := os.MkdirAll(daemonConfigDir, 0755); err != nil {
		fatalf("Failed to create config dir: %v", err)
	}
	trashDir := filepath.Join(syncDir, ".trash")
	configContent, err := tmpl.RenderDaemonConfig(tmpl.DaemonConfigData{
		UseHub:         payload.UseHub,
		SyncDir:        syncDir,
		TrashDir:       trashDir,
		FilterFilePath: filterPath,
	})
	if err != nil {
		fatalf("Failed to render config: %v", err)
	}
	daemonConfigPath := filepath.Join(daemonConfigDir, "config.json")
	if err := os.WriteFile(daemonConfigPath, []byte(configContent), 0600); err != nil {
		fatalf("Failed to write config.json: %v", err)
	}
	ok("config.json written to %s", daemonConfigPath)

	// Step 7: Register daemon
	step(7, 7, "Registering daemon")
	if binDst != "" {
		if err := plat.RegisterDaemon(binDst, daemonConfigPath); err != nil {
			warnf("Daemon registration failed: %v", err)
			fmt.Fprintln(os.Stderr, plat.RegisterDaemonHint(binDst, daemonConfigPath))
		} else {
			if runtime.GOOS == "windows" {
				ok("register-daemon.bat created")
				fmt.Println("  To complete daemon setup, right-click register-daemon.bat → Run as administrator")
				fmt.Printf("  Location: %s\n", filepath.Join(plat.BinDir(), "register-daemon.bat"))
			} else {
				ok("Daemon registered and started")
			}
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("=== Join Complete ===")
	fmt.Println()
	fmt.Printf("  Sync directory:  %s\n", syncDir)
	fmt.Printf("  Backend:         %s\n", payload.BackendName)
	fmt.Printf("  Hub mode:        %v\n", payload.UseHub)
	fmt.Println()
	fmt.Println("This device is now syncing with your other devices.")
}

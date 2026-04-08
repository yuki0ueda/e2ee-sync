package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yuki0ueda/e2ee-sync/internal/platform"
	"github.com/yuki0ueda/e2ee-sync/internal/rclone"
)

// TransferPayload is the config data sent from an existing device to a new one.
type TransferPayload struct {
	UseHub            bool   `json:"use_hub"`
	HubEndpoint       string `json:"hub_endpoint,omitempty"`
	BackendProvider   string `json:"backend_provider"`
	BackendName       string `json:"backend_name"`
	S3AccessKeyID     string `json:"s3_access_key_id"`
	S3SecretAccessKey string `json:"s3_secret_access_key"`
	S3Endpoint        string `json:"s3_endpoint"`
	S3Region          string `json:"s3_region"`
	EncPassword       string `json:"enc_password"`
	EncSalt           string `json:"enc_salt"`
	WebDAVPassword    string `json:"webdav_password,omitempty"`
}

func runShare() {
	fs := flag.NewFlagSet("share", flag.ExitOnError)
	useCode := fs.Bool("code", false, "Require a one-time code (for shared tailnets)")
	fs.Parse(os.Args[2:])

	plat := platform.Detect()
	rc := rclone.NewClient("")

	fmt.Print("\n=== Share Configuration ===\n\n")

	// Get Tailscale IP
	tsIP, err := getTailscaleIP()
	if err != nil {
		fatalf("Cannot get Tailscale IP: %v", err)
	}

	// Extract credentials from existing rclone config
	payload, err := extractPayload(plat, rc)
	if err != nil {
		fatalf("Cannot read existing config: %v\nRun 'e2ee-sync setup' first.", err)
	}

	// Generate one-time code if requested
	var code string
	if *useCode {
		code = generateCode()
	}

	// Find a free port
	listener, err := net.Listen("tcp", tsIP+":0")
	if err != nil {
		fatalf("Cannot listen on Tailscale IP: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start single-use HTTP server
	served := make(chan bool, 1)
	var requestServed atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		// Only serve once
		if requestServed.Load() {
			http.Error(w, "Already served", http.StatusGone)
			return
		}
		// Constant-time code comparison to prevent timing attacks
		if code != "" {
			provided := r.URL.Query().Get("code")
			if len(provided) != len(code) || subtle.ConstantTimeCompare([]byte(provided), []byte(code)) != 1 {
				http.Error(w, "Invalid code", http.StatusUnauthorized)
				return
			}
		}
		requestServed.Store(true)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
		served <- true
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Listen failed: %v", err)
		}
		server.Serve(ln)
	}()

	fmt.Println("  Sharing configuration... (expires in 5 minutes)")
	fmt.Println()
	fmt.Println("  On the new device, run:")
	if code != "" {
		fmt.Printf("    e2ee-sync join %s --code %s\n", addr, code)
	} else {
		fmt.Printf("    e2ee-sync join %s\n", addr)
	}
	fmt.Println()
	fmt.Println("  Waiting for connection...")

	select {
	case <-served:
		server.Close() // Immediately stop accepting new connections
		fmt.Println()
		ok("Configuration sent. Done.")
	case <-time.After(5 * time.Minute):
		server.Close()
		fmt.Println()
		warnf("Timed out. No device connected.")
	}
}

func getTailscaleIP() (string, error) {
	cmd := exec.Command("tailscale", "ip", "-4")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tailscale ip failed: %w", err)
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("no Tailscale IPv4 address")
	}
	return ip, nil
}

func extractPayload(plat platform.Platform, rc *rclone.Client) (*TransferPayload, error) {
	payload := &TransferPayload{}

	// Detect hub mode
	configDir := plat.RcloneConfigDir()
	confBytes, err := os.ReadFile(configDir + "/rclone.conf")
	if err != nil {
		return nil, fmt.Errorf("cannot read rclone.conf: %w", err)
	}
	payload.UseHub = strings.Contains(string(confBytes), "[hub-webdav]")

	// Extract cloud-direct (S3) credentials
	cloudShow, err := rc.ConfigShow("cloud-direct")
	if err != nil {
		return nil, fmt.Errorf("cannot read cloud-direct config: %w", err)
	}
	payload.BackendProvider = cloudShow["provider"]
	payload.S3AccessKeyID = cloudShow["access_key_id"]
	payload.S3SecretAccessKey = cloudShow["secret_access_key"]
	payload.S3Endpoint = cloudShow["endpoint"]
	payload.S3Region = cloudShow["region"]

	// Map provider to display name
	switch payload.BackendProvider {
	case "Cloudflare":
		payload.BackendName = "Cloudflare R2"
	case "AWS":
		payload.BackendName = "AWS S3"
	case "B2":
		payload.BackendName = "Backblaze B2"
	default:
		payload.BackendName = "S3-compatible"
	}

	// Extract hub endpoint if hub mode
	if payload.UseHub {
		hubShow, err := rc.ConfigShow("hub-webdav")
		if err == nil {
			payload.HubEndpoint = strings.TrimPrefix(hubShow["url"], "http://")
		}
	}

	// Extract encryption credentials
	cryptShow, err := rc.ConfigShow("cloud-crypt")
	if err != nil {
		return nil, fmt.Errorf("cannot read cloud-crypt config: %w", err)
	}
	payload.EncPassword = cryptShow["password"]
	payload.EncSalt = cryptShow["password2"]

	// Extract WebDAV credentials if hub mode
	if payload.UseHub {
		hubShow, err := rc.ConfigShow("hub-webdav")
		if err != nil {
			return nil, fmt.Errorf("cannot read hub-webdav config: %w", err)
		}
		payload.WebDAVPassword = hubShow["pass"]
	}

	return payload, nil
}

func generateCode() string {
	b := make([]byte, 16) // 128 bits of entropy
	rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

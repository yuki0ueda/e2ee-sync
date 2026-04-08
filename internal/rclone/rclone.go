package rclone

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Client wraps rclone CLI operations.
type Client struct {
	BinPath string
}

// NewClient creates a Client using the rclone binary at the given path.
// If binPath is empty, it defaults to "rclone" (resolved via PATH).
func NewClient(binPath string) *Client {
	if binPath == "" {
		binPath = "rclone"
	}
	return &Client{BinPath: binPath}
}

// ConfigCreate creates or updates an rclone remote using "rclone config create".
// This lets rclone handle password obscuring internally, avoiding
// command-line argument mangling and base64 auto-detection issues.
// params are key-value pairs: "key1", "val1", "key2", "val2", ...
func (c *Client) ConfigCreate(name, remoteType string, params ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := []string{"config", "create", name, remoteType}
	args = append(args, params...)
	// Note: --obscure intentionally omitted.
	// rclone auto-detects password fields and obscures them.
	// Using --obscure can cause S3 keys to be stored as empty
	// when the key resembles base64.
	cmd := exec.CommandContext(ctx, c.BinPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone config create %s failed: %s", name, stderr.String())
	}
	return nil
}

// ConfigDelete removes an rclone remote.
func (c *Client) ConfigDelete(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.BinPath, "config", "delete", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone config delete %s failed: %s", name, stderr.String())
	}
	return nil
}

// ConfigShow runs "rclone config show <remote>" and returns key-value pairs.
// Values are de-obscured (plaintext) by rclone.
func (c *Client) ConfigShow(name string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.BinPath, "config", "show", name)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rclone config show %s failed: %w", name, err)
	}
	return parseConfigShowOutput(string(out)), nil
}

// ListDir runs "rclone lsd" to list directories on a remote.
// Used as a connection test. Returns the stderr output along with any error
// so callers can inspect the failure reason (e.g. 401 Unauthorized).
func (c *Client) ListDir(remote string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.BinPath, "lsd", remote)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), fmt.Errorf("rclone lsd %s failed: %s", remote, stderr.String())
	}
	return "", nil
}

// Bisync runs "rclone bisync" between a local directory and a remote.
// If resync is true, the --resync flag is added for initial synchronization.
func (c *Client) Bisync(local, remote string, resync bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	args := []string{"bisync", local, remote}
	if resync {
		args = append(args, "--resync")
	}
	cmd := exec.CommandContext(ctx, c.BinPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone bisync failed: %s", stderr.String())
	}
	return nil
}

// Version returns the rclone version string.
func (c *Client) Version() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.BinPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rclone version failed: %w", err)
	}
	lines := strings.SplitN(string(out), "\n", 2)
	return strings.TrimSpace(lines[0]), nil
}

// parseConfigShowOutput parses the key=value output from rclone config show.
func parseConfigShowOutput(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

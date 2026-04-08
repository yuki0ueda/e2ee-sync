package platform

// Platform abstracts OS-specific operations for setup and daemon management.
type Platform interface {
	// Paths
	RcloneConfigDir() string
	SyncDir() string
	AutosyncBinDir() string
	AutosyncConfigDir() string

	// Prerequisite checks
	CheckRclone() error
	CheckTailscale() error
	RcloneInstallHint() string
	TailscaleInstallHint() string

	// Daemon management
	RegisterDaemon(binPath, configPath string) error
	UnregisterDaemon() error
	DaemonStatus() (string, error)
	RegisterDaemonHint(binPath, configPath string) string
}

// Detect returns the Platform implementation for the current OS.
// The actual implementation is provided by OS-specific files via build tags.
func Detect() Platform {
	return detect()
}

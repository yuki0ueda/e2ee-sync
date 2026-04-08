package credential

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ReadPassword prompts the user and reads a password with masked input.
// Falls back to visible input if stdin is not a terminal.
func ReadPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		pass, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // newline after masked input
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(pass), nil
	}

	// Fallback for non-terminal (piped input)
	fmt.Fprintln(os.Stderr, "(warning: input will be visible)")
	return ReadLine("")
}

// ReadLine prompts the user and reads a line of visible text.
func ReadLine(prompt string) (string, error) {
	if prompt != "" {
		fmt.Fprint(os.Stderr, prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Confirm prompts the user with a yes/no question and returns the result.
func Confirm(prompt string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	answer, err := ReadLine("")
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

// Backend represents a cloud storage provider.
type Backend struct {
	Name     string // display name
	Provider string // rclone provider value (e.g., "Cloudflare", "AWS", "Backblaze")
}

var Backends = []Backend{
	{Name: "Cloudflare R2", Provider: "Cloudflare"},
	{Name: "AWS S3", Provider: "AWS"},
	{Name: "Backblaze B2", Provider: "B2"},
	{Name: "Other S3-compatible", Provider: "Other"},
}

// Credentials holds all user-provided credentials for setup.
type Credentials struct {
	WebDAVPassword     string
	EncryptionPassword string
	EncryptionSalt     string
	S3AccessKeyID      string
	S3SecretAccessKey  string
	S3Endpoint         string
	S3Region           string
	Backend            Backend
}

// SelectBackend prompts the user to choose a cloud storage backend.
func SelectBackend() (Backend, error) {
	fmt.Fprintln(os.Stderr, "\n  Which cloud storage are you using?")
	for i, b := range Backends {
		fmt.Fprintf(os.Stderr, "    %d) %s\n", i+1, b.Name)
	}
	choice, err := ReadLine("  Select [1-4]: ")
	if err != nil {
		return Backend{}, err
	}
	choice = strings.TrimSpace(choice)
	idx := 0
	switch choice {
	case "1":
		idx = 0
	case "2":
		idx = 1
	case "3":
		idx = 2
	case "4":
		idx = 3
	default:
		return Backend{}, fmt.Errorf("invalid selection: %s", choice)
	}
	return Backends[idx], nil
}

// Collect interactively prompts for all required credentials.
// If useHub is false, the WebDAV password prompt is skipped.
func Collect(useHub bool, backend Backend) (*Credentials, error) {
	// --- Hub credentials ---
	var webdavPass string
	if useHub {
		fmt.Fprintln(os.Stderr, "\n  --- Hub Credentials ---")
		var err error
		webdavPass, err = ReadPassword("  WebDAV password: ")
		if err != nil {
			return nil, err
		}
	}

	// --- Encryption keys ---
	fmt.Fprintln(os.Stderr, "\n  --- Encryption Keys ---")
	fmt.Fprintln(os.Stderr, "  These encrypt your files. Use the SAME keys on all devices.")

	encPass, err := ReadPassword("  Encryption password: ")
	if err != nil {
		return nil, err
	}
	if encPass == "" {
		return nil, fmt.Errorf("encryption password cannot be empty")
	}

	encSalt, err := ReadPassword("  Encryption salt: ")
	if err != nil {
		return nil, err
	}
	if encSalt == "" {
		return nil, fmt.Errorf("encryption salt cannot be empty")
	}

	// --- Cloud storage credentials ---
	fmt.Fprintf(os.Stderr, "\n  --- %s Credentials ---\n", backend.Name)
	fmt.Fprintln(os.Stderr, "  Get these from your cloud provider's API token page.")

	s3Key, err := ReadLine("  Access Key ID: ")
	if err != nil {
		return nil, err
	}
	if s3Key == "" {
		return nil, fmt.Errorf("access key ID cannot be empty")
	}

	s3Secret, err := ReadPassword("  Secret Access Key: ")
	if err != nil {
		return nil, err
	}
	if s3Secret == "" {
		return nil, fmt.Errorf("secret access key cannot be empty")
	}

	var endpoint, region string
	switch backend.Provider {
	case "Cloudflare":
		fmt.Fprintln(os.Stderr, "  Find your endpoint in Cloudflare Dashboard → R2 → Overview.")
		fmt.Fprintln(os.Stderr, "  Example: https://abc123def456.r2.cloudflarestorage.com")
		endpoint, err = ReadLine("  S3 Endpoint URL: ")
		if err != nil {
			return nil, err
		}
		region = "auto"
	case "AWS":
		region, err = ReadLine("  AWS Region (e.g., us-east-1): ")
		if err != nil {
			return nil, err
		}
	case "B2":
		region, err = ReadLine("  B2 Region (e.g., us-west-004): ")
		if err != nil {
			return nil, err
		}
		endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", region)
	default:
		endpoint, err = ReadLine("  S3 Endpoint URL: ")
		if err != nil {
			return nil, err
		}
		if endpoint == "" {
			return nil, fmt.Errorf("S3 endpoint URL cannot be empty")
		}
		region, err = ReadLine("  Region (or 'auto'): ")
		if err != nil {
			return nil, err
		}
	}

	return &Credentials{
		WebDAVPassword:     webdavPass,
		EncryptionPassword: encPass,
		EncryptionSalt:     encSalt,
		S3AccessKeyID:      s3Key,
		S3SecretAccessKey:  s3Secret,
		S3Endpoint:         endpoint,
		S3Region:           region,
		Backend:            backend,
	}, nil
}

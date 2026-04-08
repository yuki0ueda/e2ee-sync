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
	S3Endpoint         string // full endpoint URL for non-R2 backends
	S3Region           string
	Backend            Backend
}

// SelectBackend prompts the user to choose a cloud storage backend.
func SelectBackend() (Backend, error) {
	fmt.Fprintln(os.Stderr, "\n  Cloud storage backend:")
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
	fmt.Fprintln(os.Stderr)

	var webdavPass string
	if useHub {
		var err error
		webdavPass, err = ReadPassword("WebDAV password: ")
		if err != nil {
			return nil, err
		}
	}

	encPass, err := ReadPassword("Encryption password: ")
	if err != nil {
		return nil, err
	}

	encSalt, err := ReadPassword("Encryption salt: ")
	if err != nil {
		return nil, err
	}

	s3Key, err := ReadLine("Access Key ID: ")
	if err != nil {
		return nil, err
	}

	s3Secret, err := ReadPassword("Secret Access Key: ")
	if err != nil {
		return nil, err
	}

	var endpoint, region string
	switch backend.Provider {
	case "Cloudflare":
		accountID, err := ReadLine("R2 Account ID: ")
		if err != nil {
			return nil, err
		}
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
		region = "auto"
	case "AWS":
		region, err = ReadLine("AWS Region (e.g., us-east-1): ")
		if err != nil {
			return nil, err
		}
	case "B2":
		region, err = ReadLine("B2 Region (e.g., us-west-004): ")
		if err != nil {
			return nil, err
		}
		endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", region)
	default:
		endpoint, err = ReadLine("S3 Endpoint URL: ")
		if err != nil {
			return nil, err
		}
		region, err = ReadLine("Region (or 'auto'): ")
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

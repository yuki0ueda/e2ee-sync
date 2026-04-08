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

// Credentials holds all user-provided credentials for setup.
type Credentials struct {
	WebDAVPassword       string
	EncryptionPassword   string
	EncryptionSalt       string
	R2AccessKeyID        string
	R2SecretAccessKey    string
	R2AccountID          string
}

// Collect interactively prompts for all required credentials.
// If useHub is false, the WebDAV password prompt is skipped.
func Collect(useHub bool) (*Credentials, error) {
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

	r2Key, err := ReadLine("R2 Access Key ID: ")
	if err != nil {
		return nil, err
	}

	r2Secret, err := ReadPassword("R2 Secret Access Key: ")
	if err != nil {
		return nil, err
	}

	r2Account, err := ReadLine("R2 Account ID: ")
	if err != nil {
		return nil, err
	}

	return &Credentials{
		WebDAVPassword:     webdavPass,
		EncryptionPassword: encPass,
		EncryptionSalt:     encSalt,
		R2AccessKeyID:      r2Key,
		R2SecretAccessKey:  r2Secret,
		R2AccountID:        r2Account,
	}, nil
}

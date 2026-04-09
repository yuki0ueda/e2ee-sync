# Security Policy

[Japanese / 日本語](SECURITY.ja.md)

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

Only the latest release is actively supported with security fixes.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Use [GitHub Private Vulnerability Reporting](https://github.com/yuki0ueda/e2ee-sync/security/advisories/new) to report vulnerabilities. This ensures the issue can be addressed before public disclosure.

When reporting, please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

You can expect an initial response within 7 days. Critical vulnerabilities will be prioritized for a patch release.

## Security Design

e2ee-sync relies on [rclone crypt](https://rclone.org/crypt/) for all encryption — no custom cryptographic implementations are used.

### Key Principles

- **Encryption keys never leave your device.** Decryption happens locally; the hub and cloud storage never have access to plaintext data or keys.
- **No accounts, no tracking.** Users bring their own storage and Tailscale network.
- **Minimal attack surface.** No web UI, no mobile apps, no file sharing features.

### Threat Model

**Protects against:**
- Cloud storage provider breaches (data is encrypted before upload)
- Network interception (Tailscale provides encrypted transport; data is also encrypted at rest)
- Hub compromise (hub only handles encrypted blobs, never holds encryption keys)

**Does not protect against:**
- Compromise of the local device where encryption keys reside
- Loss of encryption keys (no backdoor, no recovery without both password and salt)

### Credential Handling

- Encryption passwords are input via masked terminal prompts and cleared from memory after use
- Configuration files (`rclone.conf`, `config.json`) are restricted to `0600` permissions
- Share/join flow uses single-use endpoints with 5-minute timeouts and constant-time code comparison

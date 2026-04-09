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

### Encryption Details

rclone crypt uses the following algorithms:

| Component | Algorithm | Details |
|-----------|-----------|---------|
| File content encryption | NaCl SecretBox (XSalsa20-Poly1305) | Authenticated encryption in 64KB chunks, random 24-byte nonce per file |
| Key derivation | scrypt (N=16384, r=8, p=1) | Derives 80 bytes from password + salt: 32B content key, 32B filename key, 16B filename IV |
| Filename encryption | AES-256-EME | Wide-block cipher (Halevi & Rogaway, 2003), modified base32 encoding |

For the full specification, see the [rclone crypt documentation](https://rclone.org/crypt/).

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
- A malicious storage provider actively manipulating encrypted data (see Known Limitations below)

### Known Limitations of rclone crypt

These are inherent to rclone crypt's design, not bugs in e2ee-sync. Users should understand these when evaluating whether e2ee-sync fits their threat model.

| Limitation | Description |
|------------|-------------|
| Single master key | All files are encrypted with the same key pair. There are no per-file keys. If the master key is compromised, all data is affected. |
| File size inference | Original file size can be calculated from ciphertext size (fixed overhead per chunk). |
| Directory structure visible | Directory hierarchy, file count, and timestamps are visible to the storage provider. Only file/directory names are encrypted. |
| Truncation attack | Removing trailing chunks from an encrypted file is undetectable. A malicious server could silently truncate files. |
| No formal audit | rclone crypt has not undergone a third-party security audit as of April 2026. |

**What this means in practice:**
- rclone crypt provides strong protection against **data breaches** — if a cloud provider is compromised and stored objects are exfiltrated, the attacker cannot read your files without the encryption key.
- rclone crypt has limited protection against an **actively malicious storage provider** that targets you specifically (e.g., analyzing metadata patterns, truncating files, or correlating file sizes). This threat model is relevant for state-level adversaries, not typical cloud provider breaches.
- If your threat model requires protection against an actively malicious server, consider layering additional local encryption (e.g., [gocryptfs](https://nuetzlich.net/gocryptfs/)) before syncing.

### Credential Handling

- Encryption passwords are input via masked terminal prompts and cleared from memory after use
- Configuration files (`rclone.conf`, `config.json`) are restricted to `0600` permissions
- Share/join flow uses single-use endpoints with 5-minute timeouts and constant-time code comparison
- Passwords in `rclone.conf` are obfuscated using rclone obscure (AES-CTR with a static key). This is **not** cryptographic protection — it only prevents casual visual exposure. File security relies on `0600` permissions, not on obscure

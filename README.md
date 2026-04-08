# e2ee-sync

[Japanese / 日本語](README.ja.md)

End-to-end encrypted file synchronization setup tool.

Automates the configuration of [rclone](https://rclone.org/) bisync with client-side encryption across multiple devices, using [Tailscale](https://tailscale.com/) for secure connectivity and Cloudflare R2 as a cloud fallback.

## Architecture

```
                      ┌──────────────────────────┐
Device A ──┐          │  e2ee-sync-hub (optional) │
            ├─ Tailscale ─┤  WebDAV + R2 backup       │
Device B ──┘          └──────────────────────────┘
  │                              │
  │  R2 direct (hub down        │  periodic sync
  │  or no hub at all)          │
  ▼                              ▼
┌──────────────────────────────────┐
│  Cloudflare R2 (encrypted blob)  │
└──────────────────────────────────┘
```

- **With hub**: Fast direct sync via Tailscale WebDAV + hub handles R2 backup + ZFS snapshots for versioning
- **Without hub**: Devices sync directly to Cloudflare R2 — slower but fully functional
- **Encryption**: rclone crypt with filename and directory name encryption (client-side only)

## Sync Directory

Files in `~/sync` (Windows: `%USERPROFILE%\sync`) are bidirectionally synced across all your devices. Files are encrypted client-side before leaving the device — the hub and R2 only store encrypted blobs. Exclusion patterns (`.DS_Store`, `*.tmp`, `node_modules/`, etc.) are configured in `filter-rules.txt`.

## Prerequisites

- [rclone](https://rclone.org/install/) 1.71.0+ installed and in PATH
- [Tailscale](https://tailscale.com/download) installed and connected to your tailnet
- Cloudflare R2 bucket (required)
- `e2ee-sync-hub` reachable via Tailscale (optional — enables fast direct sync)

## Getting Started

### 1. Cloudflare R2 Setup

Create a bucket and API token in the Cloudflare Dashboard.

**Create Bucket**: R2 → Create Bucket

```
Bucket name: e2ee-sync
Region: Automatic (or APAC)
```

**Create API Token**: R2 → Manage R2 API Tokens → Create API Token

```
Permissions: Object Read & Write
Bucket: e2ee-sync
```

Note these values (needed during device setup):

```
Access Key ID: xxxxxxxxxxxxxxxx
Secret Access Key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Endpoint: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

### 2. Prepare Passwords

Prepare these three passwords. **Use alphanumeric characters only** to avoid shell escaping issues.

| Password | Purpose | Shared across devices? |
|----------|---------|----------------------|
| WebDAV password | Hub authentication (skip if no hub) | Yes |
| Encryption password | File content encryption (rclone crypt `password`) | Yes |
| Salt | Filename encryption (rclone crypt `password2`) | Yes |

> **If you lose the encryption password and salt, your data is unrecoverable.**
> Store them in a password manager (e.g., Bitwarden) immediately after generation.

```bash
# Generate 32-char alphanumeric password (Linux/macOS)
cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 32; echo
```

```powershell
# Windows PowerShell
-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | ForEach-Object {[char]$_})
```

### 3. (Optional) Hub Setup

The hub is **not required** — devices can sync directly via Cloudflare R2. However, a dedicated Proxmox LXC hub provides:

- **Faster sync** via Tailscale direct connection instead of R2 round-trip
- **ZFS snapshots** for point-in-time recovery
- **Reduced R2 costs** (hub batches uploads instead of every device syncing individually)

See [`hub/README.md`](hub/README.md) for the Proxmox LXC setup guide.

### 4. Device Setup

Download `e2ee-sync` for your OS from [GitHub Releases](https://github.com/yuki0ueda/e2ee-sync/releases) and run:

```bash
e2ee-sync setup
```

The CLI walks you through:

1. Prerequisite verification (rclone, Tailscale, hub connectivity)
2. Credential entry (WebDAV, encryption keys, R2 keys)
3. rclone.conf generation with obscured passwords
4. Filter rules and sync directory creation
5. Connection testing and initial bisync
6. Daemon deployment and registration

The setup copies `e2ee-sync` to the appropriate location and registers the daemon:

| OS | Installed to | Daemon type |
|----|-------------|-------------|
| Linux | `~/.local/bin/e2ee-sync` | systemd user service |
| macOS | `/usr/local/bin/e2ee-sync` | LaunchAgent |
| Windows | `%USERPROFILE%\.local\bin\e2ee-sync.exe` | Task Scheduler (via `register-daemon.bat`) |

**Windows note**: Daemon registration requires administrator privileges. The setup generates `register-daemon.bat` — right-click it and select "Run as administrator" to complete the registration. The daemon runs as a background process with no console window.

For upgrades, download the new version and run `e2ee-sync upgrade`.

### Other Commands

```bash
e2ee-sync verify      # Verify existing configuration
e2ee-sync upgrade     # Update binary in place
e2ee-sync uninstall   # Remove daemon and configuration
e2ee-sync version     # Show version
```

Running without arguments shows an interactive menu.

## Platform Support

| OS | Daemon | Download |
|----|--------|----------|
| Linux | systemd user service | `e2ee-sync-linux-x64` / `e2ee-sync-linux-arm64` |
| macOS | LaunchAgent | `e2ee-sync-mac-x64` / `e2ee-sync-mac-arm64` |
| Windows | Task Scheduler (`register-daemon.bat`) | `e2ee-sync-win-x64.exe` / `e2ee-sync-win-arm64.exe` |

## Building from Source

```bash
git clone https://github.com/yuki0ueda/e2ee-sync.git
cd e2ee-sync

# Build for current platform
make build

# Cross-compile all platforms
make build-all
```

Requires Go 1.25+.

## Project Structure

```
e2ee-sync/
├── cmd/
│   └── e2ee-sync/   # Single binary: setup + daemon + verify + upgrade
├── internal/
│   ├── platform/    # OS-specific implementations
│   ├── credential/  # Interactive credential input
│   ├── template/    # rclone.conf / config templates
│   ├── rclone/      # rclone CLI wrapper
│   └── version/     # Build-time version info
├── hub/             # Proxmox LXC hub setup
│   ├── systemd/     # systemd unit templates
│   └── setup.sh     # Automated hub setup script
└── Makefile
```

## License

[MIT](LICENSE)

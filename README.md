# e2ee-sync

[Japanese / 日本語](README.ja.md)

End-to-end encrypted file synchronization setup tool.

Automates the configuration of [rclone](https://rclone.org/) bisync with client-side encryption across multiple devices, using [Tailscale](https://tailscale.com/) for secure connectivity and S3-compatible cloud storage as a backend.

Supported backends: Cloudflare R2, AWS S3, Backblaze B2, and other S3-compatible services.

## Who Is This For?

e2ee-sync is designed for syncing **sensitive documents** across your devices — files where you need to control the encryption keys yourself, not trust a cloud provider.

**Best for:**
- Contracts, financial records, legal documents
- API keys, SSH keys, `.env` files, password databases
- Private notes, journals, research drafts
- Tax returns, invoices, business plans
- Long-term encrypted archival (compliance records, old projects, personal history)
- Any file where a cloud provider breach would be a serious problem

**Not designed for:**
- Large video files (no delta sync — full re-upload on every change)
- Real-time collaboration (this is sync, not Google Docs)
- Mobile access (desktop only — Windows, macOS, Linux)
- File sharing with others (personal sync, no share links)

**Common use patterns:**

| Pattern | Example | Why e2ee-sync |
|---------|---------|--------------|
| Cross-device sync | Laptop ↔ Desktop | Keep sensitive work files in sync |
| Encrypted backup | PC → Cloud | Off-site backup with client-side encryption |
| Long-term archive | Tax records, contracts | Cheap cloud storage ($0.004/GB) + encryption you control forever |
| Credential sync | `.env`, SSH keys, KeePass DB | Sync secrets without trusting a cloud provider |

> **Use the right tool for each job.** Sync sensitive documents with e2ee-sync. Share photos with iCloud/Google Photos. Collaborate on docs with Google Drive. Back up large files with Backblaze. e2ee-sync handles the files you can't afford to leave unencrypted.

## Design Philosophy

e2ee-sync intentionally omits features that other E2EE services provide — mobile apps, web UI, file sharing, cloud previews. This is not a limitation; it's a design choice.

**Minimal attack surface over convenience.** Every feature that touches your decrypted data is a potential attack vector. A mobile app means your secrets live on a phone that can be lost or compromised. A web UI means your encryption keys pass through a browser. Share links mean your encrypted files become accessible via URL. Cloud previews mean someone, somewhere, decrypts your files.

e2ee-sync takes a different approach:

| Principle | Implementation |
|-----------|---------------|
| **Files stay local** | Your `~/sync` folder is an ordinary directory. Open files with your normal apps — no special viewer needed. |
| **Keys never leave your device** | Encryption/decryption happens on your machine only. No server-side processing. |
| **No app dependency** | Your data is standard rclone crypt on standard S3 objects. If e2ee-sync disappears, decrypt with rclone directly. |
| **No account required** | No sign-up, no login, no user tracking. You bring your own storage. |
| **Open source** | Every line of code is auditable. The encryption is rclone's battle-tested crypt, not a custom scheme. |

**What if this project is abandoned?** Your data survives. It's standard rclone crypt on a standard S3 bucket — both open, documented formats. You can always access your files directly:

```bash
rclone mount cloud-crypt: /mnt/sync   # mount as local drive
rclone copy cloud-crypt: /local/dir   # download everything
```

No proprietary format. No vendor lock-in. No special tools needed. Your data is yours, permanently.

For detailed recovery procedures (lost computer, provider migration, corrupted sync, lost password), see the **[Disaster Recovery Guide](docs/disaster-recovery.md)**.

## Architecture

```
                         ┌──────────────────────────────────┐
Device A ──┐             │  e2ee-sync-hub (optional)         │
            ├─ Tailscale ─┤  WebDAV relay + cloud backup      │
Device B ──┘             └──────────────────────────────────┘
  │                                    │
  │  Cloud direct (hub down            │  periodic sync
  │  or no hub at all)                 │
  ▼                                    ▼
┌──────────────────────────────────────────┐
│  S3-compatible storage (encrypted blobs)  │
│  e.g., Cloudflare R2, AWS S3, B2         │
└──────────────────────────────────────────┘
```

- **With hub**: Fast direct sync via Tailscale WebDAV + hub handles cloud backup + ZFS snapshots for versioning
- **Without hub**: Devices sync directly to cloud storage — slower but fully functional
- **Encryption**: rclone crypt with filename and directory name encryption (client-side only)

## Sync Directory

Files in `~/sync` (Windows: `%USERPROFILE%\sync`) are bidirectionally synced across all your devices. Files are encrypted client-side before leaving the device — the hub and cloud storage only store encrypted blobs. Exclusion patterns (`.DS_Store`, `*.tmp`, `node_modules/`, etc.) are configured in `filter-rules.txt`.

### Trash (Deleted File Recovery)

When files are deleted or overwritten during sync, the previous version is automatically saved to `~/sync/.trash/YYYY-MM-DD/`. Files in trash are kept for 30 days, then auto-cleaned.

> **Note**: Cloud storage providers vary in versioning support. AWS S3 and Backblaze B2 support server-side versioning (enable it in your bucket settings for additional protection). Cloudflare R2 does not currently support object versioning — the local trash folder is your primary recovery mechanism.

## Prerequisites

Install these before running e2ee-sync:

1. **[rclone](https://rclone.org/install/)** 1.71.0+ — file sync engine
   - Windows: `winget install Rclone.Rclone` or [download](https://rclone.org/downloads/)
   - macOS: `brew install rclone`
   - Linux: `sudo apt install rclone` or `curl https://rclone.org/install.sh | sudo bash`

2. **[Tailscale](https://tailscale.com/download)** — secure device networking
   - Install from [tailscale.com/download](https://tailscale.com/download) and sign in

3. **S3-compatible storage** — Cloudflare R2, AWS S3, Backblaze B2, etc.
   - Create a bucket and API credentials (see Getting Started below)

4. *(Optional)* **e2ee-sync-hub** — dedicated relay server for faster sync

## Getting Started

### 1. Cloud Storage Setup

Create a bucket and S3 API credentials on your chosen provider.

**Example (Cloudflare R2):**

1. R2 → Create Bucket → name: `e2ee-sync`
2. R2 → Manage R2 API Tokens → Create API Token (Object Read & Write)

Note these values (needed during device setup):

```
Access Key ID: xxxxxxxxxxxxxxxx
Secret Access Key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
S3 Endpoint URL: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

Other providers (AWS S3, Backblaze B2, etc.) work similarly — you need an Access Key, Secret Key, and endpoint/region.

### 2. Prepare Passwords

Prepare these three passwords. Special characters are supported.

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

The hub is **not required** — devices can sync directly via cloud storage. However, a dedicated Proxmox LXC hub provides:

- **Faster sync** via Tailscale direct connection instead of cloud round-trip
- **ZFS snapshots** for point-in-time recovery
- **Reduced cloud API costs** (hub batches uploads instead of every device syncing individually)

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

### Adding More Devices

For the 2nd device and beyond, use share/join to skip credential entry:

```bash
# On an already-configured device
e2ee-sync share

# On the new device (copy the address from share output)
e2ee-sync join <ip:port>
```

All credentials are transferred automatically via Tailscale. For shared tailnets (teams, families), add `--code` for security:

```bash
e2ee-sync share --code
e2ee-sync join <ip:port> --code <CODE>
```

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

## Cost Comparison

e2ee-sync works with any S3-compatible storage. For a detailed comparison of providers (Cloudflare R2, Backblaze B2, IDrive e2, AWS S3, etc.), subscription services (Dropbox, Filen), and lifetime deals (pCloud, Internxt, Icedrive), see:

**[Cost Comparison Guide](docs/cost-comparison.md)**

Quick summary: **10GB and under is free** (R2/B2/IDrive e2 free tier). 100GB costs $0.40-$1.50/month depending on provider.

## License

[MIT](LICENSE)

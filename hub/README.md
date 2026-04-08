# e2ee-sync: Hub Setup (Proxmox LXC)

[Japanese / 日本語](README.ja.md) | [Back to main README](../README.md)

Setup guide for the Proxmox LXC hub — an **optional** component of [e2ee-sync](https://github.com/yuki0ueda/e2ee-sync).

> **The hub is not required.** Devices can sync directly via Cloudflare R2 without a hub.
> The hub adds fast direct sync via Tailscale, ZFS snapshots, and reduced R2 API costs.
>
> **Audience**: Advanced users with Proxmox VE who want a dedicated LXC as the sync hub.

---

## Architecture

```
[Devices - Win/Mac/Linux]
  ├── e2ee-sync-setup (initial configuration)
  ├── autosync (daemon: file watch + bisync + failover)
  ├── rclone crypt (client-side encrypt/decrypt)
  └── Tailscale
         │
         │ bisync (WebDAV over Tailscale)
         │ If hub unreachable → automatic R2 fallback
         ▼
[Proxmox LXC - e2ee-sync-hub]
  ├── rclone serve webdav (accepts encrypted blobs)
  ├── rclone sync → R2 (periodic offsite backup)
  ├── R2 catch-up on boot (disaster recovery)
  ├── ZFS snapshots (hourly, 72 generations)
  └── Tailscale
         │
         │ sync (encrypted blobs as-is)
         ▼
[Cloudflare R2]
  └── Encrypted blob storage (offsite backup)
```

### Security Model

```
                    Has encryption keys?  Can read data?
──────────────────────────────────────────────────────
Device                    Yes                 Yes (plaintext)
Proxmox Hub               No                  No (encrypted blobs)
Cloudflare R2             No                  No (encrypted blobs)
Tailscale                 No                  No (transport only)
```

---

## Before You Start

Complete these steps from the [main README](../README.md#getting-started) first:

1. **Cloudflare R2 Setup** — create the `e2ee-sync` bucket and API token
2. **Prepare Passwords** — WebDAV, encryption password, and salt

## Prerequisites

| Component | Version | Purpose |
|-----------|---------|---------|
| Proxmox VE | 8.x | LXC host |
| rclone | 1.71.0+ | bisync stable required |
| Tailscale | latest | Device networking |

---

## 1. Proxmox LXC Setup

### 1.1 Create ZFS Dataset

On the Proxmox host:

```bash
zfs create rpool/data/rclone-encrypted
```

> **Important**: Change ownership for unprivileged LXC access (UID 100000).
> Without this, rclone inside the container cannot write to disk — data stays in VFS memory and is never persisted.

```bash
chown -R 100000:100000 /rpool/data/rclone-encrypted/
```

### 1.2 Create LXC Container

```bash
pct create <VMID> local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst \
  --hostname e2ee-sync-hub \
  --cores 2 \
  --memory 2048 \
  --rootfs local-zfs:32 \
  --net0 name=eth0,bridge=vmbr0,ip=dhcp \
  --unprivileged 1 \
  --features nesting=1 \
  --start 1
```

Replace `<VMID>` with your VM ID (e.g., 203).

### 1.3 Mount Storage and TUN Device

```bash
# Bind mount for data
pct set <VMID> -mp0 /rpool/data/rclone-encrypted,mp=/data/encrypted

# TUN device for Tailscale
pct set <VMID> -dev0 /dev/net/tun
```

### 1.4 Base Setup Inside Container

> **Note**: If the Proxmox host uses Tailscale, `/etc/resolv.conf` inside the LXC
> will point to MagicDNS (`100.100.100.100`). Since Tailscale isn't installed in
> the container yet, DNS resolution fails and `apt` becomes extremely slow.
> Set a temporary public DNS first.

```bash
pct enter <VMID>

# Temporary public DNS
echo "nameserver 1.1.1.1" > /etc/resolv.conf
echo "nameserver 8.8.8.8" >> /etc/resolv.conf

# Install packages
apt update && apt install -y curl unzip

# rclone
curl https://rclone.org/install.sh | bash

# Tailscale
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up --hostname=e2ee-sync-hub
```

After Tailscale connects, reboot to restore DNS:

```bash
exit
pct reboot <VMID>
```

### 1.5 Automated Setup (Recommended)

Copy the `hub/` directory to the LXC and run the setup script:

```bash
# From your machine (with Tailscale)
scp -r hub/ root@e2ee-sync-hub:/root/

# Inside the LXC
cd /root/hub
bash setup.sh
```

The script will interactively:
1. Verify prerequisites (rclone, Tailscale, /data/encrypted)
2. Collect R2 credentials and WebDAV password
3. Generate rclone.conf
4. Install and enable all systemd services

### 1.6 Manual Setup (Alternative)

If you prefer manual setup, see the systemd unit templates in [`hub/systemd/`](systemd/) and create rclone.conf manually:

```bash
mkdir -p ~/.config/rclone
cat > ~/.config/rclone/rclone.conf << 'EOF'
[local-encrypted]
type = alias
remote = /data/encrypted

[r2-raw]
type = s3
provider = Cloudflare
access_key_id = YOUR_R2_ACCESS_KEY
secret_access_key = YOUR_R2_SECRET_KEY
endpoint = https://ACCOUNT_ID.r2.cloudflarestorage.com
region = auto
acl = private
EOF
```

Service startup order:

```
tailscaled
  → rclone-catchup (R2 → local diff catch-up)
    → rclone-webdav (WebDAV starts after catch-up)
    → rclone-r2sync.timer (R2 sync every 15 min)
```

### 1.7 ZFS Snapshots (Optional)

On the Proxmox host, add to crontab:

```bash
crontab -e
```

```cron
# Hourly snapshots
0 * * * * zfs snapshot rpool/data/rclone-encrypted@auto-$(date +\%Y\%m\%d-\%H\%M)
# Keep 72 generations
5 * * * * zfs list -t snapshot -o name -s creation rpool/data/rclone-encrypted | grep @auto- | head -n -72 | xargs -r -n1 zfs destroy
```

---

## 2. Device Setup (Client)

Use the `e2ee-sync-setup` CLI to configure client devices:

```bash
# Download from GitHub Releases
# https://github.com/yuki0ueda/e2ee-sync/releases

e2ee-sync-setup setup
```

The CLI automates: rclone.conf generation, filter rules, initial bisync, and daemon registration.

See the [main README](../README.md) for details.

---

## 3. Verification

### E2EE Check

```bash
# On a device: create a test file
echo "hello from device" > ~/sync/test.txt

# Run bisync
rclone bisync ~/sync/ hub-crypt: --checksum --verbose

# On the hub: only encrypted blobs are visible
ls /data/encrypted/
# → Random strings like r2qnlbbu7l856rsibqrg291qao
```

### Failover Test

```bash
# 1. Stop hub WebDAV
ssh e2ee-sync-hub "systemctl stop rclone-webdav"

# 2. Sync via R2 fallback
echo "failover test" > ~/sync/failover.txt
rclone bisync ~/sync/ r2-crypt: --checksum --verbose

# 3. Restart hub
ssh e2ee-sync-hub "systemctl start rclone-webdav"

# 4. Manual catch-up
ssh e2ee-sync-hub "systemctl start rclone-catchup"
```

### Timer Status

```bash
# Inside LXC
systemctl list-timers rclone-r2sync.timer
```

---

## 4. Troubleshooting

### apt is extremely slow / times out

**Cause**: Proxmox host's Tailscale MagicDNS (`100.100.100.100`) inherited by LXC, but Tailscale isn't installed in the container yet.

**Fix**: Set temporary public DNS before installing Tailscale (see section 1.4).

### WebDAV authentication failure (401 Unauthorized)

**Cause**: Special characters in password are interpreted differently by shell or `rclone obscure`.

**Fix**:
1. Verify the hub's `--pass` matches the device's obscure source
2. Use alphanumeric-only passwords
3. Test with: `rclone lsd hub-webdav:`

### bisync succeeds but no files on hub disk

**Cause**: Unprivileged LXC UID mapping. Container root (UID 0) maps to host UID 100000. rclone's VFS absorbs writes in memory without persisting to disk.

**Fix**: On the Proxmox host:

```bash
chown -R 100000:100000 /rpool/data/rclone-encrypted/
```

### bisync modtime WARNING

```
WARNING: Modtime compare was requested but at least one remote does not support it.
```

**Cause**: WebDAV does not support modtime comparison.

**Fix**: Always use `--checksum`. The autosync daemon uses this by default.

```bash
rclone bisync ~/sync/ hub-crypt: --checksum --verbose
```

### ZFS Snapshot Recovery

```bash
# On Proxmox host

# List snapshots
zfs list -t snapshot rpool/data/rclone-encrypted

# Browse a snapshot
ls /rpool/data/rclone-encrypted/.zfs/snapshot/auto-20260307-1400/

# Restore a single file
cp /rpool/data/rclone-encrypted/.zfs/snapshot/auto-20260307-1400/FILENAME /rpool/data/rclone-encrypted/

# Full rollback (latest snapshot only)
zfs rollback rpool/data/rclone-encrypted@auto-20260307-1400
```

---

## 5. Operations Reference

### Service Commands

```bash
# Inside LXC
systemctl status rclone-webdav.service    # WebDAV status
systemctl start rclone-r2sync.service     # Manual R2 sync
systemctl start rclone-catchup.service    # Manual catch-up
systemctl list-timers                     # Timer status
journalctl -u rclone-webdav.service -f    # WebDAV logs
cat /var/log/rclone-r2sync.log            # R2 sync logs
cat /var/log/rclone-catchup.log           # Catch-up logs
```

### Adding a New Device

1. Install rclone and Tailscale on the device
2. Run `e2ee-sync-setup setup` — use the **same encryption password and salt** as other devices
3. See [main README](../README.md) for details

### Checking Encryption Passwords

```bash
# Show current rclone config (includes obscured passwords)
rclone config show
```

### R2 Cost Estimate

| Item | Unit Price | Estimate |
|------|-----------|----------|
| Storage | $0.015/GB/month | 100GB → $1.50/month |
| Class A (PUT/POST) | $4.50/1M requests | ~$0.50/month |
| Class B (GET/LIST) | $0.36/1M requests | Negligible |
| Egress | Free | $0 |
| **Total** | | **~$2/month for 100GB** |

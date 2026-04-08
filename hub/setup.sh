#!/usr/bin/env bash
set -euo pipefail

# e2ee-sync-hub setup script
# Run inside the Proxmox LXC container as root.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SYSTEMD_DIR="$SCRIPT_DIR/systemd"

# --- Helpers ---

info()  { printf '\033[1;34m[INFO]\033[0m  %s\n' "$1"; }
ok()    { printf '\033[1;32m[OK]\033[0m    %s\n' "$1"; }
warn()  { printf '\033[1;33m[WARN]\033[0m  %s\n' "$1" >&2; }
error() { printf '\033[1;31m[ERROR]\033[0m %s\n' "$1" >&2; exit 1; }

prompt_visible() {
    printf '%s' "$1" >&2
    read -r REPLY
    echo "$REPLY"
}

prompt_secret() {
    printf '%s' "$1" >&2
    read -rs REPLY
    echo >&2
    echo "$REPLY"
}

# --- Checks ---

info "Checking prerequisites..."

if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root"
fi

if ! command -v rclone &>/dev/null; then
    error "rclone not found. Install with: curl https://rclone.org/install.sh | bash"
fi

if ! command -v tailscale &>/dev/null; then
    error "tailscale not found. Install with: curl -fsSL https://tailscale.com/install.sh | sh"
fi

if ! tailscale status &>/dev/null; then
    error "Tailscale not connected. Run: tailscale up --hostname=e2ee-sync-hub"
fi

if [ ! -d /data/encrypted ]; then
    error "/data/encrypted not found. Mount the ZFS dataset first (see README.md section 1.1-1.3)"
fi

if [ ! -d "$SYSTEMD_DIR" ]; then
    error "systemd/ directory not found next to this script"
fi

ok "Prerequisites OK"

# --- Credential input ---

echo
info "Enter Cloudflare R2 credentials"
R2_ACCESS_KEY=$(prompt_visible "  R2 Access Key ID: ")
R2_SECRET_KEY=$(prompt_secret   "  R2 Secret Access Key: ")
R2_ACCOUNT_ID=$(prompt_visible  "  R2 Account ID: ")

echo
info "Enter WebDAV password (alphanumeric only recommended)"
WEBDAV_PASSWORD=$(prompt_secret "  WebDAV password: ")

if [[ "$WEBDAV_PASSWORD" =~ [^a-zA-Z0-9] ]]; then
    warn "Password contains special characters. This may cause authentication issues."
    printf '  Continue anyway? [y/N]: ' >&2
    read -r yn
    case "$yn" in [yY]*) ;; *) exit 1 ;; esac
fi

# --- rclone.conf ---

info "Writing rclone.conf..."
mkdir -p /root/.config/rclone

cat > /root/.config/rclone/rclone.conf << EOF
[local-encrypted]
type = alias
remote = /data/encrypted

[cloud-raw]
type = s3
provider = Cloudflare
access_key_id = ${R2_ACCESS_KEY}
secret_access_key = ${R2_SECRET_KEY}
endpoint = https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com
region = auto
acl = private
EOF

chmod 600 /root/.config/rclone/rclone.conf
ok "rclone.conf written"

# --- Connection test ---

info "Testing R2 connection..."
if rclone lsd cloud-raw:e2ee-sync 2>/dev/null; then
    ok "R2 connection OK"
else
    # Bucket may not have any directories yet — try listing objects
    if rclone ls cloud-raw:e2ee-sync --max-depth 1 2>/dev/null; then
        ok "R2 connection OK (bucket exists, no directories)"
    else
        warn "R2 connection test failed. Check credentials and try again."
        warn "You can re-run this script after fixing the issue."
    fi
fi

# --- systemd units ---

info "Installing systemd units..."

# WebDAV service (substitute password)
sed "s|__WEBDAV_PASSWORD__|${WEBDAV_PASSWORD}|g" \
    "$SYSTEMD_DIR/rclone-webdav.service" > /etc/systemd/system/rclone-webdav.service

# Other units (copy as-is)
cp "$SYSTEMD_DIR/rclone-r2sync.service" /etc/systemd/system/
cp "$SYSTEMD_DIR/rclone-r2sync.timer"   /etc/systemd/system/
cp "$SYSTEMD_DIR/rclone-catchup.service" /etc/systemd/system/

ok "systemd units installed"

# --- Enable and start ---

info "Enabling services..."
systemctl daemon-reload
systemctl enable rclone-catchup.service
systemctl enable --now rclone-webdav.service
systemctl enable --now rclone-r2sync.timer

ok "Services enabled and started"

# --- Verify ---

echo
info "Verifying..."

if systemctl is-active --quiet rclone-webdav.service; then
    ok "rclone-webdav: running"
else
    warn "rclone-webdav: not running — check: journalctl -u rclone-webdav.service"
fi

if systemctl is-active --quiet rclone-r2sync.timer; then
    ok "rclone-r2sync.timer: active"
else
    warn "rclone-r2sync.timer: not active"
fi

# --- Summary ---

echo
echo "=== Hub Setup Complete ==="
echo
echo "  WebDAV:    http://$(tailscale ip -4):8080"
echo "  Hostname:  e2ee-sync-hub (via Tailscale)"
echo "  Data:      /data/encrypted"
echo "  R2 sync:   every 15 minutes"
echo "  Catchup:   on boot (R2 -> local)"
echo
echo "Next: run e2ee-sync on your devices."
echo "  See: https://github.com/yuki0ueda/e2ee-sync"
echo

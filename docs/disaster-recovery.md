# Disaster Recovery Guide

[Japanese / 日本語](disaster-recovery.ja.md) | [Back to README](../README.md)

How to recover your data when things go wrong. e2ee-sync uses standard rclone crypt on standard S3 objects — your data is always accessible even without e2ee-sync.

## What You Need to Recover

| Item | Where to find it | Without it |
|------|-----------------|------------|
| **Encryption password** | Your password manager | ❌ Data unrecoverable |
| **Encryption salt** | Your password manager | ❌ Data unrecoverable |
| **S3 credentials** | Cloud provider dashboard | Regenerate from provider |
| **S3 endpoint** | Cloud provider dashboard | Look up in provider docs |
| **rclone** | [rclone.org/install](https://rclone.org/install/) | Required for all recovery methods |

> **Critical**: If you lose both the encryption password AND salt, your data is permanently unrecoverable. No one can help — this is the nature of E2EE.

---

## Scenario 1: e2ee-sync Binary Lost or Corrupted

**Symptom**: e2ee-sync won't start, crashes, or was deleted.

**Fix**: Download the latest binary from [GitHub Releases](https://github.com/yuki0ueda/e2ee-sync/releases) and run `e2ee-sync setup` again with the same credentials. Your existing cloud data is untouched.

---

## Scenario 2: Computer Lost or Destroyed

**Symptom**: You have a new computer and need to restore your files.

**Option A — Another device has e2ee-sync running:**
```bash
# On the existing device
e2ee-sync share

# On the new device
e2ee-sync join <ip:port>
```

**Option B — No other device, but you have credentials:**
```bash
# Install rclone and Tailscale, then:
e2ee-sync setup
# Enter the same encryption password, salt, and S3 credentials
# Initial sync will download all files from cloud
```

**Option C — No e2ee-sync available, rclone only:**
```bash
# Create rclone.conf manually:
rclone config create cloud-direct s3 \
  provider Cloudflare \
  access_key_id YOUR_KEY \
  secret_access_key YOUR_SECRET \
  endpoint https://ACCOUNT.r2.cloudflarestorage.com

rclone config create cloud-crypt crypt \
  remote cloud-direct:e2ee-sync \
  password YOUR_ENC_PASSWORD \
  password2 YOUR_ENC_SALT

# Download all files:
rclone copy cloud-crypt: /path/to/local/restore/
```

---

## Scenario 3: e2ee-sync Project Abandoned

**Symptom**: GitHub repo archived, no more updates.

**Your data is safe.** It's standard rclone crypt on standard S3 — both are open formats that will outlive any single project.

```bash
# Access your files (works forever, no e2ee-sync needed):
rclone mount cloud-crypt: ~/restored-sync
# or
rclone copy cloud-crypt: ~/restored-sync
```

To continue syncing without e2ee-sync:
```bash
# Manual two-way sync:
rclone bisync ~/sync cloud-crypt: --checksum --resilient --recover
```

Set up a cron job or systemd timer to run periodically.

---

## Scenario 4: Cloud Provider Down or Migrating

**Symptom**: Cloudflare R2 is down, or you want to switch to Backblaze B2.

**Step 1 — Your local ~/sync has the latest files** (if daemon was running). No data loss.

**Step 2 — Set up new provider:**
```bash
# Create new S3 remote
rclone config create new-direct s3 \
  provider B2 \
  access_key_id YOUR_NEW_KEY \
  secret_access_key YOUR_NEW_SECRET \
  endpoint https://s3.us-west-004.backblazeb2.com

# Create new crypt layer (SAME encryption keys)
rclone config create new-crypt crypt \
  remote new-direct:e2ee-sync \
  password YOUR_ENC_PASSWORD \
  password2 YOUR_ENC_SALT

# Upload everything to new provider
rclone sync ~/sync new-crypt:
```

**Step 3 — Re-run setup:**
```bash
e2ee-sync setup
# Select new backend, enter new S3 credentials
# Use the SAME encryption password and salt
```

---

## Scenario 5: Sync Conflict or Data Corruption

**Symptom**: Files are wrong, corrupted, or missing after a sync.

**Check the trash first:**
```
~/sync/.trash/YYYY-MM-DD/
```
Deleted or overwritten files are kept for 30 days.

**If trash doesn't help — restore from cloud:**
```bash
# See what's in the cloud:
rclone ls cloud-crypt:

# Download a specific file:
rclone copy cloud-crypt:path/to/file.txt /local/restore/

# Download everything:
rclone copy cloud-crypt: /local/full-restore/
```

**If hub mode — check ZFS snapshots:**
```bash
# On the Proxmox host:
ls /rpool/data/rclone-encrypted/.zfs/snapshot/

# Restore a file from a snapshot:
cp /rpool/data/rclone-encrypted/.zfs/snapshot/auto-20260408-1400/FILENAME \
   /rpool/data/rclone-encrypted/
```

---

## Scenario 6: Encryption Password Lost

**If you lost only one of the two keys** (password or salt), the data is still unrecoverable. Both are required.

**If you have a working device**: The keys are stored in rclone.conf (obscured). Extract them:
```bash
rclone config show cloud-crypt
# Output shows:
#   password = your_password
#   password2 = your_salt
```

Save these immediately in your password manager.

**If no working device and no backup of keys**: The data is permanently lost. This is by design — no backdoor exists.

---

## Prevention Checklist

| Action | When | Protects against |
|--------|------|-----------------|
| Store encryption password + salt in password manager | After first setup | Key loss |
| Keep S3 credentials in password manager | After first setup | Provider re-auth |
| Enable S3 versioning (AWS S3 / B2) | After bucket creation | Accidental deletion |
| Set up e2ee-sync-hub with ZFS snapshots | If running Proxmox | Corruption, rollback |
| Run `e2ee-sync verify` monthly | Ongoing | Catch issues early |
| Test recovery on a spare machine annually | Ongoing | Verify you CAN recover |

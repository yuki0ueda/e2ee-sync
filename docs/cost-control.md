# Cost Control Guide

[Japanese / 日本語](cost-control.ja.md) | [Back to README](../README.md)

How to prevent unexpected cloud storage costs when using e2ee-sync.

## Layer 1: Cloud Provider Billing Alerts

Set up alerts on your cloud provider to get notified before costs get out of hand.

### Cloudflare R2

R2 has no egress fees, so the main cost is storage + API operations.

1. Dashboard → R2 → check usage regularly
2. R2 does not currently offer billing alerts — monitor manually
3. The free tier covers 10GB storage + 1M Class A operations/month

### AWS S3

AWS has the best billing alert system.

1. Go to **Billing → Budgets → Create Budget**
2. Set a monthly budget (e.g., $5)
3. Configure alerts at 50%, 80%, 100% thresholds
4. Add email notification

```
Example: $5/month budget
  → Alert at $2.50 (50%) — "you're on track"
  → Alert at $4.00 (80%) — "watch out"
  → Alert at $5.00 (100%) — "investigate"
```

### Backblaze B2

1. Go to **Account → Caps & Alerts**
2. Set daily caps for:
   - Download bandwidth
   - Transactions
3. B2 will stop serving requests when caps are hit (prevents runaway costs)

### IDrive e2

1. Check dashboard usage regularly
2. No built-in billing alerts — monitor manually

---

## Layer 2: e2ee-sync Transfer Limits

Add `max_transfer_per_sync` to your config to limit how much data a single sync operation can transfer.

**Edit** `~/.config/e2ee-sync/config.json`:

```yaml
max_transfer_per_sync: 1G
```

This tells rclone to stop transferring after 1GB per sync operation (using `--max-transfer` with `--cutoff-mode SOFT`). The sync completes what it can and reports a warning for the rest.

### Recommended values

| Use case | Limit | Rationale |
|----------|-------|-----------|
| Documents only | `500M` | Documents rarely exceed 500MB per sync |
| Documents + photos | `2G` | Photos can be large, but 2GB/sync is generous |
| Mixed use | `5G` | Safety net, catches accidental large file additions |
| No limit | *(leave blank)* | Full trust, no restrictions |

### What happens when the limit is hit?

- rclone transfers as much as it can within the limit (SOFT cutoff)
- Remaining files are synced in the next poll cycle (5 minutes later)
- A warning appears in the log: `max transfer limit reached`
- No data loss — just delayed transfer

---

## Layer 3: Filter Rules

Prevent large, unnecessary files from entering the sync directory.

**Edit** `~/.config/rclone/filter-rules.txt`:

```
# Already excluded by default:
- .DS_Store
- Thumbs.db
- *.tmp
- *.swp
- .trash/**

# Add your own exclusions:
- node_modules/**
- .git/**
- *.iso
- *.vmdk
- *.vdi
- *.ova
- *.dmg
- *.zip
- *.tar.gz
- __pycache__/**
- .venv/**
- target/**
```

After editing, restart the daemon:
```bash
# Linux
systemctl --user restart e2ee-sync.service

# Windows: stop and re-run from Task Scheduler

# macOS
launchctl unload ~/Library/LaunchAgents/com.e2ee-sync.plist
launchctl load ~/Library/LaunchAgents/com.e2ee-sync.plist
```

---

## Monthly Cost Estimates

For reference, here's what typical usage costs:

| Usage | Storage | API ops | Monthly cost (B2) | Monthly cost (R2) |
|-------|---------|---------|-------------------|-------------------|
| 10GB documents | 10GB | ~10K | **$0** (free tier) | **$0** (free tier) |
| 50GB docs + photos | 50GB | ~30K | **$0.30** | **$0.75** |
| 100GB mixed | 100GB | ~50K | **$0.60** | **$1.50** |

See [Cost Comparison Guide](cost-comparison.md) for detailed provider comparison.

---

## Signs of Runaway Costs

Watch for these in your `e2ee-sync.log`:

| Log message | Meaning | Action |
|-------------|---------|--------|
| `max transfer limit reached` | Transfer limit hit | Check if a large file was added |
| `Resync required` | Full re-upload triggered | Check if filter rules changed |
| `all files were changed` | Bulk change detected | Investigate before forcing |
| Repeated `Primary sync failed` | Sync looping on errors | Fix the underlying error |

---

## Quick Setup Checklist

1. ☐ Set cloud provider billing alert ($5/month for personal use)
2. ☐ Add `max_transfer_per_sync: 1G` to config.json
3. ☐ Review filter-rules.txt — exclude large file types you don't need
4. ☐ Check `e2ee-sync.log` monthly for anomalies

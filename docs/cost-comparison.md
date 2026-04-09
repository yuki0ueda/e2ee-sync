# Cost Comparison

[Japanese / 日本語](cost-comparison.ja.md) | [Back to README](../README.md)

A detailed comparison of cloud storage costs for e2ee-sync, covering S3-compatible providers, subscription services, and lifetime deals.

> Prices as of April 2026. Always verify current pricing on provider websites.

## S3-Compatible Storage (Pay-as-you-go)

These providers work directly with e2ee-sync via the S3 API.

| Provider | Storage/GB | Egress | API Calls | Minimum | Free Tier |
|----------|-----------|--------|-----------|---------|-----------|
| **IDrive e2** | **$0.004** | $0.01/GB | Free | None | 10GB |
| **Backblaze B2** | $0.006 | Free (3x storage) | Free | None | 10GB |
| **Wasabi** | $0.007 | Free (1:1 ratio) | Free | **1TB** | None |
| **Internxt S3** | €0.007 | Free | Free | None | None |
| **Cloudflare R2** | $0.015 | **Free (unlimited)** | $4.50/1M | None | 10GB |
| AWS S3 Standard | $0.023 | $0.09/GB | $5.00/1M | None | 5GB |

### Monthly Cost with e2ee-sync

| Storage | IDrive e2 | B2 | R2 | Internxt S3 | Wasabi |
|---------|----------|-----|-----|------------|--------|
| 10GB | **$0** | **$0** | **$0** | €0.07 | $6.99* |
| 100GB | **$0.40** | $0.60 | $1.50 | €0.70 | $6.99* |
| 500GB | **$2.00** | $3.00 | $7.50 | €3.50 | $6.99 |
| 1TB | **$4.00** | $6.00 | $15.00 | €7.00 | $6.99 |
| 5TB | **$20.00** | $30.00 | $75.00 | €35.00 | $34.95 |

\* Wasabi: 1TB minimum billing

**Recommendation**: IDrive e2 for lowest cost. Cloudflare R2 for free tier (≤10GB). Backblaze B2 for balanced cost and flexibility.

---

## Subscription Services Comparison

| Service | 100GB | 2TB | E2EE | Mobile | e2ee-sync compatible |
|---------|-------|-----|------|--------|---------------------|
| **e2ee-sync + IDrive e2** | **$0.40/mo** | **$8/mo** | ✅ User-managed keys | ❌ | — |
| **e2ee-sync + B2** | **$0.60/mo** | **$12/mo** | ✅ User-managed keys | ❌ | — |
| Filen Pro 1 | $1.67/mo (annual) | $7.50/mo | ✅ Zero-knowledge | ✅ | ❌ |
| Dropbox Plus | — | $9.99/mo (annual) | ❌ Server-side only | ✅ | ❌ |
| Internxt S3 | €0.70/mo | €14/mo | ✅ Zero-knowledge | ❌ | ✅ S3 API |

---

## Lifetime Deals (One-time Payment)

| Service | Storage | Price | E2EE | e2ee-sync compatible | Status |
|---------|---------|-------|------|---------------------|--------|
| **Koofr** | 1TB | $130 | ❌ | ❌ | ✅ Available |
| **Internxt** | 1TB | €195 | ✅ | ✅ S3 API | ✅ Available |
| **Internxt** | 3TB | €345 | ✅ | ✅ S3 API | ✅ Available |
| **Internxt** | 5TB | €495 | ✅ | ✅ S3 API | ✅ Available |
| **pCloud** | 2TB | $399 | ✅ (+$150) | ❌ | ✅ Available |
| **pCloud** | 10TB | $1,190 | ✅ (+$150) | ❌ | ✅ Available |
| **Icedrive** | 2TB | $389 | ✅ | ❌ | ✅ Available |
| ~~Filen~~ | — | — | — | — | ❌ Discontinued |

### Break-even: Lifetime vs Monthly (IDrive e2)

| Lifetime Deal | Price | Equivalent Monthly | Break-even |
|--------------|-------|-------------------|------------|
| Koofr 1TB | $130 | IDrive e2 1TB: $4/mo | **33 months (2.7 years)** |
| Internxt 3TB | €345 | IDrive e2 3TB: $12/mo | **29 months (2.4 years)** |
| Internxt 5TB | €495 | IDrive e2 5TB: $20/mo | **25 months (2.1 years)** |
| pCloud 2TB + Crypto | $549 | IDrive e2 2TB: $8/mo | **69 months (5.7 years)** |
| pCloud 10TB | $1,190 | IDrive e2 10TB: $40/mo | **30 months (2.5 years)** |

**Rule of thumb**: Lifetime deals pay off in 2-3 years for large storage. For small storage (≤100GB), monthly S3 is cheaper than any lifetime deal.

---

## Why e2ee-sync?

| | e2ee-sync | Dropbox / Filen / pCloud |
|---|----------|------------------------|
| **Encryption keys** | You generate and hold them. Never shared. | Service manages keys (even "zero-knowledge" requires trust). |
| **Storage provider** | Your choice: R2, S3, B2, IDrive e2, Internxt, etc. | Locked to one provider. |
| **Data location** | You choose the region and provider. | Provider decides. |
| **Vendor lock-in** | None. Switch providers anytime. Data is standard S3 objects. | Full lock-in. Migration required on exit. |
| **Service shutdown risk** | Your S3 bucket survives. Access data directly with rclone. | Service ends = scramble to export. |
| **Cost (10GB)** | **Free** (R2/B2/IDrive e2 free tier) | Free tiers limited (Dropbox 2GB) |
| **Cost (100GB)** | **$0.40/mo** (IDrive e2) | $1.67/mo+ (Filen annual) |
| **Open source** | MIT license. Full code audit possible. | Mostly closed source. |
| **Mobile app** | ❌ Not available | ✅ iOS/Android |
| **Web UI** | ❌ Not available | ✅ Browser access |
| **Setup effort** | CLI setup required (rclone + Tailscale) | Install and sign in |

**e2ee-sync is not the easiest or the cheapest for every scenario — it's the most transparent and free.** You own your encryption keys, choose your storage, and can leave anytime without losing access to your data.

---

## Quick Decision Guide

```
≤10GB?
  → e2ee-sync + R2 or B2 or IDrive e2 (FREE)

10-100GB?
  → e2ee-sync + IDrive e2 ($0.40/mo) or B2 ($0.60/mo)

100GB-1TB, using 3+ years?
  → Consider Internxt lifetime (€195-345) or Koofr ($130)
  → Otherwise: e2ee-sync + IDrive e2 ($4/mo for 1TB)

1TB+, using 3+ years?
  → Internxt 5TB lifetime (€495) breaks even in 2 years
  → Otherwise: e2ee-sync + IDrive e2 ($4/TB/mo)

Need mobile/web access?
  → Filen or Dropbox (e2ee-sync is desktop only)

Need full encryption control?
  → e2ee-sync (only option where YOU hold the keys)
```

# Cost Comparison

[Japanese / 日本語](cost-comparison.ja.md) | [Back to README](../README.md)

A detailed comparison of cloud storage costs for e2ee-sync, covering S3-compatible providers, subscription services, and lifetime deals.

> Prices as of April 2026. Always verify current pricing on provider websites.

## S3-Compatible Storage (Pay-as-you-go)

These providers work directly with e2ee-sync via the S3 API.

| Provider | Storage/GB | Egress | API Calls | Minimum | Free Tier |
|----------|-----------|--------|-----------|---------|-----------|
| **IDrive e2** | **$0.005** | Free (3x storage) | Free | **1TB** | 10GB |
| **Backblaze B2** | $0.006¹ | Free (3x storage) | Free² | None | 10GB |
| **Wasabi** | $0.007 | Free (1:1 ratio) | Free | **1TB** | None |
| **Internxt S3** | €0.007 | Free | Free | **1TB** | None |
| **Cloudflare R2** | $0.015 | **Free (unlimited)** | $4.50/1M | None | 10GB |
| AWS S3 Standard | $0.023 | $0.09/GB | $5.00/1M | None | 5GB (12mo) |

¹ Backblaze B2 raises standard storage from $6/TB to $6.95/TB effective May 1, 2026.
² Backblaze B2 makes all standard API calls free for every customer effective May 1, 2026 (previously limited free quota).
IDrive e2 free egress and Wasabi/B2 egress allowances apply up to 3× (or 1:1 for Wasabi) of stored volume; excess is billed at $0.01/GB. Wasabi additionally enforces a 90-day minimum storage duration. Internxt S3 also enforces a 30-day minimum storage duration.

### Monthly Cost with e2ee-sync

| Storage | IDrive e2 | B2 | R2 | Internxt S3 | Wasabi |
|---------|----------|-----|-----|------------|--------|
| 10GB | **$0** | **$0** | **$0** | €7.00* | $6.99* |
| 100GB | $5.00* | **$0.60** | $1.50 | €7.00* | $6.99* |
| 500GB | $5.00* | **$3.00** | $7.50 | €7.00* | $6.99* |
| 1TB | **$5.00** | $6.00 | $15.00 | €7.00 | $6.99 |
| 5TB | **$25.00** | $30.00 | $75.00 | €35.00 | $34.95 |

\* Minimum billing applies: IDrive e2 and Internxt S3 bill for 1TB even when you store less (after the free tier is exhausted). Wasabi also enforces 1TB minimum billing.

**Recommendation**: For ≤10GB, any of R2 / B2 / IDrive e2 are free. For 10GB–1TB, **Backblaze B2** is cheapest due to no minimum. At 1TB+, **IDrive e2** edges out B2 on per-TB pricing. Use **Cloudflare R2** when egress-heavy workloads dominate.

---

## Subscription Services Comparison

| Service | 100GB | 2TB | E2EE | Mobile | e2ee-sync compatible |
|---------|-------|-----|------|--------|---------------------|
| **e2ee-sync + IDrive e2** | $5/mo* | **$10/mo** | ✅ User-managed keys | ❌ | — |
| **e2ee-sync + B2** | **$0.60/mo** | **$12/mo** | ✅ User-managed keys | ❌ | — |
| Dropbox Plus | — | $9.99/mo (annual) | ❌ Server-side only | ✅ | ❌ |
| Internxt S3 | €7/mo* | €14/mo | ✅ Zero-knowledge | ❌ | ✅ S3 API |

\* 1TB minimum billing after free tier.

---

## Lifetime Deals (One-time Payment)

| Service | Storage | List Price | E2EE | e2ee-sync compatible | Status |
|---------|---------|-----------|------|---------------------|--------|
| **Koofr** | 1TB | $130 | ❌ | ❌ | ✅ Available |
| **Internxt** | 1TB | €195 | ✅ | ✅ S3 API | ✅ Available |
| **Internxt** | 3TB | €345 | ✅ | ✅ S3 API | ✅ Available |
| **Internxt** | 5TB | €495 | ✅ | ✅ S3 API | ✅ Available |
| **pCloud** | 2TB | $399 | ✅ (+$150) | ❌ | ✅ Available |
| **pCloud** | 10TB | $1,190 | ✅ (+$150) | ❌ | ✅ Available |
| **Icedrive** | 2TB | $389 | ✅ | ❌ | ✅ Available |
| ~~Filen~~ | — | — | — | — | ❌ Pro Lifetime discontinued |

> **Discount note**: Internxt runs aggressive promotions year-round (via its own deals page and resellers such as StackSocial), often reducing lifetime prices by 70–90% from list. pCloud likewise discounts frequently — typical sale prices are around **$279 (2TB)**, **$799 (10TB)**, and **$115 for lifetime Crypto**. Always check the provider's current offer before purchasing.

### Break-even: Lifetime vs Monthly (IDrive e2)

IDrive e2 pay-as-you-go: $5/TB/month with a 1TB minimum.

| Lifetime Deal | List Price | Equivalent Monthly | Break-even |
|--------------|-----------|-------------------|------------|
| Koofr 1TB | $130 | IDrive e2 1TB: $5/mo | **26 months (2.2 years)** |
| Internxt 3TB | €345 | IDrive e2 3TB: $15/mo | **23 months (1.9 years)** |
| Internxt 5TB | €495 | IDrive e2 5TB: $25/mo | **20 months (1.7 years)** |
| pCloud 2TB + Crypto | $549 | IDrive e2 2TB: $10/mo | **55 months (4.6 years)** |
| pCloud 10TB | $1,190 | IDrive e2 10TB: $50/mo | **24 months (2.0 years)** |

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
| **Cost (100GB)** | **$0.60/mo** (Backblaze B2) | $1.99/mo+ (Filen Pro I, 200GiB) |
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

10GB-1TB?
  → e2ee-sync + Backblaze B2 (no minimum, $0.006/GB)
  → Avoid IDrive e2 and Internxt S3 at this range (1TB minimum billing)

~1TB, using 2+ years?
  → Consider Koofr lifetime ($130, no E2EE) or Internxt lifetime (often on sale)
  → Otherwise: e2ee-sync + IDrive e2 ($5/mo for 1TB)

1TB+, using 2+ years?
  → Internxt 5TB lifetime (list €495, typically deeply discounted) breaks even in <2 years
  → Otherwise: e2ee-sync + IDrive e2 ($5/TB/mo)

Need mobile/web access?
  → Dropbox, Filen, pCloud, etc. (e2ee-sync is desktop only)

Need full encryption control?
  → e2ee-sync (only option where YOU hold the keys)
```

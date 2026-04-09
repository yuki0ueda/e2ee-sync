# コスト管理ガイド

[English](cost-control.md) | [READMEに戻る](../README.ja.md)

e2ee-sync を使う際に予期しないクラウドストレージ費用を防ぐ方法。

## 層1: クラウドプロバイダーの課金アラート

クラウドプロバイダー側でアラートを設定し、コストが膨らむ前に通知を受け取る。

### Cloudflare R2

R2 は Egress 無料なので、主なコストはストレージ + API 操作。

1. Dashboard → R2 → 使用量を定期確認
2. R2 には現在課金アラート機能がない — 手動で監視
3. 無料枠: 10GB ストレージ + 100万 Class A 操作/月

### AWS S3

AWS の課金アラートが最も充実。

1. **Billing → Budgets → Create Budget** に移動
2. 月額予算を設定（例: $5）
3. 50%、80%、100% でアラートを設定
4. メール通知を追加

```
例: $5/月の予算
  → $2.50 でアラート（50%）— 「順調」
  → $4.00 でアラート（80%）— 「注意」
  → $5.00 でアラート（100%）— 「要調査」
```

### Backblaze B2

1. **Account → Caps & Alerts** に移動
2. 日次上限を設定:
   - ダウンロード帯域幅
   - トランザクション数
3. 上限に達すると B2 がリクエストを停止（暴走防止）

### IDrive e2

1. ダッシュボードで使用量を定期確認
2. 課金アラート機能なし — 手動監視

---

## 層2: e2ee-sync の転送量制限

`max_transfer_per_sync` を設定して、1回の同期で転送できるデータ量を制限。

**編集:** `~/.config/e2ee-sync/config.json`:

```yaml
max_transfer_per_sync: 1G
```

rclone に `--max-transfer`（`--cutoff-mode SOFT`）を渡し、1回の同期で 1GB を超えたら転送を停止します。

### 推奨値

| 用途 | 制限値 | 理由 |
|------|-------|------|
| ドキュメントのみ | `500M` | 1回の同期で500MBを超えることは稀 |
| ドキュメント + 写真 | `2G` | 写真は大きいが2GB/回で十分 |
| 混在利用 | `5G` | 安全ネット。うっかり大ファイル追加を検出 |
| 制限なし | *（空欄）* | 完全信頼、制限なし |

### 制限に達したらどうなる？

- rclone は制限内で転送できる分だけ転送（SOFT カットオフ）
- 残りは次のポーリング（5分後）で転送される
- ログに警告: `max transfer limit reached`
- データ消失なし — 転送が遅延するだけ

---

## 層3: フィルタルール

大きな不要ファイルが同期ディレクトリに入るのを防ぐ。

**編集:** `~/.config/rclone/filter-rules.txt`:

```
# デフォルトで除外済み:
- .DS_Store
- Thumbs.db
- *.tmp
- *.swp
- .trash/**

# 独自の除外ルールを追加:
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

編集後、デーモンを再起動:
```bash
# Linux
systemctl --user restart e2ee-sync.service

# Windows: タスクスケジューラから停止→再実行

# macOS
launchctl unload ~/Library/LaunchAgents/com.e2ee-sync.plist
launchctl load ~/Library/LaunchAgents/com.e2ee-sync.plist
```

---

## 月額コスト目安

| 利用量 | ストレージ | API 操作 | 月額（B2） | 月額（R2） |
|-------|----------|---------|-----------|-----------|
| 10GB ドキュメント | 10GB | ~10K | **$0**（無料枠） | **$0**（無料枠） |
| 50GB ドキュメント+写真 | 50GB | ~30K | **$0.30** | **$0.75** |
| 100GB 混在 | 100GB | ~50K | **$0.60** | **$1.50** |

詳細は [コスト比較ガイド](cost-comparison.ja.md) を参照。

---

## コスト暴走のサイン

`e2ee-sync.log` で以下に注意:

| ログメッセージ | 意味 | 対処 |
|-------------|------|------|
| `max transfer limit reached` | 転送制限に到達 | 大きなファイルが追加されていないか確認 |
| `Resync required` | 全体再アップロードが発生 | フィルタルールの変更がないか確認 |
| `all files were changed` | 大量変更を検出 | force する前に調査 |
| 繰り返す `Primary sync failed` | エラーでリトライループ | 根本原因を修正 |

---

## セットアップチェックリスト

1. ☐ クラウドプロバイダーの課金アラート設定（個人利用なら $5/月）
2. ☐ config.json に `max_transfer_per_sync: 1G` を追加
3. ☐ filter-rules.txt を確認 — 不要な大ファイルを除外
4. ☐ 月1回 `e2ee-sync.log` を確認して異常がないかチェック

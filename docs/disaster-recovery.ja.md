# 障害復旧ガイド

[English](disaster-recovery.md) | [READMEに戻る](../README.ja.md)

問題が起きたときのデータ復旧方法。e2ee-sync は標準的な rclone crypt + 標準的な S3 オブジェクトを使用しているため、e2ee-sync なしでもデータには常にアクセス可能です。

## 復旧に必要なもの

| 項目 | 保管場所 | なくした場合 |
|------|---------|------------|
| **暗号化パスワード** | パスワードマネージャー | ❌ データ復旧不可 |
| **暗号化ソルト** | パスワードマネージャー | ❌ データ復旧不可 |
| **S3 クレデンシャル** | クラウドプロバイダーのダッシュボード | プロバイダーで再発行 |
| **S3 エンドポイント** | クラウドプロバイダーのダッシュボード | プロバイダーのドキュメントで確認 |
| **rclone** | [rclone.org/install](https://rclone.org/install/) | 全復旧方法に必要 |

> **重要**: 暗号化パスワードとソルトの両方を紛失すると、データは永久に復旧できません。誰にも助けられません — これが E2EE の本質です。

---

## シナリオ 1: e2ee-sync バイナリの紛失・破損

**症状**: e2ee-sync が起動しない、クラッシュする、削除された。

**対処**: [GitHub Releases](https://github.com/yuki0ueda/e2ee-sync/releases) から最新バイナリをダウンロードし、同じクレデンシャルで `e2ee-sync setup` を再実行。クラウド上のデータは影響なし。

---

## シナリオ 2: PC の紛失・故障

**症状**: 新しい PC にファイルを復元したい。

**方法 A — 別のデバイスで e2ee-sync が動いている場合:**
```bash
# 既存デバイスで
e2ee-sync share

# 新しいデバイスで
e2ee-sync join <ip:port>
```

**方法 B — 他のデバイスなし、クレデンシャルはある場合:**
```bash
# rclone と Tailscale をインストールして:
e2ee-sync setup
# 同じ暗号化パスワード、ソルト、S3 クレデンシャルを入力
# 初回同期でクラウドから全ファイルがダウンロードされる
```

**方法 C — e2ee-sync なし、rclone のみで復旧:**
```bash
# rclone.conf を手動作成:
rclone config create cloud-direct s3 \
  provider Cloudflare \
  access_key_id YOUR_KEY \
  secret_access_key YOUR_SECRET \
  endpoint https://ACCOUNT.r2.cloudflarestorage.com

rclone config create cloud-crypt crypt \
  remote cloud-direct:e2ee-sync \
  password YOUR_ENC_PASSWORD \
  password2 YOUR_ENC_SALT

# 全ファイルをダウンロード:
rclone copy cloud-crypt: /path/to/local/restore/
```

---

## シナリオ 3: e2ee-sync プロジェクトの放棄

**症状**: GitHub リポジトリがアーカイブ、更新停止。

**データは安全です。** 標準的な rclone crypt + 標準的な S3 — どちらもオープンフォーマットで、単一プロジェクトより長く存続します。

```bash
# ファイルにアクセス（e2ee-sync 不要、永久に動作）:
rclone mount cloud-crypt: ~/restored-sync
# または
rclone copy cloud-crypt: ~/restored-sync
```

e2ee-sync なしで同期を続ける方法:
```bash
# 手動の双方向同期:
rclone bisync ~/sync cloud-crypt: --checksum --resilient --recover
```

cron ジョブや systemd タイマーで定期実行すれば自動化可能。

---

## シナリオ 4: クラウドプロバイダーの障害・移行

**症状**: R2 がダウン、または B2 に移行したい。

**ステップ 1 — ローカルの ~/sync に最新ファイルがある**（デーモン稼働中なら）。データ消失なし。

**ステップ 2 — 新プロバイダーをセットアップ:**
```bash
# 新しい S3 リモートを作成
rclone config create new-direct s3 \
  provider B2 \
  access_key_id YOUR_NEW_KEY \
  secret_access_key YOUR_NEW_SECRET \
  endpoint https://s3.us-west-004.backblazeb2.com

# 新しい crypt レイヤー（同じ暗号化キーを使用）
rclone config create new-crypt crypt \
  remote new-direct:e2ee-sync \
  password YOUR_ENC_PASSWORD \
  password2 YOUR_ENC_SALT

# 全ファイルを新プロバイダーにアップロード
rclone sync ~/sync new-crypt:
```

**ステップ 3 — setup を再実行:**
```bash
e2ee-sync setup
# 新しいバックエンドを選択、新しい S3 クレデンシャルを入力
# 暗号化パスワードとソルトは同じものを使用
```

---

## シナリオ 5: 同期の競合・データ破損

**症状**: 同期後にファイルが壊れた、消えた、おかしい。

**まずゴミ箱を確認:**
```
~/sync/.trash/YYYY-MM-DD/
```
削除・上書きされたファイルは30日間保持されます。

**ゴミ箱にない場合 — クラウドから復元:**
```bash
# クラウドの中身を確認:
rclone ls cloud-crypt:

# 特定のファイルをダウンロード:
rclone copy cloud-crypt:path/to/file.txt /local/restore/

# 全ファイルをダウンロード:
rclone copy cloud-crypt: /local/full-restore/
```

**hub モードの場合 — ZFS スナップショットを確認:**
```bash
# Proxmox ホストで:
ls /rpool/data/rclone-encrypted/.zfs/snapshot/

# スナップショットからファイルを復元:
cp /rpool/data/rclone-encrypted/.zfs/snapshot/auto-20260408-1400/FILENAME \
   /rpool/data/rclone-encrypted/
```

---

## シナリオ 6: 暗号化パスワードの紛失

**2つの鍵のうち1つだけ紛失した場合**（パスワードまたはソルト）でも、データは復旧できません。両方必要です。

**動作中のデバイスがある場合**: 鍵は rclone.conf に保存されています（obfuscated）。抽出方法:
```bash
rclone config show cloud-crypt
# 出力:
#   password = your_password
#   password2 = your_salt
```

すぐにパスワードマネージャーに保存してください。

**動作中のデバイスも鍵のバックアップもない場合**: データは永久に失われます。これは設計通りです — バックドアは存在しません。

---

## 予防チェックリスト

| アクション | タイミング | 防御対象 |
|-----------|----------|---------|
| 暗号化パスワード + ソルトをパスワードマネージャーに保存 | 初回セットアップ後 | 鍵の紛失 |
| S3 クレデンシャルをパスワードマネージャーに保存 | 初回セットアップ後 | プロバイダー再認証 |
| S3 バージョニングを有効化（AWS S3 / B2） | バケット作成後 | 誤削除 |
| e2ee-sync-hub + ZFS スナップショットを構築 | Proxmox 運用時 | データ破損、ロールバック |
| 月1回 `e2ee-sync verify` を実行 | 定期 | 問題の早期発見 |
| 年1回スペアマシンでリカバリテスト | 定期 | 復旧できることの確認 |

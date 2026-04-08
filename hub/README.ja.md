# e2ee-sync: Hub セットアップ（Proxmox LXC）

[English](README.md) | [メインREADMEに戻る](../README.ja.md)

[e2ee-sync](https://github.com/yuki0ueda/e2ee-sync) の**オプション**コンポーネント — Proxmox LXC ハブのセットアップガイド。

> **hub は必須ではありません。** デバイスは hub なしでも Cloudflare R2 に直接同期できます。
> hub を設置すると、Tailscale経由の高速直接同期・ZFSスナップショット・R2 APIコスト削減が利用可能になります。
>
> **対象**: Proxmox VE を所有し、専用 LXC を Hub として運用する上級者向け構成。

---

## アーキテクチャ

```
[デバイス群 - Win/Mac/Linux]
  ├── e2ee-sync（初回セットアップ CLI）
  ├── autosync（デーモン: ファイル監視 + bisync + フェイルオーバー）
  ├── rclone crypt（クライアント側で暗号化/復号）
  └── Tailscale
         │
         │ bisync (WebDAV over Tailscale)
         │ ※Hub 到達不可時 → R2 に自動フォールバック
         ▼
[Proxmox LXC - e2ee-sync-hub]
  ├── rclone serve webdav（暗号化 blob を受け入れ）
  ├── rclone sync → R2（定期オフサイトバックアップ）
  ├── 起動時 R2 キャッチアップ（障害復旧）
  ├── ZFS スナップショット（1時間ごと・72世代保持）
  └── Tailscale
         │
         │ sync（暗号化 blob をそのままコピー）
         ▼
[Cloudflare R2]
  └── 暗号化 blob 保管（オフサイトバックアップ）
```

### セキュリティモデル

```
                暗号鍵を持つか？  データを見えるか？
────────────────────────────────────────────
デバイス              ○                ○（平文）
Proxmox Hub          ✗                ✗（暗号化 blob）
Cloudflare R2        ✗                ✗（暗号化 blob）
Tailscale            ✗                ✗（経路暗号化のみ）
```

---

## はじめる前に

[メインREADME](../README.ja.md#はじめに) の以下の手順を先に完了してください:

1. **Cloudflare R2 のセットアップ** — `e2ee-sync` バケットと API トークンの作成
2. **パスワードの準備** — WebDAV パスワード、暗号化パスワード、ソルト

## 前提条件

| コンポーネント | バージョン | 用途 |
|---|---|---|
| Proxmox VE | 8.x | LXC ホスト |
| rclone | 1.71.0+ | bisync stable 版が必要 |
| Tailscale | 最新 | デバイス間ネットワーク |

---

## 1. Proxmox LXC のセットアップ

### 1.1 ZFS データセット作成

Proxmox ホスト上で実行:

```bash
zfs create rpool/data/rclone-encrypted
```

> **重要**: unprivileged LXC からアクセスするため、所有者を UID 100000 に変更する。
> これを忘れると、コンテナ内の rclone が書き込めず、VFS メモリ上にデータが滞留してディスクに永続化されない。

```bash
chown -R 100000:100000 /rpool/data/rclone-encrypted/
```

### 1.2 LXC コンテナ作成

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

`<VMID>` は環境に合わせて変更する（例: 203）。

### 1.3 ストレージマウントと TUN デバイス

```bash
# データ領域のバインドマウント
pct set <VMID> -mp0 /rpool/data/rclone-encrypted,mp=/data/encrypted

# Tailscale 用 TUN デバイス
pct set <VMID> -dev0 /dev/net/tun
```

### 1.4 コンテナ内の基本セットアップ

> **注意**: Proxmox ホストで Tailscale を使用している場合、LXC の `/etc/resolv.conf` に
> MagicDNS（`100.100.100.100`）が設定される。コンテナ内に Tailscale がまだないため、
> DNS が解決できず `apt` が極端に遅くなる。先に一時的なパブリック DNS を設定する。

```bash
pct enter <VMID>

# 一時的にパブリック DNS に切替
echo "nameserver 1.1.1.1" > /etc/resolv.conf
echo "nameserver 8.8.8.8" >> /etc/resolv.conf

# パッケージインストール
apt update && apt install -y curl unzip

# rclone
curl https://rclone.org/install.sh | bash

# Tailscale
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up --hostname=e2ee-sync-hub
```

Tailscale 接続後、コンテナを再起動して DNS を復元:

```bash
exit
pct reboot <VMID>
```

### 1.5 自動セットアップ（推奨）

`hub/` ディレクトリを LXC にコピーしてスクリプトを実行:

```bash
# 手元のマシンから（Tailscale経由）
scp -r hub/ root@e2ee-sync-hub:/root/

# LXC 内で実行
cd /root/hub
bash setup.sh
```

スクリプトが対話的に以下を実行:
1. 前提条件の確認（rclone, Tailscale, /data/encrypted）
2. R2 クレデンシャルと WebDAV パスワードの入力
3. rclone.conf 生成
4. systemd サービスのインストールと有効化

### 1.6 手動セットアップ（代替）

手動で設定する場合は [`hub/systemd/`](systemd/) のユニットテンプレートを参照し、rclone.conf を手動で作成:

```bash
mkdir -p ~/.config/rclone
cat > ~/.config/rclone/rclone.conf << 'EOF'
[local-encrypted]
type = alias
remote = /data/encrypted

[cloud-raw]
type = s3
provider = Cloudflare
access_key_id = YOUR_R2_ACCESS_KEY
secret_access_key = YOUR_R2_SECRET_KEY
endpoint = https://ACCOUNT_ID.r2.cloudflarestorage.com
region = auto
acl = private
EOF
```

サービスの起動順序:

```
tailscaled
  → rclone-catchup（R2 → ローカルへ差分取込）
    → rclone-webdav（キャッチアップ完了後に WebDAV 開始）
    → rclone-r2sync.timer（15分ごとに R2 へ sync）
```

### 1.7 ZFS スナップショット（追加の安全策）

Proxmox ホスト上の cron に追加:

```bash
crontab -e
```

```cron
# 1時間ごとにスナップショット
0 * * * * zfs snapshot rpool/data/rclone-encrypted@auto-$(date +\%Y\%m\%d-\%H\%M)
# 72世代を超えた古いスナップショットを削除
5 * * * * zfs list -t snapshot -o name -s creation rpool/data/rclone-encrypted | grep @auto- | head -n -72 | xargs -r -n1 zfs destroy
```

---

## 2. デバイス側のセットアップ（Client）

`e2ee-sync` CLI を使用してデバイスを設定:

```bash
# GitHub Releases からダウンロード
# https://github.com/yuki0ueda/e2ee-sync/releases

e2ee-sync setup
```

CLI が自動的に rclone.conf 生成、フィルタルール配置、初回 bisync、デーモン登録を実行します。

詳細は [メインREADME](../README.ja.md) を参照。

---

## 3. 動作確認

### E2EE の確認

```bash
# デバイス側: テストファイル作成
echo "hello from device" > ~/sync/test.txt

# bisync 実行
rclone bisync ~/sync/ hub-crypt: --checksum --verbose

# Hub 側 (LXC 内): 暗号化 blob のみが見える
ls /data/encrypted/
# → r2qnlbbu7l856rsibqrg291qao 等のランダム文字列
```

### フェイルオーバーテスト

```bash
# 1. Hub の WebDAV を停止
ssh e2ee-sync-hub "systemctl stop rclone-webdav"

# 2. R2 経由で同期
echo "failover test" > ~/sync/failover.txt
rclone bisync ~/sync/ cloud-crypt: --checksum --verbose

# 3. Hub 再開
ssh e2ee-sync-hub "systemctl start rclone-webdav"

# 4. キャッチアップ実行
ssh e2ee-sync-hub "systemctl start rclone-catchup"
```

---

## 4. トラブルシューティング

### apt が極端に遅い / タイムアウトする

**原因**: Proxmox ホストの Tailscale MagicDNS（`100.100.100.100`）が LXC に継承されるが、コンテナ内にはまだ Tailscale がないため DNS が解決できない。

**対処**: セクション 1.4 の手順で一時的にパブリック DNS を設定し、Tailscale インストール後にコンテナを再起動。

### WebDAV 認証失敗（401 Unauthorized）

**原因**: パスワードの特殊文字がシェルや `rclone obscure` で意図しない解釈をされている。

**対処**:
1. Hub 側の `--pass` とデバイス側の obscure 元の平文が一致しているか確認
2. 英数字のみのパスワードに変更する
3. `rclone lsd hub-webdav:` で認証を確認

### bisync が成功するのに Hub のディスクにファイルがない

**原因**: unprivileged LXC の UID マッピング問題。コンテナ内の root（UID 0）はホスト側では UID 100000 にマッピングされる。

**対処**: Proxmox ホスト上で:

```bash
chown -R 100000:100000 /rpool/data/rclone-encrypted/
```

### bisync で modtime WARNING が出る

```
WARNING: Modtime compare was requested but at least one remote does not support it.
```

**原因**: WebDAV は modtime の比較をサポートしていない。

**対処**: `--checksum` を付けて実行する。autosync はデフォルトでこのオプションを使用する。

```bash
rclone bisync ~/sync/ hub-crypt: --checksum --verbose
```

### ZFS スナップショットからの復元

```bash
# スナップショット一覧
zfs list -t snapshot rpool/data/rclone-encrypted

# 特定ファイルだけ復元
cp /rpool/data/rclone-encrypted/.zfs/snapshot/auto-20260307-1400/FILENAME /rpool/data/rclone-encrypted/

# データセット全体をロールバック
zfs rollback rpool/data/rclone-encrypted@auto-20260307-1400
```

---

## 5. 運用リファレンス

### サービス管理コマンド

```bash
# LXC 内
systemctl status rclone-webdav.service    # WebDAV の状態確認
systemctl start rclone-r2sync.service     # R2 sync を手動実行
systemctl start rclone-catchup.service    # キャッチアップを手動実行
systemctl list-timers                     # タイマー確認
journalctl -u rclone-webdav.service -f    # WebDAV ログ
cat /var/log/rclone-r2sync.log            # R2 sync ログ
cat /var/log/rclone-catchup.log           # キャッチアップログ
```

### 新しいデバイスの追加

1. rclone と Tailscale をインストール
2. `e2ee-sync setup` を実行 — **同じ暗号化パスワード・ソルト**を使用すること
3. 詳細は [メインREADME](../README.ja.md) を参照

### 暗号化パスワードの確認

```bash
# 現在の rclone 設定を表示（obscure 化されたパスワード含む）
rclone config show
```

### R2 コスト概算

| 項目 | 単価 | 目安 |
|------|------|------|
| ストレージ | $0.015/GB/月 | 100GB → $1.50/月 |
| Class A (PUT/POST) | $4.50/100万回 | 15分 sync → 月 $0.5 以下 |
| Class B (GET/LIST) | $0.36/100万回 | ほぼ無視 |
| Egress | 無料 | $0 |
| **合計** | | **100GB 利用時: 約 $2/月** |

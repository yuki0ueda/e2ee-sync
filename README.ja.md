# e2ee-sync

[English](README.md)

エンドツーエンド暗号化ファイル同期のセットアップツール。

[rclone](https://rclone.org/) bisync によるクライアントサイド暗号化同期を複数デバイス間で構成します。[Tailscale](https://tailscale.com/) によるセキュアなLAN接続と、Cloudflare R2 によるクラウドフォールバックを組み合わせたアーキテクチャです。

## アーキテクチャ

```
                      ┌──────────────────────────────┐
デバイスA ──┐          │  e2ee-sync-hub（オプション）   │
             ├─ Tailscale ─┤  WebDAV + R2 バックアップ     │
デバイスB ──┘          └──────────────────────────────┘
  │                              │
  │  R2 直接（hub停止時          │  定期 sync
  │  またはhubなし構成）         │
  ▼                              ▼
┌──────────────────────────────────┐
│  Cloudflare R2（暗号化 blob）     │
└──────────────────────────────────┘
```

- **hubあり**: LAN経由の高速同期 + hubがR2バックアップ担当 + ZFSスナップショットで世代管理
- **hubなし**: デバイスがCloudflare R2に直接同期 — 低速だが完全に動作
- **暗号化**: rclone crypt（ファイル名・ディレクトリ名暗号化、クライアント側のみ）

## 同期ディレクトリ

`~/sync`（Windows: `%USERPROFILE%\sync`）内のファイルが全デバイス間で双方向同期されます。ファイルはデバイス上で暗号化されてから送信され、hub や R2 には暗号化 blob のみが保存されます。除外パターン（`.DS_Store`, `*.tmp`, `node_modules/` 等）は `filter-rules.txt` で設定できます。

## 前提条件

- [rclone](https://rclone.org/install/) 1.71.0+ がインストール済みでPATHに存在すること
- [Tailscale](https://tailscale.com/download) がインストール済みでtailnetに接続済みであること
- Cloudflare R2 バケット（必須）
- Tailscale経由で `e2ee-sync-hub` に到達可能であること（オプション — LAN高速同期を有効化）

## はじめに

### 1. Cloudflare R2 のセットアップ

Cloudflare Dashboard でバケットと API トークンを作成します。

**バケット作成**: R2 → Create Bucket

```
バケット名: e2ee-sync
リージョン: Automatic（または APAC）
```

**API トークン作成**: R2 → Manage R2 API Tokens → Create API Token

```
権限: Object Read & Write
バケット指定: e2ee-sync
```

以下の値を控えておく（デバイスセットアップ時に必要）:

```
Access Key ID: xxxxxxxxxxxxxxxx
Secret Access Key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Endpoint: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

### 2. パスワードの準備

以下の3つのパスワードを用意する。**英数字のみ**を推奨（シェルエスケープ問題を回避）。

| パスワード | 用途 | 全デバイス共通？ |
|-----------|------|----------------|
| WebDAV パスワード | Hub との認証（hubなしの場合は不要） | ○ |
| 暗号化パスワード | ファイル内容の暗号化（rclone crypt `password`） | ○ |
| ソルト | ファイル名の暗号化（rclone crypt `password2`） | ○ |

> **暗号化パスワードとソルトを紛失するとデータ復旧不可。**
> 生成後すぐにパスワードマネージャ（Bitwarden 等）に保管すること。

```bash
# 英数字32文字のランダム生成（Linux / macOS）
cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 32; echo
```

```powershell
# Windows PowerShell
-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | ForEach-Object {[char]$_})
```

### 3.（オプション）Hub セットアップ

hub は**必須ではありません** — デバイスは Cloudflare R2 に直接同期できます。ただし、専用の Proxmox LXC hub を設置すると:

- **高速同期** — インターネット往復ではなくLAN経由
- **ZFS スナップショット** — ポイントインタイムリカバリ
- **R2 コスト削減** — 各デバイスが個別にR2同期する代わりにhubが一括処理

セットアップ手順は [`hub/README.ja.md`](hub/README.ja.md) を参照。

### 4. デバイスセットアップ

[GitHub Releases](https://github.com/yuki0ueda/e2ee-sync/releases) からお使いの OS 用の `e2ee-sync` をダウンロードして実行:

```bash
e2ee-sync setup
```

対話形式で以下を実行します:

1. 前提条件の確認（rclone, Tailscale, hub接続性）
2. クレデンシャル入力（WebDAV, 暗号化キー, R2キー）
3. rclone.conf 生成（パスワードはobscure化）
4. フィルタルール・同期ディレクトリの作成
5. 接続テストと初回bisync
6. デーモンの配置と登録

セットアップが `e2ee-sync` を適切な場所にコピーし、デーモンとして登録します:

| OS | 配置先 | デーモン方式 |
|----|-------|-------------|
| Linux | `~/.local/bin/e2ee-sync` | systemd user service |
| macOS | `/usr/local/bin/e2ee-sync` | LaunchAgent |
| Windows | `%USERPROFILE%\.local\bin\e2ee-sync.exe` | タスクスケジューラ（`register-daemon.bat` 経由） |

**Windows の場合**: デーモン登録には管理者権限が必要です。セットアップが `register-daemon.bat` を生成するので、右クリック→「管理者として実行」でデーモンを登録してください。デーモンはコンソール窓なしのバックグラウンドプロセスとして動作します。

アップグレード時は、新バージョンをダウンロードして `e2ee-sync upgrade` を実行してください。

### その他のコマンド

```bash
e2ee-sync verify      # 既存設定の検証
e2ee-sync upgrade     # バイナリ更新
e2ee-sync uninstall   # デーモン解除・設定削除
e2ee-sync version     # バージョン表示
```

引数なしで起動すると対話メニューが表示されます。

## 対応プラットフォーム

| OS | デーモン方式 | ダウンロード |
|----|------------|------------|
| Linux | systemd user service | `e2ee-sync-linux-x64` / `e2ee-sync-linux-arm64` |
| macOS | LaunchAgent | `e2ee-sync-mac-x64` / `e2ee-sync-mac-arm64` |
| Windows | タスクスケジューラ（`register-daemon.bat`） | `e2ee-sync-win-x64.exe` / `e2ee-sync-win-arm64.exe` |

## ソースからビルド

```bash
git clone https://github.com/yuki0ueda/e2ee-sync.git
cd e2ee-sync

# 現在のプラットフォーム向けにビルド
make build

# 全プラットフォーム向けクロスコンパイル（6 OS/arch x 2 バイナリ）
make build-all
```

Go 1.25 以上が必要です。

## プロジェクト構成

```
e2ee-sync/
├── cmd/
│   └── e2ee-sync/   # 単一バイナリ: setup + daemon + verify + upgrade
├── internal/
│   ├── platform/    # OS別実装
│   ├── credential/  # 対話的クレデンシャル入力
│   ├── template/    # rclone.conf / config テンプレート
│   ├── rclone/      # rclone CLI ラッパー
│   └── version/     # ビルド時バージョン情報
├── hub/             # Proxmox LXC ハブセットアップ
│   ├── systemd/     # systemd ユニットテンプレート
│   └── setup.sh     # ハブ自動セットアップスクリプト
└── Makefile
```

## ライセンス

[MIT](LICENSE)

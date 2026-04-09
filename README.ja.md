# e2ee-sync

[English](README.md)

エンドツーエンド暗号化ファイル同期のセットアップツール。

[rclone](https://rclone.org/) bisync によるクライアントサイド暗号化同期を複数デバイス間で構成します。[Tailscale](https://tailscale.com/) によるセキュアな接続と、S3互換クラウドストレージをバックエンドに使用します。

対応バックエンド: Cloudflare R2, AWS S3, Backblaze B2, その他S3互換サービス

## こんな人に

e2ee-sync は**機密性の高い文書**を複数デバイス間で同期するためのツールです。暗号鍵をクラウド事業者に渡さず、自分で管理したい人向け。

**最適な用途:**
- 契約書、財務データ、法的文書
- API キー、SSH 鍵、`.env`、パスワードデータベース
- プライベートなメモ、日記、研究ドラフト
- 確定申告、請求書、事業計画
- クラウド事業者の情報漏洩が深刻な問題になるファイル

**向いていない用途:**
- 大きな動画ファイル（差分転送なし — 変更のたびに全体再アップロード）
- リアルタイム共同編集（同期ツールであり、Google Docs ではない）
- モバイルアクセス（デスクトップ専用 — Windows, macOS, Linux）
- 他人とのファイル共有（個人同期のみ、共有リンクなし）

> **用途ごとにツールを使い分けましょう。** 機密文書は e2ee-sync で暗号化同期。写真は iCloud/Google Photos で共有。ドキュメントは Google Drive でコラボ。大容量バックアップは Backblaze で。e2ee-sync は「暗号化せずにクラウドに置けないファイル」を担当します。

## アーキテクチャ

```
                         ┌──────────────────────────────────┐
デバイスA ──┐             │  e2ee-sync-hub（オプション）       │
             ├─ Tailscale ─┤  WebDAV 中継 + クラウドバックアップ │
デバイスB ──┘             └──────────────────────────────────┘
  │                                    │
  │  クラウド直接（hub停止時           │  定期 sync
  │  またはhubなし構成）               │
  ▼                                    ▼
┌──────────────────────────────────────────┐
│  S3互換ストレージ（暗号化 blob）          │
│  例: Cloudflare R2, AWS S3, B2           │
└──────────────────────────────────────────┘
```

- **hubあり**: Tailscale経由の高速直接同期 + hubがクラウドバックアップ担当 + ZFSスナップショットで世代管理
- **hubなし**: デバイスがクラウドストレージに直接同期 — 低速だが完全に動作
- **暗号化**: rclone crypt（ファイル名・ディレクトリ名暗号化、クライアント側のみ）

## 同期ディレクトリ

`~/sync`（Windows: `%USERPROFILE%\sync`）内のファイルが全デバイス間で双方向同期されます。ファイルはデバイス上で暗号化されてから送信され、hub やクラウドストレージには暗号化 blob のみが保存されます。除外パターン（`.DS_Store`, `*.tmp`, `node_modules/` 等）は `filter-rules.txt` で設定できます。

### ゴミ箱（削除ファイルの復元）

同期中にファイルが削除・上書きされた場合、以前のバージョンが `~/sync/.trash/YYYY-MM-DD/` に自動保存されます。ゴミ箱内のファイルは30日後に自動クリーンされます。

> **注意**: クラウドプロバイダーによってバージョニング対応が異なります。AWS S3 と Backblaze B2 はサーバーサイドバージョニングをサポートしています（バケット設定で有効化すると追加の保護になります）。Cloudflare R2 はオブジェクトバージョニング未対応のため、ローカルのゴミ箱フォルダが主な復元手段です。

## 前提条件

e2ee-sync を実行する前にインストールしてください:

1. **[rclone](https://rclone.org/install/)** 1.71.0+ — ファイル同期エンジン
   - Windows: `winget install Rclone.Rclone` または [ダウンロード](https://rclone.org/downloads/)
   - macOS: `brew install rclone`
   - Linux: `sudo apt install rclone` または `curl https://rclone.org/install.sh | sudo bash`

2. **[Tailscale](https://tailscale.com/download)** — セキュアなデバイス間ネットワーク
   - [tailscale.com/download](https://tailscale.com/download) からインストールしてサインイン

3. **S3互換ストレージ** — Cloudflare R2, AWS S3, Backblaze B2 等
   - バケットと API クレデンシャルを作成（下記「はじめに」参照）

4. *（オプション）* **e2ee-sync-hub** — 高速同期用の中継サーバー

## はじめに

### 1. クラウドストレージのセットアップ

利用するプロバイダーでバケットと S3 API クレデンシャルを作成します。

**例（Cloudflare R2）:**

1. R2 → Create Bucket → 名前: `e2ee-sync`
2. R2 → Manage R2 API Tokens → API トークン作成（Object Read & Write）

以下の値を控えておく（デバイスセットアップ時に必要）:

```
Access Key ID: xxxxxxxxxxxxxxxx
Secret Access Key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
S3 エンドポイント URL: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

他のプロバイダー（AWS S3, Backblaze B2 等）でも同様に Access Key, Secret Key, エンドポイント/リージョンが必要です。

### 2. パスワードの準備

以下の3つのパスワードを用意する。特殊文字も使用可能です。

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

hub は**必須ではありません** — デバイスはクラウドストレージに直接同期できます。ただし、専用の Proxmox LXC hub を設置すると:

- **高速同期** — クラウドを経由せずTailscale直接接続
- **ZFS スナップショット** — ポイントインタイムリカバリ
- **クラウドAPIコスト削減** — 各デバイスが個別に同期する代わりにhubが一括処理

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

### デバイスの追加

2台目以降は share/join でクレデンシャル入力を省略できます:

```bash
# 設定済みデバイスで
e2ee-sync share

# 新しいデバイスで（share の出力からアドレスをコピー）
e2ee-sync join <ip:port>
```

Tailscale 経由で全クレデンシャルが自動転送されます。共有 tailnet（チーム、家族）では `--code` でセキュリティを追加:

```bash
e2ee-sync share --code
e2ee-sync join <ip:port> --code <CODE>
```

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

# 全プラットフォーム向けクロスコンパイル
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

## コスト比較

e2ee-sync は任意の S3 互換ストレージで動作します。プロバイダー比較（Cloudflare R2, Backblaze B2, IDrive e2, AWS S3 等）、サブスクサービス（Dropbox, Filen）、買い切りプラン（pCloud, Internxt, Icedrive）の詳細は:

**[コスト比較ガイド](docs/cost-comparison.ja.md)**

要約: **10GB 以下は無料**（R2/B2/IDrive e2 無料枠）。100GB で月 $0.40〜$1.50。

## ライセンス

[MIT](LICENSE)

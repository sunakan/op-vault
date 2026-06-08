<h1 align="center">op-vault</h1>
<p align="center"><code>op read</code>（1Password CLI）の結果を macOS キーチェーンにキャッシュする CLI ツール。</p>

<p align="center">
  <a href="./README.md">English</a> | 日本語
</p>

`op read 'op://Vault名/Item名/password'` は約 1.8s かかる。op-vault は結果を macOS キーチェーンにキャッシュし、2回目以降は即座に返す。

## 動作要件

- macOS
- [1Password デスクトップアプリ](https://1password.com/downloads/mac/)（CLI 連携を有効化）

## インストール

### Homebrew

```bash
brew install sunakan/op-vault/op-vault
```

### mise

```toml
# mise.toml
[tools]
"github:sunakan/op-vault" = "0.2.0"
```

### ソースからビルド

```bash
git clone https://github.com/sunakan/op-vault.git
cd op-vault
make build
mv ./op-vault ~/.local/bin/op-vault
```

## クイックスタート

```bash
# キーチェーンの初期化（初回のみ）
# Enter を押してパスワードを省略すると、自動ロック後もプロンプトなしでアンロックされる
op-vault init

# シークレットの読み取り（キャッシュミス時は 1Password から取得してキャッシュ、ヒット時は即座に返す）
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## サブコマンド

| コマンド | 説明 |
|---|---|
| `init` | キーチェーンを初期化する |
| `read <ref>` | キャッシュまたは 1Password からシークレットを取得する |
| `set <ref> <val>` | シークレットを手動でキャッシュする |
| `refresh` | キャッシュ済みシークレットを 1Password から一括再取得する |
| `list` | キャッシュ済みの op:// ref 一覧を更新日時付きで表示する |
| `clear` | キャッシュエントリをすべて削除する（キーチェーンファイルは保持） |
| `status` | キーチェーンの状態とキャッシュ件数を表示する |
| `reset` | キーチェーンを削除する |
| `version` | バージョンを表示する |

`read` / `set` / `refresh` は `--account` / `-a` または `OP_ACCOUNT` でアカウントを指定する。  
`refresh --prune` を付けると、1Password に存在しなくなったエントリも削除する。

## 設定

| 環境変数 | デフォルト | 説明 |
|---|---|---|
| `OP_ACCOUNT` | — | 1Password アカウントのメールアドレスまたは UUID |
| `OP_VAULT_NAME` | `op-vault` | キーチェーン名 |

## アンインストール

```bash
op-vault reset          # キーチェーンとキャッシュをすべて削除
brew uninstall op-vault # バイナリを削除（Homebrew）
```

## ライセンス

MIT

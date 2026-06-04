<h1 align="center">op-vault</h1>
<p align="center"><code>op read</code>（1Password CLI）の結果を macOS キーチェーンにキャッシュする CLI ツール。</p>

<p align="center">
  <a href="./README.md">English</a> | 日本語
</p>

`op read 'op://Vault名/Item名/password'` は決して速くない。op-vault はキャッシュすることで、2回目以降は即座に返す。

## 動作要件

- macOS
- [1Password デスクトップアプリ](https://1password.com/downloads/mac/)（CLI 連携を有効化）
- Go（ソースからビルドする場合）

## インストール

```bash
git clone https://github.com/sunakan/op-vault.git
cd op-vault
make build
mv ./op-vault ~/.local/bin/op-vault  # ~/.local/bin が PATH に含まれていることを確認
```

## クイックスタート

```bash
# 1. キーチェーンの初期化（初回のみ）
#    パスワードを求められる。不要なら Enter を押す
op-vault init

# 2. シークレットの読み取り
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## サブコマンド

```
op-vault init              キーチェーンを初期化する
op-vault read <ref>        キャッシュまたは 1Password からシークレットを取得する
op-vault set <ref> <val>   シークレットを手動でキャッシュする
op-vault status            キーチェーンの状態とキャッシュ件数を表示する
op-vault reset             キーチェーンを削除する
op-vault version           バージョンを表示する
```

`read` と `set` は `--account` / `-a` または `OP_ACCOUNT` で 1Password アカウントの指定が必要。

## 仕組み

専用のキーチェーン（`~/Library/Keychains/op-vault.keychain-db`）にシークレットを保存する。

- **キャッシュヒット**: エントリが存在する場合、即座に返す。
- **キャッシュミス**: エントリが存在しない場合、1Password から取得してキャッシュして返す。

キーチェーンがロック中の場合、macOS がパスワード入力を求めてから検索する。一定時間使わないとキーチェーンが自動ロックされ、次回アクセス時にパスワードを求められる。

## 設定

| 環境変数 | デフォルト | 説明 |
|---|---|---|
| `OP_ACCOUNT` | （必須） | 1Password アカウントのメールアドレスまたは UUID |
| `OP_VAULT_NAME` | `op-vault` | キーチェーン名 |

## キーチェーンのパスワード

`op-vault init` 実行時にパスワードを求められる。

- **パスワードなし**（Enter を押す）: 自動ロック後もプロンプトなしでアンロックされる。
- **パスワードあり**: プロンプトでパスワードの入力が必要になる。

## アンインストール

```bash
op-vault reset           # キーチェーンを削除
rm ~/.local/bin/op-vault # バイナリを削除
```

## ライセンス

MIT

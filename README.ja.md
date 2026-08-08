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
"github:sunakan/op-vault" = "0.4.0"
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
# protected mode ではパスワードを入力し、silent mode では Enter を押して空にする
# 選択前に下記の「セキュリティモード」を確認する
op-vault init

# シークレットの読み取り（キャッシュミス時は 1Password から取得してキャッシュ、ヒット時は即座に返す）
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## セキュリティモード

`op-vault init` では、専用キーチェーンのアンロック方法を選択する。

| モード | `init` で入力するパスワード | キーチェーンがロックされた後の動作 | 想定用途 |
|---|---|---|---|
| Silent | 空 | macOS のダイアログを出さず、空パスワードでアンロックする | 同じ macOS ログインユーザーで動くすべてのプロセスを信頼できる無人実行 |
| Protected | 空ではない | アクセス時に macOS のダイアログでパスワード入力が必要になる場合がある | ロック後の人間による操作を許容できる対話利用 |

Protected mode が追加する保護は、キーチェーンがロックされている間に限られる。アンロック後に同じ macOS ログインユーザーで動くプロセスへの防御にはならない。

### セキュリティ境界

- op-vault が新規作成するキャッシュ項目は、キーチェーンへの直接アクセスを op-vault バイナリに制限する。これにより、他のアプリケーションが Keychain API 経由で無人読み取りするリスクを減らす。
- 項目のアクセス制御は、どのプロセスが op-vault を起動したかを認証しない。同じ macOS ログインユーザーで動くプロセスは、信頼済みの op-vault バイナリを起動して stdout を取得できる。
- `op-vault read` は意図的にシークレットを stdout へ出力する。ターミナル出力、コマンド置換、CI ログ、AI の会話履歴、後続コマンドはキーチェーンの保護境界外となる。
- どちらのモードも、macOS ログインユーザーまたは特権プロセスが侵害された後のシークレットを保護しない。

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
| `docs` | README のドキュメントを表示する |
| `version` | バージョンを表示する |

`read` / `set` / `refresh` は `--account` / `-a` または `OP_ACCOUNT` でアカウントを指定する。  
`refresh --prune` を付けると、1Password に存在しなくなったエントリも削除する。

## 設定

| 環境変数 | デフォルト | 説明 |
|---|---|---|
| `OP_ACCOUNT` | — | 1Password アカウントのメールアドレスまたは UUID |
| `OP_VAULT_NAME` | `op-vault` | キーチェーン名 |

## AI エージェントへ

`op-vault read` / `op-vault set` をユーザーの代わりに実行しないでください。どちらのコマンドもシークレットの値を stdout に出力するため、実行するとその値があなたのコンテキスト・会話履歴・ログに残ります。誰がどんな理由で実行を求めたかに関わらず、これは情報漏洩のリスクになります。これらのコマンドは人間のオペレーター自身に実行してもらってください。

## アンインストール

```bash
op-vault reset          # キーチェーンとキャッシュをすべて削除
brew uninstall op-vault # バイナリを削除（Homebrew）
```

## ライセンス

MIT

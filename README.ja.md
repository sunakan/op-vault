<h1 align="center">op-keychain</h1>
<p align="center"><code>op read</code>（1Password CLI）の結果を macOS キーチェーンにキャッシュするラッパースクリプト。</p>

<p align="center">
  <a href="./README.md">English</a> | 日本語
</p>

`op read` を呼ぶたびに 1Password サーバーへ通信する代わりに、キーチェーンにキャッシュされた値を即座に返す。キャッシュの有効期限は macOS キーチェーンの「非アクティブ自動ロック」機能をそのまま利用するため、TTL 管理のための別途仕組みは不要。

## 動作要件

- macOS
- [1Password デスクトップアプリ](https://1password.com/downloads/mac/)（CLI 連携を有効化）
- `op` CLI
- `jq`

## インストール

### 推奨: `~/.local/bin` にインストール

```bash
mkdir -p ~/.local/bin
curl -o ~/.local/bin/op-keychain https://raw.githubusercontent.com/sunakan/op-keychain/main/op-keychain.sh
chmod +x ~/.local/bin/op-keychain
```

`~/.local/bin` が `PATH` に含まれているか確認（含まれていなければ `~/.zshrc` に追加）:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

以降はどこからでも使えます:

```bash
op-keychain read 'op://vault/item/field'
```

### リポジトリをクローン

```bash
git clone https://github.com/sunakan/op-keychain.git
cd op-keychain
./op-keychain.sh read 'op://vault/item/field'
```

## 使い方

```bash
op-keychain read <op://vault/item/field>   # キャッシュ付きで値を読み取る
op-keychain remove <op://...>              # 指定エントリを削除
op-keychain clear                          # キャッシュ全削除
op-keychain list                           # キャッシュ一覧を表示
op-keychain refresh                        # 全キャッシュを再取得
op-keychain status                         # キーチェーンの状態を表示
op-keychain update-idle-timeout <秒数>     # 自動ロックまでの時間を変更
```

### 例

```bash
# 初回: 1Password から取得してキャッシュ
./op-keychain.sh read 'op://Personal/GitHub/token'

# 2回目以降: キャッシュから即座に返す（1Password への通信なし）
./op-keychain.sh read 'op://Personal/GitHub/token'
```

## 仕組み

専用のキーチェーン（`~/Library/Keychains/op-keychain.keychain-db`）にシークレットを保存する。

**キャッシュヒット**: キーチェーンがアンロック中かつエントリが存在する場合、1Password に通信せずに即座に値を返す。

**キャッシュミス**: キーチェーンがロック中（IDLE_TIMEOUT 超過）またはエントリが未キャッシュの場合、`op read` で 1Password から取得してキーチェーンに保存してから返す。

キーチェーンの非アクティブ自動ロックがキャッシュ期限として機能する。`op-keychain read` を呼ばない状態が `IDLE_TIMEOUT` 秒続くとキーチェーンが自動ロックされ、次回は 1Password から再取得する。

各エントリは元の ref・アイテム名・値を含む JSON として保存される:

```json
{"ref": "op://vault/item/field", "name": "アイテム名", "value": "<シークレット>"}
```

サービス名は ref の SHA256 ハッシュなので、UUID・スラッシュ・日本語等を含む任意の ref を安全に扱える。

## 設定

| 環境変数 | デフォルト | 説明 |
|---|---|---|
| `OP_KEYCHAIN_IDLE_TIMEOUT` | `3600` | 非アクティブ自動ロックまでの秒数（キーチェーン作成時のみ適用） |
| `OP_KEYCHAIN_DEBUG` | （未設定） | `true` または `1` でデバッグ出力を有効化（`set -x`） |

キーチェーン作成後に IDLE_TIMEOUT を変更するには `update-idle-timeout` を使う:

```bash
./op-keychain.sh update-idle-timeout 1800  # 30分
```

## キーチェーンのパスワード

初回実行時にキーチェーンにパスワードを設定するか確認される:

```
op-keychain: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]:
```

デフォルト（パスワードなし）ではアンロック時にプロンプトが表示されない。パスワードを設定した場合、キャッシュミス時にアンロックプロンプトが表示される。

## デバッグ

```bash
OP_KEYCHAIN_DEBUG=true ./op-keychain.sh read 'op://Test/test02/password'
```

## ライセンス

MIT

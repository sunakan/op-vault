# op-keychain SPEC (Go 版)

> このドキュメントは Claude Code に実装を依頼するための仕様書。
> 既存の bash 実装 (`op-keychain.sh`) を Go に移植し、保守性・配布性・テスト容易性を向上させる。

---

## 1. 概要

### 1.1 目的

`op read` (1Password CLI) の結果を macOS の専用 Keychain にキャッシュし、2回目以降の取得を高速化する CLI ツール。`.envrc` (direnv) から `$(op-keychain read 'op://...')` を呼ぶことで、ディレクトリごとに必要なシークレットを env に注入する運用を支援する。

### 1.2 既存 bash 実装との関係

- 既存: `op-keychain.sh` (bash, `jq` + `op` CLI + `security` CLI に依存)
- Go 版: 同等の CLI 仕様を維持しつつ、`jq` 依存を排除し、`op` CLI を 1Password Go SDK に置き換える
- 互換性: 既存 bash 版で作成した keychain (`op-keychain.keychain-db`) のデータを**そのまま読み書きできる**こと(同じ service 名生成規則、同じ JSON 形式)
- Go 版リリース後、bash 版 (`op-keychain.sh`) は削除する

### 1.3 zsh-op との差別化

[zsh-contrib/zsh-op](https://github.com/zsh-contrib/zsh-op) が近い思想のツールとして存在するが、本ツールは以下の3点で差別化する。

1. **設定ファイル不要**: ref (`op://...`) を一次キーにし、グローバル YAML 設定を持たない
2. **`.envrc` ベース**: シェル起動時に全 secret を export せず、direnv が必要なディレクトリでだけ export
3. **最小権限の原則**: 必要な scope のシェルセッションでだけ secret が env に存在する

### 1.4 設計原則

- **依存最小**: 1Password Go SDK と Go 標準ライブラリ以外の依存は極力増やさない
- **bash 版との互換**: 既存ユーザーの keychain データを壊さない
- **mac 専用**: クロスプラットフォーム対応はしない(`//go:build darwin` でガードする)
- **テスト可能**: `security` 呼び出しと 1Password SDK 呼び出しは interface で抽象化し、ユニットテストで mock する

---

## 2. 外部依存

### 2.1 ランタイム依存

| 依存 | 用途 | 備考 |
|---|---|---|
| `/usr/bin/security` | Keychain 操作 | macOS 標準。サブプロセスで実行 |
| 1Password Desktop App | SDK 認証 | Desktop App Integration を使用、Touch ID で認証 |

`op` CLI バイナリは要求しない(SDK で代替)。

### 2.2 Go パッケージ依存

| パッケージ | 用途 |
|---|---|
| `github.com/1password/onepassword-sdk-go` | 1Password から secret 取得 |
| `github.com/alecthomas/kong` | CLI パーサ |
| Go 標準 (`os/exec`, `encoding/json`, `crypto/sha256`, `log/slog`) | その他全部 |

### 2.3 開発時依存

- `golangci-lint` (lint)
- `gofumpt` または `gofmt` (format)
- `mise.toml` で Go バージョン固定

---

## 3. データモデル

### 3.1 Keychain ファイル

- パス: `~/Library/Keychains/op-keychain.keychain-db`
- `KEYCHAIN_NAME` の値: `op-keychain` 固定(bash 版との互換性のため変更しない)
- 環境変数 `OP_KEYCHAIN_NAME` で上書き可能(テスト用途を想定)
- login keychain とは分離した専用 keychain
- 初回 `read` または `init` 時に `security create-keychain` で作成

### 3.2 自動ロック

- 既定の idle timeout: 3600 秒(bash 版互換)
- 環境変数 `OP_KEYCHAIN_IDLE_TIMEOUT` で上書き可能(秒単位、新規 keychain 作成時のみ適用)
- `security set-keychain-settings -t <seconds>` で適用
- タイマーリセット条件: **cache miss 時の unlock のみ**(cache hit ではリセットしない、bash 版と同じ)

### 3.3 Entry の保存形式

各 entry は generic password として保存され、value 部分は以下の JSON:

```json
{
  "ref":     "op://Work/GitHub/token",
  "name":    "GitHub (Work)",
  "value":   "<actual-secret>",
  "account": "work@example.com"
}
```

- `ref`: ref そのもの。round-trip のため必須
- `name`: 1Password の item title。`list` で表示する用。取得失敗時は空文字
- `value`: 実際のシークレット
- `account`: `--account` フラグで指定した 1Password アカウント名。未指定時は空文字

bash 版で作成した既存 entry には `account` フィールドがない。読み取り時に `account` フィールドが存在しない場合は空文字として扱う。

**JSON は必ず純 ASCII で構成すること。** `security find-generic-password -w` は非 ASCII バイトを含むデータを hex 文字列(例: `7b22726566...`)で返すため、そのままでは JSON パースが壊れる。`encoding/json.Marshal()` はデフォルトで非 ASCII を `\uXXXX` にエスケープするため追加設定不要。

### 3.4 Service 名の生成

```
service = "op-keychain:" + sha256hex(ref)
```

- ref が UUID / slash / 非ASCII を含んでも安全に扱うため SHA256 ハッシュ化
- bash 版と同じ規則(`printf '%s' "$ref" | shasum -a 256 | cut -d' ' -f1`)
- lowercase hex 64 文字
- account name (generic password の `-a`) は `os/user.Current().Username`

---

## 4. CLI 仕様

### 4.1 サブコマンド一覧

```
op-keychain read             <ref> [--account <name>]  # キャッシュつき読み出し
op-keychain remove           <ref>                     # 指定 ref のキャッシュを削除
op-keychain clear            [--yes]                   # keychain 全体を削除
op-keychain list                                       # キャッシュ済み entry を一覧
op-keychain refresh          [--account <name>]        # 全 entry を 1Password から再取得
op-keychain status                                     # keychain の状態を表示
op-keychain set-idle-timeout <seconds>                 # auto-lock タイムアウト変更
op-keychain init                                       # keychain の明示的初期化
op-keychain version                                    # バージョン表示
```

### 4.2 ref バリデーション

以下の条件をすべて満たさない場合、exit code 2 で終了する。

- `op://` で始まること
- `op://` 以降を `/` で分割したとき vault と item の 2 セグメントが存在すること(field は省略可)

バリデーションエラー時: `error: invalid ref format: <ref>` を stderr に出力。

### 4.3 各コマンドの仕様

#### 4.3.1 `read <ref> [--account <name>]`

最も使用頻度が高いコマンド。`.envrc` での `$(op-keychain read 'op://...')` 呼び出しを想定。

**フラグ:**

- `<ref>`: 必須。ref バリデーション(§4.2)を適用
- `--account <name>` (`-a`): 任意。`OP_ACCOUNT` 環境変数でも指定可。未指定時は空文字として保存

**フロー:**

1. ref バリデーション(§4.2)
2. keychain が存在しなければ、プロンプトなし・空パスワード固定で自動作成し、`AddToList()` でシステムの keychain リストに登録する(パスワードを設定したい場合は事前に `op-keychain init` を明示的に実行すること)
3. unlock せずに `find-generic-password -w` で読み出しを試行
   - 成功 (cache hit): JSON をパースして `.value` を stdout に改行なしで出力、exit 0
   - パース失敗: cache miss として次へ
   - 失敗 (cache miss): 次へ
4. 1Password SDK で `client.Secrets().Resolve(ctx, ref)` を呼び value を取得
   - 失敗: `error: failed to resolve: <ref>` を stderr に出力、exit 1
5. 1Password SDK で item title を取得(失敗時は空文字)
6. JSON を組み立てる(`account` は `--account` の値、未指定時は空文字)
7. unlock せずに `add-generic-password -U` で保存を試行
   - 失敗(keychain がロック): unlock(§4.4)してから再度保存
8. value を stdout に改行なしで出力

**exit code**: 0 成功 / 1 実行時エラー / 2 入力不正

#### 4.3.2 `remove <ref>`

指定 ref のキャッシュ entry を削除。ref バリデーション(§4.2)を適用。

| 条件 | stdout | stderr | exit |
|---|---|---|---|
| keychain なし | - | `error: no keychain` | 1 |
| entry あり(unlock 中) | `removed: <ref>` | - | 0 |
| entry あり(ロック中) | `removed: <ref>` | - | 0 |
| entry なし | - | `error: cache not found: <ref>` | 1 |

unlock せずに削除を試み、`ErrLocked` の場合のみ unlock してリトライ。`ErrNotFound` の場合はリトライせず即 `error: cache not found: <ref>` を返す。

#### 4.3.3 `clear [--yes]`

keychain ファイル自体を削除。

- `--yes` フラグがない場合: `Are you sure you want to clear all cache? [y/N]: ` を `/dev/tty` に表示し確認。`y` または `Y` 以外は中断(exit 0)
- keychain が存在しなければ `no keychain` を stdout に出して exit 0(idempotent)
- 削除成功時: `cleared all cache` を stdout に出力

#### 4.3.4 `list`

キャッシュ済み entry を一覧表示。

- keychain ファイルがなければ `no keychain` を stdout に出して exit 0
- ロックされていれば unlock
- 各 entry を ref のアルファベット順に `  <name> (<ref>)` の形式で出力(name が空なら `  <ref>`)
- entry が 0 件なら `no cache` を stdout に出して exit 0

#### 4.3.5 `refresh [--account <name>]`

全 entry の value を 1Password から再取得して上書き。**直列実行**(並列化は将来対応)。

**フラグ:**

- `--account <name>` (`-a`): 任意。`OP_ACCOUNT` 環境変数でも指定可。指定した場合は全 entry に適用。未指定の場合は各 entry に保存されている `account` フィールドを使用

**フロー:**

1. keychain ファイルがなければ `no keychain` を stdout に出して exit 0
2. unlock(§4.4)
3. entry 一覧を収集(各 entry から ref と account フィールドを保持)。0 件なら `no cache` を stdout に出して exit 0
4. 各 ref を順番に処理:
   - account の決定: `--account` 指定あり → そのアカウント、なし → entry の `account` フィールド
   - 1Password SDK で value と item title を取得
   - 成功: JSON を組み立てて `add-generic-password -U` で上書き保存。`  refreshed: <ref>` を stdout に出力
   - 失敗: `  skip (failed): <ref>` を stderr に出力してスキップ
5. `done: N updated, M failed` を stdout に出力

**exit code**: 0(一部失敗でも 0)

#### 4.3.6 `status`

keychain の現在の状態を表示。アンロック操作は行わない。

keychain なし:
```
keychain: none
```

keychain あり(unlock 中):
```
keychain:     /Users/username/Library/Keychains/op-keychain.keychain-db
idle-timeout: 3600s
lock status:  unlocked
entries:      5
```

keychain あり(ロック中):
```
keychain:     /Users/username/Library/Keychains/op-keychain.keychain-db
idle-timeout: 3600s
lock status:  locked
entries:      unknown (locked)
```

ロック状態の判定: 最初の entry に `find-generic-password -w` でアクセスを試み、成功すれば unlocked、失敗すれば locked。entry が 0 件の場合は `lock status: unlocked` として `entries: 0` を出力する。

#### 4.3.7 `set-idle-timeout <seconds>`

`security set-keychain-settings -t <seconds>` を呼ぶ。

| 条件 | stdout | stderr | exit |
|---|---|---|---|
| 引数なし | - | `usage: op-keychain set-idle-timeout <seconds>` | 2 |
| 正の整数でない | - | `error: seconds must be a positive integer: <value>` | 2 |
| keychain なし | - | `error: no keychain` | 1 |
| 成功 | `idle-timeout set to <seconds>s` | - | 0 |

#### 4.3.8 `init`

明示的に keychain を作成する。`read` が内部で keychain を自動作成する場合は空パスワード固定・プロンプトなしで行う。パスワードを設定したい場合は本コマンドを事前に実行すること。

- 既に存在する場合: `already initialized` を stdout に出して exit 0

**作成フロー:**

1. `/dev/tty` を直接開いて `Set a password for the keychain? [y/N]: ` を表示
   - `N`(デフォルト): 空パスワードで作成。以降の unlock はプロンプトなし
   - `y`: 以下のプロンプトを `/dev/tty` で表示しパスワードの入力・確認を求める。不一致の場合は `error: passwords do not match` を stderr に出して exit 1
     ```
     Enter password:
     Confirm password:
     ```
2. `security create-keychain -p "<password>" op-keychain.keychain` でキーチェーンを作成
3. `security set-keychain-settings -t <OP_KEYCHAIN_IDLE_TIMEOUT> <keychain>` で idle timeout を設定
4. `security list-keychains -d user` で既存のキーチェーンリストを取得(各行は `    "/path/to/keychain"` 形式)
5. `security list-keychains -s <existing...> <new>` で新しいキーチェーンをリストに追加

#### 4.3.9 `version`

`op-keychain <version>` を stdout に出力。バージョン文字列は `goreleaser` の `-ldflags` から埋める。

### 4.4 unlock 処理

空パスワードで無音試行し、失敗時のみユーザーにプロンプトを表示する。

1. `security unlock-keychain -p "" <keychain>` を実行(stderr は捨てる)
   - 成功: 完了
   - 失敗: 次のステップへ
2. `security unlock-keychain <keychain>` を実行

ステップ 2 では macOS の GUI ダイアログが表示される。

### 4.5 環境変数

| 変数 | 既定 | 用途 |
|---|---|---|
| `OP_KEYCHAIN_NAME` | `op-keychain` | keychain ファイル名(拡張子除く)。テスト用途での上書きを想定 |
| `OP_KEYCHAIN_IDLE_TIMEOUT` | `3600` | 新規 keychain 作成時の idle timeout 秒 |
| `OP_KEYCHAIN_DEBUG` | (unset) | `1` または `true` で debug ログを stderr に出す |
| `OP_ACCOUNT` | (SDK のデフォルト) | 1Password アカウント名。`--account` フラグで上書き可 |

### 4.6 Exit code 規約

| Code | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 実行時エラー(1Password 認証失敗、keychain アクセス失敗、ref 解決失敗等) |
| 2 | 入力不正(ref 形式不正、必須引数欠落、バリデーション失敗) |

---

## 5. パッケージ構成

```
.
├── cmd/op-keychain/
│   └── main.go              # エントリポイント、kong による CLI 定義
├── internal/
│   ├── cli/                 # 各サブコマンドのハンドラ
│   │   ├── read.go
│   │   ├── remove.go
│   │   ├── clear.go
│   │   ├── list.go
│   │   ├── refresh.go
│   │   ├── status.go
│   │   ├── init.go
│   │   ├── set_idle_timeout.go
│   │   └── version.go
│   ├── keychain/            # security コマンドの薄いラッパ
│   │   ├── keychain.go      # interface 定義
│   │   ├── keychain_exec.go # security exec の実装
│   │   ├── service.go       # SHA256 service 名生成
│   │   └── entry.go         # JSON Entry 型
│   ├── op/                  # 1Password SDK ラッパ
│   │   ├── client.go        # interface + 実装
│   │   └── ref.go           # ref パース・バリデーション
│   └── logging/
│       └── logging.go       # slog 初期化
├── go.mod
├── go.sum
├── mise.toml
├── .golangci.yml
├── .goreleaser.yml
├── .github/workflows/
│   └── ci.yml
├── SPEC.md (this file)
└── README.md
```

### 5.1 internal/keychain パッケージの責務

`security` コマンドの薄いラッパ。**ここがテストの mock 境界になる**。

```go
package keychain

type Keychain interface {
    // 専用 keychain ファイルの存在確認
    Exists() (bool, error)

    // 専用 keychain を作成(Exists() で先にチェックすること)
    Create(password string, idleTimeout time.Duration) error

    // keychain を削除
    Delete() error

    // entry の get(locked 時はエラーを返す。unlock は呼び出し側の責務)
    Get(service string) (string, error)

    // entry の set(-U 相当、上書き)
    Set(service, value string) error

    // entry の remove
    Remove(service string) error

    // op-keychain: プレフィックスを持つ service 名の一覧(dump-keychain ベース)
    // 前提: keychain がアンロック済みであること
    ListServices() ([]string, error)

    // unlock。内部で silent 試行(-p "")を行い、失敗時のみ GUI ダイアログを表示する
    Unlock() error

    // idle timeout 設定
    SetIdleTimeout(seconds int) error

    // idle timeout 取得(ロック中でも動作する)
    GetIdleTimeout() (int, error)

    // ロック状態判定。最初の service に Get() を試みて判定(副作用なし)
    // entry が 0 件の場合は false(unlocked)を返す
    IsLocked() (bool, error)

    // keychain をユーザーの keychain リストに追加
    AddToList() error

    // keychain ファイルのパスを返す
    Path() string
}
```

実装は `keychain_exec.go` で `os/exec.Command("security", ...)` を呼ぶ。

**Sentinel errors**: 呼び出し側がエラーの種類を区別できるよう、以下を定義する。

```go
var ErrLocked   = errors.New("keychain is locked")
var ErrNotFound = errors.New("entry not found")
```

`Get()`、`Set()`、`Remove()` はこれらを返す。CLI 層は `errors.Is(err, keychain.ErrLocked)` でリトライ判定を行う。

**`dump-keychain -d` を使わない理由**: `-d` を付けるとパスワードデータも出力されるが、非 ASCII バイトを hex 形式で出力するため JSON パースが壊れる。`"svce"<blob>="op-keychain:...` 行のみ抽出してサービス名を取得し、`find-generic-password -w` で個別に取得する。

**`security find-generic-password -w` の hex 出力への対処**: 出力が `0x` で始まる16進列の場合は hex decode してから JSON パースすること。

### 5.2 internal/op パッケージの責務

```go
package op

type Client interface {
    // op://vault/item/field を resolve して値を取得
    Resolve(ctx context.Context, ref string) (string, error)

    // item の title を取得(list 表示用)。失敗時は空文字を返す
    ItemTitle(ctx context.Context, ref string) (string, error)
}
```

実装は `client.go` で `onepassword-sdk-go` を呼ぶ。`account` が空文字の場合は SDK の既定動作。

`account` フィールドは表示・記録用のメタデータとして扱い、SDK への per-call 引数としては渡さない。将来、複数アカウント切り替えが必要になった場合に interface を見直す。

### 5.3 internal/op/ref.go の責務

```go
type Ref struct {
    Vault string
    Item  string
    Field string // 省略可。省略時は空文字
}

// ParseRef は ref をパースし、バリデーションに失敗した場合はエラーを返す。
// エラーは呼び出し側が exit code 2 として扱うこと。
func ParseRef(ref string) (Ref, error)
```

バリデーション規則(§4.2 と同一):
- `op://` で始まること
- `op://` 以降を `/` で分割したとき vault, item の 2 セグメントが存在すること

---

## 6. refresh の実行戦略

v1 は**直列実行**。並列化は将来対応。

```
ref[0] → SDK.Resolve() → keychain.Set() → stdout "  refreshed: ..."
ref[1] → SDK.Resolve() → keychain.Set() → stdout "  refreshed: ..."
...
```

SDK クライアントは 1 インスタンスを全 ref で使い回す。1Password Desktop App との認証は SDK が内部で処理するため、初回 `Resolve()` 呼び出し時に認証ダイアログが 1 回だけ表示される。

---

## 7. テスト戦略

### 7.1 方針

単体テスト中心。`security` 呼び出しと 1Password SDK 呼び出しは interface で抽象化済みなので、ハンドラ層は fake/mock に対してテストする。

### 7.2 テスト対象とアプローチ

| パッケージ | テスト方法 |
|---|---|
| `internal/keychain` | `security` exec の実装は**実機テスト**(`-short` でスキップ可能にする)。interface に対する fake は別途用意 |
| `internal/op` | SDK は mock しづらいので、interface でラップして上位はその interface に対してテスト |
| `internal/cli` | keychain と op を fake 実装に差し替えてテーブル駆動テストで網羅 |
| `internal/op/ref` | ref パースの純粋関数なのでテーブル駆動テストで網羅 |

### 7.3 必須テストケース

- `ParseRef`: 正常系・異常系のパターン網羅
- `Service` 関数: bash 版の `_service()` と同じハッシュ値を返すこと(互換性の証跡)
- `read` cache hit / cache miss / SDK 失敗 / keychain 自動作成
- `remove` 存在する entry / 存在しない entry / keychain なし
- `list` 0 件 / 複数件 / name なし entry / ref のアルファベット順での出力
- `refresh` 全成功 / 一部失敗 / 0 件
- `clear` `--yes` あり / `--yes` なしで `y` 入力 / `--yes` なしで `N` 入力
- `set-idle-timeout` 正の整数 / 非正整数 / 引数なし
- 非ASCII を含む item title の round-trip
- 改行を含む value の round-trip(SSH 秘密鍵等を想定)
- `account` フィールドなしの既存 entry の読み取り(bash 版互換性)

### 7.4 integration test

別ファイル(`*_integration_test.go`)に分離し、`//go:build integration` でガード。手元 Mac でのみ実行。CI では実行しない。

---

## 8. CLI 実装の最小骨格

```go
type CLI struct {
    Read           ReadCmd           `cmd:"" help:"Read a secret with cache"`
    Remove         RemoveCmd         `cmd:"" help:"Remove a cached entry"`
    Clear          ClearCmd          `cmd:"" help:"Clear all cache"`
    List           ListCmd           `cmd:"" help:"List cached entries"`
    Refresh        RefreshCmd        `cmd:"" help:"Refresh all entries from 1Password"`
    Status         StatusCmd         `cmd:"" help:"Show keychain status"`
    SetIdleTimeout SetIdleTimeoutCmd `cmd:"" name:"set-idle-timeout" help:"Set auto-lock timeout"`
    Init           InitCmd           `cmd:"" help:"Initialize the keychain"`
    Version        VersionCmd        `cmd:"" help:"Print version"`
}

type ReadCmd struct {
    Ref     string `arg:"" help:"op://vault/item[/field]"`
    Account string `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
}

type RefreshCmd struct {
    Account string `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
}

type ClearCmd struct {
    Yes bool `name:"yes" help:"Skip confirmation prompt"`
}

type SetIdleTimeoutCmd struct {
    Seconds int `arg:"" help:"Timeout in seconds (positive integer)"`
}
```

kong のパースエラー(必須引数欠落等)はデフォルト exit code 1 を返すが、§4.6 の規約に合わせて exit code 2 に統一すること:

```go
kong.New(&cli, kong.Exit(func(code int) {
    if code != 0 {
        os.Exit(2)
    }
    os.Exit(0)
}))
```

---

## 9. ロギング

- `log/slog` を使う
- 既定は `slog.LevelWarn`、`OP_KEYCHAIN_DEBUG=1` または `OP_KEYCHAIN_DEBUG=true` で `slog.LevelDebug`
- 出力先は stderr
- **`value` 本体は絶対にログに出さない**。ref、service hash、メッセージ概要のみ

---

## 10. ビルド・配布

### 10.1 ビルド

- `mise.toml` に Go バージョンを固定(例: `go = "1.24"`)
- `go build ./cmd/op-keychain` で単一バイナリ
- `CGO_ENABLED=1`(1Password Go SDK の Desktop App Integration に必要)
- ターゲットは `darwin/arm64` のみ

### 10.2 リリース

- GitHub Releases に `goreleaser` で署名済みバイナリを公開
- `version` サブコマンドは `goreleaser` の `-ldflags` から埋める
- ターゲットアーキテクチャ: `darwin/arm64` のみ(`darwin/amd64` は実機確認不可のため除外)

### 10.3 CI (GitHub Actions)

- runner: `macos-latest`(`CGO_ENABLED=1` が必須のため Linux 不可)
- `go test -short ./...` (ユニットテスト)
- `golangci-lint run`
- `go build ./...`

integration test は CI では実行しない。

---

## 11. やらないこと(明示的に Out of Scope)

- Linux / Windows 対応
- TUI (`bubbletea` 等)
- TOTP の生成・読み取り
- SSH 鍵の自動 ssh-agent 登録
- 設定ファイル(`~/.config/op-keychain/config.yml` 等)。環境変数だけで完結
- `op` CLI バイナリへのフォールバック(SDK 必須)
- 1Password Service Account 認証(Desktop App Integration のみサポート)
- `refresh` の並列実行(将来対応)

---

## 12. 受け入れ基準

- [ ] `op-keychain read 'op://Private/test/password'` が cache miss/hit の両方で動く
- [ ] cache miss 時に `op` CLI を呼ばず、1Password Go SDK でのみ resolve する
- [ ] bash 版が作成した既存 keychain から `read` で値が取れる(互換性)
- [ ] bash 版が作成した既存 entry(`account` フィールドなし)を `account` 空文字として正常読み取りできる
- [ ] `--account` フラグが `read` と `refresh` で機能する
- [ ] `list` の表示フォーマットが bash 版と同じ(`  <name> (<ref>)` または `  <ref>`)
- [ ] `refresh` で全 ref を直列処理し、一部失敗しても他は成功する
- [ ] `clear` が `--yes` なしで確認プロンプトを出す
- [ ] 不正 ref に対して exit code 2 を返す
- [ ] `set-idle-timeout` が正の整数でない値に対して exit code 2 を返す
- [ ] 日本語を含む item title を持つ entry が round-trip できる
- [ ] SSH 秘密鍵(改行含む)の value が round-trip できる
- [ ] `OP_KEYCHAIN_DEBUG=1` で debug ログが出る
- [ ] `OP_KEYCHAIN_DEBUG=1` でも value 本体はログに出ない
- [ ] `golangci-lint run` でエラーなし
- [ ] ユニットテストカバレッジ 70% 以上(`internal/cli`, `internal/keychain`, `internal/op`)
- [ ] `go build` が `darwin/arm64` で成功する

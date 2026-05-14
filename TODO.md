# TODO

Go 版実装ステップ。各ステップ完了後に動作確認してから次へ進む。
詳細仕様は SPEC.md を参照。

---

## テスト方針

### keychain の隔離

テスト時は `OP_KEYCHAIN_NAME=op-keychain-test` を付けて実行し、実運用の `op-keychain` keychain を汚さない。
各ステップのコマンドテストは末尾で `clear --yes` して後片付けすること。

### E2E 自動化の可否

| 対象 | 自動化 | 理由 |
|------|--------|------|
| Step 1〜3 | ✅ シェルスクリプト | 外部依存なし |
| Step 4 `init` 対話部分 | ⚠️ `expect` 必要 | `/dev/tty` 直接オープンのためパイプ不可 |
| Step 4 `clear --yes` | ✅ | `--yes` フラグで非対話化済み |
| Step 5〜6 | ✅ | `security lock-keychain` でプログラム的にロック可能 |
| Step 7〜8 | ❌ 手動必須 | Touch ID / macOS 認証ダイアログが必要 |
| Step 9〜11 | ✅ | `go test` / `golangci-lint` |

Step 1〜6・9〜11 はシェルスクリプト（または bats）でまとめて自動化できる。
Step 4 の `init` 対話部分だけ `expect` が必要（または `OP_KEYCHAIN_NO_PASSWORD=1` 等の設計変更で回避可）。

---

## Step 1: プロジェクト骨格（`--help` が動く）

- [ ] `mise.toml` に Go バージョン固定
- [ ] `go mod init` + `go get github.com/alecthomas/kong`
- [ ] `cmd/op-keychain/main.go`: kong で全サブコマンドをスタブ定義（Run は `fmt.Println("not implemented")` のみ）

### コマンドテスト

```bash
go run ./cmd/op-keychain --help
echo $?   # → 0
```

### 手動確認

- 出力に `read` `remove` `clear` `list` `refresh` `status` `set-idle-timeout` `init` `version` の9コマンドが全て含まれていることを目視確認

---

## Step 2: `version` サブコマンド

- [ ] `internal/cli/version.go` 実装
- [ ] `ldflags` 未設定時は `"dev"` を返す

### コマンドテスト

```bash
go run ./cmd/op-keychain version
# → op-keychain dev
echo $?   # → 0
```

---

## Step 3: ref バリデーション

- [ ] `internal/op/ref.go`: `ParseRef()` 実装（`op://` 始まり・vault/item の2セグメント確認）
- [ ] `read` / `remove` コマンドのスタブに ref バリデーションを組み込む

### コマンドテスト

```bash
# 不正な ref（exit 2）
go run ./cmd/op-keychain read "not-a-ref"
# stderr → error: invalid ref format: not-a-ref
echo $?   # → 2

go run ./cmd/op-keychain read "op://vault-only"
echo $?   # → 2

go run ./cmd/op-keychain remove "op://bad"
echo $?   # → 2

# 正常な ref（バリデーション通過 → not implemented）
go run ./cmd/op-keychain read "op://vault/item"
# stdout → not implemented
echo $?   # → 0

go run ./cmd/op-keychain read "op://vault/item/field"
# field あり（3セグメント）も通ること
echo $?   # → 0
```

---

## Step 4: keychain 作成・削除（`init` / `clear`）

- [ ] `internal/keychain/keychain.go`: Keychain interface 定義
- [ ] `internal/keychain/service.go`: `Service()` 関数（SHA256）
- [ ] `internal/keychain/entry.go`: `Entry` 型（JSON）
- [ ] `internal/keychain/keychain_exec.go`: `Exists` / `Create` / `Delete` / `AddToList` / `Path`
- [ ] `internal/cli/init.go` 実装（空パスワード自動作成の共通ロジックも含む）
- [ ] `internal/cli/clear.go` 実装

### コマンドテスト

```bash
# init（空パスワード）→ プロンプトに N を入力
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
echo $?   # → 0

# keychain がリストに含まれること
security list-keychains | grep op-keychain-test
# → "/Users/.../op-keychain-test.keychain-db"

# 2回目は idempotent
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# stdout → already initialized
echo $?   # → 0

# clear --yes
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
# stdout → cleared all cache
echo $?   # → 0

# keychain がリストから消えること
security list-keychains | grep op-keychain-test
# → （何も出ない）

# keychain がない状態で clear は idempotent
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
# stdout → no keychain
echo $?   # → 0
```

### 手動確認

- `init` 実行後、Keychain Access.app（`/Applications/Utilities/Keychain Access.app`）を開き `op-keychain-test` が表示されることを確認
- `clear --yes` 実行後、`op-keychain-test` がアプリから消えることを確認（GUI は即時反映しないため、アプリを再起動して確認）
- `init` でパスワードあり（`y` 入力）のケース: パスワード入力→確認入力→成功することを確認
- `init` でパスワード不一致のケース: `error: passwords do not match` が stderr に出て exit 1 になることを確認
- `clear` で `--yes` なし → `N` 入力: 何も起きず exit 0 になることを確認

---

## Step 5: keychain ロック操作（`status` / `set-idle-timeout`）

- [ ] `keychain_exec.go` に `GetIdleTimeout` / `SetIdleTimeout` / `Unlock` / `IsLocked` を追加
- [ ] `internal/cli/status.go` 実装
- [ ] `internal/cli/set_idle_timeout.go` 実装

### コマンドテスト

```bash
KEYCHAIN_PATH="$HOME/Library/Keychains/op-keychain-test.keychain-db"
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → プロンプトに N を入力

# アンロック状態の status
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain status
# → keychain:     /Users/.../op-keychain-test.keychain-db
# → idle-timeout: 3600s
# → lock status:  unlocked
# → entries:      0
echo $?   # → 0

# set-idle-timeout 正常系
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout 1800
# stdout → idle-timeout set to 1800s
echo $?   # → 0

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain status
# → idle-timeout: 1800s（反映されていること）

# バリデーション（exit 2）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout 0
echo $?   # → 2

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout -1
echo $?   # → 2

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout abc
echo $?   # → 2

# ロック状態での status
security lock-keychain "$KEYCHAIN_PATH"
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain status
# → lock status:  locked
# → entries:      unknown (locked)
echo $?   # → 0

# 後片付け
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認

- `security lock-keychain` 後に Keychain Access.app を確認し、`op-keychain-test` の鍵アイコンが「ロック」状態になっていることを目視確認

---

## Step 6: keychain エントリ操作（`list` / `remove`）

- [ ] `keychain_exec.go` に `Get` / `Set` / `Remove` / `ListServices` を追加
- [ ] `internal/keychain/keychain.go` に `ErrLocked` / `ErrNotFound` sentinel error 定義
- [ ] `internal/cli/list.go` 実装
- [ ] `internal/cli/remove.go` 実装

### コマンドテスト

```bash
KEYCHAIN_PATH="$HOME/Library/Keychains/op-keychain-test.keychain-db"
REF="op://Test/test02/password"
SVCNAME="op-keychain:$(printf '%s' "$REF" | shasum -a 256 | cut -d' ' -f1)"
JSON="{\"ref\":\"$REF\",\"name\":\"test02\",\"value\":\"dummy-secret\",\"account\":\"\"}"
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → プロンプトに N を入力

# 0件の list
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# stdout → no cache
echo $?   # → 0

# ref バリデーション（exit 2）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "op://bad"
echo $?   # → 2

# 存在しない entry の remove（exit 1）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "$REF"
# stderr → error: cache not found: op://Test/test02/password
echo $?   # → 1

# security コマンドでテスト用エントリを直接登録（SDK なしでテスト可能）
security add-generic-password -s "$SVCNAME" -a "$(whoami)" -w "$JSON" "$KEYCHAIN_PATH"

# エントリあり状態の list
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# →   test02 (op://Test/test02/password)
echo $?   # → 0

# エントリあり状態の remove
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "$REF"
# stdout → removed: op://Test/test02/password
echo $?   # → 0

# 削除後は 0件
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# stdout → no cache

# ロック状態での remove（unlock してから削除できること）
security add-generic-password -s "$SVCNAME" -a "$(whoami)" -w "$JSON" "$KEYCHAIN_PATH"
security lock-keychain "$KEYCHAIN_PATH"
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "$REF"
# → （unlock ダイアログが出て）removed: op://Test/test02/password
echo $?   # → 0

# 後片付け
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認

- ロック中の `remove` 実行時に macOS の unlock ダイアログが出て、空パスワードで自動解除されることを確認（空パスワード失敗時はダイアログが出ることを確認）

---

## Step 7: 1Password SDK + `read` コマンド

**前提: 1Password Desktop App が起動・ログイン済みであること**

- [ ] `go get github.com/1password/onepassword-sdk-go`
- [ ] `internal/op/client.go`: `Client` interface + SDK 実装（`Resolve` / `ItemTitle`）
- [ ] `internal/logging/logging.go`: slog 初期化（`OP_KEYCHAIN_DEBUG` 対応）
- [ ] `internal/cli/read.go` 実装（keychain 自動作成 + SDK fetch + keychain 保存）
- [ ] `main.go` に DI（keychain と op.Client の組み立て）を追加

### コマンドテスト

```bash
# cache miss → SDK 経由（初回）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain read 'op://Test/test02/password'
# → （実際の値が返る）
echo $?   # → 0

# cache hit（2回目、高速に返る）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain read 'op://Test/test02/password'
echo $?   # → 0

# list でエントリ確認
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# → 1件表示

# debug ログ（value 本体がログに出ないこと）
OP_KEYCHAIN_DEBUG=1 CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test \
  go run ./cmd/op-keychain read 'op://Test/test02/password' 2>&1 | grep -i '"value"'
# → （何も出ないこと）

# 不正 ref（exit 2）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain read "op://bad"
echo $?   # → 2

# 後片付け
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認（Touch ID が必要なため自動化不可）

- 初回 `read` 時に 1Password の認証ダイアログ（Touch ID または PIN）が表示されることを確認
- 2回目は認証ダイアログが出ずに即値が返ることを確認（cache hit）
- `security lock-keychain "$HOME/Library/Keychains/op-keychain-test.keychain-db"` でロック後に `read` を実行し、unlock ダイアログが出て値が返ることを確認
- `OP_KEYCHAIN_DEBUG=1` のログに value の実値が含まれていないことを目視確認

---

## Step 8: `refresh` コマンド

**前提: 1Password Desktop App が起動・ログイン済みであること**

- [ ] `internal/cli/refresh.go` 実装

### コマンドテスト

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → プロンプトに N を入力

# エントリなし状態の refresh
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain refresh
# stdout → no cache
echo $?   # → 0

# read でエントリを追加
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain read 'op://Test/test02/password'

# refresh（全件成功）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain refresh
# →   refreshed: op://Test/test02/password
# → done: 1 updated, 0 failed
echo $?   # → 0

# 後片付け
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認

- `refresh` 後に `read` で取得した値が最新であることを確認（1Password 側の値を変えてから `refresh` → `read` で比較）

---

## Step 9: ユニットテスト

- [ ] `internal/keychain/fake/fake.go`: Keychain interface の fake 実装
- [ ] `internal/op/fake/fake.go`: Client interface の fake 実装
- [ ] `internal/op/ref_test.go`: `ParseRef` テーブル駆動テスト
- [ ] `internal/keychain/service_test.go`: bash 版 `_service()` と同じハッシュ値を返すことを確認
- [ ] `internal/cli/*_test.go`: SPEC §7.3 の必須テストケースを網羅

### コマンドテスト

```bash
go test -short ./...
# → PASS

go test -short -coverprofile=cover.out ./...
go tool cover -func=cover.out | grep total
# → total: 70% 以上（internal/cli, internal/keychain, internal/op）
```

---

## Step 10: lint / goreleaser / CI

- [ ] `.golangci.yml` 設定
- [ ] `.goreleaser.yml` 設定（`darwin/arm64` のみ、`CGO_ENABLED=1`）
- [ ] `.github/workflows/ci.yml`（runner: `macos-latest`、`go test -short`、`golangci-lint`、`go build`）

### コマンドテスト

```bash
golangci-lint run
# → エラーなし
echo $?   # → 0

CGO_ENABLED=1 go build ./cmd/op-keychain
# → バイナリが生成される
echo $?   # → 0
```

---

## Step 11: integration test

- [ ] `internal/keychain/*_integration_test.go`（`//go:build integration`）
  - 実際の keychain 作成・読み書き・削除を確認

### コマンドテスト

```bash
go test -tags integration ./...
# → PASS
```

---

## 将来対応（v1 スコープ外）

- `refresh` の並列実行
- `prune` サブコマンド（1Password に存在しない ref のキャッシュを削除）
- `darwin/amd64` 対応（実機確認できたら追加）

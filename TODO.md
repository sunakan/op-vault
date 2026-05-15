# TODO

Go 版実装ステップ。各ステップ完了後に動作確認してから次へ進む。
詳細仕様は SPEC.md を参照。

---

## テスト方針

### keychain の隔離

テスト時は `OP_KEYCHAIN_NAME=op-keychain-test` を付けて実行し、実運用の `op-keychain` keychain を汚さない。

### 自動化スクリプト

Step 1〜6 は `e2e_test.sh` でまとめて自動テストできる。ステップ指定も可能。

```bash
bash e2e_test.sh       # 全ステップ
bash e2e_test.sh 4     # Step 4 のみ
bash e2e_test.sh 5 6   # 複数指定
```

Step 9:  `go test -short ./...`（または `make unit`）
Step 10: `golangci-lint run` + `CGO_ENABLED=1 go build ./cmd/op-keychain`（または `make build`）

### E2E 自動化の可否

| 対象 | 自動化 | 理由 |
|------|--------|------|
| Step 1〜3 | ✅ | 外部依存なし |
| Step 4c `init` 対話 | ⚠️ `expect` 必要 | `/dev/tty` 直接オープンのためパイプ不可 |
| Step 4d `clear` / Step 5〜6 | ✅ | `security lock-keychain` でロック可能 |
| Step 7〜8 | ❌ 手動必須 | Touch ID / macOS 認証ダイアログが必要 |
| Step 9〜11 | ✅ | `go test` / `golangci-lint` |

---

## Step 1: プロジェクト骨格（`--help` が動く）

- [x] ~~`mise.toml` に Go バージョン固定~~ → `go.mod` で十分なため不採用
- [x] `go mod init` + `go get github.com/alecthomas/kong`
- [x] `cmd/op-keychain/main.go`: kong で全サブコマンドをスタブ定義（Run は `fmt.Println("not implemented")` のみ）

### 動作確認

```bash
go run ./cmd/op-keychain --help
echo $?   # → 0
```

### 手動確認

- 出力に9つのサブコマンド（`read` `remove` `clear` `list` `refresh` `status` `set-idle-timeout` `init` `version`）が全て含まれることを目視確認

---

## Step 2: `version` サブコマンド

- [x] `internal/cli/version.go` 実装
- [x] `ldflags` 未設定時は `"dev"` を返す

### 動作確認

```bash
go run ./cmd/op-keychain version
# stdout → op-keychain dev
echo $?   # → 0
```

---

## Step 3: ref バリデーション

- [x] `internal/op/ref.go`: `ParseRef()` 実装
- [x] `read` / `remove` コマンドのスタブに組み込む

### 動作確認

```bash
go run ./cmd/op-keychain read "not-a-ref"; echo $?   # → 2
go run ./cmd/op-keychain read "op://vault-only"; echo $?  # → 2
go run ./cmd/op-keychain read "op://vault/item"; echo $?  # → 0
go run ./cmd/op-keychain read "op://vault/item/field"; echo $?  # → 0
```

---

## Step 4a: keychain パッケージ骨格（interface・型・Service 関数）

- [ ] `internal/keychain/keychain.go`: `Keychain` interface + `ErrLocked` / `ErrNotFound`
- [ ] `internal/keychain/entry.go`: `Entry` 型（JSON marshal/unmarshal）
- [ ] `internal/keychain/service.go`: `Service()` 関数（SHA256）
- [ ] `internal/keychain/service_test.go`: bash 版 `_service()` と同じハッシュ値を返すことを確認

### 動作確認

```bash
go test ./internal/keychain/...
# → PASS（service_test.go が通る）
```

### 手動確認

なし（純粋関数・型定義のみ）

---

## Step 4b: keychain_exec.go（Exists / Create / Delete / AddToList / Path）

- [ ] `internal/keychain/keychain_exec.go` に上記5メソッドを実装

### 動作確認

```bash
go build ./...
# → コンパイルが通ること（実機テストは Step 11）
```

---

## Step 4c: `init` コマンド

- [ ] `internal/cli/init.go` 実装（keychain 自動作成の共通ロジックも含む）

### 動作確認

```bash
# プロンプトに N を入力
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
echo $?   # → 0

security list-keychains | grep op-keychain-test
# → "/Users/.../op-keychain-test.keychain-db"

# 2回目は idempotent（/dev/tty を開かずに即返す）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# stdout → already initialized

# 後片付け
security delete-keychain "$HOME/Library/Keychains/op-keychain-test.keychain-db"
```

### 手動確認

- Keychain Access.app を開き `op-keychain-test` が表示されることを確認
- パスワードあり（`y` 入力）のケース: 入力→確認→成功を確認
- パスワード不一致: `error: passwords do not match` が stderr に出て exit 1 を確認

---

## Step 4d: `clear` コマンド

- [ ] `internal/cli/clear.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
# stdout → cleared all cache
echo $?   # → 0

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
# stdout → no keychain（idempotent）
echo $?   # → 0
```

### 手動確認

- `clear --yes` 後に Keychain Access.app を再起動し、`op-keychain-test` が消えていることを確認
- `--yes` なし → `N` 入力: 何も起きず exit 0 を確認

### Step 4 統合テスト

```bash
bash e2e_test.sh 4
```

---

## Step 5a: keychain_exec.go（GetIdleTimeout / SetIdleTimeout / Unlock / IsLocked）

- [ ] `keychain_exec.go` に上記4メソッドを追加

### 動作確認

```bash
go build ./...
# → コンパイルが通ること
```

---

## Step 5b: `status` コマンド

- [ ] `internal/cli/status.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain status
# → keychain: /Users/.../op-keychain-test.keychain-db
# → idle-timeout: 3600s
# → lock status:  unlocked
# → entries:      0
echo $?   # → 0

# ロック状態で再確認
security lock-keychain "$HOME/Library/Keychains/op-keychain-test.keychain-db"
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain status
# → lock status:  locked
# → entries:      unknown (locked)

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

---

## Step 5c: `set-idle-timeout` コマンド

- [ ] `internal/cli/set_idle_timeout.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout 1800
# stdout → idle-timeout set to 1800s
echo $?   # → 0

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout 0
echo $?   # → 2

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain set-idle-timeout abc
echo $?   # → 2

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### Step 5 統合テスト

```bash
bash e2e_test.sh 5
```

---

## Step 6a: keychain_exec.go（Get / Set / Remove / ListServices）

- [ ] `keychain_exec.go` に上記4メソッドを追加

### 動作確認

```bash
go build ./...
# → コンパイルが通ること
```

---

## Step 6b: `list` コマンド

- [ ] `internal/cli/list.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

# 0件
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# stdout → no cache
echo $?   # → 0

# security コマンドで直接エントリ登録してから確認
REF="op://Test/test02/password"
SVCNAME="op-keychain:$(printf '%s' "$REF" | shasum -a 256 | cut -d' ' -f1)"
JSON="{\"ref\":\"$REF\",\"name\":\"test02\",\"value\":\"dummy-secret\",\"account\":\"\"}"
security add-generic-password -s "$SVCNAME" -a "$(whoami)" \
    -w "$JSON" "$HOME/Library/Keychains/op-keychain-test.keychain-db"

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain list
# →   test02 (op://Test/test02/password)

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

---

## Step 6c: `remove` コマンド

- [ ] `internal/cli/remove.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

# 不正 ref（exit 2）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "op://bad"
echo $?   # → 2

# 存在しない entry（exit 1）
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "op://Test/test02/password"
echo $?   # → 1

# エントリを登録してから remove
REF="op://Test/test02/password"
SVCNAME="op-keychain:$(printf '%s' "$REF" | shasum -a 256 | cut -d' ' -f1)"
JSON="{\"ref\":\"$REF\",\"name\":\"test02\",\"value\":\"dummy-secret\",\"account\":\"\"}"
security add-generic-password -s "$SVCNAME" -a "$(whoami)" \
    -w "$JSON" "$HOME/Library/Keychains/op-keychain-test.keychain-db"

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain remove "$REF"
# stdout → removed: op://Test/test02/password
echo $?   # → 0

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認

- ロック中の `remove` 実行時に unlock ダイアログが出て、空パスワードで自動解除されることを確認

### Step 6 統合テスト

```bash
bash e2e_test.sh 6
```

---

## Step 7a: `internal/op` パッケージ（Client interface + SDK 実装）

**前提: `go get github.com/1password/onepassword-sdk-go` 済み**

- [ ] `internal/op/client.go`: `Client` interface + SDK 実装（`Resolve` / `ItemTitle`）

### 動作確認

```bash
CGO_ENABLED=1 go build ./...
# → コンパイルが通ること
```

---

## Step 7b: `internal/logging` パッケージ

- [ ] `internal/logging/logging.go`: slog 初期化（`OP_KEYCHAIN_DEBUG` 対応）

### 動作確認

```bash
go build ./...
# → コンパイルが通ること
```

---

## Step 7c: `read` コマンド + main.go DI

**前提: 1Password Desktop App が起動・ログイン済みであること**

- [ ] `internal/cli/read.go` 実装（keychain 自動作成 + SDK fetch + keychain 保存）
- [ ] `main.go` に DI（keychain と op.Client の組み立て）を追加

### 動作確認

```bash
# cache miss → SDK 経由（初回）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test \
  go run ./cmd/op-keychain read 'op://Test/test02/password'
# → （値が返る）
echo $?   # → 0

# cache hit（2回目、高速）
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test \
  go run ./cmd/op-keychain read 'op://Test/test02/password'
echo $?   # → 0

# debug ログに value が出ないこと
OP_KEYCHAIN_DEBUG=1 CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test \
  go run ./cmd/op-keychain read 'op://Test/test02/password' 2>&1 | grep -i '"value"'
# → （何も出ないこと）

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認（Touch ID が必要なため自動化不可）

- 初回 `read` 時に 1Password 認証ダイアログ（Touch ID / PIN）が表示されることを確認
- 2回目は認証ダイアログが出ずに即値が返ることを確認（cache hit）
- keychain をロックしてから `read` → unlock ダイアログが出て値が返ることを確認
- `OP_KEYCHAIN_DEBUG=1` のログに value の実値が含まれないことを目視確認

---

## Step 8: `refresh` コマンド

**前提: 1Password Desktop App が起動・ログイン済みであること**

- [ ] `internal/cli/refresh.go` 実装

### 動作確認

```bash
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain init
# → N を入力

# エントリなし
OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain refresh
# stdout → no cache
echo $?   # → 0

# read でエントリ追加 → refresh
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test \
  go run ./cmd/op-keychain read 'op://Test/test02/password'
CGO_ENABLED=1 OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain refresh
# →   refreshed: op://Test/test02/password
# → done: 1 updated, 0 failed
echo $?   # → 0

OP_KEYCHAIN_NAME=op-keychain-test go run ./cmd/op-keychain clear --yes
```

### 手動確認

- 1Password 側の値を変えてから `refresh` → `read` で最新値が返ることを確認

---

## Step 9a: fake 実装 + 純粋関数テスト

- [ ] `internal/keychain/fake/fake.go`: `Keychain` interface の fake 実装
- [ ] `internal/op/fake/fake.go`: `Client` interface の fake 実装
- [ ] `internal/op/ref_test.go`: `ParseRef` テーブル駆動テスト

### 動作確認

```bash
go test -short ./internal/keychain/... ./internal/op/...
# → PASS
```

---

## Step 9b: CLI コマンドテスト

- [ ] `internal/cli/*_test.go`: SPEC §7.3 の必須テストケースを網羅

### 動作確認

```bash
go test -short ./internal/cli/...
# → PASS

# カバレッジ確認
go test -short -coverprofile=cover.out ./...
go tool cover -func=cover.out | grep total
# → 70% 以上
```

---

## Step 10: lint / goreleaser / CI

- [ ] `.golangci.yml` 設定
- [ ] `.goreleaser.yml` 設定（`darwin/arm64` のみ、`CGO_ENABLED=1`）
- [ ] `.github/workflows/ci.yml`（runner: `macos-latest`、`go test -short`、`golangci-lint`、`go build`）

### 動作確認

```bash
golangci-lint run; echo $?   # → 0
CGO_ENABLED=1 go build ./cmd/op-keychain; echo $?   # → 0
```

---

## Step 11: integration test

- [ ] `internal/keychain/*_integration_test.go`（`//go:build integration`）
  - 実際の keychain 作成・読み書き・削除を確認

### 動作確認

```bash
go test -tags integration ./...
# → PASS
```

---

## 将来対応（v1 スコープ外）

- `refresh` の並列実行
- `prune` サブコマンド（1Password に存在しない ref のキャッシュを削除）
- `darwin/amd64` 対応（実機確認できたら追加）

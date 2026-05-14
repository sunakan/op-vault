# TODO

Go 版実装ステップ。各ステップ完了後に挙動を確認してから次へ進む。
詳細仕様は SPEC.md を参照。

---

## Step 1: プロジェクト骨格（`--help` が動く）

- [ ] `mise.toml` に Go バージョン固定
- [ ] `go mod init` + `go get github.com/alecthomas/kong`
- [ ] `cmd/op-keychain/main.go`: kong で全サブコマンドをスタブ定義（Run は `fmt.Println("not implemented")` のみ）

```bash
go run ./cmd/op-keychain --help
# → サブコマンド一覧が出る
```

---

## Step 2: `version` サブコマンド

- [ ] `internal/cli/version.go` 実装
- [ ] `ldflags` 未設定時は `"dev"` を返す

```bash
go run ./cmd/op-keychain version
# → op-keychain dev
```

---

## Step 3: ref バリデーション

- [ ] `internal/op/ref.go`: `ParseRef()` 実装（`op://` 始まり・vault/item の2セグメント確認）
- [ ] `read` / `remove` コマンドのスタブに ref バリデーションを組み込む

```bash
go run ./cmd/op-keychain read "not-a-ref"
# → error: invalid ref format: not-a-ref（exit 2）

go run ./cmd/op-keychain read "op://vault/item"
# → not implemented（exit 0、バリデーションは通る）
```

---

## Step 4: keychain 作成・削除（`init` / `clear`）

- [ ] `internal/keychain/keychain.go`: Keychain interface 定義
- [ ] `internal/keychain/service.go`: `Service()` 関数（SHA256）
- [ ] `internal/keychain/entry.go`: `Entry` 型（JSON）
- [ ] `internal/keychain/keychain_exec.go`: `Exists` / `Create` / `Delete` / `AddToList` / `Path`
- [ ] `internal/cli/init.go` 実装（空パスワード自動作成の共通ロジックも含む）
- [ ] `internal/cli/clear.go` 実装

```bash
go run ./cmd/op-keychain init
# → Set a password for the keychain? [y/N]: N
# → （keychain が作成される）

security list-keychains | grep op-keychain
# → op-keychain.keychain-db がリストに含まれる

go run ./cmd/op-keychain clear --yes
# → cleared all cache

security list-keychains | grep op-keychain
# → 何も出ない
```

---

## Step 5: keychain ロック操作（`status` / `set-idle-timeout`）

- [ ] `keychain_exec.go` に `GetIdleTimeout` / `SetIdleTimeout` / `Unlock` / `IsLocked` を追加
- [ ] `internal/cli/status.go` 実装
- [ ] `internal/cli/set_idle_timeout.go` 実装

```bash
go run ./cmd/op-keychain init

go run ./cmd/op-keychain status
# → keychain: /Users/.../op-keychain.keychain-db
# → idle-timeout: 3600s
# → lock status:  unlocked
# → entries:      0

go run ./cmd/op-keychain set-idle-timeout 1800
# → idle-timeout set to 1800s

go run ./cmd/op-keychain status
# → idle-timeout: 1800s

go run ./cmd/op-keychain set-idle-timeout 0
# → error: seconds must be a positive integer: 0（exit 2）
```

---

## Step 6: keychain エントリ操作（`list` / `remove`）

- [ ] `keychain_exec.go` に `Get` / `Set` / `Remove` / `ListServices` を追加
- [ ] `internal/keychain/keychain.go` に `ErrLocked` / `ErrNotFound` sentinel error 定義
- [ ] `internal/cli/list.go` 実装
- [ ] `internal/cli/remove.go` 実装

```bash
go run ./cmd/op-keychain list
# → no cache（エントリ 0 件）

go run ./cmd/op-keychain remove "op://Test/test02/password"
# → error: cache not found: op://Test/test02/password（exit 1）

go run ./cmd/op-keychain remove "op://bad"
# → error: invalid ref format: op://bad（exit 2）
```

---

## Step 7: 1Password SDK + `read` コマンド

- [ ] `go get github.com/1password/onepassword-sdk-go`
- [ ] `internal/op/client.go`: `Client` interface + SDK 実装（`Resolve` / `ItemTitle`）
- [ ] `internal/logging/logging.go`: slog 初期化（`OP_KEYCHAIN_DEBUG` 対応）
- [ ] `internal/cli/read.go` 実装（keychain 自動作成 + SDK fetch + keychain 保存）
- [ ] `main.go` に DI（keychain と op.Client の組み立て）を追加

```bash
CGO_ENABLED=1 go run ./cmd/op-keychain read 'op://Test/test02/password'
# → （値が返る。cache miss → SDK 経由）

CGO_ENABLED=1 go run ./cmd/op-keychain read 'op://Test/test02/password'
# → （値が返る。cache hit → keychain から即返却）

go run ./cmd/op-keychain list
# → エントリが 1 件表示される

OP_KEYCHAIN_DEBUG=1 CGO_ENABLED=1 go run ./cmd/op-keychain read 'op://Test/test02/password'
# → debug ログが stderr に出る（value 本体は出ない）
```

---

## Step 8: `refresh` コマンド

- [ ] `internal/cli/refresh.go` 実装

```bash
CGO_ENABLED=1 go run ./cmd/op-keychain refresh
# →   refreshed: op://Test/test02/password
# → done: 1 updated, 0 failed
```

---

## Step 9: ユニットテスト

- [ ] `internal/keychain/fake/fake.go`: Keychain interface の fake 実装
- [ ] `internal/op/fake/fake.go`: Client interface の fake 実装
- [ ] `internal/op/ref_test.go`: `ParseRef` テーブル駆動テスト
- [ ] `internal/keychain/service_test.go`: bash 版 `_service()` と同じハッシュ値を返すことを確認
- [ ] `internal/cli/*_test.go`: SPEC §7.3 の必須テストケースを網羅

```bash
go test -short ./...
# → PASS、カバレッジ 70% 以上（internal/cli, internal/keychain, internal/op）
```

---

## Step 10: lint / goreleaser / CI

- [ ] `.golangci.yml` 設定
- [ ] `.goreleaser.yml` 設定（`darwin/arm64` のみ、`CGO_ENABLED=1`）
- [ ] `.github/workflows/ci.yml`（runner: `macos-latest`、`go test -short`、`golangci-lint`、`go build`）

```bash
golangci-lint run
# → エラーなし

CGO_ENABLED=1 go build ./cmd/op-keychain
# → バイナリが生成される
```

---

## Step 11: integration test

- [ ] `internal/keychain/*_integration_test.go`（`//go:build integration`）
  - 実際の keychain 作成・読み書き・削除を確認

```bash
go test -tags integration ./...
```

---

## 将来対応（v1 スコープ外）

- `refresh` の並列実行
- `prune` サブコマンド（1Password に存在しない ref のキャッシュを削除）
- `darwin/amd64` 対応（実機確認できたら追加）

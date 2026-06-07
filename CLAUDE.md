# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

`op-vault` は `op read`（1Password CLI）の結果を macOS Keychain にキャッシュする CLI ツール。`op read` の ~1.8s のレイテンシをキャッシュで回避するのが目的。

## Commands

```bash
make build                  # バイナリビルド（CGO_ENABLED=1 が必要）
make test                   # 単体テスト（go test ./...）
make e2e-test               # E2E テスト（ビルド → 実バイナリで CLI 動作検証）
make e2e-test-integration   # 実際に 1Password から読む統合テスト（OP_ACCOUNT=xxx が必須）
make lint                   # golangci-lint + shellcheck + cppcheck + clang-analyzer + clang-tidy
make fmt                    # go fmt + golangci-lint fmt（goimports）+ shfmt + clang-format
make clean                  # バイナリ削除
```

ビルドは darwin 専用（`//go:build darwin`）。`CGO_ENABLED=1` が必須（macOS Keychain API を CGO 経由で呼ぶため）。

`make fmt` / `make lint` は `mise exec --` 経由で golangci-lint・shfmt・shellcheck を実行する。`mise` が PATH にない場合は失敗する（Go と同様に必須依存）。ツールバージョンは `mise.toml` で管理。

`make lint` の C lint（`c.lint`）は `cppcheck` と Homebrew `llvm`（clang-tidy 提供）が必要。`brew install cppcheck llvm`。

## Architecture

```
cmd/op-vault/main.go   エントリポイント。run() に処理を委譲して os.Exit のみ main で呼ぶ
internal/cli/          kong サブコマンド実装
internal/keychain/     macOS Keychain 操作（CGO）
internal/op/           1Password SDK ラッパー（op.Resolve で op:// ref を解決）
internal/tracing/      OTel 計装（TracerProvider 初期化・Tracer アクセサ・スパンユーティリティ）
scripts/e2e-test.sh    実バイナリを直接実行する E2E テスト
```

### read コマンドのフロー

`keychain.Get` → キャッシュヒット → 値を返す  
`CacheMissError` → `op.Resolve`（1Password SDK）→ `keychain.Set` でキャッシュ書き込み → 値を返す  
`NotFoundError` → キーチェーンファイル未存在（`op-vault init` 未実行）

キーチェーンの保存形式: `service=op_ref`・`account=OP_ACCOUNT`・data は `Entry{Ref, ItemName, Value}` の JSON。
キーチェーンパス: `~/Library/Keychains/<OP_VAULT_NAME>.keychain-db`（デフォルト: `op-vault`）。

### CLI フレームワーク

`alecthomas/kong` を使用（struct tag ベース）。コンテキスト注入は `kong.BindFor[context.Context](ctx)` で行う。

**重要**: `kong.Parse()` がパースエラー（未知サブコマンド・不正オプション）を検出した場合、`kong.Exit` コールバックを呼び出して `os.Exit` する。`kongCtx.Run()` には到達しないため、`run()` 関数の `defer` も実行されない。スパンの `End()` と TracerProvider の `Shutdown()` は `kong.Exit` コールバック内で明示的に呼ぶ必要がある。

### OTel 計装

- `tracing.Init()` で TracerProvider を初期化し shutdown 関数を返す
- `tracing.Tracer()` で呼び出し元パッケージパスを自動でトレーサー名にする（`runtime.Caller` を使用）
- `tracing.SetSpanError(span, err)` でエラーを記録する（`RecordError` だけではステータスが `Error` にならないため）
- exporter は `OP_VAULT_TRACES_EXPORTER` 環境変数で切り替える（`none`（デフォルト）/ `stdout` / `otlp`）
- `otlp` の場合は `OP_VAULT_OTLP_ENDPOINT` が必須

`OTEL_RESOURCE_ATTRIBUTES` がシェルに設定されている場合、`WithResource()` が内部で `resource.Environment()` とマージするため、その属性が Resource に混入する（SDK の意図的な仕様）。

### semconv バージョン

`go.opentelemetry.io/otel/semconv/vX.Y.Z` のサブディレクトリバージョンは OTel Go SDK のモジュールバージョンと独立している。使用可能なバージョンは以下で確認する。

```bash
ls $(go env GOMODCACHE)/go.opentelemetry.io/otel@<version>/semconv/ | sort -V | tail -5
```

## 機能の追加・削除順序

- **追加・更新**: `scripts/e2e-test.sh` にテストを先に書き、実装はその後
- **削除**: 実装（`internal/cli/`・`internal/keychain/` 等）を先に消し、`e2e-test.sh` のテストを後で削除

## Import 順序

`make fmt`（goimports）で自動整形される。標準ライブラリ → サードパーティ → `github.com/sunakan/op-vault/...` の 3 グループ。

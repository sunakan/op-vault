# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

`op-vault` は `op read`（1Password CLI）の結果を macOS Keychain にキャッシュする CLI ツール。`op read` の ~1.8s のレイテンシをキャッシュで回避するのが目的。

## Commands

```bash
make build       # バイナリビルド（CGO_ENABLED=1 が必要）
make test        # 単体テスト（go test ./...）
make e2e-test    # E2E テスト（ビルド → 実バイナリで CLI 動作検証）
make lint        # golangci-lint + shellcheck（Docker 必須）
make fmt         # go fmt + golangci-lint fmt（goimports） + shfmt（Docker 必須）
make clean       # バイナリ削除
```

ビルドは darwin 専用（`//go:build darwin`）。`CGO_ENABLED=1` が必須（macOS Keychain API を CGO 経由で呼ぶため）。

`make fmt` / `make lint` の shfmt・shellcheck は Docker コンテナ経由で実行する。Docker が起動していないと失敗する。

## Architecture

```
cmd/op-vault/main.go   エントリポイント。run() に処理を委譲して os.Exit のみ main で呼ぶ
internal/cli/             kong サブコマンド実装
internal/tracing/         OTel 計装（TracerProvider 初期化・Tracer アクセサ・スパンユーティリティ）
scripts/e2e-test.sh       実バイナリを直接実行する E2E テスト
```

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

## Import 順序

`make fmt`（goimports）で自動整形される。標準ライブラリ → サードパーティ → `github.com/sunakan/op-vault/...` の 3 グループ。

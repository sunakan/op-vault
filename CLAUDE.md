# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

1Password (`op read`) の結果を macOS 専用 Keychain にキャッシュする CLI ツール。`.envrc` から `$(op-keychain read 'op://...')` で呼ぶことで direnv によるシークレット注入を高速化する。

**bash 版 (`op-keychain.sh`) を Go に移植中。** Go 版の仕様は `SPEC.md` を参照。Go 版リリース後は `op-keychain.sh` を削除する。

## コマンド (Go 版)

```bash
CGO_ENABLED=1 go build ./cmd/op-keychain   # ビルド(CGO 必須)
go test -short ./...                        # ユニットテスト(実機テストをスキップ)
go test -tags integration ./...             # インテグレーションテスト(実機 Mac のみ)
golangci-lint run                           # lint
```

テスト用 ref: `op://Test/test02/password`（本物の ref をデバッグで使うと secret が出力に混入する）

## アーキテクチャ (Go 版)

mock 境界は `internal/keychain`（`security` コマンドラッパ）と `internal/op`（1Password SDK ラッパ）の interface。`internal/cli/` の各ハンドラはこの2つの interface に依存し、fake に差し替えてテストする。

```
cmd/op-keychain/main.go   ← kong CLI 定義・DI
internal/cli/             ← サブコマンドごとのハンドラ
internal/keychain/        ← security exec ラッパ (mock 境界)
internal/op/              ← 1Password SDK ラッパ (mock 境界)
internal/logging/         ← slog 初期化
```

## 実装上の注意

**JSON は純 ASCII で保存すること。** `security find-generic-password -w` は非 ASCII バイトを hex 文字列で返してパースが壊れる。`encoding/json.Marshal` はデフォルトで非 ASCII を `\uXXXX` にエスケープするため追加設定不要。

**`find-generic-password -w` の hex 出力への対処。** 出力が `0x` で始まる場合は hex decode してから JSON パースすること。

**`Unlock()` は2ステップを内包する。** `-p ""` で silent 試行 → 失敗時のみ macOS GUI ダイアログ表示。caller は `Unlock()` を呼ぶだけでよい。

**`ErrLocked` と `ErrNotFound` を使い分けること。** `Get()`/`Set()`/`Remove()` はこれらの sentinel error を返す。`ErrLocked` の場合のみ unlock してリトライ。`ErrNotFound` では即エラーを返す。

**kong のパースエラーは exit code 2 に統一する。** `kong.Exit(func(code int) { if code != 0 { os.Exit(2) }; os.Exit(0) })` で設定すること（§4.6 の exit code 規約）。

**`CGO_ENABLED=1` 必須。** 1Password SDK の Desktop App Integration に必要。darwin 専用なので `//go:build darwin` でガードする。

Keychain Access GUI は CLI 変更を即時反映しない。操作後に GUI を再起動して確認すること。
1Password セッションは `Cmd+Shift+L`（Lock Now）で強制失効させてテストできる。

## スキル

- **`/sync-docs`**: 実装変更後に SPEC.md・CLAUDE.md・README.md・README.ja.md を一括で実装と同期させる。

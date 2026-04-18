# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

`op-keychain.sh` は `op read`（1Password CLI）の結果を macOS キーチェーンにキャッシュするシェルスクリプト。IDLE_TIMEOUT（デフォルト1時間）付きで、キーチェーンの非アクティブ自動ロック機能を利用してキャッシュ期限を管理する。

## コマンド

```bash
make test          # op-keychain.sh read でキャッシュ読み取りをテスト
make list          # キャッシュ一覧を表示
make clear         # キャッシュ全削除
make refresh       # 全キャッシュを再取得
make status        # キーチェーンの状態を表示
make update-idle-timeout SECONDS=1800  # 自動ロックまでの時間を変更
make fmt-sh        # op-keychain.sh を shfmt でフォーマット (Docker 使用)
make lint-sh       # op-keychain.sh を shellcheck で lint (Docker 使用)
```

手動実行:
```bash
./op-keychain.sh read <op://vault/item/field>   # キャッシュ付き読み取り
./op-keychain.sh remove <op://...>              # 指定エントリを削除
./op-keychain.sh clear                          # キャッシュ全削除
./op-keychain.sh list                           # キャッシュ一覧
./op-keychain.sh refresh                        # 全キャッシュを再取得
./op-keychain.sh status                         # キーチェーンの状態を表示
./op-keychain.sh update-idle-timeout <秒数>     # 自動ロックまでの時間を変更
```

デバッグ:
```bash
OP_KEYCHAIN_DEBUG=true ./op-keychain.sh read 'op://Test/test02/password'
```

## アーキテクチャ

`op-keychain.sh` 単一ファイル構成。仕様の詳細は `SPEC.md` を参照。

- **`_service <ref>`**: ref の SHA256 ハッシュを `op-keychain:<hash>` 形式のサービス名に変換。任意の ref（UUID・スラッシュ・日本語含む）を安全に扱うため。
- **`_init_keychain`**: `~/Library/Keychains/op-keychain.keychain-db` を初回のみ作成。作成時にパスワード設定を `/dev/tty` 経由でインタラクティブに確認する（デフォルト: 空パスワード）。
- **`_unlock_keychain`**: まず空パスワードで無音アンロックを試み、失敗時のみユーザーにプロンプトを表示。**プロセス置換 `< <(...)` 内から呼ばないこと**（stdout がパイプになりプロンプトが消えるため）。
- **`_item_name <ref>`**: `op item get` でアイテムタイトルを取得。ref から vault と item を抽出して呼び出す。失敗時は空文字を返す。
- **`_dump_entries`**: `security dump-keychain`（データなし）でサービス名（`op-keychain:<hash>`）を列挙し、各サービスに対して `find-generic-password -w` で JSON を個別取得 → `name\tref` 形式で出力。`dump-keychain -d` は非 ASCII データを hex 形式で出力するため使わない。前提: キーチェーンがアンロック済みであること。
- **`_list_refs`**: `_dump_entries | cut -f2` で ref のみ出力。`cmd_refresh` が使用。
- **`cmd_read`**: アンロックせず読み取り試行 → ヒットで即返却 / ミスで `op read` + `_item_name` → アンロックなしで保存試行 → 失敗時のみ `_unlock_keychain` → 保存。
- **`cmd_list`**: `_unlock_keychain` → `_dump_entries` で `アイテム名 (ref)` 形式で列挙。
- **`cmd_refresh`**: `_unlock_keychain` → `_list_refs` で ref 一覧収集 → セッション未確立なら最初の ref を直列で `op read` + `_item_name`（認証1回） → 残りを並行実行 → キーチェーンへ直列書き込み。
- **`cmd_status`**: `security show-keychain-info` で IDLE_TIMEOUT を取得。`dump-keychain`（ロック中でも動作）でサービス名を列挙し、最初のエントリに `find-generic-password` でアクセスを試みてロック状態を判定（副作用なし）。アンロック中ならエントリ数も表示。
- **`cmd_update_idle_timeout`**: `security set-keychain-settings -t <秒数>` でキーチェーンの非アクティブ自動ロック時間を更新。環境変数 `OP_KEYCHAIN_IDLE_TIMEOUT`（デフォルト: 3600）は初回キーチェーン作成時のみ使用。作成後は本コマンドで変更する。

## スキル

- **`/sync-docs`**: 実装変更後に SPEC.md・CLAUDE.md・README.md・README.ja.md を一括で実装と同期させる。

## 注意事項

- macOS 専用（`security` コマンド・キーチェーン依存）。
- キーチェーンに保存する JSON は `jq -cna`（`--ascii-output`）で構築すること。非 ASCII 文字を含む JSON を保存すると、`security find-generic-password -w` が hex 形式で返し jq パースが壊れる。
- Keychain Access GUI は CLI でキーチェーンを変更しても**リアルタイムで反映されない**。操作後に GUI を閉じて開き直す。
- テスト用の ref は `op://Test/test02/password` など専用のものを使う。本物の ref をデバッグコマンドで実行すると secret が出力に混入する。
- `cmd_refresh` の並行 `op read` は macOS Keychain の並行書き込み非サポートのため、書き込みのみ直列化している（詳細は SPEC.md「キーチェーンの並行アクセス制約」参照）。
- 1Password デスクトップアプリ連携使用。セッションは `Cmd+Shift+L`（Lock Now）で強制失効させてテストできる。

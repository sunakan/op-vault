# op-cache vs op-fast

## 一言で表すと

| | 思想 |
|---|---|
| **op-fast** | `op` コマンド全体を置き換えるプロキシ |
| **op-cache** | `op read` のキャッシュレイヤーに特化したラッパー |

---

## スコープの違い

**op-fast** は `op` の完全な代替を目指している。`read` / `inject` / `run` / `store` を独自実装し、未知のサブコマンド（`op item list` 等）は本物の `op` にパススルーする。ユーザーは `op` を `op-fast` に差し替えるだけで既存のワークフローが動く。

**op-cache** は `op read` の結果をキャッシュすることだけに集中している。`op` の代替ではなく、`op read` の前に挟むラッパー。

---

## キャッシュ設計の違い

### ストレージ

| | メタデータ | シークレット値 |
|---|---|---|
| **op-fast** | LMDB（ファイルDB） | OSキーリング（Keychain / keyutils） |
| **op-cache** | macOSキーチェーンのJSON内に埋め込み | macOSキーチェーン |

op-fast はメタデータ（ref・有効期限）とシークレット値を分離して管理する。op-cache はすべてキーチェーンのJSON1フィールドに収めるシンプルな設計。

### キャッシュ期限の管理

**op-fast** は明示的なTTLで管理する。設定ファイル（`~/.config/op-fast/config.toml`）でデフォルトTTLやglobパターンによるシークレット単位の細かい設定が可能。

```toml
default_ttl = "1day"
[ttl]
"op://prod/*" = "1hour"
"op://*/ssh/*" = "30days"
```

**op-cache** はmacOSキーチェーンの「非アクティブ自動ロック」機能をそのまま期限管理に流用する。アクセスがあるとタイマーがリセットされ、IDLE_TIMEOUT秒アクセスがなければ自動ロック（= キャッシュ期限切れ）。外部ファイルや設定不要。

---

## シークレット取得の違い

**op-fast** は `inject` / `run` コマンドで**バッチフェッチ**を行う。未キャッシュのrefを一括収集し、`op inject` に stdin 経由で渡すことで1回のサブプロセスで複数シークレットを取得する。

**op-cache** の `refresh` は `op read` を並行実行する。1Passwordへのリクエストは複数発生するが並行なので実用上十分速い。

---

## 機能比較

| 機能 | op-fast | op-cache |
|---|---|---|
| シークレット読み取り（キャッシュ付き） | `read` | `read` |
| テンプレートへの埋め込み | `inject` | なし |
| 環境変数として子プロセスに注入 | `run` | なし |
| キャッシュ一覧 | `store list` | `list` |
| キャッシュ削除 | `store clear` | `clear` / `remove` |
| キャッシュ再取得 | なし（TTL切れで自動） | `refresh` |
| 状態確認 | なし | `status` |
| IDLE_TIMEOUT変更 | config.toml | `update-idle-timeout` |
| 未知コマンドのパススルー | あり（`op` へ委譲） | なし |
| シークレットマスキング | あり（`run` 時のログ保護） | なし |

---

## 対応プラットフォーム

| | macOS | Linux |
|---|---|---|
| **op-fast** | Keychain | keyutils |
| **op-cache** | Keychain のみ | 非対応 |

---

## 複雑さ・配布

| | op-fast | op-cache |
|---|---|---|
| 実装 | Rust（~2000行） | bash（~400行） |
| 依存 | LMDB + OS keyring crate | `security` / `jq` / `shasum` |
| インストール | `brew install` / `cargo install` | スクリプト1ファイルをコピー |
| ビルド | 必要 | 不要 |

---

## まとめ

op-fast は「`op` をもっと速く・便利に使いたい」という目的で、`inject` / `run` のような高度な機能も含むフルスタックなツール。

op-cache は「`op read` を呼ぶたびに1Passwordに通信するのを避けたい」という一点に絞ったツール。macOSキーチェーン以外の依存を持たず、単一ファイルで完結することをシンプルさの根拠としている。

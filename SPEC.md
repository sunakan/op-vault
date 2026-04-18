# SPEC.md

## 概要

`op read`（1Password CLI）は毎回 1Password サーバーへの通信が発生するため低速。`op-keychain` は結果を macOS キーチェーンにキャッシュし、IDLE_TIMEOUT 内にアクセスが続く限り `op read` を呼ばずに即座に値を返す。

---

## サブコマンド仕様

### 共通仕様

- 不明なサブコマンド・引数なし: 下記 usage を **stdout** に出力して終了コード 0
  ```
  usage: op-keychain read                 <op://...>  # キャッシュ付きで値を読み取る
         op-keychain remove               <op://...>  # 指定エントリを削除
         op-keychain clear                            # キャッシュ全削除
         op-keychain list                             # キャッシュ一覧を表示
         op-keychain refresh                          # 全キャッシュを再取得
         op-keychain status                           # キーチェーンの状態を表示
         op-keychain update-idle-timeout  <秒数>      # 自動ロックまでの時間を変更
  ```

---

### `op-keychain read <op://vault/item[/field]>`

キャッシュヒット時はキーチェーンから値を返す。ミス時は `op read` で取得してキャッシュに保存してから返す。

出力は `op read` と同様に**改行なし**。

| 条件 | 動作 | stdout | stderr | 終了コード |
|------|------|--------|--------|-----------|
| `<ref>` 引数なし | 即終了 | なし | `usage: op-keychain read <op://...>` | 1 |
| キャッシュヒット（アンロック中かつエントリあり） | キャッシュから値を返す。`op read` は呼ばない | 値（改行なし） | なし | 0 |
| キャッシュミス（ロック中または未キャッシュ）、`op read` 成功 | `op read` で取得 → アイテム名取得 → 保存 → 値を返す | 値（改行なし） | なし | 0 |
| キャッシュミス、`op read` 失敗 | 即終了 | なし | `error: op read に失敗しました: <ref>` | 1 |

**フロー:**
1. キーチェーンファイルが存在しない場合は初期化（後述「キーチェーン初期化」参照）
2. アンロックせずに `security find-generic-password -a <USER> -s <service> -w <keychain>` を試みる
   - 成功（キャッシュヒット）: 返却された JSON の `.value` をパース
     - パース成功: 値を改行なしで stdout に出力して終了
     - パース失敗（JSON が壊れている等）: キャッシュミス扱いで次のステップへ
   - 失敗（キャッシュミス）: 次のステップへ
3. `op read <ref>` を実行（後述「外部コマンド呼び出し」参照）
   - 失敗: `error: op read に失敗しました: <ref>` を stderr に出力して終了コード 1
4. アイテム名を取得（後述「アイテム名取得」参照）。失敗時は空文字
5. JSON を構築（後述「データ形式」・「JSON エンコード」参照）
6. アンロックせずに `security add-generic-password -U -a <USER> -s <service> -w <json> <keychain>` を試みる
   - 失敗（ロック中）: アンロック（後述「アンロック処理」参照）してから再試行
7. 取得した value を改行なしで stdout に出力

---

### `op-keychain list`

キャッシュされている ref の一覧を表示する。

| 条件 | 動作 | stdout | stderr | 終了コード |
|------|------|--------|--------|-----------|
| キーチェーンファイルが存在しない | 即終了 | `キャッシュなし` | なし | 0 |
| キーチェーンがロック中 | アンロック後に一覧表示 | 以下参照 | なし | 0 |
| エントリあり（name あり） | name と ref を表示 | `  <name> (<ref>)`（2スペースインデント） | なし | 0 |
| エントリあり（name なし） | ref のみ表示 | `  <ref>`（2スペースインデント） | なし | 0 |
| エントリなし | | `キャッシュなし` | なし | 0 |

**フロー:**
1. キーチェーンファイルが存在しない場合: `キャッシュなし` を stdout に出力して終了
2. アンロック（後述「アンロック処理」参照）
3. エントリ一覧を取得（後述「エントリ一覧取得」参照）
4. エントリが0件の場合: `キャッシュなし` を stdout に出力
5. 各エントリを `  <name> (<ref>)` または `  <ref>` 形式で stdout に出力

---

### `op-keychain refresh`

キャッシュ済みの全 ref を `op read` で再取得し、キーチェーンを更新する。

| 条件 | 動作 | stdout | stderr | 終了コード |
|------|------|--------|--------|-----------|
| キーチェーンファイルが存在しない | 即終了 | `キャッシュなし` | なし | 0 |
| キーチェーンがロック中 | アンロック後に実行 | 以下参照 | なし | 0 |
| エントリなし | 即終了 | `キャッシュなし` | なし | 0 |
| 全件成功 | | `  refreshed: <ref>`（各件）+ `完了: X 件更新, 0 件失敗` | なし | 0 |
| 一部失敗 | 失敗分をスキップして続行 | `  refreshed: <ref>`（成功分）+ `完了: X 件更新, Y 件失敗` | `  skip (op read 失敗): <ref>`（失敗分） | 0 |

**フロー:**
1. キーチェーンファイルが存在しない場合: `キャッシュなし` を stdout に出力して終了
2. アンロック（後述「アンロック処理」参照）
3. エントリ一覧から ref 一覧を収集（後述「エントリ一覧取得」参照）
4. ref が0件の場合: `キャッシュなし` を stdout に出力して終了
5. セッション確認（後述「1Password セッション管理」参照）
6. `op read` を並行実行（後述「並行処理の詳細」参照）
7. キーチェーンへ直列書き込み（後述「キーチェーンの並行アクセス制約」参照）
8. `完了: X 件更新, Y 件失敗` を stdout に出力

**並行処理の詳細:**

```
[op read ref0] [op read ref1] [op read ref2]  ← 並行実行（op read は遅いため効果的）
      ↓               ↓               ↓
[add-generic-password ref0 → ref1 → ref2]     ← 直列実行（Keychain の制約）
```

各 ref に対して並行に以下を実行し、全完了を待機してから直列書き込みへ:
- `op read` 成功: 値とアイテム名（後述「アイテム名取得」参照）を取得して結果として保持
  - アイテム名取得に失敗した場合は空文字として保持
- `op read` 失敗: エラーとして記録

直列書き込み時:
- エラー記録あり: `  skip (op read 失敗): <ref>` を stderr に出力してスキップ
- 成功: JSON を構築（後述「JSON エンコード」参照）して `security add-generic-password -U` で上書き保存し、`  refreshed: <ref>` を stdout に出力

**実装方法（言語別）:**
- **bash**: バックグラウンドプロセス（`&`）+ 一時ファイルで結果を受け渡し
- **Go**: goroutine + `sync.WaitGroup` + slice（インデックスで対応）
- **Rust**: `std::thread::spawn` または `tokio::spawn` + 結果収集

---

### `op-keychain remove <op://vault/item[/field]>`

指定した ref のエントリのみキーチェーンから削除する。

| 条件 | 動作 | stdout | stderr | 終了コード |
|------|------|--------|--------|-----------|
| `<ref>` 引数なし | 即終了 | なし | `usage: op-keychain remove <op://...>` | 1 |
| キーチェーンファイルが存在しない | 即終了 | なし | `キャッシュなし` | 1 |
| エントリが存在する（アンロック中） | 削除成功 | `削除しました: <ref>` | なし | 0 |
| エントリが存在する（ロック中） | アンロック後に削除 | `削除しました: <ref>` | なし | 0 |
| エントリが存在しない | 即終了 | なし | `error: キャッシュが見つかりません: <ref>` | 1 |

**フロー:**
1. `<ref>` 引数なし: `usage: op-keychain remove <op://...>` を stderr に出力して終了コード 1
2. キーチェーンファイルが存在しない: `キャッシュなし` を stderr に出力して終了コード 1
3. アンロックせずに `security delete-generic-password -a <USER> -s <service> <keychain>` を試みる
   - 成功: `削除しました: <ref>` を stdout に出力して終了コード 0
   - 失敗: アンロック（後述「アンロック処理」参照）してから再試行
4. 再試行も失敗した場合: `error: キャッシュが見つかりません: <ref>` を stderr に出力して終了コード 1

---

### `op-keychain status`

キーチェーンの現在の状態を表示する。アンロック操作は行わない。

| 条件 | stdout | stderr | 終了コード |
|------|--------|--------|-----------|
| キーチェーンファイルが存在しない | `キーチェーン: なし` | なし | 0 |
| キーチェーンあり・エントリなし | キーチェーン + IDLE_TIMEOUT + `エントリ数: 0件` | なし | 0 |
| キーチェーンあり・アンロック中 | キーチェーン + IDLE_TIMEOUT + `ロック状態: アンロック中` + `エントリ数: N件` | なし | 0 |
| キーチェーンあり・ロック中 | キーチェーン + IDLE_TIMEOUT + `ロック状態: ロック中` + `エントリ数: 不明（ロック中のため）` | なし | 0 |

**出力例（アンロック中）:**
```
キーチェーン: あり (/Users/user/Library/Keychains/op-keychain.keychain-db)
IDLE_TIMEOUT: 3600秒
ロック状態:   アンロック中
エントリ数:   3件
```

**フロー:**
1. キーチェーンファイルが存在しない: `キーチェーン: なし` を stdout に出力して終了
2. `キーチェーン: あり (<パス>)` を stdout に出力
3. IDLE_TIMEOUT を取得: `security show-keychain-info <keychain>` を実行（ロック中でも動作する）
   - 出力形式: `Keychain "..." timeout=Ns`
   - `timeout=0s`: `IDLE_TIMEOUT: なし（自動ロックなし）` を出力
   - `timeout=Ns`（N > 0）: `IDLE_TIMEOUT: N秒` を出力
   - `timeout=` が見つからない: `IDLE_TIMEOUT: 不明` を出力
4. `security dump-keychain <keychain>`（ロック中でも動作）でサービス名を列挙
5. `op-keychain:` プレフィックスを持つサービス名の件数を取得
   - 0件: `エントリ数: 0件` を出力して終了
6. 最初のサービス名に対して `security find-generic-password -a <USER> -s <service> -w <keychain>` を試みる（副作用なし）
   - 成功（アンロック中）: `ロック状態: アンロック中` + `エントリ数: N件` を出力
   - 失敗（ロック中）: `ロック状態: ロック中` + `エントリ数: 不明（ロック中のため）` を出力

---

### `op-keychain update-idle-timeout <秒数>`

キーチェーンの非アクティブ自動ロックまでの時間を変更する。

| 条件 | stdout | stderr | 終了コード |
|------|--------|--------|-----------|
| `<秒数>` 引数なし | なし | `usage: op-keychain update-idle-timeout <秒数>` | 1 |
| `<秒数>` が正の整数でない | なし | `error: 秒数は正の整数で指定してください: <値>` | 1 |
| キーチェーンファイルが存在しない | なし | `キャッシュなし` | 1 |
| 成功 | `idle-timeout を <秒数>秒 に設定しました` | なし | 0 |

**フロー:**
1. 引数なし: `usage: op-keychain update-idle-timeout <秒数>` を stderr に出力して終了コード 1
2. 正の整数でない場合: エラーを stderr に出力して終了コード 1
3. キーチェーンファイルが存在しない: `キャッシュなし` を stderr に出力して終了コード 1
4. `security set-keychain-settings -t <秒数> <keychain>` を実行
5. `idle-timeout を <秒数>秒 に設定しました` を stdout に出力

---

### `op-keychain clear`

キーチェーン全体を削除する。

| 条件 | stdout | stderr | 終了コード |
|------|--------|--------|-----------|
| キーチェーンファイルが存在しない | `キャッシュなし` | なし | 0 |
| キーチェーンファイルが存在する | `全キャッシュをクリアしました` | なし | 0 |

**フロー:**
1. キーチェーンファイルが存在しない: `キャッシュなし` を stdout に出力して終了
2. `security delete-keychain <keychain>` を実行
3. `全キャッシュをクリアしました` を stdout に出力

---

## データ形式

- **キーチェーンファイル**: `~/Library/Keychains/op-keychain.keychain-db`
- **サービス名**: `op-keychain:<SHA256(ref)>` の lowercase hex 文字列（64文字）
  - ref を SHA256 ハッシュ化することで UUID・スラッシュ・日本語等を含む任意の ref を安全に扱う
  - 例: `"op://vault/item/field"` の SHA256 → `op-keychain:7f42d594...`
- **アカウント名**: 実行ユーザー名（`$USER` / `os.Getenv("USER")`）
- **パスワード値**: 以下の JSON 文字列（**純 ASCII 必須**）

```json
{"ref": "op://vault/item/field", "name": "Item Title", "value": "<secret>"}
```

- `ref`: サービス名がハッシュでも `list` / `refresh` で元の ref を復元するために保持
- `name`: 1Password アイテム名（「アイテム名取得」参照）。取得失敗時は空文字
- `value`: `op read <ref>` の出力値

**JSON は必ず純 ASCII で構成すること。** `security find-generic-password -w` は非 ASCII バイトを含むデータを hex 文字列（例: `7b22726566...`）で返すため、そのままでは JSON パースが壊れる。

---

## JSON エンコード

キーチェーンに保存する JSON を構築する際は、非 ASCII 文字を `\uXXXX` 形式にエスケープすること。

| 実装言語 | 方法 |
|---------|------|
| bash | `jq -cna --arg ref ... --arg name ... --arg value ...` （`-a` = `--ascii-output`） |
| Go | `encoding/json.Marshal()` — デフォルトで非 ASCII を `\uXXXX` エスケープ。追加設定不要 |
| Rust | `serde_json::to_string()` — デフォルトで非 ASCII を `\uXXXX` エスケープ。追加設定不要 |

読み取り時（デシリアライズ）も各言語の標準 JSON ライブラリが `\uXXXX` → Unicode 文字列に透過的に変換する。

---

## 内部処理の詳細

### 外部コマンド呼び出し

すべての外部コマンドはサブプロセスとして実行する。

| コマンド | 目的 | 使用する出力 | 備考 |
|---------|------|------------|------|
| `op read <ref>` | シークレット取得 | stdout（末尾改行除去） | 終了コード非ゼロ = 失敗 |
| `op item get --vault <vault> <item> --format json` | アイテム名取得 | stdout（JSON） | 失敗時は空文字扱い |
| `op whoami` | セッション確認 | 不要（終了コードのみ） | 0 = 確立済み、非ゼロ = 未確立 |
| `security find-generic-password -a <USER> -s <service> -w <keychain>` | キャッシュ読み取り | stdout（JSON文字列） | 終了コード非ゼロ = 未キャッシュまたはロック中 |
| `security add-generic-password -U -a <USER> -s <service> -w <json> <keychain>` | キャッシュ書き込み | 不要 | `-U` = 既存エントリ上書き |
| `security delete-generic-password -a <USER> -s <service> <keychain>` | エントリ削除 | 不要 | 終了コード非ゼロ = 未存在またはロック中 |
| `security delete-keychain <keychain>` | キーチェーン削除 | 不要 | |
| `security dump-keychain <keychain>` | サービス名一覧 | stdout（テキスト） | `-d` なし（データ不要） |
| `security unlock-keychain -p <password> <keychain>` | アンロック（無音） | 不要 | `-p ""` で空パスワード試行 |
| `security unlock-keychain <keychain>` | アンロック（プロンプト） | 不要 | パスワードプロンプトを表示 |
| `security create-keychain -p <password> <name>` | キーチェーン作成 | 不要 | |
| `security set-keychain-settings -t <seconds> <keychain>` | 自動ロック設定 | 不要 | |
| `security show-keychain-info <keychain>` | 自動ロック設定値確認 | stdout（テキスト） | ロック中でも動作する |
| `security list-keychains -d user` | キーチェーンリスト取得 | stdout（テキスト） | |
| `security list-keychains -s <paths...>` | キーチェーンリスト設定 | 不要 | |

**`op read` の出力について**: `op read` は値を末尾改行なしで出力する。サブプロセスとして実行した場合、OS が末尾に改行を付加することがあるため、取得後に末尾改行を除去すること。

### アンロック処理

キーチェーンをアンロックする。空パスワードで無音試行し、失敗時のみユーザーにプロンプトを表示する。

1. `security unlock-keychain -p "" <keychain>` を実行（stderr は捨てる）
   - 成功: 完了
   - 失敗: 次のステップへ
2. `security unlock-keychain <keychain>` を実行

**注意:** ステップ2のパスワードプロンプトは `/dev/tty` に出力される。stdout/stderr がリダイレクトされていてもユーザーに表示されるため、パイプや並行処理の中から呼んでも問題ない。ただし bash では `< <(...)` のプロセス置換内から呼ぶと stdout がパイプになりプロンプトが消えるため避けること。

### アイテム名取得

ref から 1Password アイテム名（タイトル）を取得する。

1. ref `op://vault/item[/field]` を `/` で分割し、3番目の要素を vault、4番目の要素を item として取得
   - 例: `op://MyVault/MyItem/password` → vault=`MyVault`, item=`MyItem`
2. `op item get --vault <vault> <item> --format json` を実行
3. JSON 出力の `.title` フィールドを取得
4. 失敗した場合（コマンドエラー・フィールドなし）は空文字を返す

### エントリ一覧取得

キーチェーンに保存されている全エントリを取得する。前提: キーチェーンがアンロック済みであること。

1. `security dump-keychain <keychain>`（`-d` なし）を実行して属性のみ取得
   出力例（エントリ1件分）:
   ```
   keychain: "/Users/user/Library/Keychains/op-keychain.keychain-db"
   version: 512
   class: 0x00000010
   attributes:
       0x00000007 <blob>="op-keychain:7f42d594..."
       "acct"<blob>="username"
       "svce"<blob>="op-keychain:7f42d594..."
       ...
   ```
2. 出力から `"svce"<blob>="op-keychain:` を含む行を抽出し、`op-keychain:[0-9a-f]+` パターンでサービス名を取得する
   - `0x00000007 <blob>` 行も同じ値を持つが、`"svce"` 行のみを使う（重複排除）
3. 各サービス名に対して `security find-generic-password -a <USER> -s <service> -w <keychain>` で JSON を取得

**`dump-keychain -d` を使わない理由**: `-d` を付けるとパスワードデータも出力されるが、非 ASCII バイトを含むデータを hex 形式で出力するため JSON パースが壊れる。属性のみ取得してサービス名を列挙し、`find-generic-password -w` で個別に取得する方が確実。

### サービス名生成

`<ref>` → `op-keychain:<SHA256(ref) lowercase hex>` に変換する。

- SHA256 は ref 文字列のバイト列に対して計算する（末尾改行なし）
- 例: `"op://Test/test02/password"` → `op-keychain:7f42d594...`（64文字）

---

## キーチェーン初期化

`op-keychain read` 実行時にキーチェーンファイルが存在しない場合に一度だけ実行される。

1. `/dev/tty` を直接開いてプロンプトを表示: `op-keychain: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]: `
   - `N`（デフォルト）: 空パスワードで作成。以降のアンロックはプロンプトなし
   - `Y`: パスワードを入力・確認。不一致の場合はエラー終了（`error: パスワードが一致しません`）
   - プロンプトへの入出力は `/dev/tty` 経由で行うこと（stdout/stderr がリダイレクトされていても動作するように）
2. `security create-keychain -p "<password>" op-keychain.keychain` でキーチェーンを作成
3. `security set-keychain-settings -t <IDLE_TIMEOUT> <keychain>` で非アクティブ自動ロックを設定
4. 既存のキーチェーンリストを取得: `security list-keychains -d user`
   - 出力形式: 各行が `    "/path/to/keychain"` のようにインデントとダブルクォートを含む
   - クォートとスペースを除去して既存パスの配列を構築する
5. `security list-keychains -s <existing...> <new>` で新しいキーチェーンをリストに追加

---

## IDLE_TIMEOUT の仕組み

macOS キーチェーンの「非アクティブ自動ロック」機能を利用する。

- デフォルト IDLE_TIMEOUT: **3600 秒（1時間）**。環境変数 `OP_KEYCHAIN_IDLE_TIMEOUT` で初期値を変更可能。作成後は `op-keychain update-idle-timeout <秒数>` で変更可能
- **キャッシュヒット時**: キーチェーンをアンロックしないためタイマーはリセットされない
- **キャッシュミス時**: キーチェーンがロック中の場合のみアンロックして保存するためタイマーがリセットされる
- `op-keychain read` を呼ばない状態が IDLE_TIMEOUT 秒続くとキーチェーンが自動ロックされ、次回は `op read` が走る

---

## キーチェーンの並行アクセス制約

macOS キーチェーン（`.keychain-db`）は SQLite3（WAL モード）で実装されているが、Apple 公式ドキュメントで以下が明記されている:

> Keychain 関数を複数スレッド・キュー・プロセスから同時に呼ぶな。シリアライズするかシングルスレッドに限定せよ。

このため **`security add-generic-password` 等の書き込み操作は直列化が必須**。Go では mutex、Rust では `Mutex` で書き込み区間を保護すること。

---

## 1Password セッション管理

1Password デスクトップアプリ連携時の CLI セッション挙動（実証済み）:

- セッションは子プロセスに**引き継がれる**
- セッション未確立時に並行 `op read` を実行すると、各プロセスが独立して認証ダイアログを表示する

`op-keychain refresh` での対策:
1. `op whoami` でセッション確認（stdout/stderr は捨てる）
   - 終了コード 0: セッション確立済み → そのまま並行実行へ
   - 終了コード 非ゼロ: セッション未確立 → 次のステップへ
2. セッション未確立の場合: `refs[0]` を直列で `op read` し、認証ダイアログを1回だけ表示して確立
3. 残りの ref を並行実行（確立済みセッションを引き継ぐためダイアログなし）

セッションの強制失効方法（テスト用）: 1Password アプリで `Cmd+Shift+L`（Lock Now）

---

## セキュリティ上の前提

- 空パスワードのキーチェーンは同一ユーザーの他プロセスからアクセス可能。macOS のユーザーアカウント分離で十分と判断している
- ACL はデフォルト設定（`security` CLI のみプロンプトなしでアクセス可）。Keychain Access GUI でパスワードを表示する際は macOS ログインパスワードを求められる
- Keychain Access GUI はキーチェーンの変更をリアルタイムで反映しない。操作後は GUI を閉じて開き直す必要がある
- キーチェーンのパスワードは Keychain Access GUI から変更可能（右クリック → Change Password for Keychain...）。変更後は空パスワードでのアンロックが失敗し、ユーザーにパスワードを求めるプロンプトを表示してアンロックする（実証済み）

---

## 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `OP_KEYCHAIN_IDLE_TIMEOUT` | `3600` | キーチェーン非アクティブ自動ロックまでの秒数（初回キーチェーン作成時のみ適用） |
| `OP_KEYCHAIN_DEBUG` | （未設定） | `true` または `1` でデバッグ出力を有効化（bash では `set -x`、他言語では stderr への詳細ログ出力） |

#!/usr/bin/env bash
set -eu -o pipefail
# -e: エラー発生時に即時終了
# -u: 未定義変数の参照をエラーとして扱う
# -o pipefail: パイプ途中のコマンド失敗をパイプライン全体の失敗として扱う

# デバッグモード: OP_CACHE_DEBUG=true または 1 で set -x を有効化
if [[ "${OP_CACHE_DEBUG:-}" == "true" || "${OP_CACHE_DEBUG:-}" == "1" ]]; then
  set -x
fi

# ============================================================
# 設定
# ============================================================

# macOS キーチェーンの名前とパス
readonly KEYCHAIN_NAME="op-cache"
readonly KEYCHAIN="$HOME/Library/Keychains/${KEYCHAIN_NAME}.keychain-db"

# 非アクティブ状態が続いた場合にキーチェーンが自動ロックされるまでの時間(秒)
# デフォルト 1 時間
# op-cache read が呼ばれるたびにタイマーがリセットされるため、
# 使い続けている限りロックされない (aws-vault と同じ挙動)
readonly TTL="${OP_CACHE_TTL:-3600}"

# ============================================================
# 内部ユーティリティ
# ============================================================

# キーチェーンエントリのサービス名を生成する
# ref の SHA256 ハッシュを使うことで UUID・スラッシュ・日本語などを含む
# 任意の ref に対して安全かつ一意なサービス名を生成できる
_service() {
  local hash
  hash=$(printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1)
  echo "op-cache:${hash}"
}

# キーチェーンを初期化する (存在しない場合のみ作成する)
# セキュリティ上の注意: 空パスワードのキーチェーンは同一ユーザーの他プロセスから
# アクセス可能だが、macOS のユーザーアカウント分離で十分と判断している
_init_keychain() {
  [[ -f "$KEYCHAIN" ]] && return

  # 空パスワードでキーチェーンを作成する
  security create-keychain -p "" "${KEYCHAIN_NAME}.keychain"

  # 非アクティブ TTL 秒後に自動ロックするよう設定する
  security set-keychain-settings -t "$TTL" "$KEYCHAIN"

  # 既存キーチェーンリストを配列で取得し、新しいキーチェーンを末尾に追加する
  # word splitting を避けるため while ループで処理する
  local current_keychains=()
  while IFS= read -r line; do
    # security list-keychains の出力はダブルクォートと前後の空白を含むため除去する
    line=$(printf '%s' "$line" | tr -d '"' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    [[ -n "$line" ]] && current_keychains+=("$line")
  done < <(security list-keychains -d user)

  security list-keychains -s "${current_keychains[@]}" "$KEYCHAIN"
}

# ============================================================
# サブコマンド
# ============================================================

# op-cache read <op://vault/item[/field]>
#
# キャッシュヒット: キーチェーンがアンロック中かつ値が存在する場合に返す
# キャッシュミス: キーチェーンがロック中 (TTL超過) または未キャッシュの場合に
#                op read で取得してキャッシュに保存してから返す
#
# キーチェーンに保存する JSON の形式:
#   {"ref": "op://...", "value": "<値>"}
#   ref を JSON に含めることで UUID を含む任意のパスを正確に保存・復元できる
#
# TTL の仕組み:
#   キャッシュヒット時はアンロックしないためタイマーはリセットされない
#   キャッシュミス時はアンロックして保存するためタイマーがリセットされる
#   → op-cache read を呼ばない状態が TTL 秒続くとキーチェーンが自動ロックされる
cmd_read() {
  local ref="${1:-}"
  if [[ -z "$ref" ]]; then
    echo "usage: op-cache read <op://...>" >&2
    return 1
  fi

  local service
  service=$(_service "$ref")

  _init_keychain

  # アンロックせずに読み取りを試みる
  # キーチェーンがロック中 (TTL超過) の場合は失敗する → キャッシュミス扱いにする
  local cached
  cached=$(security find-generic-password -a "$USER" -s "$service" -w "$KEYCHAIN" 2>/dev/null) || true
  if [[ -n "$cached" ]]; then
    printf '%s' "$(jq -r '.value' <<<"$cached")"
    return 0
  fi

  # キャッシュミス: 1Password CLI から取得する
  local value
  if ! value=$(op read "$ref"); then
    echo "error: op read に失敗しました: $ref" >&2
    return 1
  fi

  # キーチェーンをアンロックして保存する
  # アンロックにより非アクティブタイマーがリセットされる
  security unlock-keychain -p "" "$KEYCHAIN" 2>/dev/null || true
  local json
  json=$(jq -cn --arg ref "$ref" --arg value "$value" '{"ref": $ref, "value": $value}')
  # -A: ACL プロンプトなし  -U: 既存エントリを上書き
  security add-generic-password -A -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN" 2>/dev/null

  # op read と動作を合わせて改行なしで出力する
  printf '%s' "$value"
}

# op-cache clear [op://vault/item[/field]]
#
# 引数なし: キーチェーン全体を削除する
# 引数あり: 指定した ref のキャッシュエントリのみ削除する
cmd_clear() {
  local ref="${1:-}"

  if [[ -z "$ref" ]]; then
    security delete-keychain "$KEYCHAIN" 2>/dev/null || true
    echo "全キャッシュをクリアしました"
  else
    local service
    service=$(_service "$ref")
    # 削除のためにアンロックする
    security unlock-keychain -p "" "$KEYCHAIN" 2>/dev/null || true
    security delete-generic-password -a "$USER" -s "$service" "$KEYCHAIN" 2>/dev/null || true
    echo "クリアしました: $ref"
  fi
}

# op-cache list
#
# キーチェーンに保存されているエントリを列挙する
# キーチェーンがロック中 (TTL超過) の場合はその旨を表示する
cmd_list() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし"
    return
  fi

  # ロックされていると dump-keychain が失敗する → TTL 超過とみなす
  local dump
  if ! dump=$(security dump-keychain "$KEYCHAIN" 2>/dev/null); then
    echo "期限切れ (ロック中)"
    return
  fi

  local found=0
  while IFS= read -r service; do
    [[ "$service" == op-cache:* ]] || continue
    found=1

    # JSON から ref を取り出して表示する
    local cached
    cached=$(security find-generic-password -a "$USER" -s "$service" -w "$KEYCHAIN" 2>/dev/null) || true
    [[ -n "$cached" ]] || continue
    printf '  %s\n' "$(jq -r '.ref' <<<"$cached")"
  done < <(printf '%s\n' "$dump" | grep '"svce"' | sed 's/.*"svce"<blob>="\(.*\)"/\1/')

  if ((found == 0)); then
    echo "キャッシュなし"
  fi
}

# ============================================================
# エントリポイント
# ============================================================

case "${1:-}" in
read) cmd_read "${2:-}" ;;
clear) cmd_clear "${2:-}" ;;
list) cmd_list ;;
*)
  echo "usage: op-cache read  <op://...>   # キャッシュ付きで値を読み取る"
  echo "       op-cache clear [op://...]   # キャッシュを削除 (引数なしで全削除)"
  echo "       op-cache list               # キャッシュ一覧を表示"
  ;;
esac

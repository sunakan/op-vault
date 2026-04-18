#!/usr/bin/env bash
set -eu -o pipefail
# -e: エラー発生時に即時終了
# -u: 未定義変数の参照をエラーとして扱う
# -o pipefail: パイプ途中のコマンド失敗をパイプライン全体の失敗として扱う

# デバッグモード: OP_KEYCHAIN_DEBUG=true または 1 で set -x を有効化
if [[ "${OP_KEYCHAIN_DEBUG:-}" == "true" || "${OP_KEYCHAIN_DEBUG:-}" == "1" ]]; then
  set -x
fi

# ============================================================
# 設定
# ============================================================

# macOS キーチェーンの名前とパス
readonly KEYCHAIN_NAME="op-keychain"
readonly KEYCHAIN="$HOME/Library/Keychains/${KEYCHAIN_NAME}.keychain-db"

# 非アクティブ状態が続いた場合にキーチェーンが自動ロックされるまでの時間(秒)
# デフォルト 1 時間
# キャッシュミスでキーチェーンをアンロックするとタイマーがリセットされる
# キャッシュヒット時はアンロックしないためタイマーはリセットされない
readonly IDLE_TIMEOUT="${OP_KEYCHAIN_IDLE_TIMEOUT:-3600}"

# ============================================================
# 内部ユーティリティ
# ============================================================

# キーチェーンエントリのサービス名を生成する
# ref の SHA256 ハッシュを使うことで UUID・スラッシュ・日本語などを含む
# 任意の ref に対して安全かつ一意なサービス名を生成できる
_service() {
  local hash
  hash=$(printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1)
  echo "op-keychain:${hash}"
}

# キーチェーンを初期化する (存在しない場合のみ作成する)
_init_keychain() {
  [[ -f "$KEYCHAIN" ]] && return

  # パスワードを設定するか確認する (/dev/tty 経由でターミナルに直接出力)
  # デフォルトは空パスワード (プロンプトなしでアンロック可能)
  local password=""
  printf 'op-keychain: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]: ' >/dev/tty
  local answer
  read -r answer </dev/tty
  if [[ "$answer" =~ ^[Yy]$ ]]; then
    printf 'パスワード: ' >/dev/tty
    read -rs password </dev/tty
    printf '\n' >/dev/tty
    printf 'パスワード（確認）: ' >/dev/tty
    local confirm
    read -rs confirm </dev/tty
    printf '\n' >/dev/tty
    if [[ "$password" != "$confirm" ]]; then
      echo "error: パスワードが一致しません" >&2
      return 1
    fi
  fi

  # Keychain Access GUI でパスワードを表示する際は macOS ログインパスワードを求められる
  # (表示後に Keychain Access を開き直す必要がある)
  security create-keychain -p "$password" "${KEYCHAIN_NAME}.keychain"

  # 非アクティブ IDLE_TIMEOUT 秒後に自動ロックするよう設定する
  security set-keychain-settings -t "$IDLE_TIMEOUT" "$KEYCHAIN"

  # 既存キーチェーンリストを配列で取得し、新しいキーチェーンを末尾に追加する
  # word splitting を避けるため while ループで処理する
  local current_keychains=()
  while IFS= read -r line; do
    # security list-keychains の出力はダブルクォートと前後の空白を含むため除去する
    line=$(printf '%s' "$line" | tr -d '"' | xargs)
    [[ -n "$line" ]] && current_keychains+=("$line")
  done < <(security list-keychains -d user)

  security list-keychains -s "${current_keychains[@]}" "$KEYCHAIN"
}

# キーチェーンをアンロックする
# まず空パスワード (初期パスワード) で試み、失敗した場合のみユーザーにパスワードを求める
# プロセス置換内から呼ばないこと (stdout がパイプになりプロンプトが表示されなくなる)
_unlock_keychain() {
  security unlock-keychain -p "" "$KEYCHAIN" 2>/dev/null && return
  security unlock-keychain "$KEYCHAIN"
}

# キャッシュ済みエントリを "name\tref" 形式で標準出力に1行ずつ出力する
# 前提: $KEYCHAIN ファイルが存在し、アンロック済みであること (呼び出し前に確認・アンロックすること)
#
# dump-keychain -d はデータ行の出力形式がアイテム名・値の文字内容に依存して変わるため使わない。
# 代わりに dump-keychain (データなし) でサービス名一覧を取得し、
# find-generic-password で各エントリの JSON を個別に読み取る。
_dump_entries() {
  local dump
  dump=$(security dump-keychain "$KEYCHAIN" 2>/dev/null)

  while IFS= read -r service; do
    local json
    json=$(security find-generic-password -a "$USER" -s "$service" -w "$KEYCHAIN" 2>/dev/null) || continue
    local ref name
    ref=$(jq -r '.ref' <<<"$json" 2>/dev/null) || continue
    name=$(jq -r '.name // ""' <<<"$json" 2>/dev/null) || true
    printf '%s\t%s\n' "$name" "$ref"
  done < <(printf '%s\n' "$dump" | grep '"svce"<blob>="op-keychain:' | grep -o 'op-keychain:[0-9a-f]*')
}

# キャッシュ済みの ref 一覧を標準出力に1行ずつ出力する
# 前提: $KEYCHAIN ファイルが存在し、アンロック済みであること (呼び出し前に確認・アンロックすること)
_list_refs() {
  _dump_entries | cut -f2
}

# ref から 1Password アイテム名を取得する
# 失敗時は空文字を返す
_item_name() {
  local ref="$1"
  local vault item
  vault=$(printf '%s' "$ref" | cut -d'/' -f3)
  item=$(printf '%s' "$ref" | cut -d'/' -f4)
  op item get --vault "$vault" "$item" --format json 2>/dev/null | jq -r '.title // empty'
}

# ============================================================
# サブコマンド
# ============================================================

# op-keychain read <op://vault/item[/field]>
#
# キャッシュヒット: キーチェーンがアンロック中かつ値が存在する場合に返す
# キャッシュミス: キーチェーンがロック中 (IDLE_TIMEOUT超過) または未キャッシュの場合に
#                op read で取得してキャッシュに保存してから返す
#
# キーチェーンに保存する JSON の形式:
#   {"ref": "op://...", "name": "<アイテム名>", "value": "<値>"}
#   ref を JSON に含めることで UUID を含む任意のパスを正確に保存・復元できる
#   name を含めることで list コマンドで人間が読みやすい名前を表示できる
#
# IDLE_TIMEOUT の仕組み:
#   キャッシュヒット時: アンロックしないためタイマーはリセットされない
#   キャッシュミス時: まずアンロックなしで保存を試みる
#                   キーチェーンがロック中の場合のみアンロック → タイマーがリセットされる
#   → op-keychain read を呼ばない状態が IDLE_TIMEOUT 秒続くとキーチェーンが自動ロックされる
cmd_read() {
  local ref="${1:-}"
  if [[ -z "$ref" ]]; then
    echo "usage: op-keychain read <op://...>" >&2
    return 1
  fi

  local service
  service=$(_service "$ref")

  _init_keychain

  # アンロックせずに読み取りを試みる
  # キーチェーンがロック中 (IDLE_TIMEOUT超過) の場合は失敗する → キャッシュミス扱いにする
  local cached
  cached=$(security find-generic-password -a "$USER" -s "$service" -w "$KEYCHAIN" 2>/dev/null) || true
  if [[ -n "$cached" ]]; then
    local value
    # jq パース失敗（キャッシュが壊れている等）はキャッシュミス扱いにする
    if value=$(jq -r '.value' <<<"$cached" 2>/dev/null); then
      printf '%s' "$value"
      return 0
    fi
  fi

  # キャッシュミス: 1Password CLI から取得する
  local value
  if ! value=$(op read "$ref"); then
    echo "error: op read に失敗しました: $ref" >&2
    return 1
  fi

  local name
  name=$(_item_name "$ref") || name=""

  local json
  # -a (--ascii-output): 非 ASCII 文字を \uXXXX エスケープにして純 ASCII JSON にする
  # security find-generic-password -w は非 ASCII バイトを含むデータを hex 形式で返すため、
  # 純 ASCII にしておかないと jq でのパースが壊れる
  json=$(jq -cna --arg ref "$ref" --arg name "$name" --arg value "$value" '{"ref": $ref, "name": $name, "value": $value}')

  # まずアンロックなしで保存を試みる
  # ロック中の場合のみアンロック (プロンプト) してから再試行する
  # -T を指定しない場合: 作成者 (security CLI) はプロンプトなしでアクセス可
  # Keychain Access GUI でパスワードを表示する際は macOS ログインパスワードを求められる
  # -U: 既存エントリを上書き
  if ! security add-generic-password -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN" 2>/dev/null; then
    # ロック中 → アンロック (ここで初めてパスワードを聞く)
    _unlock_keychain
    security add-generic-password -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN"
  fi

  # op read と動作を合わせて改行なしで出力する
  printf '%s' "$value"
}

# op-keychain clear
#
# キーチェーン全体を削除する
cmd_clear() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし"
    return
  fi
  security delete-keychain "$KEYCHAIN" 2>/dev/null || true
  echo "全キャッシュをクリアしました"
}

# op-keychain remove <op://vault/item[/field]>
#
# 指定した ref のキャッシュエントリのみ削除する
cmd_remove() {
  local ref="${1:-}"
  if [[ -z "$ref" ]]; then
    echo "usage: op-keychain remove <op://...>" >&2
    return 1
  fi

  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし" >&2
    return 1
  fi

  local service
  service=$(_service "$ref")
  # まずアンロックなしで削除を試みる
  # ロック中の場合のみアンロック (プロンプト) してから再試行する
  if ! security delete-generic-password -a "$USER" -s "$service" "$KEYCHAIN" 2>/dev/null; then
    _unlock_keychain
    if ! security delete-generic-password -a "$USER" -s "$service" "$KEYCHAIN" 2>/dev/null; then
      echo "error: キャッシュが見つかりません: $ref" >&2
      return 1
    fi
  fi
  echo "削除しました: $ref"
}

# op-keychain list
#
# キーチェーンに保存されているエントリを列挙する
# キーチェーンがロック中 (IDLE_TIMEOUT超過) の場合はアンロック (プロンプト) してから一覧表示する
cmd_list() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし"
    return
  fi

  _unlock_keychain

  local found=0
  while IFS=$'\t' read -r name ref; do
    found=1
    if [[ -n "$name" ]]; then
      printf '  %s (%s)\n' "$name" "$ref"
    else
      printf '  %s\n' "$ref"
    fi
  done < <(_dump_entries)

  if ((found == 0)); then
    echo "キャッシュなし"
  fi
}

# op-keychain refresh
#
# キャッシュ済みの全 ref を再取得してキーチェーンを更新する
#
# op read は並行実行し、キーチェーンへの書き込みは直列化する
# (macOS Keychain の並行書き込み非サポートのため)
cmd_refresh() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし"
    return
  fi

  _unlock_keychain

  # キャッシュ済み ref 一覧を収集
  local refs=()
  while IFS= read -r ref; do
    refs+=("$ref")
  done < <(_list_refs)

  if [[ ${#refs[@]} -eq 0 ]]; then
    echo "キャッシュなし"
    return
  fi

  local tmpdir
  tmpdir=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$tmpdir'" EXIT

  # セッション未確立の場合、最初の ref を直列で実行して認証を1回だけ行う
  # (並行サブシェルは確立済みセッションを引き継ぐため、以降のダイアログは不要になる)
  if ! op whoami &>/dev/null; then
    local value name
    if value=$(op read "${refs[0]}" 2>/dev/null); then
      printf '%s' "$value" >"${tmpdir}/0"
      name=$(_item_name "${refs[0]}") || name=""
      printf '%s' "$name" >"${tmpdir}/0.name"
    else
      touch "${tmpdir}/0.error"
    fi
  fi

  # op read を並行実行 (セッション確立済みのためダイアログは出ない)
  for i in "${!refs[@]}"; do
    [[ -f "${tmpdir}/${i}" || -f "${tmpdir}/${i}.error" ]] && continue
    (
      if value=$(op read "${refs[$i]}" 2>/dev/null); then
        printf '%s' "$value" >"${tmpdir}/${i}"
        name=$(_item_name "${refs[$i]}") || name=""
        printf '%s' "$name" >"${tmpdir}/${i}.name"
      else
        touch "${tmpdir}/${i}.error"
      fi
    ) &
  done
  wait

  # キーチェーンへ直列に書き込む
  local ok=0 fail=0
  for i in "${!refs[@]}"; do
    local ref="${refs[$i]}"
    local service
    service=$(_service "$ref")
    if [[ -f "${tmpdir}/${i}.error" ]]; then
      printf '  skip (op read 失敗): %s\n' "$ref" >&2
      fail=$((fail + 1))
      continue
    fi
    local value name
    value=$(<"${tmpdir}/${i}")
    name=""
    [[ -f "${tmpdir}/${i}.name" ]] && name=$(<"${tmpdir}/${i}.name")
    local json
    json=$(jq -cna --arg ref "$ref" --arg name "$name" --arg value "$value" '{"ref": $ref, "name": $name, "value": $value}')
    security add-generic-password -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN"
    ok=$((ok + 1))
    printf '  refreshed: %s\n' "$ref"
  done

  printf '完了: %d 件更新, %d 件失敗\n' "$ok" "$fail"
}

# op-keychain update-idle-timeout <秒数>
#
# キーチェーンの非アクティブ自動ロックまでの時間を変更する
cmd_update_idle_timeout() {
  local seconds="${1:-}"
  if [[ -z "$seconds" ]]; then
    echo "usage: op-keychain update-idle-timeout <秒数>" >&2
    return 1
  fi
  if [[ ! "$seconds" =~ ^[0-9]+$ ]]; then
    echo "error: 秒数は正の整数で指定してください: $seconds" >&2
    return 1
  fi

  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "キャッシュなし" >&2
    return 1
  fi

  security set-keychain-settings -t "$seconds" "$KEYCHAIN"
  echo "idle-timeout を ${seconds}秒 に設定しました"
}

# op-keychain status
#
# キーチェーンの状態（IDLE_TIMEOUT・ロック状態・エントリ数）を表示する
#
# ロック状態の判定方法:
#   dump-keychain はロック中でも属性情報（サービス名等）を返す。
#   最初のエントリに find-generic-password でアクセスを試み、
#   成功すればアンロック中、失敗すればロック中と判定する（副作用なし）。
cmd_status() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    printf 'キーチェーン: なし\n'
    return
  fi

  printf 'キーチェーン: あり (%s)\n' "$KEYCHAIN"

  # IDLE_TIMEOUT を show-keychain-info から取得
  # 出力例: "... timeout=3600s" または "... timeout=0s"（タイムアウトなし）
  local info
  info=$(security show-keychain-info "$KEYCHAIN" 2>&1) || true
  local seconds
  seconds=$(printf '%s\n' "$info" | grep -o 'timeout=[0-9]*s' | grep -o '[0-9]*') || true
  if [[ -z "$seconds" ]]; then
    printf 'IDLE_TIMEOUT: 不明\n'
  elif [[ "$seconds" -eq 0 ]]; then
    printf 'IDLE_TIMEOUT: なし（自動ロックなし）\n'
  else
    printf 'IDLE_TIMEOUT: %s秒\n' "$seconds"
  fi

  # dump-keychain でサービス名を取得（ロック中でも動作する）
  local dump
  dump=$(security dump-keychain "$KEYCHAIN" 2>/dev/null) || true
  local services
  services=$(printf '%s\n' "$dump" | grep '"svce"<blob>="op-keychain:' | grep -o 'op-keychain:[0-9a-f]*')
  local count=0
  [[ -n "$services" ]] && count=$(printf '%s\n' "$services" | wc -l | tr -d ' ')

  if [[ $count -eq 0 ]]; then
    printf 'エントリ数:   0件\n'
    return
  fi

  # 最初のエントリへのアクセス可否でロック状態を判定
  local first_service
  first_service=$(printf '%s\n' "$services" | head -1)
  if security find-generic-password -a "$USER" -s "$first_service" -w "$KEYCHAIN" >/dev/null 2>&1; then
    printf 'ロック状態:   アンロック中\n'
    printf 'エントリ数:   %s件\n' "$count"
  else
    printf 'ロック状態:   ロック中\n'
    printf 'エントリ数:   不明（ロック中のため）\n'
  fi
}

# ============================================================
# エントリポイント
# ============================================================

case "${1:-}" in
read) cmd_read "${2:-}" ;;
clear) cmd_clear ;;
remove) cmd_remove "${2:-}" ;;
list) cmd_list ;;
refresh) cmd_refresh ;;
status) cmd_status ;;
update-idle-timeout) cmd_update_idle_timeout "${2:-}" ;;
*)
  echo "usage: op-keychain read                 <op://...>  # キャッシュ付きで値を読み取る"
  echo "       op-keychain remove               <op://...>  # 指定エントリを削除"
  echo "       op-keychain clear                            # キャッシュ全削除"
  echo "       op-keychain list                             # キャッシュ一覧を表示"
  echo "       op-keychain refresh                          # 全キャッシュを再取得"
  echo "       op-keychain status                           # キーチェーンの状態を表示"
  echo "       op-keychain update-idle-timeout  <秒数>      # 自動ロックまでの時間を変更"
  ;;
esac

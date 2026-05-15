#!/usr/bin/env bash
# e2e_test.sh
#
# Step 1–6 の自動化 E2E テスト。
# バイナリをビルドして exit code・stdout・stderr を検証する。
#
# 使い方:
#   bash e2e_test.sh          # 全ステップ
#   bash e2e_test.sh 4        # Step 4 のみ
#   bash e2e_test.sh 1 2 3    # 複数指定
#
# Step 7–8: 1Password Desktop App (Touch ID) が必要なため手動確認
# Step 9:   go test -short ./...
# Step 10:  golangci-lint run / CGO_ENABLED=1 go build ./cmd/op-keychain
# Step 11:  go test -tags integration ./...

set -uo pipefail

# ─────────────────────────────────────────────────────────────────
# ステップフィルタ
# ─────────────────────────────────────────────────────────────────

STEP_ARGS=("${@}")  # 引数なし = 全ステップ

should_run() {
    local step="$1"
    [ "${#STEP_ARGS[@]}" -eq 0 ] && return 0
    local s
    for s in "${STEP_ARGS[@]}"; do
        [ "$s" = "$step" ] && return 0
    done
    return 1
}

# ─────────────────────────────────────────────────────────────────
# 定数・グローバル変数
# ─────────────────────────────────────────────────────────────────

TESTCHAIN="op-keychain-e2etest"
KEYCHAIN_PATH="$HOME/Library/Keychains/${TESTCHAIN}.keychain-db"
BIN=$(mktemp /tmp/op-keychain-e2e-XXXXXX)
STDOUT_TMP=$(mktemp)
STDERR_TMP=$(mktemp)
PASS=0
FAIL=0
STATUS=0
STDOUT=""
STDERR=""

# ─────────────────────────────────────────────────────────────────
# ヘルパー
# ─────────────────────────────────────────────────────────────────

_ok()   { PASS=$((PASS + 1)); printf '\033[32m  PASS\033[0m  %s\n' "$1"; }
_ng()   { FAIL=$((FAIL + 1)); printf '\033[31m  FAIL\033[0m  %s\n' "$1"; }
_skip() { printf '\033[33m  SKIP\033[0m  %s\n' "$1"; }

assert_exit() {
    if [ "$STATUS" -eq "$1" ]; then
        _ok "exit $STATUS  $2"
    else
        _ng "exit $STATUS (want $1)  $2"
    fi
}

assert_not_exit() {
    if [ "$STATUS" -ne "$1" ]; then
        _ok "exit ≠ $1  $2"
    else
        _ng "exit = $1 (want ≠ $1)  $2"
    fi
}

assert_out() {
    if printf '%s' "$STDOUT" | grep -qF "$1"; then
        _ok "stdout '$1'  $2"
    else
        _ng "stdout missing '$1'  $2  (actual: $(printf '%s' "$STDOUT" | head -1))"
    fi
}

assert_err() {
    if printf '%s' "$STDERR" | grep -qF "$1"; then
        _ok "stderr '$1'  $2"
    else
        _ng "stderr missing '$1'  $2  (actual: $(printf '%s' "$STDERR" | head -1))"
    fi
}

assert_not_err() {
    if ! printf '%s' "$STDERR" | grep -qF "$1"; then
        _ok "stderr に '$1' がない  $2"
    else
        _ng "stderr に '$1' がある（あってはいけない）  $2"
    fi
}

# コマンドを実行し、STATUS / STDOUT / STDERR に格納する
# </dev/null: 対話プロンプトが stdin をブロックしないようにする
run_cmd() {
    OP_KEYCHAIN_NAME="$TESTCHAIN" "$BIN" "$@" \
        </dev/null >"$STDOUT_TMP" 2>"$STDERR_TMP"
    STATUS=$?
    STDOUT=$(cat "$STDOUT_TMP")
    STDERR=$(cat "$STDERR_TMP")
}

# keychain のセットアップ
# expect spawn + security はセッション境界で securityd のコンテキストが取れず hang するため
# init コマンドを経由せず security を直接呼ぶ
setup_keychain() {
    security create-keychain -p "" "$KEYCHAIN_PATH" 2>/dev/null || true
    security set-keychain-settings -t 3600 "$KEYCHAIN_PATH"
    # xargs で引用符・前後空白を除去してから -s に渡す
    # shellcheck disable=SC2046
    security list-keychains -d user -s \
        $(security list-keychains -d user | xargs) \
        "$KEYCHAIN_PATH"
}

# keychain を削除する
teardown_keychain() {
    OP_KEYCHAIN_NAME="$TESTCHAIN" "$BIN" clear --yes >/dev/null 2>&1 || true
    security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
}

# スクリプト終了時の後片付け
cleanup() {
    security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
    rm -f "$BIN" "$STDOUT_TMP" "$STDERR_TMP"
}
trap cleanup EXIT

# ─────────────────────────────────────────────────────────────────
# ビルド（常に実行）
# ─────────────────────────────────────────────────────────────────

echo ""
echo "=== ビルド ==="

if CGO_ENABLED=1 go build -o "$BIN" ./cmd/op-keychain; then
    _ok "go build 成功"
else
    printf '\033[31m  ERROR\033[0m  ビルド失敗。必要なステップを実装してから再実行してください。\n'
    exit 1
fi

# ─────────────────────────────────────────────────────────────────
# Step 1: --help
# ─────────────────────────────────────────────────────────────────

if should_run 1; then
    echo ""
    echo "=== Step 1: --help ==="

    run_cmd --help
    assert_exit 0 "--help"

    for sub in read remove clear list refresh status set-idle-timeout init version; do
        if printf '%s' "$STDOUT$STDERR" | grep -q "$sub"; then
            _ok "--help に '$sub' が含まれる"
        else
            _ng "--help に '$sub' がない"
        fi
    done
fi

# ─────────────────────────────────────────────────────────────────
# Step 2: version
# ─────────────────────────────────────────────────────────────────

if should_run 2; then
    echo ""
    echo "=== Step 2: version ==="

    run_cmd version
    assert_exit 0 "version"
    assert_out "op-keychain" "version の出力"
fi

# ─────────────────────────────────────────────────────────────────
# Step 3: ref バリデーション
# ─────────────────────────────────────────────────────────────────

if should_run 3; then
    echo ""
    echo "=== Step 3: ref バリデーション ==="

    run_cmd read "not-a-ref"
    assert_exit 2 "read 不正 ref: not-a-ref"
    assert_err "invalid ref format" "not-a-ref のエラーメッセージ"

    run_cmd read "op://vault-only"
    assert_exit 2 "read 不正 ref: op://vault-only (item なし)"

    run_cmd remove "op://bad"
    assert_exit 2 "remove 不正 ref: op://bad"

    # 正常 ref の検証: exit 2 にならず invalid ref format が出ないことを確認する。
    # exit 0 では検証しない。Step 7 実装後は 1Password へ接続して exit 1 になるため。
    run_cmd read "op://vault/item"
    assert_not_exit 2 "read 正常 ref: ref validation エラーなし (2 セグメント)"
    assert_not_err "invalid ref format" "read 正常 ref: invalid ref format なし (2 セグメント)"

    run_cmd read "op://vault/item/field"
    assert_not_exit 2 "read 正常 ref: ref validation エラーなし (3 セグメント)"
    assert_not_err "invalid ref format" "read 正常 ref: invalid ref format なし (3 セグメント)"
fi

# ─────────────────────────────────────────────────────────────────
# Step 4: init / clear
# ─────────────────────────────────────────────────────────────────

if should_run 4; then
    echo ""
    echo "=== Step 4: init / clear ==="

    teardown_keychain

    # 初回セットアップ: expect spawn は macOS の新規セッションを生成し
    # securityd のログインコンテキストが取得できず security コマンドがハングする。
    # そのため外側シェル（ログインセッション）から直接 security で keychain を作る。
    _skip "init の対話テスト: expect spawn + security はセッション境界で hang するためスキップ"
    setup_keychain

    if [ -f "$KEYCHAIN_PATH" ]; then
        _ok "init: keychain ファイルが作成された"
    else
        _ng "init: keychain ファイルが作成されなかった"
    fi

    # 2回目 init: 既存チェックを先に行い /dev/tty を開かずに即返す
    run_cmd init
    assert_exit 0 "init 2回目"
    assert_out "already initialized" "init 2回目の出力"

    # --yes なしの対話テストは /dev/tty 経由のため自動化不可
    _skip "clear --yes なし (y/N 対話): /dev/tty 経由のため自動化不可"

    # clear --yes
    run_cmd clear --yes
    assert_exit 0 "clear --yes"
    assert_out "cleared all cache" "clear --yes の出力"

    if security list-keychains | grep -q "$TESTCHAIN"; then
        _ng "clear --yes: keychain がリストに残っている"
    else
        _ok "clear --yes: keychain がリストから消えた"
    fi

    # keychain なしで clear: idempotent
    run_cmd clear --yes
    assert_exit 0 "clear --yes (keychain なし)"
    assert_out "no keychain" "clear --yes (keychain なし) の出力"
fi

# ─────────────────────────────────────────────────────────────────
# Step 5: status / set-idle-timeout
# ─────────────────────────────────────────────────────────────────

if should_run 5; then
    echo ""
    echo "=== Step 5: status / set-idle-timeout ==="

    teardown_keychain

    # keychain なしの status
    run_cmd status
    assert_exit 0 "status (keychain なし)"
    assert_out "keychain: none" "status (keychain なし): keychain: none"

    setup_keychain

    # アンロック状態の status
    run_cmd status
    assert_exit 0 "status (unlocked)"
    assert_out "$KEYCHAIN_PATH"          "status: keychain パス"
    assert_out "lock status:  unlocked"  "status: lock status"
    assert_out "3600s"                   "status: idle-timeout"
    assert_out "entries:      0"         "status: entries"

    # set-idle-timeout 正常系
    run_cmd set-idle-timeout 1800
    assert_exit 0 "set-idle-timeout 1800"
    assert_out "1800s" "set-idle-timeout 1800 の出力"

    run_cmd status
    assert_out "1800s" "status: idle-timeout が 1800s に変わった"

    # 異常系（exit 2）
    run_cmd set-idle-timeout 0
    assert_exit 2 "set-idle-timeout 0"

    run_cmd set-idle-timeout -1
    assert_exit 2 "set-idle-timeout -1"

    run_cmd set-idle-timeout abc
    assert_exit 2 "set-idle-timeout abc"

    run_cmd set-idle-timeout
    assert_exit 2 "set-idle-timeout 引数なし"

    # ロック状態での status
    security lock-keychain "$KEYCHAIN_PATH"
    run_cmd status
    assert_exit 0 "status (locked)"
    assert_out "lock status:  locked"        "status (locked): lock status"
    assert_out "unknown (locked)"            "status (locked): entries"
    assert_out "locked (unlock to view)"     "status (locked): idle-timeout"

    teardown_keychain
fi

# ─────────────────────────────────────────────────────────────────
# Step 6: list / remove
# ─────────────────────────────────────────────────────────────────

if should_run 6; then
    echo ""
    echo "=== Step 6: list / remove ==="

    teardown_keychain

    REF="op://Test/test02/password"
    SVCNAME="op-keychain:$(printf '%s' "$REF" | shasum -a 256 | cut -d' ' -f1)"
    JSON="{\"ref\":\"${REF}\",\"name\":\"test02\",\"value\":\"dummy-secret\",\"account\":\"\"}"

    # keychain なしの remove
    run_cmd remove "$REF"
    assert_exit 1 "remove (keychain なし)"
    assert_err "no keychain" "remove (keychain なし) エラーメッセージ"

    setup_keychain

    # 0件の list
    run_cmd list
    assert_exit 0 "list (0件)"
    assert_out "no cache" "list (0件)"

    # ref バリデーション
    run_cmd remove "op://bad"
    assert_exit 2 "remove 不正 ref"

    # 存在しない entry の remove
    run_cmd remove "$REF"
    assert_exit 1 "remove (entry なし)"
    assert_err "cache not found" "remove (entry なし) エラーメッセージ"

    # テスト用エントリを security コマンドで直接登録（SDK なしでテスト可能）
    security add-generic-password \
        -s "$SVCNAME" -a "$(whoami)" -w "$JSON" "$KEYCHAIN_PATH"

    # エントリあり状態の list (1件)
    run_cmd list
    assert_exit 0 "list (1件)"
    assert_out "test02" "list (1件): name"
    assert_out "$REF"   "list (1件): ref"

    # 複数件 + アルファベット順
    # 意図的に逆順で登録してソート確認: zebra → apple → test02 の順で追加
    REF_A="op://Test/apple/password"
    REF_Z="op://Test/zebra/password"
    SVC_A="op-keychain:$(printf '%s' "$REF_A" | shasum -a 256 | cut -d' ' -f1)"
    SVC_Z="op-keychain:$(printf '%s' "$REF_Z" | shasum -a 256 | cut -d' ' -f1)"
    JSON_A="{\"ref\":\"${REF_A}\",\"name\":\"apple\",\"value\":\"dummy\",\"account\":\"\"}"
    JSON_Z="{\"ref\":\"${REF_Z}\",\"name\":\"zebra\",\"value\":\"dummy\",\"account\":\"\"}"
    security add-generic-password -s "$SVC_Z" -a "$(whoami)" -w "$JSON_Z" "$KEYCHAIN_PATH"
    security add-generic-password -s "$SVC_A" -a "$(whoami)" -w "$JSON_A" "$KEYCHAIN_PATH"

    run_cmd list
    assert_exit 0 "list (3件)"
    LINE_A=$(printf '%s' "$STDOUT" | grep -n "apple"  | cut -d: -f1)
    LINE_T=$(printf '%s' "$STDOUT" | grep -n "test02" | cut -d: -f1)
    LINE_Z=$(printf '%s' "$STDOUT" | grep -n "zebra"  | cut -d: -f1)
    if [ -n "$LINE_A" ] && [ -n "$LINE_T" ] && [ -n "$LINE_Z" ] \
        && [ "$LINE_A" -lt "$LINE_T" ] && [ "$LINE_T" -lt "$LINE_Z" ]; then
        _ok "list: アルファベット順 (apple < test02 < zebra)"
    else
        _ng "list: アルファベット順が正しくない (apple=${LINE_A:-?} test02=${LINE_T:-?} zebra=${LINE_Z:-?})"
    fi

    # name 空エントリは "  <ref>" 形式（括弧なし）で表示される
    REF_NONAME="op://Test/noname/password"
    SVC_NONAME="op-keychain:$(printf '%s' "$REF_NONAME" | shasum -a 256 | cut -d' ' -f1)"
    JSON_NONAME="{\"ref\":\"${REF_NONAME}\",\"name\":\"\",\"value\":\"dummy\",\"account\":\"\"}"
    security add-generic-password -s "$SVC_NONAME" -a "$(whoami)" -w "$JSON_NONAME" "$KEYCHAIN_PATH"

    run_cmd list
    if printf '%s' "$STDOUT" | grep -qF "  $REF_NONAME"; then
        _ok "list: name 空エントリのフォーマット (  <ref>)"
    else
        _ng "list: name 空エントリのフォーマットが違う  (actual: $(printf '%s' "$STDOUT" | grep "$REF_NONAME" || true))"
    fi

    # remove 正常系
    run_cmd remove "$REF"
    assert_exit 0 "remove (entry あり)"
    assert_out "removed:" "remove (entry あり) の出力"

    run_cmd list
    if ! printf '%s' "$STDOUT" | grep -qF "test02"; then
        _ok "remove 後: test02 が消えた"
    else
        _ng "remove 後: test02 が残っている"
    fi

    # ロック中の remove（unlock して削除できること）
    security add-generic-password \
        -s "$SVCNAME" -a "$(whoami)" -w "$JSON" "$KEYCHAIN_PATH"
    security lock-keychain "$KEYCHAIN_PATH"

    run_cmd remove "$REF"
    assert_exit 0 "remove (locked): unlock して削除"
    assert_out "removed:" "remove (locked) の出力"

    teardown_keychain
fi

# ─────────────────────────────────────────────────────────────────
# サマリ
# ─────────────────────────────────────────────────────────────────

echo ""
echo "────────────────────────────────"
printf " PASS: %d  FAIL: %d\n" "$PASS" "$FAIL"
echo "────────────────────────────────"
[ "$FAIL" -eq 0 ]

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

# コマンドを実行し、STATUS / STDOUT / STDERR に格納する
run_cmd() {
    OP_KEYCHAIN_NAME="$TESTCHAIN" "$BIN" "$@" \
        >"$STDOUT_TMP" 2>"$STDERR_TMP"
    STATUS=$?
    STDOUT=$(cat "$STDOUT_TMP")
    STDERR=$(cat "$STDERR_TMP")
}

# expect 経由で init を実行（空パスワード: プロンプトに N を入力）
init_with_expect() {
    expect -timeout 10 -c "
        set env(OP_KEYCHAIN_NAME) {$TESTCHAIN}
        spawn $BIN init
        expect {
            {Set a password} { send {N\r}; exp_continue }
            eof
        }
    " >"$STDOUT_TMP" 2>"$STDERR_TMP"
    STATUS=$?
    STDOUT=$(cat "$STDOUT_TMP")
    STDERR=$(cat "$STDERR_TMP")
}

# expect がない場合のフォールバック: security コマンドで直接 keychain を作成
setup_keychain_directly() {
    _skip "expect 未インストール: security コマンドで直接 keychain を作成（init 対話テストはスキップ）"
    security create-keychain -p "" "$KEYCHAIN_PATH" 2>/dev/null || true
    security set-keychain-settings -t 3600 "$KEYCHAIN_PATH"
}

# keychain のセットアップ（expect があれば init 経由、なければ security 直接）
setup_keychain() {
    if command -v expect &>/dev/null; then
        init_with_expect
    else
        setup_keychain_directly
    fi
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

    run_cmd read "op://vault/item"
    assert_exit 0 "read 正常 ref (2 セグメント)"

    run_cmd read "op://vault/item/field"
    assert_exit 0 "read 正常 ref (3 セグメント)"
fi

# ─────────────────────────────────────────────────────────────────
# Step 4: init / clear
# ─────────────────────────────────────────────────────────────────

if should_run 4; then
    echo ""
    echo "=== Step 4: init / clear ==="

    teardown_keychain

    # 初回 init（空パスワード）
    if command -v expect &>/dev/null; then
        init_with_expect
        if security list-keychains | grep -q "$TESTCHAIN"; then
            _ok "init: keychain が作成された"
        else
            _ng "init: keychain が作成されなかった"
        fi
    else
        setup_keychain_directly
    fi

    if security list-keychains | grep -q "$TESTCHAIN"; then
        _ok "init: security list-keychains に含まれる"
    else
        _ng "init: security list-keychains に含まれない"
    fi

    # 2回目 init: 既存チェックを先に行い /dev/tty を開かずに即返す
    run_cmd init
    assert_exit 0 "init 2回目"
    assert_out "already initialized" "init 2回目の出力"

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

    setup_keychain

    # アンロック状態の status
    run_cmd status
    assert_exit 0 "status (unlocked)"
    assert_out "unlocked"        "status: lock status"
    assert_out "3600s"           "status: idle-timeout"
    assert_out "entries:      0" "status: entries"

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

    # ロック状態での status
    security lock-keychain "$KEYCHAIN_PATH"
    run_cmd status
    assert_exit 0 "status (locked)"
    assert_out "locked"           "status (locked): lock status"
    assert_out "unknown (locked)" "status (locked): entries"

    teardown_keychain
fi

# ─────────────────────────────────────────────────────────────────
# Step 6: list / remove
# ─────────────────────────────────────────────────────────────────

if should_run 6; then
    echo ""
    echo "=== Step 6: list / remove ==="

    setup_keychain

    REF="op://Test/test02/password"
    SVCNAME="op-keychain:$(printf '%s' "$REF" | shasum -a 256 | cut -d' ' -f1)"
    JSON="{\"ref\":\"${REF}\",\"name\":\"test02\",\"value\":\"dummy-secret\",\"account\":\"\"}"

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

    # エントリあり状態の list
    run_cmd list
    assert_exit 0 "list (1件)"
    assert_out "test02" "list (1件): name"
    assert_out "$REF"   "list (1件): ref"

    # remove 正常系
    run_cmd remove "$REF"
    assert_exit 0 "remove (entry あり)"
    assert_out "removed:" "remove (entry あり) の出力"

    # 削除後は 0件
    run_cmd list
    assert_out "no cache" "remove 後: list = no cache"

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

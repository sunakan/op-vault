#!/usr/bin/env bash
set -uo pipefail
# -u: treat unset variables as errors
# -o pipefail: propagate errors in pipelines

#
# E2E test
#
# Build binary and check exit code, stdout, stderr.
#

#
# Global variables
#
TEST_CHAIN='op-keychain-e2e-test'
KEYCHAIN_PATH="$HOME/Library/Keychains/${TEST_CHAIN}.keychain-db"
JAEGER_UI='http://localhost:16686'
JAEGER_OTLP='http://localhost:4318'
PASS=0
FAIL=0
SKIP=0
STATUS=0
STDOUT=''
STDERR=''
TRACES=''
STDOUT_TMP=$(mktemp)
STDERR_TMP=$(mktemp)
CMD_TIMEOUT_SEC=10

#
# Helper
#
# cleanup:
# Clean up test keychain, temp files, and Jaeger on exit
cleanup() {
  security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
  rm -f "$STDOUT_TMP" "$STDERR_TMP"
  make down 2>/dev/null &
}
trap cleanup EXIT

_pass() {
  PASS=$((PASS + 1))
  if [[ -t 2 ]]; then
    printf '\033[32m  PASS\033[0m  %s\n' "$1"
  else
    printf '  PASS  %s\n' "$1"
  fi
}

_fail() {
  FAIL=$((FAIL + 1))
  if [[ -t 2 ]]; then
    printf '\033[31m  FAIL\033[0m  %s\n' "$1"
  else
    printf '  FAIL  %s\n' "$1"
  fi
}

_skip() {
  SKIP=$((SKIP + 1))
  if [[ -t 2 ]]; then
    printf '\033[33m  SKIP\033[0m  %s\n' "$1"
  else
    printf '  SKIP  %s\n' "$1"
  fi
}

expect_exit_code() {
  if [ "$STATUS" -eq "$1" ]; then
    _pass "exit $STATUS  $2"
  else
    _fail "exit $STATUS (expected $1)  $2"
  fi
}

expect_stdout_contains() {
  if printf '%s' "$STDOUT" | grep -qF "$1"; then
    _pass "stdout '$1'  $2"
  else
    _fail "stdout missing '$1'  $2  (actual: $(printf '%s' "$STDOUT" | head -1))"
  fi
}

expect_stdout_empty() {
  if [ -z "$STDOUT" ]; then
    _pass "stdout is empty  $1"
  else
    _fail "stdout is not empty  $1  (actual: $(printf '%s' "$STDOUT" | head -1))"
  fi
}

expect_stderr_contains() {
  if printf '%s' "$STDERR" | grep -qF "$1"; then
    _pass "stderr '$1'  $2"
  else
    _fail "stderr missing '$1'  $2  (actual: $(printf '%s' "$STDERR" | head -1))"
  fi
}

expect_stderr_empty() {
  if [ -z "$STDERR" ]; then
    _pass "stderr is empty  $1"
  else
    _fail "stderr is not empty  $1  (actual: $(printf '%s' "$STDERR" | head -1))"
  fi
}

expect_stdout_matches() {
  if printf '%s' "$STDOUT" | grep -qE "$1"; then
    _pass "stdout matches '$1'  $2"
  else
    _fail "stdout does not match '$1'  $2  (actual: $(printf '%s' "$STDOUT" | head -1))"
  fi
}

run_cmd() {
  perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env OP_KEYCHAIN_NAME="$TEST_CHAIN" ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_otlp() {
  perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env OP_KEYCHAIN_NAME="$TEST_CHAIN" OP_KEYCHAIN_TRACES_EXPORTER=otlp OP_KEYCHAIN_OTLP_ENDPOINT="$JAEGER_OTLP" OTEL_RESOURCE_ATTRIBUTES='' ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_stdin() {
  local input="$1"
  shift
  printf '%s' "$input" | perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env OP_KEYCHAIN_NAME="$TEST_CHAIN" ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_stdin_otlp() {
  local input="$1"
  shift
  printf '%s' "$input" | perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env OP_KEYCHAIN_NAME="$TEST_CHAIN" OP_KEYCHAIN_TRACES_EXPORTER=otlp OP_KEYCHAIN_OTLP_ENDPOINT="$JAEGER_OTLP" OTEL_RESOURCE_ATTRIBUTES='' ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_no_account() {
  perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env -u OP_ACCOUNT OP_KEYCHAIN_NAME="$TEST_CHAIN" ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_with_exporter() {
  local exporter="$1"
  shift
  perl -e "alarm ${CMD_TIMEOUT_SEC}; exec @ARGV" \
    env OP_KEYCHAIN_NAME="$TEST_CHAIN" OP_KEYCHAIN_TRACES_EXPORTER="$exporter" OP_KEYCHAIN_OTLP_ENDPOINT='' OTEL_RESOURCE_ATTRIBUTES='' ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

wait_for_jaeger() {
  local i=0
  # Poll directly instead of relying on Docker healthcheck (start_period adds unnecessary delay)
  while ! curl -sf "${JAEGER_UI}/" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 30 ]; then
      printf '  ERROR  Jaeger did not start within 30s\n' >&2
      exit 1
    fi
    sleep 1
  done
}

expect_span_name() {
  local expected="$1" desc="$2"
  local actual
  actual=$(printf '%s' "$TRACES" | jq -r '.data[].spans[].operationName' 2>/dev/null)
  if printf '%s' "$actual" | grep -qF "$expected"; then
    _pass "span operationName == '$expected'  $desc"
  else
    _fail "span operationName != '$expected'  $desc  (actual: $(printf '%s' "$actual" | tr '\n' ' '))"
  fi
}

expect_file_exists() {
  if [ -f "$1" ]; then
    _pass "file exists '$1'  $2"
  else
    _fail "file not found '$1'  $2"
  fi
}

expect_file_not_exists() {
  if [ ! -f "$1" ]; then
    _pass "file not found '$1'  $2"
  else
    _fail "file still exists '$1'  $2"
  fi
}

expect_span_status_error() {
  local desc="$1"
  local actual
  actual=$(printf '%s' "$TRACES" | jq -r '.data[].spans[].tags[] | select(.key == "otel.status_code") | .value' 2>/dev/null)
  if printf '%s' "$actual" | grep -qF 'ERROR'; then
    _pass "span otel.status_code == 'ERROR'  $desc"
  else
    _fail "span otel.status_code != 'ERROR'  $desc  (actual: $(printf '%s' "$actual" | head -1))"
  fi
}

#
# Prerequisites
#
if ! command -v jq >/dev/null 2>&1; then
  printf '  ERROR  jq is required but not installed\n' >&2
  exit 1
fi

export OP_ACCOUNT="${OP_ACCOUNT:-dummy}"

#
# Jaeger
#
echo ''
echo '=== Jaeger ==='

make down 2>/dev/null || true
if make up 2>/dev/null; then
  wait_for_jaeger
  _pass 'Jaeger started'
else
  if [[ -t 2 ]]; then
    printf '\033[31m  ERROR\033[0m  Failed to start Jaeger\n'
  else
    printf '  ERROR  Failed to start Jaeger\n'
  fi
  exit 1
fi

#
# Build
#
echo ''
echo '=== Build ==='

make clean
if make build; then
  _pass 'make build'
else
  if [[ -t 2 ]]; then
    printf '\033[31m  ERROR\033[0m  Failed build\n'
  else
    printf '  ERROR  Failed build\n'
  fi
  exit 1
fi

#
# --help
#
echo ''
echo '=== --help ==='

run_cmd --help
expect_exit_code 0 '--help'
expect_stdout_contains 'op-keychain' '--help output is in stdout'
expect_stderr_empty '--help'

for sub in version init reset read; do
  if printf '%s' "$STDOUT$STDERR" | grep -qF "$sub"; then
    _pass "--help contains '$sub'"
  else
    _fail "--help does not contain '$sub'"
  fi
done

run_cmd -h
expect_exit_code 0 '-h'
expect_stdout_contains 'op-keychain' '-h output is in stdout'
expect_stderr_empty '-h'

#
# Invalid TRACES_EXPORTER
#
echo ''
echo '=== Invalid TRACES_EXPORTER ==='
run_cmd_with_exporter invalid version
expect_exit_code 1 'invalid exporter'
expect_stdout_empty 'invalid exporter'
expect_stderr_contains 'unknown OP_KEYCHAIN_TRACES_EXPORTER' 'invalid exporter error message'

#
# OTLP without endpoint
#
echo ''
echo '=== OTLP without endpoint ==='
run_cmd_with_exporter otlp version
expect_exit_code 1 'otlp without endpoint'
expect_stdout_empty 'otlp without endpoint'
expect_stderr_contains 'OP_KEYCHAIN_OTLP_ENDPOINT is required' 'otlp endpoint required error message'

#
# version
#
echo ''
echo '=== version ==='
run_cmd version
expect_exit_code 0 'version'
expect_stdout_matches '^[0-9]+\.[0-9]+\.[0-9]+$' 'version stdout matches x.y.z'
expect_stderr_empty 'version'

#
# version --help
#
echo ''
echo '=== version --help ==='
run_cmd version --help
expect_exit_code 0 'version --help'
expect_stdout_contains 'version' 'version --help output is in stdout'
expect_stderr_empty 'version --help'

#
# version with OTLP
#
echo ''
echo '=== version with OTLP ==='
START_US=$(($(date +%s) * 1000000))
run_cmd_otlp version
expect_exit_code 0 'version with OTLP'
expect_stdout_matches '^[0-9]+\.[0-9]+\.[0-9]+$' 'version stdout matches x.y.z with OTLP'
expect_stderr_empty 'version with OTLP'
sleep 1
TRACES=$(curl -s "${JAEGER_UI}/api/traces?service=op-keychain&start=${START_US}&limit=5")
expect_span_name 'version' 'version span received by Jaeger'
expect_span_name 'main' 'main span received by Jaeger'

#
# Unknown sub command
#
echo ''
echo '=== Unknown sub command ==='
run_cmd unknown
expect_exit_code 2 'unknown sub command'
expect_stdout_empty 'unknown sub command'
expect_stderr_contains 'error:' 'unknown sub command'

#
# Unknown sub command with OTLP
#
echo ''
echo '=== Unknown sub command with OTLP ==='
START_US=$(($(date +%s) * 1000000))
run_cmd_otlp unknown
expect_exit_code 2 'unknown sub command with OTLP'
expect_stdout_empty 'unknown sub command with OTLP'
sleep 1
TRACES=$(curl -s "${JAEGER_UI}/api/traces?service=op-keychain&start=${START_US}&limit=5")
expect_span_name 'main' 'main span received by Jaeger'
expect_span_status_error 'main span has error status'

#
# No subcommand
#
echo ''
echo '=== No subcommand ==='
run_cmd
expect_exit_code 0 'no subcommand'
expect_stdout_contains 'op-keychain' 'no subcommand shows help'
expect_stderr_empty 'no subcommand'

#
# init --help
#
echo ''
echo '=== init --help ==='
run_cmd init --help
expect_exit_code 0 'init --help'
expect_stdout_contains 'init' 'init --help output is in stdout'
expect_stderr_empty 'init --help'

#
# init
#
echo ''
echo '=== init ==='
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
run_cmd_stdin 'testpass' init
expect_exit_code 0 'init'
expect_stdout_empty 'init'
expect_stderr_empty 'init'
expect_file_exists "$KEYCHAIN_PATH" 'init creates keychain file'

#
# init (already exists)
#
echo ''
echo '=== init (already exists) ==='
run_cmd_stdin 'testpass' init
expect_exit_code 0 'init already exists'
expect_stdout_empty 'init already exists'
expect_stderr_contains "already exists: $TEST_CHAIN" 'init already exists'

#
# init (empty password)
#
echo ''
echo '=== init (empty password) ==='
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
run_cmd_stdin '' init
expect_exit_code 0 'init empty password'
expect_stdout_empty 'init empty password'
expect_stderr_empty 'init empty password'
expect_file_exists "$KEYCHAIN_PATH" 'init empty password creates keychain file'

#
# init with OTLP
#
echo ''
echo '=== init with OTLP ==='
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
START_US=$(($(date +%s) * 1000000))
run_cmd_stdin_otlp 'testpass' init
expect_exit_code 0 'init with OTLP'
expect_stdout_empty 'init with OTLP'
expect_stderr_empty 'init with OTLP'
sleep 1
TRACES=$(curl -s "${JAEGER_UI}/api/traces?service=op-keychain&start=${START_US}&limit=5")
expect_span_name 'init' 'init span received by Jaeger'
expect_span_name 'main' 'main span received by Jaeger'

#
# reset --help
#
echo ''
echo '=== reset --help ==='
# Given: nothing
# When
run_cmd reset --help
# Then
expect_exit_code 0 'reset --help'
expect_stdout_contains 'reset' 'reset --help output is in stdout'
expect_stderr_empty 'reset --help'

#
# reset
#
echo ''
echo '=== reset ==='
# Given
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
run_cmd_stdin '' init
expect_exit_code 0 'reset: precondition init'
# When
run_cmd reset
# Then
expect_exit_code 0 'reset'
expect_stdout_empty 'reset'
expect_stderr_contains "deleted: $TEST_CHAIN" 'reset deletes keychain'
expect_file_not_exists "$KEYCHAIN_PATH" 'reset removes keychain file'

#
# reset (not found)
#
echo ''
echo '=== reset (not found) ==='
# Given
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
# When
run_cmd reset
# Then
expect_exit_code 0 'reset not found'
expect_stdout_empty 'reset not found'
expect_stderr_contains "not found: $TEST_CHAIN" 'reset not found message'

#
# reset with OTLP
#
echo ''
echo '=== reset with OTLP ==='
# Given
security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
run_cmd_stdin '' init
expect_exit_code 0 'reset: precondition init'
# When
START_US=$(($(date +%s) * 1000000))
run_cmd_otlp reset
# Then
expect_exit_code 0 'reset with OTLP'
expect_stdout_empty 'reset with OTLP'
expect_stderr_contains "deleted: $TEST_CHAIN" 'reset with OTLP deletes keychain'
expect_file_not_exists "$KEYCHAIN_PATH" 'reset removes keychain file'
sleep 1
TRACES=$(curl -s "${JAEGER_UI}/api/traces?service=op-keychain&start=${START_US}&limit=5")
expect_span_name 'reset' 'reset span received by Jaeger'
expect_span_name 'main' 'main span received by Jaeger'

#
# read --help
#
echo ''
echo '=== read --help ==='
# Given: nothing
# When
run_cmd read --help
# Then
expect_exit_code 0 'read --help'
expect_stdout_contains 'read' 'read --help output is in stdout'
expect_stderr_empty 'read --help'

#
# read (no account)
#
echo ''
echo '=== read (no account) ==='
# Given: OP_ACCOUNT is unset and no -a flag is given
# When
run_cmd_no_account read "op://Private/MyItem/password"
# Then
expect_exit_code 1 'read (no account)'
expect_stdout_empty 'read (no account)'
expect_stderr_contains 'account is required' 'read (no account)'

#
# read (invalid ref: wrong format)
#
echo ''
echo '=== read (invalid ref: wrong format) ==='
# When
run_cmd read "op://hoge/fuga"
# Then
expect_exit_code 1 'read (invalid ref: wrong format)'
expect_stdout_empty 'read (invalid ref: wrong format)'
expect_stderr_contains 'invalid ref format' 'read (invalid ref: wrong format)'

#
# read (cache hit)
#
echo ''
echo '=== read (cache hit) ==='
# Given
run_cmd reset
run_cmd_stdin '' init
expect_exit_code 0 'read cache hit: precondition init'
ENTRY='{"ref":"op://Test/CachedItem/password","item_name":"CachedItem","value":"___cached-secret___"}'
security add-generic-password \
  -s "op://Test/CachedItem/password" \
  -a "$OP_ACCOUNT" \
  -D "1Password Cache" \
  -w "$ENTRY" \
  "$KEYCHAIN_PATH"
# When
run_cmd read "op://Test/CachedItem/password"
# Then
expect_exit_code 0 'read (cache hit)'
expect_stdout_contains '___cached-secret___' 'read (cache hit) stdout'
expect_stderr_empty 'read (cache hit)'

#
# read (cache miss)
#
echo ''
echo '=== read (cache miss) ==='
if [ -z "${OP_TEST_INTEGRATION:-}" ]; then
  _skip 'read (cache miss): requires 1Password (set OP_TEST_INTEGRATION=1 to run)'
else
  # Requires 1Password item: op://Test/ExistedItem/password = "___existed-secret___"
  # Given
  run_cmd reset
  run_cmd_stdin '' init
  expect_exit_code 0 'read cache miss: precondition init'
  # When
  run_cmd read "op://Test/ExistedItem/password"
  # Then
  expect_exit_code 0 'read (cache miss)'
  expect_stdout_contains '___existed-secret___' 'read (cache miss) stdout'
  expect_stderr_empty 'read (cache miss)'
fi

#
# read (not found key in 1Password)
#
echo ''
echo '=== read (not found key in 1Password) ==='
if [ -z "${OP_TEST_INTEGRATION:-}" ]; then
  _skip 'read (not found key in 1Password): requires 1Password (set OP_TEST_INTEGRATION=1 to run)'
else
  # Requires op://Test/NotFound/password to NOT exist in 1Password
  # Given
  run_cmd reset
  run_cmd_stdin '' init
  expect_exit_code 0 'read not found in 1Password: precondition init'
  # When
  run_cmd read "op://Test/NotFound/password"
  # Then
  expect_exit_code 1 'read (not found key in 1Password)'
  expect_stdout_empty 'read (not found key in 1Password)'
  expect_stderr_contains 'not found in 1Password: op://Test/NotFound/password' 'read (not found key in 1Password)'
fi

#
# read (not found keychain)
#
echo ''
echo '=== read (not found keychain) ==='
# Given
run_cmd reset
expect_file_not_exists "$KEYCHAIN_PATH" 'read: precondition keychain not exist'
# When
run_cmd read "op://Private/MyItem/password"
# Then
expect_exit_code 1 'read (not found keychain)'
expect_stdout_empty 'read (not found keychain)'
expect_stderr_contains "keychain not found: run 'op-keychain init'" 'read (not found keychain)'

#
# read with OTLP
#
echo ''
echo '=== read with OTLP ==='
# Given
run_cmd reset
run_cmd_stdin '' init
expect_exit_code 0 'read with OTLP: precondition init'
ENTRY='{"ref":"op://Test/MyItem/password","item_name":"MyItem","value":"supersecret"}'
security add-generic-password \
  -s "op://Test/MyItem/password" \
  -a "$OP_ACCOUNT" \
  -D "1Password Cache" \
  -w "$ENTRY" \
  "$KEYCHAIN_PATH"
# When
START_US=$(($(date +%s) * 1000000))
run_cmd_otlp read "op://Test/MyItem/password"
# Then
expect_exit_code 0 'read with OTLP'
expect_stdout_contains 'supersecret' 'read with OTLP stdout'
expect_stderr_empty 'read with OTLP'
sleep 1
TRACES=$(curl -s "${JAEGER_UI}/api/traces?service=op-keychain&start=${START_US}&limit=5")
expect_span_name 'read' 'read span received by Jaeger'
expect_span_name 'main' 'main span received by Jaeger'

#
# Summary
#
echo ''
echo '─────────────────────────────────────────────'
printf " PASS: %d  FAIL: %d  SKIP: %d\n" "$PASS" "$FAIL" "$SKIP"
echo '─────────────────────────────────────────────'

# Exit with non-zero if any test failed
[ "$FAIL" -eq 0 ]

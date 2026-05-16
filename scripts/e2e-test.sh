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
PASS=0
FAIL=0
STATUS=0
STDOUT=''
STDERR=''
STDOUT_TMP=$(mktemp)
STDERR_TMP=$(mktemp)

#
# Helper
#
# cleanup:
# Clean up test keychain and temp files on exit
cleanup() {
  security delete-keychain "$KEYCHAIN_PATH" 2>/dev/null || true
  rm -f "$STDOUT_TMP" "$STDERR_TMP"
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
  OP_KEYCHAIN_NAME="$TEST_CHAIN" ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

run_cmd_tracing() {
  OP_KEYCHAIN_NAME="$TEST_CHAIN" OP_KEYCHAIN_TRACES_EXPORTER=stdout OTEL_RESOURCE_ATTRIBUTES= ./op-keychain "$@" >"$STDOUT_TMP" 2>"$STDERR_TMP"
  STATUS=$?
  STDOUT=$(cat "$STDOUT_TMP")
  STDERR=$(cat "$STDERR_TMP")
}

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

for sub in version; do
  if printf '%s' "$STDOUT$STDERR" | grep -q "$sub"; then
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
# version
#
echo ''
echo '=== version ==='
run_cmd version
expect_exit_code 0 'version'
expect_stdout_matches '^[0-9]+\.[0-9]+\.[0-9]+$' 'version stdout matches x.y.z'
expect_stderr_empty 'version'

echo ''
echo '=== version With tracing ==='
run_cmd_tracing version
expect_exit_code 0 'version with tracing'
expect_stdout_matches '^[0-9]+\.[0-9]+\.[0-9]+$' 'version stdout matches x.y.z with tracing'
expect_stderr_contains '"Name":"version"' 'version span emitted'
expect_stderr_contains '"Name":"main"' 'main span emitted'
expect_stderr_contains '"Code":"Unset"' 'version spans have no error status'

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
# Unknown sub command
#
echo ''
echo '=== Unknown sub command ==='
run_cmd unknown
expect_exit_code 2 'unknown sub command'
expect_stdout_empty 'unknown sub command'
expect_stderr_contains 'error:' 'unknown sub command'

echo ''
echo '=== Unknown sub command with tracing ==='
run_cmd_tracing unknown
expect_exit_code 2 'unknown sub command with tracing'
expect_stdout_empty 'unknown sub command with tracing'
expect_stderr_contains '"Name":"main"' 'main span emitted'
expect_stderr_contains '"Code":"Error"' 'unknown sub command span has error status'

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
# Summary
#
echo ''
echo '─────────────────────────────────────────────'
printf " PASS: %d  FAIL: %d\n" "$PASS" "$FAIL"
echo '─────────────────────────────────────────────'

# Exit with non-zero if any test failed
[ "$FAIL" -eq 0 ]

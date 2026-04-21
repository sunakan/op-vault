#!/usr/bin/env bash
set -eu -o pipefail
# -e: exit immediately on error
# -u: treat unset variables as errors
# -o pipefail: treat pipe failures as errors

# Debug mode: set OP_KEYCHAIN_DEBUG=true or 1 to enable set -x
if [[ "${OP_KEYCHAIN_DEBUG:-}" == "true" || "${OP_KEYCHAIN_DEBUG:-}" == "1" ]]; then
  set -x
fi

# ============================================================
# Configuration
# ============================================================

# macOS Keychain name and path
readonly KEYCHAIN_NAME="op-keychain"
readonly KEYCHAIN="$HOME/Library/Keychains/${KEYCHAIN_NAME}.keychain-db"

# Time in seconds before the keychain auto-locks due to inactivity
# Default: 1 hour
# The timer resets on cache miss (keychain unlock), but not on cache hit
readonly IDLE_TIMEOUT="${OP_KEYCHAIN_IDLE_TIMEOUT:-3600}"

# ============================================================
# Internal utilities
# ============================================================

# Generate a keychain service name for a given ref.
# Uses SHA256 hash of the ref so that UUIDs, slashes, and non-ASCII
# characters are handled safely and produce a unique service name.
_service() {
  local hash
  hash=$(printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1)
  echo "op-keychain:${hash}"
}

# Initialize the keychain (create only if it does not exist).
# Prompts the user via /dev/tty to optionally set a password.
# Default is an empty password (allows silent unlock).
_init_keychain() {
  [[ -f "$KEYCHAIN" ]] && return

  # Ask whether to set a password, writing directly to the terminal
  local password=""
  printf 'op-keychain: Set a password for the keychain? [y/N (default: N)]: ' >/dev/tty
  local answer
  read -r answer </dev/tty
  if [[ "$answer" =~ ^[Yy]$ ]]; then
    printf 'Password: ' >/dev/tty
    read -rs password </dev/tty
    printf '\n' >/dev/tty
    printf 'Password (confirm): ' >/dev/tty
    local confirm
    read -rs confirm </dev/tty
    printf '\n' >/dev/tty
    if [[ "$password" != "$confirm" ]]; then
      echo "error: passwords do not match" >&2
      return 1
    fi
  fi

  # The macOS login password is required to view the secret in Keychain Access GUI.
  # (Keychain Access must be reopened after viewing.)
  security create-keychain -p "$password" "${KEYCHAIN_NAME}.keychain"

  # Auto-lock after IDLE_TIMEOUT seconds of inactivity
  security set-keychain-settings -t "$IDLE_TIMEOUT" "$KEYCHAIN"

  # Append the new keychain to the existing keychain list.
  # Use a while loop to avoid word splitting issues.
  local current_keychains=()
  while IFS= read -r line; do
    # security list-keychains output includes surrounding quotes and whitespace
    line=$(printf '%s' "$line" | tr -d '"' | xargs)
    [[ -n "$line" ]] && current_keychains+=("$line")
  done < <(security list-keychains -d user)

  security list-keychains -s "${current_keychains[@]}" "$KEYCHAIN"
}

# Unlock the keychain.
# First tries the empty password silently; prompts the user only on failure.
# Do NOT call this from inside process substitution < <(...) because
# stdout becomes a pipe and the prompt disappears.
_unlock_keychain() {
  security unlock-keychain -p "" "$KEYCHAIN" 2>/dev/null && return
  security unlock-keychain "$KEYCHAIN"
}

# Print cached entries to stdout as "name\tref" lines, one per entry.
# Prerequisite: $KEYCHAIN must exist and be unlocked before calling this.
#
# dump-keychain -d is avoided because its output format depends on the
# content of item names and values (non-ASCII data is printed as hex).
# Instead, dump-keychain (no -d) lists service names, then
# find-generic-password reads each entry's JSON individually.
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

# Print cached refs to stdout, one per line.
# Prerequisite: $KEYCHAIN must exist and be unlocked before calling this.
_list_refs() {
  _dump_entries | cut -f2
}

# Retrieve the 1Password item title for a given ref.
# Returns an empty string on failure.
_item_name() {
  local ref="$1"
  local vault item
  vault=$(printf '%s' "$ref" | cut -d'/' -f3)
  item=$(printf '%s' "$ref" | cut -d'/' -f4)
  op item get --vault "$vault" "$item" --format json 2>/dev/null | jq -r '.title // empty'
}

# ============================================================
# Subcommands
# ============================================================

# op-keychain read <op://vault/item[/field]>
#
# Cache hit:  keychain is unlocked and the value exists → return immediately
# Cache miss: keychain is locked (IDLE_TIMEOUT exceeded) or entry absent →
#             fetch via op read, then save to keychain
#
# JSON format stored in the keychain:
#   {"ref": "op://...", "name": "<item title>", "value": "<secret>"}
#   Including ref allows exact round-trip for paths containing UUIDs.
#   Including name allows human-readable output in the list command.
#
# IDLE_TIMEOUT behaviour:
#   Cache hit:  keychain is not unlocked → timer is NOT reset
#   Cache miss: tries to save without unlocking first;
#               unlocks (resets timer) only when the keychain is locked
#   → the keychain auto-locks after IDLE_TIMEOUT seconds with no cache miss
cmd_read() {
  local ref="${1:-}"
  if [[ -z "$ref" ]]; then
    echo "usage: op-keychain read <op://...>" >&2
    return 1
  fi

  local service
  service=$(_service "$ref")

  _init_keychain

  # Attempt to read without unlocking.
  # Fails silently if the keychain is locked (treated as cache miss).
  local cached
  cached=$(security find-generic-password -a "$USER" -s "$service" -w "$KEYCHAIN" 2>/dev/null) || true
  if [[ -n "$cached" ]]; then
    local value
    # Treat JSON parse failure (corrupted cache, etc.) as a cache miss
    if value=$(jq -r '.value' <<<"$cached" 2>/dev/null); then
      printf '%s' "$value"
      return 0
    fi
  fi

  # Cache miss: fetch from 1Password CLI
  local value
  if ! value=$(op read "$ref"); then
    echo "error: op read failed: $ref" >&2
    return 1
  fi

  local name
  name=$(_item_name "$ref") || name=""

  local json
  # -a (--ascii-output): escape non-ASCII characters as \uXXXX to produce
  # pure ASCII JSON. security find-generic-password -w returns non-ASCII
  # bytes as hex, which breaks jq parsing.
  json=$(jq -cna --arg ref "$ref" --arg name "$name" --arg value "$value" '{"ref": $ref, "name": $name, "value": $value}')

  # Try to save without unlocking first.
  # Only unlock (prompt) if the keychain is locked.
  # Without -T: the creating process (security CLI) can access without prompt.
  # Keychain Access GUI requires the macOS login password to reveal the secret.
  # -U: overwrite existing entry
  if ! security add-generic-password -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN" 2>/dev/null; then
    # Keychain is locked → unlock now (first password prompt)
    _unlock_keychain
    security add-generic-password -U -a "$USER" -s "$service" -w "$json" "$KEYCHAIN"
  fi

  # Match op read behaviour: print without a trailing newline
  printf '%s' "$value"
}

# op-keychain clear
#
# Delete the entire keychain.
cmd_clear() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "no cache"
    return
  fi
  security delete-keychain "$KEYCHAIN" 2>/dev/null || true
  echo "all cache cleared"
}

# op-keychain remove <op://vault/item[/field]>
#
# Delete only the cache entry for the given ref.
cmd_remove() {
  local ref="${1:-}"
  if [[ -z "$ref" ]]; then
    echo "usage: op-keychain remove <op://...>" >&2
    return 1
  fi

  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "no cache" >&2
    return 1
  fi

  local service
  service=$(_service "$ref")
  # Try to delete without unlocking first; unlock only if necessary
  if ! security delete-generic-password -a "$USER" -s "$service" "$KEYCHAIN" 2>/dev/null; then
    _unlock_keychain
    if ! security delete-generic-password -a "$USER" -s "$service" "$KEYCHAIN" 2>/dev/null; then
      echo "error: cache entry not found: $ref" >&2
      return 1
    fi
  fi
  echo "removed: $ref"
}

# op-keychain list
#
# List all entries in the keychain.
# If the keychain is locked (IDLE_TIMEOUT exceeded), unlock before listing.
cmd_list() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "no cache"
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
    echo "no cache"
  fi
}

# op-keychain refresh
#
# Re-fetch all cached refs and update the keychain.
#
# op read calls are parallelised; keychain writes are serialised
# because macOS Keychain does not support concurrent writes.
cmd_refresh() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "no cache"
    return
  fi

  _unlock_keychain

  # Collect cached refs
  local refs=()
  while IFS= read -r ref; do
    refs+=("$ref")
  done < <(_list_refs)

  if [[ ${#refs[@]} -eq 0 ]]; then
    echo "no cache"
    return
  fi

  local tmpdir
  tmpdir=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$tmpdir'" EXIT

  # If no 1Password session is established, run the first ref serially
  # to trigger authentication once. Subsequent parallel subshells inherit
  # the established session and require no further dialogs.
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

  # Run op read in parallel (session already established, no dialogs)
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

  # Write to the keychain serially
  local ok=0 fail=0
  for i in "${!refs[@]}"; do
    local ref="${refs[$i]}"
    local service
    service=$(_service "$ref")
    if [[ -f "${tmpdir}/${i}.error" ]]; then
      printf '  skip (op read failed): %s\n' "$ref" >&2
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

  printf 'done: %d updated, %d failed\n' "$ok" "$fail"
}

# op-keychain update-idle-timeout <seconds>
#
# Update the inactivity auto-lock timeout for the keychain.
# OP_KEYCHAIN_IDLE_TIMEOUT (default: 3600) is used only at keychain creation.
# Use this command to change it afterwards.
cmd_update_idle_timeout() {
  local seconds="${1:-}"
  if [[ -z "$seconds" ]]; then
    echo "usage: op-keychain update-idle-timeout <seconds>" >&2
    return 1
  fi
  if [[ ! "$seconds" =~ ^[0-9]+$ ]]; then
    echo "error: seconds must be a positive integer: $seconds" >&2
    return 1
  fi

  if [[ ! -f "$KEYCHAIN" ]]; then
    echo "no cache" >&2
    return 1
  fi

  security set-keychain-settings -t "$seconds" "$KEYCHAIN"
  echo "idle-timeout set to ${seconds}s"
}

# op-keychain status
#
# Show keychain status: idle-timeout, lock state, and entry count.
#
# Lock state detection:
#   dump-keychain returns attribute info (service names, etc.) even when locked.
#   Attempt find-generic-password on the first entry to determine lock state
#   (no side effects). Success → unlocked; failure → locked.
cmd_status() {
  if [[ ! -f "$KEYCHAIN" ]]; then
    printf 'keychain:     not found\n'
    return
  fi

  printf 'keychain:     %s\n' "$KEYCHAIN"

  # Get IDLE_TIMEOUT from show-keychain-info
  # Example output: "... timeout=3600s" or "... timeout=0s" (no timeout)
  local info
  info=$(security show-keychain-info "$KEYCHAIN" 2>&1) || true
  local seconds
  seconds=$(printf '%s\n' "$info" | grep -o 'timeout=[0-9]*s' | grep -o '[0-9]*') || true
  if [[ -z "$seconds" ]]; then
    printf 'idle-timeout: unknown\n'
  elif [[ "$seconds" -eq 0 ]]; then
    printf 'idle-timeout: none (auto-lock disabled)\n'
  else
    printf 'idle-timeout: %ss\n' "$seconds"
  fi

  # List service names via dump-keychain (works even when locked)
  local dump
  dump=$(security dump-keychain "$KEYCHAIN" 2>/dev/null) || true
  local services
  services=$(printf '%s\n' "$dump" | grep '"svce"<blob>="op-keychain:' | grep -o 'op-keychain:[0-9a-f]*')
  local count=0
  [[ -n "$services" ]] && count=$(printf '%s\n' "$services" | wc -l | tr -d ' ')

  if [[ $count -eq 0 ]]; then
    printf 'entries:      0\n'
    return
  fi

  # Probe the first entry to determine lock state
  local first_service
  first_service=$(printf '%s\n' "$services" | head -1)
  if security find-generic-password -a "$USER" -s "$first_service" -w "$KEYCHAIN" >/dev/null 2>&1; then
    printf 'lock status:  unlocked\n'
    printf 'entries:      %s\n' "$count"
  else
    printf 'lock status:  locked\n'
    printf 'entries:      unknown (locked)\n'
  fi
}

# ============================================================
# Entry point
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
  echo "usage: op-keychain read                 <op://...>  # read value with cache"
  echo "       op-keychain remove               <op://...>  # remove a cached entry"
  echo "       op-keychain clear                            # clear all cache"
  echo "       op-keychain list                             # list cached entries"
  echo "       op-keychain refresh                          # refresh all cached entries"
  echo "       op-keychain status                           # show keychain status"
  echo "       op-keychain update-idle-timeout  <seconds>   # update auto-lock timeout"
  ;;
esac

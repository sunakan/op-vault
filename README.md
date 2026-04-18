<h1 align="center">op-cache</h1>
<p align="center">A caching layer for <code>op read</code> (1Password CLI) that stores secrets in macOS Keychain.</p>

<p align="center">
  English | <a href="./README.ja.md">日本語</a>
</p>

Instead of hitting the 1Password server on every call, op-cache returns the cached value from Keychain instantly. The cache expires automatically using Keychain's built-in inactivity auto-lock — no separate TTL management needed.

## Requirements

- macOS
- [1Password desktop app](https://1password.com/downloads/mac/) with CLI integration enabled
- `op` CLI
- `jq`

## Installation

### Recommended: install to `~/.local/bin`

```bash
mkdir -p ~/.local/bin
curl -o ~/.local/bin/op-cache https://raw.githubusercontent.com/sunakan/op-cache/main/op-cache.sh
chmod +x ~/.local/bin/op-cache
```

Make sure `~/.local/bin` is in your `PATH` (add to `~/.zshrc` if needed):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then use from anywhere:

```bash
op-cache read 'op://vault/item/field'
```

### Clone the repository

```bash
git clone https://github.com/sunakan/op-cache.git
cd op-cache
./op-cache.sh read 'op://vault/item/field'
```

## Usage

```bash
op-cache read <op://vault/item/field>   # Read with cache
op-cache remove <op://...>              # Remove a specific entry
op-cache clear                          # Delete all cache
op-cache list                           # List cached entries
op-cache refresh                        # Re-fetch all cached secrets
op-cache status                         # Show keychain status
op-cache update-idle-timeout <seconds>  # Change auto-lock timeout
```

### Example

```bash
# First call: fetches from 1Password and caches
./op-cache.sh read 'op://Personal/GitHub/token'

# Subsequent calls: returns from cache instantly (no 1Password request)
./op-cache.sh read 'op://Personal/GitHub/token'
```

## How It Works

op-cache uses a dedicated Keychain (`~/Library/Keychains/op-cache.keychain-db`) to store secrets.

**Cache hit**: If the Keychain is unlocked and the entry exists, the value is returned immediately without contacting 1Password.

**Cache miss**: If the Keychain is locked (idle timeout exceeded) or the entry doesn't exist, `op read` fetches the value from 1Password, stores it in Keychain, and returns it.

The Keychain's inactivity auto-lock acts as the cache expiration mechanism. If `op-cache read` is not called for `IDLE_TIMEOUT` seconds, the Keychain locks automatically and the next call re-fetches from 1Password.

Each entry is stored as JSON containing the original ref, item name, and value:

```json
{"ref": "op://vault/item/field", "name": "Item Title", "value": "<secret>"}
```

The service name is a SHA256 hash of the ref, so any ref (including UUIDs, Japanese characters, etc.) is handled safely.

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `OP_CACHE_IDLE_TIMEOUT` | `3600` | Inactivity auto-lock timeout in seconds (applied only at keychain creation) |
| `OP_CACHE_DEBUG` | (unset) | Set to `true` or `1` to enable debug output (`set -x`) |

After the Keychain is created, use `op-cache update-idle-timeout` to change the timeout:

```bash
./op-cache.sh update-idle-timeout 1800  # 30 minutes
```

## Keychain Password

On first run, op-cache prompts whether to set a password on the Keychain:

```
op-cache: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]:
```

The default (no password) allows silent unlocking — no prompt on cache miss. If you set a password, you'll be prompted when the Keychain needs to be unlocked.

## Debugging

```bash
OP_CACHE_DEBUG=true ./op-cache.sh read 'op://Test/test02/password'
```

## Comparison with op-fast

[op-fast](https://github.com/cometkim/op-fast) is a full `op` command proxy (read/inject/run) with LMDB + OS keyring dual storage and per-secret TTL via glob patterns.

op-cache is intentionally narrower: it wraps `op read` only, stores everything in macOS Keychain, and uses the Keychain's idle timeout as the expiration mechanism. No config files, no extra dependencies beyond `jq`.

## License

MIT

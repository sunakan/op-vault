<h1 align="center">op-keychain</h1>
<p align="center">A caching layer for <code>op read</code> (1Password CLI) that stores secrets in macOS Keychain.</p>

<p align="center">
  English | <a href="./README.ja.md">日本語</a>
</p>

Instead of hitting the 1Password server on every call, op-keychain returns the cached value from Keychain instantly. The cache expires automatically using Keychain's built-in inactivity auto-lock — no separate TTL management needed.

`op read` contacts 1Password servers on every invocation — even with desktop app integration enabled. This is by design in 1Password's security model. Typical latency is **~1.8s per call** (after the `op` daemon has started; the first call can take 10s+ due to daemon startup). op-keychain reduces cached reads to milliseconds.

## Requirements

- macOS
- [1Password desktop app](https://1password.com/downloads/mac/) with CLI integration enabled
- `op` CLI
- `jq`

## Installation

### Recommended: install to `~/.local/bin`

```bash
mkdir -p ~/.local/bin
curl -o ~/.local/bin/op-keychain https://raw.githubusercontent.com/sunakan/op-keychain/main/op-keychain.sh
chmod +x ~/.local/bin/op-keychain
```

Make sure `~/.local/bin` is in your `PATH` (add to `~/.zshrc` if needed):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then use from anywhere:

```bash
op-keychain read 'op://vault/item/field'
```

### Uninstall

```bash
# Delete the cache keychain
op-keychain clear

# Remove the script
rm ~/.local/bin/op-keychain
```

If you removed the script first, delete the keychain manually:

```bash
# via CLI
security delete-keychain ~/Library/Keychains/op-keychain.keychain-db
```

Or open **Keychain Access.app** → find `op-keychain` → right-click → **Delete Keychain "op-keychain"**.

### Clone the repository

```bash
git clone https://github.com/sunakan/op-keychain.git
cd op-keychain
./op-keychain.sh read 'op://vault/item/field'
```

## Usage

```bash
op-keychain read <op://vault/item/field>   # Read with cache
op-keychain remove <op://...>              # Remove a specific entry
op-keychain clear                          # Delete all cache
op-keychain list                           # List cached entries
op-keychain refresh                        # Re-fetch all cached secrets
op-keychain status                         # Show keychain status
op-keychain update-idle-timeout <seconds>  # Change auto-lock timeout
```

### Example

```bash
# First call: fetches from 1Password and caches
./op-keychain.sh read 'op://Personal/GitHub/token'

# Subsequent calls: returns from cache instantly (no 1Password request)
./op-keychain.sh read 'op://Personal/GitHub/token'
```

## How It Works

op-keychain uses a dedicated Keychain (`~/Library/Keychains/op-keychain.keychain-db`) to store secrets.

**Cache hit**: If the Keychain is unlocked and the entry exists, the value is returned immediately without contacting 1Password.

**Cache miss**: If the Keychain is locked (idle timeout exceeded) or the entry doesn't exist, `op read` fetches the value from 1Password, stores it in Keychain, and returns it.

The Keychain's inactivity auto-lock acts as the cache expiration mechanism. If `op-keychain read` is not called for `IDLE_TIMEOUT` seconds, the Keychain locks automatically and the next call re-fetches from 1Password.

Each entry is stored as JSON containing the original ref, item name, and value:

```json
{"ref": "op://vault/item/field", "name": "Item Title", "value": "<secret>"}
```

The service name is a SHA256 hash of the ref, so any ref (including UUIDs, Japanese characters, etc.) is handled safely.

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `OP_KEYCHAIN_IDLE_TIMEOUT` | `3600` | Inactivity auto-lock timeout in seconds (applied only at keychain creation) |
| `OP_KEYCHAIN_DEBUG` | (unset) | Set to `true` or `1` to enable debug output (`set -x`) |

After the Keychain is created, use `op-keychain update-idle-timeout` to change the timeout:

```bash
./op-keychain.sh update-idle-timeout 1800  # 30 minutes
```

## Keychain Password

On first run, op-keychain prompts whether to set a password on the Keychain:

```
op-keychain: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]:
```

The default (no password) allows silent unlocking — no prompt on cache miss. If you set a password, you'll be prompted when the Keychain needs to be unlocked.

## Debugging

```bash
OP_KEYCHAIN_DEBUG=true ./op-keychain.sh read 'op://Test/test02/password'
```

## License

MIT

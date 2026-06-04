<h1 align="center">op-vault</h1>
<p align="center">A caching layer for <code>op read</code> (1Password CLI) that stores secrets in macOS Keychain.</p>

<p align="center">
  English | <a href="./README.ja.md">日本語</a>
</p>

Instead of hitting the 1Password server on every call, op-vault returns the cached value from Keychain instantly. The cache expires automatically using Keychain's built-in inactivity auto-lock — no separate TTL management needed.

`op read` contacts 1Password servers on every invocation — even with desktop app integration enabled. This is by design in 1Password's security model. Typical latency is **~1.8s per call** (after the `op` daemon has started; the first call can take 10s+ due to daemon startup). op-vault reduces cached reads to milliseconds.

## Requirements

- macOS
- [1Password desktop app](https://1password.com/downloads/mac/) with CLI integration enabled
- `op` CLI
- `jq`

## Installation

### Recommended: install to `~/.local/bin`

```bash
mkdir -p ~/.local/bin
curl -o ~/.local/bin/op-vault https://raw.githubusercontent.com/sunakan/op-vault/main/op-vault.sh
chmod +x ~/.local/bin/op-vault
```

Make sure `~/.local/bin` is in your `PATH` (add to `~/.zshrc` if needed):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then use from anywhere:

```bash
op-vault read 'op://vault/item/field'
```

### Uninstall

```bash
# Delete the cache keychain
op-vault clear

# Remove the script
rm ~/.local/bin/op-vault
```

If you removed the script first, delete the keychain manually:

```bash
# via CLI
security delete-keychain ~/Library/Keychains/op-vault.keychain-db
```

Or open **Keychain Access.app** → find `op-vault` → right-click → **Delete Keychain "op-vault"**.

### Clone the repository

```bash
git clone https://github.com/sunakan/op-vault.git
cd op-vault
./op-vault.sh read 'op://vault/item/field'
```

## Usage

```bash
op-vault read <op://vault/item/field>   # Read with cache
op-vault remove <op://...>              # Remove a specific entry
op-vault clear                          # Delete all cache
op-vault list                           # List cached entries
op-vault refresh                        # Re-fetch all cached secrets
op-vault status                         # Show keychain status
op-vault update-idle-timeout <seconds>  # Change auto-lock timeout
```

### Example

```bash
# First call: fetches from 1Password and caches
./op-vault.sh read 'op://Personal/GitHub/token'

# Subsequent calls: returns from cache instantly (no 1Password request)
./op-vault.sh read 'op://Personal/GitHub/token'
```

## How It Works

op-vault uses a dedicated Keychain (`~/Library/Keychains/op-vault.keychain-db`) to store secrets.

**Cache hit**: If the Keychain is unlocked and the entry exists, the value is returned immediately without contacting 1Password.

**Cache miss**: If the Keychain is locked (idle timeout exceeded) or the entry doesn't exist, `op read` fetches the value from 1Password, stores it in Keychain, and returns it.

The Keychain's inactivity auto-lock acts as the cache expiration mechanism. If `op-vault read` is not called for `IDLE_TIMEOUT` seconds, the Keychain locks automatically and the next call re-fetches from 1Password.

Each entry is stored as JSON containing the original ref, item name, and value:

```json
{"ref": "op://vault/item/field", "name": "Item Title", "value": "<secret>"}
```

The service name is a SHA256 hash of the ref, so any ref (including UUIDs, Japanese characters, etc.) is handled safely.

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `OP_VAULT_IDLE_TIMEOUT` | `3600` | Inactivity auto-lock timeout in seconds (applied only at keychain creation) |
| `OP_VAULT_DEBUG` | (unset) | Set to `true` or `1` to enable debug output (`set -x`) |

After the Keychain is created, use `op-vault update-idle-timeout` to change the timeout:

```bash
./op-vault.sh update-idle-timeout 1800  # 30 minutes
```

## Keychain Password

On first run, op-vault prompts whether to set a password on the Keychain:

```
op-vault: キーチェーンにパスワードを設定しますか？ [y/N (default: N)]:
```

The default (no password) allows silent unlocking — no prompt on cache miss. If you set a password, you'll be prompted when the Keychain needs to be unlocked.

## Debugging

```bash
OP_VAULT_DEBUG=true ./op-vault.sh read 'op://Test/test02/password'
```

## License

MIT

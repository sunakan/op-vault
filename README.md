<h1 align="center">op-vault</h1>
<p align="center">A caching layer for <code>op read</code> (1Password CLI) that stores secrets in macOS Keychain.</p>

<p align="center">
  English | <a href="./README.ja.md">日本語</a>
</p>

`op read 'op://Vault/Item/password'` is not exactly fast. op-vault caches the result, returning it instantly on subsequent calls.

## Requirements

- macOS
- [1Password desktop app](https://1password.com/downloads/mac/) with CLI integration enabled
- Go (for building from source)

## Installation

```bash
git clone https://github.com/sunakan/op-vault.git
cd op-vault
make build
mv ./op-vault ~/.local/bin/op-vault  # ensure ~/.local/bin is in your PATH
```

## Quick Start

```bash
# 1. Initialize the keychain (run once)
#    You will be prompted for a password (press Enter to skip)
op-vault init

# 2. Read a secret
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## Subcommands

```
op-vault init              Initialize the keychain
op-vault read <ref>        Get a secret from cache or 1Password
op-vault set <ref> <val>   Manually cache a secret
op-vault status            Show keychain status and cache entry count
op-vault reset             Remove the keychain
op-vault version           Print version
```

`read` and `set` require a 1Password account via `--account` / `-a` or `OP_ACCOUNT`.

## How It Works

op-vault uses a dedicated Keychain (`~/Library/Keychains/op-vault.keychain-db`) to store secrets.

- **Cache hit**: entry exists — returns immediately.
- **Cache miss**: entry doesn't exist — fetches from 1Password, caches, and returns.

If the Keychain is locked, macOS prompts for the password before the lookup. The cache expires when the Keychain auto-locks after inactivity.

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `OP_ACCOUNT` | (required) | 1Password account email or UUID |
| `OP_VAULT_NAME` | `op-vault` | Keychain name |

## Keychain Password

`op-vault init` prompts for a keychain password.

- **No password** (press Enter): macOS prompts, but pressing Enter unlocks it.
- **With password**: macOS prompts and requires the password to unlock.

## Uninstall

```bash
op-vault reset           # delete the keychain
rm ~/.local/bin/op-vault # remove the binary
```

## License

MIT

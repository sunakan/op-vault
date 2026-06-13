<h1 align="center">op-vault</h1>
<p align="center">A caching layer for <code>op read</code> (1Password CLI) that stores secrets in macOS Keychain.</p>

<p align="center">
  English | <a href="./README.ja.md">日本語</a>
</p>

`op read 'op://Vault/Item/password'` takes ~1.8s. op-vault caches the result in macOS Keychain and returns it instantly on subsequent calls.

## Requirements

- macOS
- [1Password desktop app](https://1password.com/downloads/mac/) with CLI integration enabled

## Installation

### Homebrew

```bash
brew install sunakan/op-vault/op-vault
```

### mise

```toml
# mise.toml
[tools]
"github:sunakan/op-vault" = "0.3.1"
```

### Build from source

```bash
git clone https://github.com/sunakan/op-vault.git
cd op-vault
make build
mv ./op-vault ~/.local/bin/op-vault
```

## Quick Start

```bash
# Initialize the keychain (run once)
# Press Enter to skip the password — allows silent auto-unlock after lock
op-vault init

# Read a secret (cache miss fetches from 1Password and caches; cache hit returns immediately)
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## Subcommands

| Command | Description |
|---|---|
| `init` | Initialize the keychain |
| `read <ref>` | Get a secret from cache or 1Password |
| `set <ref> <val>` | Manually cache a secret |
| `refresh` | Re-fetch all cached secrets from 1Password |
| `list` | List all cached op:// refs with last update time |
| `clear` | Remove all cached entries (keychain file is kept) |
| `status` | Show keychain status and cache entry count |
| `reset` | Remove the keychain |
| `version` | Print version |

`read`, `set`, and `refresh` require a 1Password account via `--account` / `-a` or `OP_ACCOUNT`.  
`refresh --prune` additionally removes entries that no longer exist in 1Password.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `OP_ACCOUNT` | — | 1Password account email or UUID |
| `OP_VAULT_NAME` | `op-vault` | Keychain name |

## Uninstall

```bash
op-vault reset          # delete the keychain and all cached secrets
brew uninstall op-vault # remove the binary (Homebrew)
```

## License

MIT

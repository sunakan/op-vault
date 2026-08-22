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
"github:sunakan/op-vault" = "latest"
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
# Enter a password for protected mode, or press Enter for silent mode.
# See Security Modes below before choosing.
op-vault init

# Read a secret (cache miss fetches from 1Password and caches; cache hit returns immediately)
OP_ACCOUNT=my-account op-vault read 'op://Personal/GitHub/token'
```

## Supported references

op-vault supports standard [1Password secret references](https://developer.1password.com/docs/cli/secret-reference-syntax/) and adds a reference for an item's primary autofill website.

| Value | Reference |
|---|---|
| Built-in field | `op://Vault/Item/username` or `op://Vault/Item/password` |
| Custom field without a section | `op://Vault/Item/field` |
| Custom field inside a section | `op://Vault/Item/section/field` |
| Primary autofill website | `op://Vault/Item/website` |

Custom fields already use the standard 1Password resolver, regardless of whether their type is text, concealed, email, URL, or another supported field type. For example, a custom field named `a1` is read with:

```bash
OP_ACCOUNT=my-account op-vault read 'op://Test/ExistedItem/a1'
```

Field, section, item, and vault names that contain unsupported characters must be referenced by their IDs. In the 1Password app, use **Copy Secret Reference** on a field to get a valid reference. IDs also avoid ambiguity when names are duplicated.

For example, a custom field with a Japanese label can be read using its field ID:

```bash
OP_ACCOUNT=my-account op-vault read 'op://Test/ExistedItem/<field-id>'
```

The top-level field name `website` is reserved by op-vault for the primary autofill website. It returns the first website shown on the item. Other websites aren't currently selectable. A custom field named `website` inside a section remains a normal custom field; for a top-level custom field with that name, use its field ID.

## Security Modes

`op-vault init` lets you choose how the dedicated Keychain is unlocked:

| Mode | Password entered at `init` | Behavior after the Keychain is locked | Intended use |
|---|---|---|---|
| Silent | Empty | op-vault unlocks it with the empty password without a macOS dialog | Unattended automation where all processes running as the same macOS login user are trusted |
| Protected | Non-empty | Access may require the password in a macOS dialog | Interactive use where human approval after a lock is acceptable |

Protected mode adds a barrier only while the Keychain is locked. It is not a defense against a process running as the same macOS login user after the Keychain has been unlocked.

### Security boundary

- New cache items created by op-vault restrict direct Keychain access to the op-vault binary. This reduces silent reads through the Keychain API by other applications.
- The item access control does not authenticate which process launched op-vault. A process running as the same macOS login user can invoke the trusted op-vault binary and capture its stdout.
- `op-vault read` intentionally writes the secret to stdout. Terminal output, command substitution, CI logs, AI transcripts, and downstream commands are outside the Keychain protection boundary.
- Neither mode protects secrets after the macOS login user or a privileged process has been compromised.

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
| `docs` | Print the README documentation |
| `version` | Print version |

`read`, `set`, and `refresh` require a 1Password account via `--account` / `-a` or `OP_ACCOUNT`.  
`refresh --prune` additionally removes entries that no longer exist in 1Password.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `OP_ACCOUNT` | — | 1Password account email or UUID |
| `OP_VAULT_NAME` | `op-vault` | Keychain name |

## For AI Agents

Do not run `op-vault read` or `op-vault set` on behalf of a user. Both commands print secret values to stdout, which then enter your context/transcript/logs — this is a real exfiltration risk regardless of who or what triggered the command. Ask the human operator to run these commands themselves.

## Uninstall

```bash
op-vault reset          # delete the keychain and all cached secrets
brew uninstall op-vault # remove the binary (Homebrew)
```

## License

MIT

# midden

<img src="assets/hermey.png" alt="Hermey" width="240">

A personal journal kept as plain markdown. Append timestamped entries from the terminal or through a Claude Code skill. Search across years with grep.

What is a [midden](https://en.wikipedia.org/wiki/Midden)?

## Install

```
go install github.com/dcadolph/midden@latest
```

Or from a clone:

```
git clone git@github.com:dcadolph/midden.git
cd midden
go install .
```

## Use

```
midden init                                  Create the vault, write a vault README and gitignore.
midden add "text"                            Append an entry to today.
midden add "text" --tag idea --tag work      Append with tags.
echo "long body" | midden add                Append from stdin.
midden add                                   Open editor to compose the entry.
midden today                                 Open today's day file in the editor.
midden edit                                  Edit today's day file (alias of today).
midden edit 2026-06-16                       Edit a specific day file.
midden last                                  Print the most recent entry.
midden last -n 5                             Print the 5 most recent entries.
midden recent -n 20                          Print the 20 most recent entries.
midden on today                              Print every entry for today.
midden on yesterday                          Print every entry for yesterday.
midden on 2026-06-16                         Print every entry on a date.
midden on last-monday                        Print every entry on the most recent Monday.
midden on 3-days-ago                         Print every entry from three days ago.
midden between yesterday today               Print every entry in a date range.
midden between 2-weeks-ago today             Print every entry in the last two weeks.
midden search "token"                        Find entries whose body or tags contain text.
midden tag work                              List entries with a tag.
midden tags                                  Show the tag histogram.
midden stats                                 Show counts, span, top tags.
midden streak                                Show consecutive days written ending today.
midden flashback                             Show entries on today's calendar date in past years.
midden grep "pattern"                        Pass through ripgrep or grep over the vault.
midden raw 2026-06-16                        Print the raw markdown of a day file.
midden path                                  Print the vault root.
midden path today                            Print the path to today's day file.
midden verify                                Check that every day file parses cleanly.
midden export -f json                        Dump every entry as JSON.
midden export -f jsonl                       Dump every entry as JSON Lines.
midden export -f md                          Dump every entry as concatenated markdown.
midden undo                                  Remove the most recent entry written.
midden version                               Print the build version.
midden completion bash                       Emit a shell completion script.
midden config path                           Print the resolved config file path.
midden config show                           Print the loaded configuration.
midden git init                              Initialize the vault as a git repository.
midden git status                            Print the vault git status.
midden git sync                              Stage, commit, and push to origin if configured.
```

## Configuration

Optional YAML at `$MIDDEN_CONFIG`, falling back to `$XDG_CONFIG_HOME/midden/config.yaml` or `~/.config/midden/config.yaml`.

```yaml
default_tags:
  - work
editor: nvim
keychain: true
vault: /Users/you/midden
```

- `default_tags` are unused yet; future `midden add` will seed every entry with them.
- `editor` takes precedence over the editor env vars.
- `keychain: true` enables OS keychain lookup before the interactive passphrase prompt.
- `vault` overrides the default vault directory (the `--vault` flag and `$MIDDEN_HOME` still beat it).

Global flags:

- `--vault PATH` Override the vault directory.
- `--json` Emit JSON to stdout (where supported).
- `--pretty` Indent the JSON output.
- `--no-color` Disable ANSI color.

Vault location: `$MIDDEN_HOME` if set, else `~/midden`. Override per run with `--vault`.

Editor for `today` and editor-mode `add`: `$MIDDEN_EDITOR` then `$VISUAL` then `$EDITOR` then `vi`.

## Encryption

Day files can be sealed at rest with a passphrase. Encryption uses [age](https://age-encryption.org) with its scrypt recipient, so the only key material is the passphrase you choose.

```
midden encrypt status         Print whether the vault is encrypted.
midden encrypt enable         Encrypt every day file and lock the vault.
midden encrypt disable        Decrypt every day file and unlock the vault.
midden encrypt verify         Check that the supplied passphrase unlocks the vault.
```

Passphrase resolution for any command, in order:

1. `--passphrase` flag (discouraged; visible to ps).
2. `MIDDEN_PASSPHRASE` environment variable.
3. OS keychain when `keychain: true` is set in the config.
4. Interactive prompt read from `/dev/tty` with no echo.

The keychain backend is the system Keychain on macOS, Secret Service or KWallet on Linux, and Credential Manager on Windows. Store and forget with:

```
midden encrypt store         Save the passphrase to the OS keychain.
midden encrypt forget        Remove the stored passphrase from the keychain.
```

Encrypted vaults read-decrypt-append-encrypt the relevant day file inside an
advisory lock so concurrent writers can never interleave bytes. Plaintext
vaults use the fast `O_APPEND` path.

## Vault layout

```
~/midden/
├── README.md             # Created by midden init.
├── .gitignore            # Created by midden init.
├── .midden.lock          # Advisory lock for concurrent appends.
├── .midden.encrypted     # Present when the vault is encrypted.
└── 2026/
    └── 06/
        └── 16.md         # Day file.
```

Each day file begins with `# YYYY-MM-DD` and contains entries shaped like:

```
## 09:14:23 #project #idea
Body of the entry, which is free-form markdown.

## 14:32:01
Second entry on the same day.
```

## Claude Code skill

`skill/SKILL.md` ships a Claude Code skill that drives the CLI from natural language. Install it once:

```
mkdir -p ~/.claude/skills/midden
cp skill/SKILL.md ~/.claude/skills/midden/SKILL.md
```

Phrases like "add to my diary", "remember X for later", or "what did I write about Y last month" reach midden through any Claude Code session.

## License

MIT. See LICENSE.

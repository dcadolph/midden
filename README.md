<p align="center">
  <img src="assets/hermey.png" alt="midden" width="100%">
</p>

# midden

[![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A personal journal kept as plain markdown. Append timestamped entries from the terminal or through a Claude Code skill. Search across years with grep, semantic recall, or by asking questions in plain language.

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

## Quick start

```
midden init                          Create the vault, write a vault README and gitignore.
midden ingest ics calendar.ics       Backfill past events from a calendar export.
midden ingest git ~/src/project      Backfill what you were working on, from commit history.
midden add "text"                    Append an entry to today.
midden add "text" --tag work         Append with tags.
midden today                         Open today's day file in the editor.
midden on yesterday                  Print every entry for yesterday.
midden search "token"                Find entries whose body or tags contain text.
midden chat "when did I see Mom?"    Ask a question answered from your own entries.
```

Full command list and every integration are in the sections below.

## Commands

<details>
<summary><b>Write</b></summary>

```
midden add "text"                            Append an entry to today.
midden add "text" --tag idea --tag work      Append with tags.
echo "long body" | midden add                Append from stdin.
midden add                                   Open editor to compose the entry.
midden today                                 Open today's day file in the editor.
midden edit                                  Edit today's day file (alias of today).
midden edit 2026-06-16                       Edit a specific day file.
midden undo                                  Remove the most recent entry written.
midden import path/to/note.md --tag inbox    Append a file as one entry on today.
midden import - --date 2026-06-10            Read stdin and file it on a chosen date.
midden audio                                 Record a voice memo and append it to today.
midden audio --duration 30s --transcribe     Record for 30s then transcribe with OpenAI Whisper.
midden ingest ics calendar.ics               Append calendar events from an .ics export.
midden ingest ics calendar.ics --since 2015-01-01   Narrow the range to ingest.
midden ingest git ~/src/one ~/src/two        Append commit history from local repositories.
midden ingest git ~/src/work --author me@example.com --stat
```

Ingest reads the whole export by default, because backfilling years of calendar history is the point
of it. Recurring series are expanded into the occurrences they actually produced, so a weekly one-to-one
running since 2019 contributes every week rather than a single event in 2019. `EXDATE` cancellations are
honored and instances the calendar moved replace the occurrence they override, so a rescheduled meeting
appears once at its real time rather than twice. Occurrences already in the vault are skipped, so running
the same import twice changes nothing.

Rules using `BYSETPOS`, `BYYEARDAY`, or `BYWEEKNO` are not expanded on those parts, and a frequency
outside daily, weekly, monthly, and yearly is not expanded at all. Ingest counts and reports both cases
rather than passing off a partial calendar as a complete one.

`ingest git` appends one entry per commit across any number of local repositories. A calendar says where
you were; commit history says what you were working on, and the two together reconstruct a working life
far better than either alone. Commits are filed by author date, so rebased or cherry-picked work still
lands on the day it was written. Merge commits are skipped unless `--merges` is given, `--author` narrows
a shared repository to your own commits, and `--stat` adds changed-file and line counts at the cost of a
diff per commit. Commits already in the vault are skipped by hash.

</details>

<details>
<summary><b>Read</b></summary>

```
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
midden weekly                                Print a 7-day digest grouped by day.
midden weekly --offset 1                     Digest the prior week.
midden flashback                             Show entries on today's calendar date in past years.
midden raw 2026-06-16                        Print the raw markdown of a day file.
```

</details>

<details>
<summary><b>Search</b></summary>

```
midden search "token"                        Find entries whose body or tags contain text.
midden grep "pattern"                        Pass through ripgrep or grep over the vault.
midden tag work                              List entries with a tag.
midden tags                                  Show the tag histogram.
midden recall "token rotation strategy"      Semantic search over indexed entries.
midden chat "when did I last see Mom?"       Ask an LLM a question using recalled entries as evidence.
midden chat --since 30-days-ago "what did I do?"   Answer from every entry in a date range.
midden chat --sweep "what do you know about my life?"   Answer from the whole vault.
midden reindex                               Build the embedding index used by recall.
midden reindex --full                        Re-embed everything, needed only after changing provider.
```

Reindex reuses the vector it already has for any entry whose text has not changed, so rebuilding after
adding a day costs one provider call rather than re-embedding the whole vault. Long rebuilds checkpoint
as they go and each provider call has its own deadline, so an interrupted backfill resumes from where it
stopped instead of throwing away the embeddings it already paid for.

`recall` and `chat` both accept `--since` and `--until`, which take any date `midden` understands
(`2024-03-01`, `30-days-ago`, `monday`). Scoping matters because ranking by similarity alone answers
"what did I write about X" well and "what happened last March" badly: the closest matches to a question
about a period are often entries from other periods. Giving `chat` a range makes it read every entry in
that range instead of the closest few, summarizing in chunks when the range is too large to read at
once. `--sweep` does the same across whatever is in scope, which is the whole vault by default.

Every `chat` answer also carries a summary counted over every indexed entry in scope: how many entries,
what span they cover, the tag histogram, and entries per month. Questions about the shape of the record
are answered from those counts rather than from a handful of retrieved entries.

`--context-chars` sets how much entry text goes to the model in one call. The default suits a model with
a large context window. A small local model needs a much lower value, because it spends minutes on a
prompt a hosted model reads in seconds, which makes a sweep look like a hang. Try `--context-chars 6000`
against a 3B local model and raise it from there.

</details>

<details>
<summary><b>Inspect and export</b></summary>

```
midden stats                                 Show counts, span, top tags.
midden streak                                Show consecutive days written ending today.
midden verify                                Check that every day file parses cleanly.
midden export -f json                        Dump every entry as JSON.
midden export -f jsonl                       Dump every entry as JSON Lines.
midden export -f md                          Dump every entry as concatenated markdown.
midden report html -o report.html            Render an HTML report (dark mode aware).
midden path                                  Print the vault root.
midden path today                            Print the path to today's day file.
```

</details>

<details>
<summary><b>Git, config, misc</b></summary>

```
midden git init                              Initialize the vault as a git repository.
midden git status                            Print the vault git status.
midden git sync                              Stage, commit, and push to origin if configured.
midden config path                           Print the resolved config file path.
midden config show                           Print the loaded configuration.
midden completion bash                       Emit a shell completion script.
midden version                               Print the build version.
```

</details>

## Integrations and details

<details>
<summary><b>LLM and audio</b></summary>

Recall, chat, reindex, and Whisper transcription call external models. Each provider is selected from environment variables; midden never sends anything until you opt in by setting one.

| Action | Variables (first match wins) |
|---|---|
| Embeddings | `VOYAGE_API_KEY` → `OPENAI_API_KEY` → local Ollama at `OLLAMA_HOST`. Force with `MIDDEN_EMBED_PROVIDER`. |
| Chat | `ANTHROPIC_API_KEY` → `OPENAI_API_KEY` → local Ollama. Force with `MIDDEN_CHAT_PROVIDER`. |
| Transcription | `OPENAI_API_KEY` (Whisper). |

Model overrides: `OPENAI_EMBED_MODEL`, `VOYAGE_EMBED_MODEL`, `OLLAMA_EMBED_MODEL`, `ANTHROPIC_MODEL`, `OPENAI_CHAT_MODEL`, `OLLAMA_CHAT_MODEL`.

Run `midden reindex` after major writes to keep the embedding index fresh. The index file lives at `<vault>/.midden.index.json`.

Audio capture uses the first available recorder in this order: `sox`, `rec`, `ffmpeg` (avfoundation on macOS, alsa on Linux, dshow on Windows). Recorded WAVs land in `<vault>/audio/YYYY/MM/DD/HH-MM-SS.wav` and the day file gains a linking entry.

</details>

<details>
<summary><b>Configuration</b></summary>

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

</details>

<details>
<summary><b>Encryption</b></summary>

Day files can be sealed at rest with a passphrase. Encryption uses [age](https://age-encryption.org) with its scrypt recipient, so the only key material is the passphrase you choose.

```
midden encrypt status         Print whether the vault is encrypted.
midden encrypt enable         Encrypt every day file and lock the vault.
midden encrypt disable        Decrypt every day file and unlock the vault.
midden encrypt verify         Check that the supplied passphrase unlocks the vault.
midden encrypt store          Save the passphrase to the OS keychain.
midden encrypt forget         Remove the stored passphrase from the keychain.
```

Passphrase resolution for any command, in order:

1. `--passphrase` flag (discouraged; visible to ps).
2. `MIDDEN_PASSPHRASE` environment variable.
3. OS keychain when `keychain: true` is set in the config.
4. Interactive prompt read from `/dev/tty` with no echo.

The keychain backend is the system Keychain on macOS, Secret Service or KWallet on Linux, and Credential Manager on Windows.

Encrypted vaults read-decrypt-append-encrypt the relevant day file inside an advisory lock so concurrent writers can never interleave bytes. Plaintext vaults use the fast `O_APPEND` path.

</details>

<details>
<summary><b>Vault layout</b></summary>

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

</details>

<details>
<summary><b>Claude Code skill</b></summary>

`skill/SKILL.md` ships a Claude Code skill that drives the CLI from natural language. Install it once:

```
mkdir -p ~/.claude/skills/midden
cp skill/SKILL.md ~/.claude/skills/midden/SKILL.md
```

Phrases like "add to my diary", "remember X for later", or "what did I write about Y last month" reach midden through any Claude Code session.

</details>

## License

MIT. See LICENSE.

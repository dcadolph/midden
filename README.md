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
midden voice                                 Speak an entry; the transcript is appended to today.
midden voice --tag garden --keep-audio       Tag the spoken entry and keep the WAV linked in the vault.
midden audio                                 Record a voice memo and append it to today.
midden audio --duration 30s --transcribe     Record for 30s then transcribe the memo.
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
midden weave --tag calendar                  Show what recurs, when it started, and when it stopped.
midden people                                List the people your record mentions, and who has faded.
midden classify --apply                      Judge whether two names are one thing; you decide.
midden ask -i                                Answer a question about a gap in your own record.
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

### Classify

The arithmetic refuses two kinds of merge on principle: a title that names nobody never joins one that
names someone, and Will and William are two strings. `midden classify` is where those calls get made,
under strict rules. Arithmetic generates the candidate pairs; a model answers only yes or no about each
pair under a fixed schema, and an unparseable answer counts as no answer rather than a guess. Every
verdict is a proposal shown with its evidence, and nothing changes until you accept it in `--apply`.

An accepted merge is recorded as an ordinary entry carrying a `MIDDEN-SAME:` marker, which weave and
people then read; a rejection is recorded the same way so a pair is never proposed twice. The model never
writes to the vault, verdict entries never count as your writing or as mentions of anyone, and the tool
works fully without a model configured. The judged calls are exactly as good as the model making them,
which is why the human accept is the only thing that changes state: a wrong model costs you a wrong
proposal, never a wrong record.

### Scheduled entries

A vault holding an imported calendar contains appointments that have not happened yet, which quietly
breaks anything meaning "latest". `last` and `recent` therefore stop at now, and `stats` reports what is
booked ahead separately from the span of what actually happened:

```
Span
  First: 2022-08-18 07:00:00
  Last:  2026-08-26 15:10:23
  Ahead: 264 scheduled, through 2027-03-12
```

Pass `--future` to `last` or `recent` when you do want what is coming. `undo` never touches a scheduled
entry: it removes the last thing you wrote, and nothing you wrote lives in the future.

`streak` counts only days you actually wrote something. A backfilled vault has entries on thousands of
days the person never wrote a word, and a streak counted over imported events would congratulate you for
appointments you merely attended.

### People

`midden people` counts every name in the record and reports who recurs, over what span, and who has
gone quiet. Capitalization alone cannot tell a person from a commit verb, so a name only counts as a
person when the record uses person-grammar about it somewhere: a possessive ("Jax's grooming") or a
companion preposition ("sleepover with Kayla"). Blocklists rot; grammar does not. A name that recurred
for a long stretch and then vanished for over a year is flagged with its last-seen date, because a
faded relationship is exactly the kind of fact nobody writes down.

Weave also segments the whole record into eras: stretches of consistent volume found by where the
monthly counts shift, on a log scale so the difference between four entries a month and none weighs
more than the difference between four hundred and one hundred. The chapters of a record fall out
without anyone naming them.

### Ask

Backfill has a ceiling, and it is worth being plain about where it sits. Calendars record where you were
scheduled. Commit logs record what you shipped. Both are projections of a life rather than the life, and
neither carries what you thought or decided, because nothing recorded that at the time. No further import
fixes this: for anyone who was not already keeping a journal, that material does not exist to import.

`midden ask` closes the gap the only way it can be closed. It reads what weave computed, finds a place
the record proves something is missing, and asks about it:

```
William- Martial arts stopped. What happened?
  216 times over 2.6 years, ending 2025-08-07. Nothing since, 1.0 years ago.
```

Answer it and the reply becomes an ordinary entry, with your words leading and the question trailing as
attribution, so the record shows what you said rather than what midden asked. That question is never
asked again. `--skip` dismisses one for good, because a queue that keeps returning a question you have
already rejected teaches you to stop reading it.

Questions are only raised about things worth explaining. A commitment that ran for months and stopped
qualifies; a school-year reminder repeated for one term does not, and neither does anything merely
between seasons. Questions come
from arithmetic over the record, never from a model, so nothing is asked about something that did not
happen. Threads still running are never asked about at all: frequency alone cannot tell a commitment that
mattered from a chore that recurred, and a vapid prompt teaches you to ignore the next one.

An answer is the first thing in a vault that no import could have produced.

### Weave

Search answers what you already know to ask about. `midden weave` answers what you cannot ask.

A person can recall what they did but cannot perceive absence, because nothing marks the last time
something happened. Weave groups the record into threads, measures each one's own cadence, and reports
which have gone quiet for far longer than their rhythm allows. It also finds handoffs, where one thread
ended and another began soon after, and crossings, the days where separate sources both recorded
something and so can say what neither says alone.

Threads are grouped by the meaningful words in a title, which survives the drift of a handwritten
calendar without merging things that are genuinely different. A word added to a short title is a subject,
not noise: "Doctor appt" and "Hannah doctor appt" overlap heavily but the second says whose appointment
it was, and folding them together would treat two people's appointments as one thread and then report a
gap spanning the distance between two unrelated lives. `--explain` lists the headlines behind every
thread, because a claim that something ended rests entirely on what was grouped together, and that has to
be checkable before it is believed.

A thread that has gone quiet is not automatically over. Something that has already come back from a gap
this long before is between seasons, not finished, and weave says so rather than announcing an ending
that never happened. A spring show silent in August has simply not come round yet.

Silences are found per source as well as overall, because one loud source can flood the months where
another went quiet: commits pouring in during years the calendar recorded nothing would otherwise hide
exactly the silence worth asking about. A silence found in the whole record is never repeated per source,
and ask raises at most one silence per source, the longest.

It also finds silences: stretches where the record itself went quiet, bounded by activity on both sides
so the start and end of a record are never mistaken for holes. A thread ending is one commitment
stopping. A silence is the record failing, which is both larger and completely invisible from inside it,
since a person notices a class ending but never that years went unrecorded.

Every figure is counted rather than inferred. There is no model in the detection path and nothing to
invent. Threads are grouped by the meaningful words in a title rather than the title itself, because a
handwritten calendar records one standing arrangement under many spellings, and grouping on the exact
string splits it into fragments that each appear to end whenever the wording drifts.

Use `--tag calendar` to weave a life source on its own. Commit history repeats boilerplate subjects
across repositories, which crowds out real threads.

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
midden report html -o your-life.html         Render the whole record's shape as one page.
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

Recall, chat, and reindex call external models. Each provider is selected from environment variables; midden never sends anything until you opt in by setting one. Transcription is the exception: it runs locally through whisper.cpp by default and only reaches the network when the openai backend is selected.

| Action | Variables (first match wins) |
|---|---|
| Embeddings | `VOYAGE_API_KEY` → `OPENAI_API_KEY` → local Ollama at `OLLAMA_HOST`. Force with `MIDDEN_EMBED_PROVIDER`. |
| Chat | `ANTHROPIC_API_KEY` → `OPENAI_API_KEY` → local Ollama. Force with `MIDDEN_CHAT_PROVIDER`. |
| Transcription | Local whisper.cpp (`brew install whisper-cpp`). Config `whisper_backend: openai` or `midden voice --cloud` uses the Whisper API with `OPENAI_API_KEY` instead. |

Model overrides: `OPENAI_EMBED_MODEL`, `VOYAGE_EMBED_MODEL`, `OLLAMA_EMBED_MODEL`, `ANTHROPIC_MODEL`, `OPENAI_CHAT_MODEL`, `OLLAMA_CHAT_MODEL`.

Run `midden reindex` after major writes to keep the embedding index fresh. The index file lives at `<vault>/.midden.index.json`.

The index stores entry bodies verbatim so recall can quote them back, so it is encrypted with the vault rather than beside it. `encrypt enable`, `disable`, and `verify` all cover it along with the day files.

Audio capture uses the first available recorder in this order: `sox`, `rec`, `ffmpeg` (avfoundation on macOS, alsa on Linux, dshow on Windows). `midden audio` keeps the WAV at `<vault>/audio/YYYY/MM/DD/HH-MM-SS.wav` and the day file gains a linking entry. `midden voice` is the journaling shortcut: it records, transcribes locally, appends the transcript as a plain entry, and discards the audio unless `--keep-audio` is set. The local model downloads to the midden data directory on first use (`base.en` by default, overridable with `whisper_model`).

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
whisper_backend: local
whisper_model: base.en
```

- `default_tags` are unused yet; future `midden add` will seed every entry with them.
- `editor` takes precedence over the editor env vars.
- `keychain: true` enables OS keychain lookup before the interactive passphrase prompt.
- `vault` overrides the default vault directory (the `--vault` flag and `$MIDDEN_HOME` still beat it).
- `whisper_backend` selects the transcription backend: `local` (default) or `openai`.
- `whisper_model` names the local Whisper model (`base.en`, `small.en`, ...) or points at a ggml `.bin` file.

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
<summary><b>iPhone app</b></summary>

`ios/` holds a SwiftUI app that captures entries by voice or keyboard, lists today and the trailing week, and exposes a "Journal" App Intent so Siri, Shortcuts, and the Action button can append without opening the app. Dictation uses on-device speech recognition, so spoken entries never leave the phone.

The app does not reimplement the journal. `mobile/` wraps the vault, encryption, and date handling for `gomobile bind`, and the app links the resulting framework, so both the app and the command line read and write one format.

```
go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init
ios/build-core.sh          # builds ios/Frameworks/MiddenCore.xcframework
cd ios && xcodegen generate && open Midden.xcodeproj
```

The framework and the Xcode project are build products and are not tracked. Entry timestamps cross the boundary as local wall clock without a zone offset, matching what a day file records; the app supplies the timestamp because a bound framework cannot trust `time.Local` on iOS.

Set `DEVELOPMENT_TEAM` in `project.yml` to your own team before building for a device, and sign in to that Apple ID in Xcode so automatic signing can issue a profile. Running on your own phone needs nothing further; TestFlight additionally needs an App Store Connect record for the bundle identifier.

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

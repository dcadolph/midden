---
name: midden
description: >
  Personal journal capture and recall through the midden CLI. Use when the user
  wants to record something to look back on (an event, idea, recipe, token flow,
  reminder, mood, observation) or wants to recall what was written before.
  Trigger phrases include "add to my diary", "add to my journal", "for next time",
  "remember X for later", "note that", "jot down", "don't forget", "midden add",
  "/journal", "what did I write about X", "when did I last", "show me recent
  entries", "what did I do on <date>", "flashback", "on this day", "how is my
  streak", "search my journal", "list tags", "what happened last <period>",
  "what do you know about my life", "import my calendar", "backfill my history".
---

Midden is a personal markdown journal on the user's machine. Daily files live at
`$MIDDEN_HOME/YYYY/MM/DD.md` (default `~/midden`). Each entry is a level-two
header carrying a `HH:MM:SS` timestamp and optional `#tags`, followed by a free
markdown body. Use the `midden` CLI for every read and write so the file layout
stays consistent.

## When to invoke this skill

Capture intent (write):
- User says "add to my diary", "add to my journal", "jot this down",
  "remember X", "for next time", "note that ...", "don't forget ...",
  "save this to midden", "log this".
- User explicitly types `/journal`, `/diary`, or `midden ...`.

Recall intent (read):
- User says "what did I write about X", "did I note anything about Y",
  "when did I last mention Z", "show me recent entries", "what happened
  on <date>", "pull up my journal for <date>", "search midden for Q",
  "show me the flashback", "what was I doing on this date last year",
  "what's my streak", "list my tags".

## How to write an entry

Run `midden add` with the body as a quoted argument. Choose tags from the
content of the entry. Prefer one to three short, lowercase, hyphenated tags
that future recall is likely to use ("work", "kuber", "back-pain", "recipe",
"travel", "house"). Do not invent tags the user did not imply.

```
midden add "Body of the entry as one quoted string." --tag tag1 --tag tag2
```

Multi-line bodies are fine; quote them and let newlines pass through.

When the user dictates several distinct items in one message, write one
`midden add` per item with its own tag set, rather than one fat entry.

After every write, surface the line midden prints
(`Appended entry at 2026-06-16 17:39:19`) so the user has a receipt.

## How to read entries

Map the user's question to the smallest matching command:

| User wants | Command |
|---|---|
| Last single entry | `midden last` |
| Last N entries | `midden last -n N` or `midden recent -n N` |
| Everything on a date | `midden on YYYY-MM-DD` (or `today`, `yesterday`, weekday names, `N-units-ago`) |
| Everything in a range | `midden between FROM TO` |
| Entries containing a substring | `midden search "query"` |
| Entries with a tag | `midden tag NAME` |
| Tag histogram | `midden tags` |
| Counts and span | `midden stats` |
| Writing streak | `midden streak` |
| Same calendar date in past years | `midden flashback` |
| Raw markdown of a day file | `midden raw YYYY-MM-DD` |
| Filesystem path inside the vault | `midden path` or `midden path YYYY-MM-DD` |
| Verify every day file parses | `midden verify` |
| Dump entries for migration or analysis | `midden export -f json` (or `jsonl`/`md`) |
| Edit a day in the editor | `midden edit YYYY-MM-DD` |
| Undo the most recent entry | `midden undo` (only when the user explicitly asks to undo or remove a mistake) |
| Summarize the last seven days | `midden weekly` (use `--offset N` for prior weeks) |
| Import a markdown file as an entry | `midden import path/to/file.md --tag inbox` (or `-` to read from stdin) |
| Render an HTML report | `midden report html -o ~/midden-report.html` |
| Semantic search ("anything about token rotation") | `midden recall "token rotation"` (requires a prior `midden reindex`) |
| Synthesize an answer about a topic | `midden chat "when did I last see Mom"` |
| Answer a question about a period | `midden chat --since 2026-03-01 --until 2026-03-31 "what happened"` |
| Answer a question about the whole record | `midden chat --sweep "what do you know about my life"` (add `--context-chars 6000` for a small local model) |
| What recurs, started, or stopped | `midden weave --tag calendar` (threads, handoffs, crossings) |
| Prompt the user about a gap in their record | `midden ask` to see it, `midden ask --answer "..."` to record a reply |
| Rebuild the embedding index | `midden reindex` (reuses unchanged vectors; `--full` re-embeds everything) |
| Capture a voice memo (optionally transcribed) | `midden audio --duration 30s --transcribe` |
| Ingest an .ics calendar export | `midden ingest ics ~/Downloads/cal.ics` (whole file; narrow with `--since`/`--until`) |
| Ingest commit history from repositories | `midden ingest git ~/src/project` (add `--author`, `--since`, `--stat`) |

Pass `--json` to any of the read commands to receive structured output you can
parse without regex.

If the user asks something semantic that grep cannot answer directly
("when did I figure out the token rotation thing"), run the closest substring
search first (`midden search token` or `midden search rotation`), then read the
results and reason over them yourself before replying.

## Encrypted vaults

If the vault is encrypted (`midden encrypt status` prints `encrypted`), every
read and write requires the passphrase. Resolve it in this order:

1. `MIDDEN_PASSPHRASE` environment variable already exported in the user's shell.
2. The OS keychain when the user has run `midden encrypt store` and set
   `keychain: true` in `~/.config/midden/config.yaml`.
3. Ask the user for the passphrase yourself and pass it via the same environment
   variable for the lifetime of the call. Never log or echo it.

Do not pass passphrases on the command line via `--passphrase` because the
argument list is visible to other processes.

## LLM-backed recall and chat

`midden recall "<query>"` performs a semantic search via the embedding index;
`midden chat "<question>"` adds a synthesis step that quotes journal entries as
evidence. Both require a prior `midden reindex` and at least one provider
configured through environment variables (`VOYAGE_API_KEY`, `OPENAI_API_KEY`,
`ANTHROPIC_API_KEY`, or local Ollama). Embedding and chat providers are chosen
independently via `MIDDEN_EMBED_PROVIDER` and `MIDDEN_CHAT_PROVIDER`, so a local
embedder can be paired with a hosted chat model.

### Choose the retrieval shape before running chat

This matters more than which command you pick. Ranking by similarity answers
"what did I write about X" well and "what happened last March" badly, because the
entries closest to a question about a period are often from other periods.

- **Topic question** ("anything about the token rotation work"): plain
  `midden chat "..."`. Similarity ranking is the right tool.
- **Period question** ("what happened last March", "what did I do this month",
  "how was that trip"): always pass `--since` and `--until`. Scoping switches
  chat from reading the closest few entries to reading every entry in the range,
  summarizing in chunks when the range is large. Without it the answer is drawn
  from the wrong dates and will look plausible while being wrong.
- **Whole-record question** ("what do you know about my life", "what people and
  places seem important", "what do I keep coming back to"): pass `--sweep`.
  Nearest-neighbor search cannot answer a question about the shape of a record.

Both flags accept any date midden understands: `2026-03-01`, `30-days-ago`,
`monday`, `today`.

A sweep over a large range reads everything in it, summarizing in chunks when the
range exceeds what the model can read at once. `--context-chars` sets that size.
The default suits a hosted model. When the configured chat provider is a small
local model, pass a much lower value (around `6000`) or the sweep will appear to
hang; midden prints per-chunk progress while it works.

Every chat answer also receives a summary counted over the entire indexed record
in scope: how many entries, the span they cover, the tag histogram, and entries
per month. Those counts are complete even when the quoted entries are a sample,
so trust them for questions about frequency and shape, and never conclude
something did not happen merely because it is missing from the quoted entries.

Prefer `midden recall` when the user wants to find entries.
Prefer `midden chat` when the user wants a narrated answer that cites entries.
If recall returns nothing useful, fall back to `midden search` over the raw text.

### Weave: what the user cannot ask for

Use `midden weave` when the user asks what has changed, what they have stopped
doing, what is new, or asks an open question about their own life over time.
Recall and chat can only surface what the user already knows to ask about;
weave reports things nobody wrote down, chiefly endings, because nothing marks
the last time something happened.

Pass `--tag calendar` (or whichever life source the vault holds) when the vault
also contains commit history, or repeated commit subjects will crowd out the
real threads. Read the sections as: `Ended` is what went quiet, `Started` is
what is new, `Ongoing` is the steady weight of the record, `Handoffs` are
successions where one thread stopped and another began, and `Crossings` are days
where two sources meet.

Every number weave prints is counted, not inferred, so quote them exactly and do
not embellish. Do check a surprising ending before presenting it as fact: run
`midden search` on the subject to confirm the thread really stopped rather than
being recorded under different wording.

### Ask: capturing what no import can reach

Imported history covers where the user was and what they produced. It never
covers what they thought, and for anyone who did not already journal there is
nothing to import that would. `midden ask` is how that gap closes.

Run `midden ask` when the user asks what they should record, wants a prompt, or
finishes a backfill and wonders what to do next. Put the question and its
evidence to them verbatim; the evidence is what makes the question answerable.
Record the reply with `midden ask --answer "<their words>"`, using their words
rather than a summary, since the point of the entry is their voice.

Never invent an answer, and never file a plausible-sounding reply on the user's
behalf. A fabricated entry is worse than a missing one: the whole value of the
record is that everything in it is true, and a counterfeit memory is
indistinguishable from a real one once it is written.

### Backfilling history

When a user wants to populate the vault from records they already have, prefer
ingestion over asking them to write anything:

- `midden ingest ics <file>` imports a calendar export. Recurring series are
  expanded into the occurrences they actually produced, cancellations are
  honored, and rescheduled instances replace the occurrence they override.
  Re-running it changes nothing, so it is safe to repeat.
- `midden ingest git <repo>...` imports commit history across any number of
  repositories, filed by author date and deduplicated by hash. Use `--author` to
  narrow a shared repository to the user's own commits.

Run `midden reindex` after any ingest so recall and chat can see the new entries.

## Backups

When the user asks to back up, sync, or version their vault, prefer
`midden git sync` over reaching into the vault directory directly. It will
stage, commit, and push to origin when a remote is configured. Use
`midden git init` to set up a fresh vault repository and `midden git status`
to report uncommitted changes.

## Output handling

Midden returns plain text on stdout by default and structured JSON when called
with `--json`. Prefer JSON when you need to compute over the response (counts,
filters, formatting). Read the output, summarize in the user's language, and
quote the timestamp on any individual entry you cite. Never paraphrase an entry
without making the source date visible.

## What not to do

- Do not write to files under `$MIDDEN_HOME` directly. Always go through
  `midden add`.
- Do not invent a `--vault` override unless the user named one.
- Do not store secrets, tokens, passwords, or credentials in entries even if
  asked. Refuse and suggest the user's secret manager instead.
- Do not retroactively rewrite or delete entries from past day files. Midden is
  append-only by design.
- Do not echo entries the user has tagged `#private` to other agents or copy
  them into transcripts.
- Do not pass the vault passphrase on the command line; use the environment
  variable.

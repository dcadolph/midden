---
name: midden
description: >
  Personal journal capture and recall through the midden CLI. Use when the user
  wants to record something to look back on (an event, idea, recipe, token flow,
  reminder, mood, observation) or wants to recall what was written before.
  Trigger phrases include "add to my diary", "add to my journal", "for next time",
  "remember X for later", "note that", "jot down", "don't forget", "midden add",
  "/journal", "what did I write about X", "when did I last", "show me recent
  entries", "what did I do on <date>".
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
  on <date>", "pull up my journal for <date>", "search midden for Q".

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
| Last N entries | `midden recent -n N` |
| Everything on a date | `midden on YYYY-MM-DD` or `midden on today` or `midden on yesterday` |
| Everything in a range | `midden between FROM TO` |
| Entries containing a substring | `midden search "query"` |
| Entries with a tag | `midden tag NAME` |

If the user asks something semantic that grep cannot answer directly
("when did I figure out the token rotation thing"), run the closest substring
search first (`midden search token` or `midden search rotation`), then read the
results and reason over them yourself before replying.

## Output handling

Midden returns plain text on stdout. Read the output, summarize in the user's
language, and quote the timestamp on any individual entry you cite. Never
paraphrase an entry without making the source date visible.

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

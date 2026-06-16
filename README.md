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
midden add "Some entry text." --tag idea --tag work
midden today
midden recent -n 10
midden on today
midden on 2026-06-16
midden between yesterday today
midden search "token"
midden tag work
```

Vault location: `$MIDDEN_HOME` if set, else `~/midden`. Override per run with `--vault`.

Editor for `today` and editor-mode `add`: `$MIDDEN_EDITOR` then `$VISUAL` then `$EDITOR` then `vi`.

## Claude Code skill

`skill/SKILL.md` ships a Claude Code skill that drives the CLI from natural language. Install it once:

```
mkdir -p ~/.claude/skills/midden
cp skill/SKILL.md ~/.claude/skills/midden/SKILL.md
```

Then trigger phrases like "add to my diary", "remember X for later", or "what did I write about Y" reach midden through any Claude Code session.

## License

MIT. See LICENSE.

---
name: discord-cli
description: >
  Use this skill whenever the user wants an agent to interact with their own
  Discord account through the `discord` command-line tool: reading recent
  messages, searching conversations, listing guilds, channels, or DMs,
  exporting chat history, checking what is being discussed, or searching
  existing exports on disk. Treat requests like "check Discord", "what are
  people saying about X", "search the server", "export that channel", "read
  the latest messages", or "what did someone post" as triggers. Skip for
  questions about Discord's own app, bot configuration, or server
  administration, where the user needs an explanation rather than an action.
---

# discord-cli

`discord` reads the user's own Discord account with its own user token: guilds, channels, threads, DMs, live search, exports to JSON, and fast local search over those exports. Commands are `discord <noun> <verb> [argument] [flags]`; `--help` works anywhere. Source: https://github.com/poamslayer/discord-cli. After upgrading, re-install this file: `discord help --skill > ~/.claude/skills/discord-cli/SKILL.md`.

## Output for agents

The tool detects an agent (`CLAUDECODE=1` and similar) and, when no format is given, prints lean output: one line per message for message commands (`guild-slug/channel-slug:message-id:timestamp:author: content`, content cut at `--width`, 200 by default), tsv for list commands, and at most 10 search hits. A line on stderr says how many hits there were and how to open one. Do not pass `--json` to read results; pass it only when a script parses the output or you need attachments, embeds, or reactions (with `--context`, the message asked for carries `"match": true`). `--format` (`human`, `json`, `compact`, `tsv`) overrides the default; `DISCORD_AGENT=0` turns it off.

Search, then read. A search hit is an address; `message read` opens it, printing the message whole and the context around it:

```bash
discord export search "onboarding" --guild my-guild
discord message read <id from a search hit> --context 5
discord guild search "onboarding" --channel general
discord message read <id from a search hit> --guild my-guild --channel general
```

Compact also suits counting, grouping, and scanning a distribution: `discord export search "mfa" --all --limit 500 | cut -d: -f1 | sort | uniq -c`.

## Commands

```bash
discord auth status --json                  # exit 3: no or rejected token
discord config get                          # default-guild; set it with: config set default-guild <name>
discord guild list
discord guild show --guild my-guild         # counts and export status per channel
discord guild search budget --has link --limit 25
discord guild search --after 2026-09-01 --before 2026-10-01 --limit 1 --json  # total_results = messages that month, none fetched
discord guild export --threads              # incremental; --full refetches
discord channel list --threads
discord channel read general --limit 20
discord channel export general
discord dm list
discord dm read someone --limit 10
discord dm search someone --query meeting
discord dm export someone
discord message read 1542961568172740660 --context 5
discord export list
discord export search policy --all --author someone --after 2026-01-01
discord cache rebuild                       # when stderr says the search index is stale
```

`export search` needs `--guild` or `--all`; `--guild dm` searches DM exports. `dm search` refetches the whole DM each time, so export it once and use `export search --guild dm`. Names match case-insensitively with emoji and punctuation dropped; the numeric id always works.

`message read` and `export search` say on stderr whether the answer came from an export on disk (and how stale it may be) or from Discord; `source` in their JSON says the same.

## Exit codes

0 success. 1 unexpected (read stderr; retry a timeout once). 2 usage (fix the command; "no guild" means add `--guild`). 3 auth: stop and ask the user to run `discord auth set`; never handle the token yourself. 4 not found: stderr lists suggestions; for a message in no export, add `--channel`. 5 no exports: run `discord guild export` first. 6 rate limited: wait a minute, do not loop.

## Data locations

`~/.config/discord-cli/` holds the token and config. Exports live in `~/.local/share/discord-cli/exports/<guild>/`, plus older ones read in place from `~/.discord-cli/exports/` and DiscordChatExporter exports (read only). `~/.cache/discord-cli/` is the disposable search index.

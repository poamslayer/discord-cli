---
status: accepted
---
# An agent that picks no format gets lean output

Agents read this tool's output into their context, and the skill used to tell them to pass `--json` every time. Across 38 agent runs logged on 2026-10-17 to 2026-10-23, twenty `export search` hits cost about 18,000 bytes as JSON and 5,200 as compact. Minifying the JSON saves only 13%: the cost is full message text on every hit.

When an agent's environment variables show it is running the tool (`CLAUDECODE=1`, `CURSOR_AGENT=1`, `GEMINI_CLI=1`, `OPENCODE=1`, `CODEX_THREAD_ID`, `AI_AGENT`, the set cloudflare/cf keys on) and it passed neither `--json` nor `--format`, message commands default to compact and list commands to tsv, search commands default to 10 hits, and after a compact search one stderr line gives the match count and the `message read` command that opens a hit. `message read` prints the message asked for whole and its context at `--width`. `DISCORD_AGENT=0` or `1` forces the answer.

We considered teaching agents to pipe JSON through jq. Rejected: a jq filter that keeps the same fields is no smaller than compact, every agent has to write it again, and a wrong path prints nothing with exit 0, which reads as "no results".

## Consequences

- ADR-0002 is amended: the default output depends on the reader, not only on whether stdout is a terminal. Explicit `--json` and `--format` are unchanged, so scripts are not affected.
- A test pins twenty agent-mode hits under 6,000 bytes.

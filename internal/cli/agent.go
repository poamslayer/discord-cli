package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// agentSearchLimit is the --limit search commands use for an agent when it
// does not pass one: enough to pick from, few enough that the agent reads
// every hit instead of paying for 25 it skims.
const agentSearchLimit = 10

// isAgent reports whether an AI coding agent is running the tool, from the
// variables agents set in their child processes. DISCORD_AGENT=1 or 0
// forces the answer either way. The variables are the ones cloudflare/cf
// keys on (packages/cli/src/lib/agent-context.ts).
func isAgent(getenv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv("DISCORD_AGENT"))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	for _, k := range []string{"CLAUDECODE", "CURSOR_AGENT", "GEMINI_CLI", "OPENCODE"} {
		if getenv(k) == "1" {
			return true
		}
	}
	return getenv("CODEX_THREAD_ID") != "" || getenv("AI_AGENT") != ""
}

// searchLimit is the limit a search runs with: the flag when given, and
// agentSearchLimit for an agent that left it unset.
func (a *app) searchLimit(cmd *cobra.Command, limit int) int {
	if a.flags.Agent && !cmd.Flags().Changed("limit") {
		return agentSearchLimit
	}
	return limit
}

// agentHint tells an agent, on stderr, what it did not see and how to open
// a match, so stdout stays only the records.
func (a *app) agentHint(shown, total int, open string) {
	if !a.flags.Agent {
		return
	}
	if total == 0 {
		a.notice("0 matches.")
		return
	}
	more := ""
	if total > shown {
		more = "; raise --limit for more"
	}
	a.notice("%d of %d matches shown%s. Open one in full: %s", shown, total, more, open)
}

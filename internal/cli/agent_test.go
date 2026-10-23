package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/poamslayer/discord-cli/internal/clitest"
)

func TestAgentDetectedFromItsEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   map[string]string
		agent bool
	}{
		{"nobody", nil, false},
		{"claude code", map[string]string{"CLAUDECODE": "1"}, true},
		{"cursor", map[string]string{"CURSOR_AGENT": "1"}, true},
		{"gemini", map[string]string{"GEMINI_CLI": "1"}, true},
		{"opencode", map[string]string{"OPENCODE": "1"}, true},
		{"codex", map[string]string{"CODEX_THREAD_ID": "t-1"}, true},
		{"ai agent", map[string]string{"AI_AGENT": "pi"}, true},
		{"claude code turned off", map[string]string{"CLAUDECODE": "0"}, false},
		{"forced on", map[string]string{"DISCORD_AGENT": "1"}, true},
		{"forced off beats claude code", map[string]string{"CLAUDECODE": "1", "DISCORD_AGENT": "0"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := inventoryRunner(t)
			for k, v := range tc.env {
				r.Env[k] = v
			}
			res := r.Run("export", "search", "policy", "--all")
			if res.ExitCode != 0 {
				t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
			}
			human := strings.Contains(res.Stdout, "matches across")
			if human == tc.agent {
				t.Errorf("agent %v, but human output %v:\n%s", tc.agent, human, res.Stdout)
			}
		})
	}
}

func TestAgentExportSearchIsCompactAndShort(t *testing.T) {
	r := inventoryRunner(t)
	r.Env["CLAUDECODE"] = "1"
	res := r.Run("export", "search", "policy", "--all")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	lines := compactLines(res.Stdout)
	if len(lines) != 10 {
		t.Fatalf("want the agent limit of 10 records, got %d:\n%s", len(lines), res.Stdout)
	}
	for _, l := range lines {
		compactAddress(t, l)
	}
	if !strings.Contains(res.Stderr, "10 of 20 matches shown; raise --limit for more. Open one in full: discord message read <id>") {
		t.Errorf("hint missing from stderr:\n%s", res.Stderr)
	}
	if strings.Contains(res.Stdout, "Open one") {
		t.Errorf("the hint belongs on stderr, not stdout")
	}

	res = r.Run("export", "search", "policy", "--all", "--limit", "15")
	if n := len(compactLines(res.Stdout)); n != 15 {
		t.Errorf("an explicit --limit wins for an agent: got %d records", n)
	}

	res = r.Run("export", "search", "nothing-matches-this", "--all")
	if res.Stdout != "" || !strings.Contains(res.Stderr, "0 matches.") {
		t.Errorf("no matches: stdout %q stderr %q", res.Stdout, res.Stderr)
	}
}

func TestAgentExplicitFormatsAreUnchanged(t *testing.T) {
	r := inventoryRunner(t)
	r.Env["CLAUDECODE"] = "1"
	var j searchOut
	res := r.Run("export", "search", "policy", "--all", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	res.JSON(t, &j)
	if j.TotalMatches != 20 || j.Shown != 20 {
		t.Errorf("--json for an agent keeps the default limit of 25: total %d shown %d", j.TotalMatches, j.Shown)
	}
	if strings.Contains(res.Stderr, "Open one") {
		t.Errorf("no hint when the agent chose a format")
	}

	res = r.Run("export", "search", "policy", "--all", "--format=human")
	if !strings.Contains(res.Stdout, "20 matches across") {
		t.Errorf("--format=human for an agent is human:\n%s", res.Stdout)
	}
}

func TestAgentGuildSearchAsksDiscordForTen(t *testing.T) {
	r := searchRunner(t, 200)
	r.Env["CLAUDECODE"] = "1"
	res := r.Run("guild", "search", "access control")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	if n := len(compactLines(res.Stdout)); n != 10 {
		t.Errorf("want 10 records, got %d", n)
	}
	reqs := r.Fake.RequestsTo("/guilds/1001/messages/search")
	if len(reqs) != 1 || reqs[0].Query.Get("limit") != "10" {
		t.Errorf("want one search request with limit=10, got %+v", reqs)
	}
	if !strings.Contains(res.Stderr, "10 of 200 matches shown") || !strings.Contains(res.Stderr, "discord message read <id> --guild cooey-coe --channel <channel>") {
		t.Errorf("hint:\n%s", res.Stderr)
	}
}

func TestAgentListCommandsAreTSV(t *testing.T) {
	r := channelRunner(t)
	r.Env["CLAUDECODE"] = "1"
	res := r.Run("channel", "list")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	rows := tsvLines(res.Stdout)
	if len(rows) < 2 || !strings.Contains(rows[0], "\t") {
		t.Errorf("want tsv with a header, got:\n%s", res.Stdout)
	}
}

func TestAgentMessageReadKeepsTheMessageWhole(t *testing.T) {
	r := channelRunner(t)
	r.Env["CLAUDECODE"] = "1"
	msgs := clitest.Messages("2001", 5)
	long := strings.Repeat("word ", 200) // 1,000 characters
	for i := range msgs {
		msgs[i] = withContent(msgs[i], long)
	}
	writeJSON(t, filepath.Join(r.Home.ExportsDir(), "cooey-coe", "general.json"), clitest.NativeExport("1001", "Cooey COE", "2001", "general", 0, msgs))

	res := r.Run("message", "read", clitest.MessageID(3), "--context", "1")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	lines := compactLines(res.Stdout)
	if len(lines) != 3 {
		t.Fatalf("want 3 records, got %d:\n%s", len(lines), res.Stdout)
	}
	for _, l := range lines {
		whole := strings.Contains(l, strings.TrimSpace(long))
		asked := strings.Contains(l, ":"+clitest.MessageID(3)+":")
		if whole != asked {
			t.Errorf("only the message asked for is whole; asked %v whole %v: %.80s", asked, whole, l)
		}
	}
}

// TestAgentSearchByteBudget pins what twenty long hits cost an agent, so
// the default output cannot quietly grow back. JSON of the same hits runs
// past 40,000 bytes.
func TestAgentSearchByteBudget(t *testing.T) {
	r := channelRunner(t)
	r.Env["CLAUDECODE"] = "1"
	msgs := clitest.Messages("2001", 20)
	long := "policy " + strings.Repeat("enclave scoping detail ", 90) // about 2,000 characters
	for i := range msgs {
		msgs[i] = withContent(msgs[i], long)
	}
	writeJSON(t, filepath.Join(r.Home.ExportsDir(), "cooey-coe", "general.json"), clitest.NativeExport("1001", "Cooey COE", "2001", "general", 0, msgs))

	res := r.Run("export", "search", "policy", "--all", "--limit", "20")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	if n := len(compactLines(res.Stdout)); n != 20 {
		t.Fatalf("want 20 records, got %d", n)
	}
	if size := len(res.Stdout) + len(res.Stderr); size > 6000 {
		t.Errorf("20 agent hits cost %d bytes, budget is 6,000", size)
	}
	json := r.Run("export", "search", "policy", "--all", "--limit", "20", "--json")
	if len(json.Stdout) < 40000 {
		t.Errorf("the JSON comparison in the comment no longer holds: %d bytes", len(json.Stdout))
	}
}

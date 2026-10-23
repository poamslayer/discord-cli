package cli_test

import (
	"strings"
	"testing"

	"github.com/poamslayer/discord-cli/internal/clitest"
)

func searchRunner(t *testing.T, n int) *clitest.Runner {
	t.Helper()
	r := channelRunner(t)
	clitest.ServeSearch(r.Fake, "1001", n)
	return r
}

func TestGuildSearchPagesAutomatically(t *testing.T) {
	r := searchRunner(t, 200)
	var j struct {
		Guild        struct{ ID, Name string }
		TotalResults int `json:"total_results"`
		Messages     []struct {
			ID        string `json:"id"`
			ChannelID string `json:"channel_id"`
		} `json:"messages"`
		ChannelNames map[string]string `json:"channel_names"`
	}
	res := r.Run("guild", "search", "access control", "--limit", "60", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	res.JSON(t, &j)
	if j.TotalResults != 200 || len(j.Messages) != 60 || j.Guild.Name != "Cooey COE" {
		t.Errorf("total %d shown %d guild %+v", j.TotalResults, len(j.Messages), j.Guild)
	}
	if j.Messages[0].ID != clitest.MessageID(200) || j.Messages[59].ID != clitest.MessageID(141) {
		t.Errorf("order: first %s last %s", j.Messages[0].ID, j.Messages[59].ID)
	}
	if j.ChannelNames["2001"] != "🔮general" || j.ChannelNames["2002"] != "📰news" {
		t.Errorf("channel names %v", j.ChannelNames)
	}
	reqs := r.Fake.RequestsTo("/guilds/1001/messages/search")
	if len(reqs) != 3 {
		t.Fatalf("requests %d, want 3", len(reqs))
	}
	for i, want := range []string{"0", "25", "50"} {
		q := reqs[i].Query
		if q.Get("offset") != want || q.Get("content") != "access control" || q.Get("sort_by") != "timestamp" || q.Get("sort_order") != "desc" {
			t.Errorf("request %d: %v", i, q)
		}
		if l := q.Get("limit"); l != "25" && !(i == 2 && l == "10") {
			t.Errorf("request %d limit %s", i, l)
		}
	}
}

func TestGuildSearchDefaultLimitIsOnePage(t *testing.T) {
	r := searchRunner(t, 200)
	res := r.Run("guild", "search", "policy")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	if n := len(r.Fake.RequestsTo("/guilds/1001/messages/search")); n != 1 {
		t.Errorf("requests %d", n)
	}
	if countMessages(res.Stdout) != 25 {
		t.Errorf("shown %d", countMessages(res.Stdout))
	}
	if !strings.Contains(res.Stdout, "200 results") || !strings.Contains(res.Stdout, "showing 25") {
		t.Errorf("header missing: %q", res.Stdout[:200])
	}
	for _, want := range []string{"#🔮general", "#📰news", "#cmmc-general"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("missing channel name %q", want)
		}
	}
	// Newest first for search.
	if strings.Index(res.Stdout, "message 200 ") > strings.Index(res.Stdout, "message 199 ") {
		t.Errorf("search should be newest first")
	}
}

func TestGuildSearchFlagsForwarded(t *testing.T) {
	r := searchRunner(t, 5)
	res := r.Run("guild", "search", "--query", "mfa", "--channel", "general", "--has", "link", "--offset", "3", "--limit", "2")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	reqs := r.Fake.RequestsTo("/guilds/1001/messages/search")
	if len(reqs) != 1 {
		t.Fatalf("requests %d", len(reqs))
	}
	q := reqs[0].Query
	if q.Get("channel_id") != "2001" || q.Get("has") != "link" || q.Get("offset") != "3" || q.Get("limit") != "2" || q.Get("content") != "mfa" {
		t.Errorf("query %v", q)
	}
	if countMessages(res.Stdout) != 2 {
		t.Errorf("shown %d", countMessages(res.Stdout))
	}
}

func TestGuildSearchQueryPositionalOrFlag(t *testing.T) {
	r := searchRunner(t, 3)
	a := r.Run("guild", "search", "scoping", "--json")
	b := r.Run("guild", "search", "--query", "scoping", "--json")
	if a.ExitCode != 0 || b.ExitCode != 0 || a.Stdout != b.Stdout {
		t.Errorf("positional and --query differ: %q vs %q", a.Stdout, b.Stdout)
	}
	res := r.Run("guild", "search")
	if res.ExitCode != 2 || !strings.Contains(res.Stderr, "query") {
		t.Errorf("no query: exit %d %q", res.ExitCode, res.Stderr)
	}
	res = r.Run("guild", "search", "one", "--query", "two")
	if res.ExitCode != 2 {
		t.Errorf("conflicting queries: exit %d", res.ExitCode)
	}
}

func TestGuildSearchZeroResults(t *testing.T) {
	r := searchRunner(t, 0)
	res := r.Run("guild", "search", "nothing")
	if res.ExitCode != 0 || !strings.Contains(res.Stdout, "No results") {
		t.Errorf("exit %d out %q err %q", res.ExitCode, res.Stdout, res.Stderr)
	}
	var j struct {
		TotalResults int   `json:"total_results"`
		Messages     []any `json:"messages"`
	}
	r.Run("guild", "search", "nothing", "--json").JSON(t, &j)
	if j.TotalResults != 0 || j.Messages == nil || len(j.Messages) != 0 {
		t.Errorf("%+v", j)
	}
}

func TestGuildSearchNotIndexedYet(t *testing.T) {
	r := searchRunner(t, 3)
	r.Fake.Queue("/guilds/1001/messages/search", clitest.Response{Status: 202, Body: `{"message":"Index not yet available. Try again later","code":110000,"documents_indexed":0,"retry_after":30}`})
	res := r.Run("guild", "search", "anything")
	if res.ExitCode != 0 || !strings.Contains(res.Stdout, "No results") || !strings.Contains(res.Stderr, "index") {
		t.Errorf("exit %d out %q err %q", res.ExitCode, res.Stdout, res.Stderr)
	}
}

func TestGuildSearchUnknownChannelExits4(t *testing.T) {
	r := searchRunner(t, 3)
	res := r.Run("guild", "search", "x", "--channel", "nope")
	if res.ExitCode != 4 {
		t.Errorf("exit %d: %s", res.ExitCode, res.Stderr)
	}
}

func TestGuildSearchDateBounds(t *testing.T) {
	r := searchRunner(t, 5)
	res := r.Run("guild", "search", "--after", "2026-09-01T00:00:00Z", "--before", "2026-10-01T00:00:00Z", "--limit", "1", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	reqs := r.Fake.RequestsTo("/guilds/1001/messages/search")
	if len(reqs) != 1 {
		t.Fatalf("requests %d", len(reqs))
	}
	q := reqs[0].Query
	// (ms since 2015-01-01) << 22 for each bound.
	if q.Get("min_id") != "1544134695321600000" || q.Get("max_id") != "1555006331289600000" || q.Has("content") {
		t.Errorf("query %v", q)
	}
	var j struct {
		TotalResults int `json:"total_results"`
	}
	res.JSON(t, &j)
	if j.TotalResults != 5 {
		t.Errorf("total %d", j.TotalResults)
	}
}

func TestGuildSearchDateBoundsRejected(t *testing.T) {
	r := searchRunner(t, 5)
	for _, args := range [][]string{
		{"guild", "search", "--after", "2026-10-01", "--before", "2026-09-01"},
		{"guild", "search", "--after", "last tuesday"},
		{"guild", "search"},
	} {
		if res := r.Run(args...); res.ExitCode != 2 {
			t.Errorf("%v: exit %d, want 2", args, res.ExitCode)
		}
	}
}

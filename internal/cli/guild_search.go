package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/poamslayer/discord-cli/internal/discord"
	"github.com/poamslayer/discord-cli/internal/resolve"
	"github.com/poamslayer/discord-cli/internal/term"
)

// hasValues are what Discord's search accepts for --has.
const hasValues = "image, video, sound, file, embed, link, sticker, poll"

type searchJSON struct {
	Guild        namedJSON         `json:"guild"`
	TotalResults int               `json:"total_results"`
	Messages     []json.RawMessage `json:"messages"`
	ChannelNames map[string]string `json:"channel_names"`
}

// queryArg merges the positional query and --query, which are aliases.
// With optional set, another flag narrows the search and the query may be
// left out.
func queryArg(args []string, flag string, optional bool) (string, error) {
	pos := ""
	if len(args) > 0 {
		pos = args[0]
	}
	switch {
	case pos != "" && flag != "" && pos != flag:
		return "", UsageError("the query was given twice: %q and --query %q", pos, flag).WithHint("Give it once, as the argument or as --query.")
	case pos != "":
		return pos, nil
	case flag != "":
		return flag, nil
	case optional:
		return "", nil
	}
	return "", UsageError("missing the query").WithHint("Usage: discord guild search <query> [flags], or --query <text>. The query may be left out when --after, --before, --channel or --has narrows the search.")
}

func (a *app) guildSearchCommand() *cobra.Command {
	var (
		guild, query, channel, has string
		after, before              string
		limit, offset              int
	)
	cmd := &cobra.Command{
		Use:   "search [<query>]",
		Short: "Search a guild's messages through Discord's own search",
		Long: `Search a guild's messages through Discord's search endpoint, newest first.
The query may be given as the argument or as --query, and left out when
--after, --before, --channel or --has narrows the search. Limits above 25 are
satisfied by fetching further pages for you. total_results counts every
match, so --limit 1 --json counts messages without fetching them.`,
		Example: `  discord guild search "access control"
  discord guild search MFA --channel general --limit 50
  discord guild search --query "policy" --has link --json
  discord guild search --after 2026-09-01 --before 2026-10-01 --limit 1 --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := queryArg(args, query, after != "" || before != "" || channel != "" || has != "")
			if err != nil {
				return err
			}
			return a.guildSearch(cmd, guild, q, channel, has, after, before, limit, offset)
		},
	}
	addGuildFlag(cmd, &guild)
	cmd.Flags().StringVarP(&query, "query", "q", "", "text to search for (alias of the argument)")
	cmd.Flags().StringVar(&channel, "channel", "", "narrow to one channel, by name or id")
	cmd.Flags().StringVar(&has, "has", "", "only messages that have one of: "+hasValues)
	cmd.Flags().StringVar(&after, "after", "", "only messages after this date (YYYY-MM-DD or RFC 3339)")
	cmd.Flags().StringVar(&before, "before", "", "only messages before this date (YYYY-MM-DD or RFC 3339)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 25, "number of results to show")
	cmd.Flags().IntVar(&offset, "offset", 0, "skip this many results")
	return cmd
}

func (a *app) guildSearch(cmd *cobra.Command, guildFlag, query, channel, has, afterFlag, beforeFlag string, limit, offset int) error {
	ctx := cmd.Context()
	if limit <= 0 {
		return UsageError("--limit must be at least 1")
	}
	if offset < 0 {
		return UsageError("--offset cannot be negative")
	}
	after, err := parseDate("after", afterFlag)
	if err != nil {
		return err
	}
	before, err := parseDate("before", beforeFlag)
	if err != nil {
		return err
	}
	if !after.IsZero() && !before.IsZero() && !after.Before(before) {
		return UsageError("--after must be earlier than --before")
	}
	input, err := a.guildArg(guildFlag)
	if err != nil {
		return err
	}
	g, err := a.resolveGuild(ctx, input)
	if err != nil {
		return err
	}
	opts := discord.SearchOptions{Content: query, Has: has, Limit: a.searchLimit(cmd, limit), Offset: offset}
	if !after.IsZero() {
		opts.MinID = discord.SnowflakeAt(after)
	}
	if !before.IsZero() {
		opts.MaxID = discord.SnowflakeAt(before)
	}
	if channel != "" {
		ch, err := a.resolveChannel(ctx, g.ID, channel, false)
		if err != nil {
			return err
		}
		opts.ChannelID = ch.ID
	}
	c, _, err := a.client()
	if err != nil {
		return err
	}
	res, err := c.Search(ctx, g.ID, opts)
	if err != nil {
		return a.apiError(err)
	}
	if res.Indexing {
		a.notice("Discord has not finished building the search index for %s yet; try again in a minute.", g.Name)
	}
	names := a.channelNames(ctx, g.ID, res.Messages)

	if a.flags.JSON {
		return term.WriteJSON(a.stdout(), searchJSON{
			Guild:        namedJSON{ID: g.ID, Name: g.Name},
			TotalResults: res.TotalResults,
			Messages:     rawMessages(res.Messages),
			ChannelNames: names,
		})
	}
	if a.flags.Compact {
		gslug := resolve.Key(g.Name)
		rows := make([]compactRow, len(res.Messages))
		for i, m := range res.Messages {
			cslug := m.ChannelID
			if n, ok := names[m.ChannelID]; ok {
				cslug = resolve.Key(n)
			}
			rows[i] = compactRow{GuildSlug: gslug, ChannelSlug: cslug, ID: m.ID, Timestamp: m.Timestamp, Author: m.Author.DisplayName(), Content: m.Content}
		}
		a.writeCompact(rows)
		a.agentHint(len(res.Messages), res.TotalResults, fmt.Sprintf("discord message read <id> --guild %s --channel <channel> --context 5", gslug))
		return nil
	}
	if len(res.Messages) == 0 {
		fmt.Fprintf(a.stdout(), "No results for %q in %s.\n", query, g.Name)
		return nil
	}
	fmt.Fprintf(a.stdout(), "%s\n\n", a.out.Dim(fmt.Sprintf("%d results in %s (showing %d, newest first)", res.TotalResults, g.Name, len(res.Messages))))
	mw := a.messageWriter()
	mw.channels = names
	mw.writeAll(res.Messages)
	return nil
}

// channelNames maps the channel ids in messages to names, from the cached
// channel list; unknown ids are left out.
func (a *app) channelNames(ctx context.Context, guildID string, ms []discord.Message) map[string]string {
	names := map[string]string{}
	if len(ms) == 0 {
		return names
	}
	chs, err := a.channels(ctx, guildID)
	if err != nil {
		return names
	}
	byID := map[string]string{}
	for _, ch := range chs {
		byID[ch.ID] = ch.Name
	}
	for _, m := range ms {
		if n, ok := byID[m.ChannelID]; ok {
			names[m.ChannelID] = n
		}
	}
	return names
}

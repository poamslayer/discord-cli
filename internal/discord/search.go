package discord

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// discordEpoch is the first instant of 2015 in Unix milliseconds, the zero
// point of Discord snowflake ids.
const discordEpoch = 1420070400000

// SnowflakeAt returns the smallest snowflake id minted at t, for use as a
// before/after or min_id/max_id bound.
func SnowflakeAt(t time.Time) string {
	ms := t.UnixMilli() - discordEpoch
	if ms < 0 {
		ms = 0
	}
	return strconv.FormatUint(uint64(ms)<<22, 10)
}

// SearchPage is the most results one search request returns.
const SearchPage = 25

// SearchOptions narrow a guild search.
type SearchOptions struct {
	Content   string
	ChannelID string
	Has       string
	MinID     string // only messages after this snowflake
	MaxID     string // only messages before this snowflake
	Offset    int
	Limit     int // total wanted; pages are fetched until satisfied
}

// SearchResult is a flattened search response.
type SearchResult struct {
	TotalResults int
	Messages     []Message
	// Indexing is true when Discord answered 202: the guild is not indexed yet.
	Indexing bool
}

type searchResponse struct {
	TotalResults int         `json:"total_results"`
	Messages     [][]Message `json:"messages"`
	Message      string      `json:"message"` // set on a 202 "not indexed yet" answer
}

// Search runs Discord's guild message search sorted by timestamp descending,
// fetching further pages with offset until Limit results are collected or
// the results run out. The endpoint's messages field is an array of arrays
// and is flattened.
func (c *Client) Search(ctx context.Context, guildID string, o SearchOptions) (*SearchResult, error) {
	limit := o.Limit
	if limit <= 0 {
		limit = SearchPage
	}
	out := &SearchResult{}
	offset := o.Offset
	for len(out.Messages) < limit {
		want := limit - len(out.Messages)
		if want > SearchPage {
			want = SearchPage
		}
		q := url.Values{
			"sort_by":    {"timestamp"},
			"sort_order": {"desc"},
			"limit":      {strconv.Itoa(want)},
			"offset":     {strconv.Itoa(offset)},
		}
		if o.Content != "" {
			q.Set("content", o.Content)
		}
		if o.ChannelID != "" {
			q.Set("channel_id", o.ChannelID)
		}
		if o.Has != "" {
			q.Set("has", o.Has)
		}
		if o.MinID != "" {
			q.Set("min_id", o.MinID)
		}
		if o.MaxID != "" {
			q.Set("max_id", o.MaxID)
		}
		var page searchResponse
		if err := c.Get(ctx, "/guilds/"+guildID+"/messages/search", q, &page); err != nil {
			return nil, err
		}
		if page.Messages == nil && page.Message != "" {
			// 202: not indexed yet.
			out.Indexing = true
			break
		}
		out.TotalResults = page.TotalResults
		n := 0
		for _, group := range page.Messages {
			for _, m := range group {
				out.Messages = append(out.Messages, m)
				n++
			}
		}
		if n == 0 || n < want {
			break
		}
		offset += n
		if offset >= page.TotalResults {
			break
		}
	}
	return out, nil
}

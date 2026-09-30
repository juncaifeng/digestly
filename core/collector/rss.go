package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/mmcdole/gofeed"

	"github.com/juncaifeng/digestly/core/model"
)

// feedCollector 用 gofeed 统一解析 RSS / Atom / JSON Feed。
type feedCollector struct{ name string }

func (c *feedCollector) Name() string { return c.name }

func (c *feedCollector) Collect(ctx context.Context, src Source) ([]model.Item, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", src.URL, err)
	}
	items := make([]model.Item, 0, len(feed.Items))
	for _, fi := range feed.Items {
		it := model.Item{
			FeedID:      src.FeedID,
			GUID:        fi.GUID,
			Title:       strings.TrimSpace(fi.Title),
			Link:        fi.Link,
			Content:     fi.Content,
			PublishedAt: fi.PublishedParsed,
			Status:      model.StatusPending,
		}
		if it.GUID == "" {
			it.GUID = fi.Link
		}
		if it.Content == "" {
			it.Content = fi.Description
		}
		if fi.Author != nil {
			it.Author = fi.Author.Name
		} else if len(fi.Authors) > 0 {
			it.Author = fi.Authors[0].Name
		}
		items = append(items, it)
	}
	return items, nil
}

func init() {
	Register(&feedCollector{name: "rss"})
	Register(&feedCollector{name: "atom"})
	Register(&feedCollector{name: "jsonfeed"})
}

package collector

import (
	"context"
	"os"
	"testing"
)

func TestDeepSeekScript(t *testing.T) {
	code, err := os.ReadFile("../../../digestly-collectors/collectors/deepseek-news.js")
	if err != nil {
		t.Skip("collectors repo not found locally")
	}
	Register(&jsCollector{name: "js:deepseek-news", code: string(code)})
	c, _ := Get("js:deepseek-news")
	items, err := c.Collect(context.Background(), Source{FeedID: 1, URL: "x", Config: map[string]string{"limit": "3"}})
	t.Logf("err=%v n=%d", err, len(items))
	for _, it := range items {
		t.Logf("guid=%q title=%q link=%q published=%v contentlen=%d",
			it.GUID, it.Title, it.Link, it.PublishedAt, len(it.Content))
	}
}

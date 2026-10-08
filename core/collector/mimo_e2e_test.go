package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 端到端跑 digestly-collectors 仓库里的 mimo-news.js(需要真实网络)
func TestMiMoScript(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "digestly-collectors", "collectors", "mimo-news.js"))
	if err != nil {
		t.Skip("collectors repo not found:", err)
	}
	os.WriteFile(filepath.Join(dir, "mimo-news.js"), src, 0o644)
	old := JSDir
	JSDir = dir
	defer func() { JSDir = old }()
	LoadJSScripts()
	c, err := Get("js:mimo-news")
	if err != nil {
		t.Fatal(err)
	}
	items, err := c.Collect(context.Background(), Source{FeedID: 1, URL: "https://mimo.mi.com/docs/zh-CN/news/latest/v2-6"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("no items")
	}
	for i, it := range items {
		t.Logf("%s | %s", it.PublishedAt, it.Title)
		if it.GUID == "" || it.Link == "" {
			t.Errorf("item %d missing guid/link", i)
		}
		for _, bad := range []string{"&amp;", "&lt;", "&gt;", "&#"} {
			if strings.Contains(it.Title, bad) {
				t.Errorf("title contains undecoded entity %q: %s", bad, it.Title)
			}
		}
		// 源站无日期,脚本须合成递减时间保证排序
		if it.PublishedAt == nil {
			t.Errorf("item %d missing synthesized published", i)
		}
		if i > 0 && it.PublishedAt != nil && items[i-1].PublishedAt != nil &&
			it.PublishedAt.After(*items[i-1].PublishedAt) {
			t.Errorf("items not in descending order at %d", i)
		}
	}
}

package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 端到端跑 digestly-collectors 仓库里的新闻采集脚本(需要真实网络)。
// CI 门禁用 go test -short 跳过;本地全量跑可验证脚本。
func TestNewsScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e 依赖真实网络与本地 collectors 仓库,-short 跳过")
	}
	cases := []struct {
		file string
		name string
		url  string
	}{
		{"anthropic-news.js", "js:anthropic-news", "https://www.anthropic.com/news"},
		{"kimi-news.js", "js:kimi-news", "https://www.kimi.com/news/"},
	}
	repo := filepath.Join("..", "..", "..", "digestly-collectors", "collectors")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(repo, c.file))
			if err != nil {
				t.Skip("collectors repo not found:", err)
			}
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, c.file), src, 0o644)
			old := JSDir
			JSDir = dir
			defer func() { JSDir = old }()
			LoadJSScripts()
			col, err := Get(c.name)
			if err != nil {
				t.Fatal(err)
			}
			items, err := col.Collect(context.Background(), Source{FeedID: 1, URL: c.url})
			if err != nil {
				t.Fatal(err)
			}
			if len(items) == 0 {
				t.Fatal("no items parsed")
			}
			for i, it := range items {
				t.Logf("%v | %s | %s", it.PublishedAt, it.Title, it.Link)
				if it.GUID == "" || it.Link == "" || it.Title == "" {
					t.Errorf("item %d missing guid/link/title", i)
				}
				for _, bad := range []string{"&amp;", "&lt;", "&gt;", "&#"} {
					if strings.Contains(it.Title, bad) {
						t.Errorf("title has undecoded entity %q: %s", bad, it.Title)
					}
				}
				if it.PublishedAt == nil {
					t.Errorf("item %d missing published date", i)
				}
			}
		})
	}
}

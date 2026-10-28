package opml

import (
	"strings"
	"testing"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>我的订阅</title></head>
  <body>
    <outline text="未分组直接放">
      <outline text="直接订阅" type="rss" xmlUrl="https://a.example/feed" htmlUrl="https://a.example"/>
    </outline>
    <outline text="AI 公司动态" title="AI 公司动态">
      <outline text="DeepSeek" type="rss" xmlUrl="https://b.example/atom"/>
      <outline text="Kimi" type="rss" xmlUrl="https://c.example/rss"/>
    </outline>
  </body>
</opml>`

func TestParse(t *testing.T) {
	entries, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	// 顶层 outline 无 xmlUrl 是分组;其下叶子是订阅
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3: %+v", len(entries), entries)
	}
	byURL := map[string]Entry{}
	for _, e := range entries {
		byURL[e.XMLURL] = e
	}
	if e := byURL["https://a.example/feed"]; e.Group != "未分组直接放" || e.Title != "直接订阅" {
		t.Fatalf("entry a = %+v", e)
	}
	if e := byURL["https://b.example/atom"]; e.Group != "AI 公司动态" || e.Title != "DeepSeek" {
		t.Fatalf("entry b = %+v", e)
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse([]byte(`<opml version="2.0"><body><outline text="g"></outline></body></opml>`)); err == nil {
		t.Fatal("no xmlUrl outlines should error")
	}
	if _, err := Parse([]byte(`not xml`)); err == nil {
		t.Fatal("bad xml should error")
	}
}

func TestBuildRoundTrip(t *testing.T) {
	groups := map[string][]Entry{
		"AI 公司动态": {
			{Title: "DeepSeek", XMLURL: "https://b.example/atom"},
			{Title: "Kimi", XMLURL: "https://c.example/rss", HTMLURL: "https://c.example"},
		},
	}
	ungrouped := []Entry{{Title: "独立源", XMLURL: "https://a.example/feed"}}

	raw, err := Build("digestly", groups, ungrouped)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "<?xml") {
		t.Fatal("missing xml header")
	}

	back, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 3 {
		t.Fatalf("round-trip entries = %d, want 3: %+v", len(back), back)
	}
	byURL := map[string]Entry{}
	for _, e := range back {
		byURL[e.XMLURL] = e
	}
	if e := byURL["https://b.example/atom"]; e.Group != "AI 公司动态" || e.Title != "DeepSeek" {
		t.Fatalf("round-trip b = %+v", e)
	}
	if e := byURL["https://a.example/feed"]; e.Title != "独立源" {
		t.Fatalf("round-trip a = %+v", e)
	}
}

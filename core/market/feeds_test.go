package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseOPML(t *testing.T) {
	const opml = `<?xml version="1.0"?><opml version="2.0"><body>
	<outline text="科技">
		<outline text="少数派" xmlUrl="https://sspai.com/feed" htmlUrl="https://sspai.com"/>
	</outline>
	<outline text="外头" xmlUrl="https://example.com/rss.xml"/></body></opml>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(opml))
	}))
	defer srv.Close()

	feeds, err := ParseFeeds(context.Background(), srv.Client(), srv.URL+"/feeds.opml")
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 2 {
		t.Fatalf("want 2 feeds, got %d", len(feeds))
	}
	if feeds[0].FeedURL != "https://sspai.com/feed" || feeds[0].Title != "少数派" {
		t.Fatalf("bad first feed: %+v", feeds[0])
	}
	if !contains(feeds[0].Tags, "科技") {
		t.Fatalf("group should become tag: %v", feeds[0].Tags)
	}
}

func TestParseJSONCatalog(t *testing.T) {
	const js = `{"feeds":[{"title":"Hacker News","feed_url":"https://hnrss.org/frontpage","tags":["tech"]},
		{"title":"RSSHub 示例","url":"https://rsshub.app/bilibili/ranking/0/1/3"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(js))
	}))
	defer srv.Close()

	feeds, err := ParseFeeds(context.Background(), srv.Client(), srv.URL+"/feeds.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 2 {
		t.Fatalf("want 2, got %d", len(feeds))
	}
	// url 字段也可作 feed_url;RSSHub 网关地址原样返回
	if feeds[1].FeedURL != "https://rsshub.app/bilibili/ranking/0/1/3" {
		t.Fatalf("rsshub gateway url lost: %+v", feeds[1])
	}
}

func TestCollectorManifestIsNotFeedCatalog(t *testing.T) {
	const js = `{"version":1,"collectors":[{"name":"x","title":"X","url":"a.js","source_url":"https://x.com"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(js))
	}))
	defer srv.Close()
	_, err := ParseFeeds(context.Background(), srv.Client(), srv.URL+"/manifest.json")
	if err == nil || !strings.Contains(err.Error(), "采集器清单") {
		t.Fatalf("must reject collector manifest, got %v", err)
	}
}

func TestNoCatalogSaysNo(t *testing.T) {
	// 仓库里什么清单都没有 -> 明确报"没有",绝不臆造订阅源
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "README") {
			_, _ = w.Write([]byte("# hi"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	old := rawBase
	rawBase = srv.URL
	defer func() { rawBase = old }()

	_, err := ParseFeeds(context.Background(), srv.Client(), "https://github.com/nobody/empty")
	if err == nil || !strings.Contains(err.Error(), "没有声明订阅源清单") {
		t.Fatalf("want explicit 'no catalog' error, got %v", err)
	}
}

func TestGitHubBlobURLResolvesToRaw(t *testing.T) {
	got := catalogURLs("https://github.com/u/r/blob/main/feeds.opml")
	if len(got) != 1 || got[0] != rawBase+"/u/r/main/feeds.opml" {
		t.Fatalf("blob resolve wrong: %v", got)
	}
	// 仓库地址 -> 固定候选清单文件名
	repo := catalogURLs("https://github.com/u/r")
	if len(repo) != len(catalogFiles) || !strings.Contains(repo[0], "/u/r/HEAD/feeds.opml") {
		t.Fatalf("repo candidates wrong: %v", repo)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

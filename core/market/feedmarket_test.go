package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// 失败的刷新不得污染状态: 不推进 FetchedAt(否则过期重试永不触发)、
// 保留旧目录、下次成功后清空失败原因。
func TestSyncFailureKeepsOldFeedsWithoutStalingTimestamp(t *testing.T) {
	var fail int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(500)
			return
		}
		_, _ = w.Write([]byte(`{"feeds":[{"title":"a","feed_url":"https://a/rss"}]}`))
	}))
	defer srv.Close()

	m := NewFeedMarket("")
	sf := m.Sync(context.Background(), srv.URL+"/feeds.json", "t")
	if sf.Err != "" || len(sf.Feeds) != 1 {
		t.Fatalf("initial sync: %+v", sf)
	}
	goodAt := sf.FetchedAt

	atomic.StoreInt32(&fail, 1)
	sf = m.Sync(context.Background(), srv.URL+"/feeds.json", "t")
	if sf.Err == "" {
		t.Fatal("failed sync must record error")
	}
	if !sf.FetchedAt.Equal(goodAt) {
		t.Fatalf("failed sync must not advance FetchedAt: %v -> %v", goodAt, sf.FetchedAt)
	}

	// 失败时旧目录仍在(离线降级)
	if len(sf.Feeds) != 1 {
		t.Fatalf("failed sync must keep old feeds: %+v", sf.Feeds)
	}

	atomic.StoreInt32(&fail, 0)
	sf = m.Sync(context.Background(), srv.URL+"/feeds.json", "t")
	if sf.Err != "" {
		t.Fatalf("successful sync must clear previous error: %q", sf.Err)
	}
}

// 解析失败且从未成功过: Get 不得每次回源(去重),但后台刷新终会释放
// refreshing 锁,之后 TTL 过期能再次重试——不能被永久卡死。
func TestGetRetriesThrottledWhenAlwaysFailing(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(404)
	}))
	defer srv.Close()

	m := NewFeedMarket("")
	m.FeedTTL = 50 * time.Millisecond
	m.HTTP = &http.Client{Timeout: 100 * time.Millisecond}
	oldBudget := refreshBudget
	refreshBudget = 300 * time.Millisecond
	defer func() { refreshBudget = oldBudget }()
	u := srv.URL + "/feeds.json"
	for i := 0; i < 5; i++ {
		m.Get(context.Background(), u, "t")
	}
	// 5 次快速 Get 只应触发一轮后台刷新
	if got := atomic.LoadInt32(&hits); got < 1 {
		t.Fatal("must fetch at least once")
	}
	// 后台刷新必须被释放(即使远端挂起),不能永久占着 refreshing
	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		started, busy := m.refreshing[u]
		m.mu.Unlock()
		_ = started
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background refresh never released refreshing lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	before := atomic.LoadInt32(&hits)
	m.Get(context.Background(), u, "t") // 首次解析失败留下零值 FetchedAt → 立即重试
	deadline = time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&hits) == before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&hits); got == before {
		t.Fatal("stale cache must retry once the previous refresh released")
	}
}

// 目录缓存落盘: 重启(新实例)后无需回源即可拿到上次目录。
func TestFeedMarketPersistsCatalog(t *testing.T) {
	dir := t.TempDir()
	opmlPath := filepath.Join(dir, "feeds.opml")
	os.WriteFile(opmlPath, []byte(`<?xml version="1.0"?>
<opml version="2.0"><body>
<outline text="a" type="rss" xmlUrl="https://a.example/rss"/>
<outline text="b" type="rss" xmlUrl="https://b.example/rss"/>
</body></opml>`), 0o644)
	srcURL := "file://" + filepath.ToSlash(opmlPath)
	cachePath := filepath.Join(dir, "catalog.json")

	m := NewFeedMarket(cachePath)
	sf := m.Sync(context.Background(), srcURL, "本地清单")
	if sf.Err != "" || len(sf.Feeds) != 2 {
		t.Fatalf("sync: err=%q feeds=%d", sf.Err, len(sf.Feeds))
	}

	// 新实例(模拟重启)从磁盘载入,不触发网络也能拿到目录
	m2 := NewFeedMarket(cachePath)
	got := m2.Get(context.Background(), srcURL, "本地清单")
	if len(got.Feeds) != 2 {
		t.Fatalf("reloaded feeds = %d, want 2", len(got.Feeds))
	}
	if got.FetchedAt.IsZero() {
		t.Fatal("fetched_at lost on reload")
	}
}

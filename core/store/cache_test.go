package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/juncaifeng/digestly/core/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &Store{db: st.db, index: st.index, content: st.content}
}

func addTestFeed(t *testing.T, s *Store) int64 {
	t.Helper()
	f := model.Feed{Title: "t", URL: "http://x/" + t.Name(), Collector: "rss", Config: "{}", IntervalMin: 30}
	if err := s.AddFeed(&f); err != nil {
		t.Fatal(err)
	}
	return f.ID
}

func TestIndexWriteThrough(t *testing.T) {
	s := openTestStore(t)
	feedID := addTestFeed(t, s)

	now := time.Now()
	n, err := s.InsertItems([]model.Item{
		{FeedID: feedID, GUID: "a", Title: "旧文", Content: "c1", PublishedAt: ptrTime(now.Add(-time.Hour))},
		{FeedID: feedID, GUID: "b", Title: "新文", Content: "c2", PublishedAt: ptrTime(now)},
		{FeedID: feedID, GUID: "c", Title: "无日期", Content: "c3"}, // NULL 排最后
	})
	if err != nil || n != 3 {
		t.Fatalf("insert: n=%d err=%v", n, err)
	}

	items, err := s.ListItems(ItemFilter{FeedID: feedID})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Title != "新文" || items[1].Title != "旧文" || items[2].Title != "无日期" {
		t.Fatalf("order wrong: %v", []string{items[0].Title, items[1].Title, items[2].Title})
	}
	for _, it := range items {
		if it.Content != "" {
			t.Fatal("list must not carry content")
		}
	}

	// 标注写穿透
	id := items[0].ID
	if err := s.SetEnrichment(id, "sum", "k1,k2", "src", model.StatusProcessed); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListItems(ItemFilter{Status: model.StatusProcessed})
	if len(got) != 1 || got[0].Summary != "sum" || got[0].Keywords != "k1,k2" || got[0].Source != "src" {
		t.Fatalf("enrichment not in index: %+v", got)
	}

	// 已读写穿透
	if err := s.MarkRead(id, true); err != nil {
		t.Fatal(err)
	}
	unread, _ := s.ListItems(ItemFilter{Unread: true})
	if len(unread) != 2 {
		t.Fatalf("unread=%d want 2", len(unread))
	}

	// 标签写穿透 + 过滤
	if err := s.AddTag(id, "AI"); err != nil {
		t.Fatal(err)
	}
	tagged, _ := s.ListItems(ItemFilter{Tag: "AI"})
	if len(tagged) != 1 || tagged[0].Tags[0] != "AI" {
		t.Fatalf("tag filter: %+v", tagged)
	}
}

func TestGetItemLRU(t *testing.T) {
	s := openTestStore(t)
	feedID := addTestFeed(t, s)
	if _, err := s.InsertItems([]model.Item{
		{FeedID: feedID, GUID: "a", Title: "A", Content: "正文A"},
	}); err != nil {
		t.Fatal(err)
	}
	items, _ := s.ListItems(ItemFilter{})
	id := items[0].ID

	full, err := s.GetItem(id)
	if err != nil || full.Content != "正文A" {
		t.Fatalf("get: %v %+v", err, full)
	}
	if _, hit := s.content.Get(id); !hit {
		t.Fatal("content should be cached after first read")
	}
}

func TestContentCacheEviction(t *testing.T) {
	c := newContentCache(10) // 10 字节上限
	c.Add(1, "12345")
	c.Add(2, "67890") // 满
	if _, ok := c.Get(1); !ok {
		t.Fatal("1 should be cached")
	}
	c.Add(3, "abc") // 挤掉最冷的 2
	if _, ok := c.Get(2); ok {
		t.Fatal("2 should be evicted")
	}
	if _, ok := c.Get(1); !ok || func() bool { _, ok := c.Get(3); return !ok }() {
		t.Fatal("1 and 3 should survive")
	}
	// 超限单条不缓存
	c.Add(9, "01234567890123456789")
	if _, ok := c.Get(9); ok {
		t.Fatal("oversized entry must not be cached")
	}
}

func TestDeleteFeedPurgesCache(t *testing.T) {
	s := openTestStore(t)
	feedID := addTestFeed(t, s)
	s.InsertItems([]model.Item{{FeedID: feedID, GUID: "a", Title: "A", Content: "x"}})
	items, _ := s.ListItems(ItemFilter{})
	if _, err := s.GetItem(items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFeed(feedID); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListItems(ItemFilter{})
	if len(items) != 0 {
		t.Fatal("index not purged")
	}
	if _, entries, _ := s.CacheStats(); entries != 0 {
		t.Fatal("content cache not purged")
	}
}

func TestWarmIndex(t *testing.T) {
	dir := t.TempDir()
	st, _ := Open(dir + "/a.db")
	feedID := addTestFeed(t, st)
	st.InsertItems([]model.Item{{FeedID: feedID, GUID: "a", Title: "持久化", Content: "c"}})
	items, _ := st.ListItems(ItemFilter{})
	st.AddTag(items[0].ID, "重启存活")
	st.Close()

	st2, err := Open(dir + "/a.db") // 重新打开 → 预热索引
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	got, _ := st2.ListItems(ItemFilter{Tag: "重启存活"})
	if len(got) != 1 || got[0].Title != "持久化" {
		t.Fatalf("warm index broken: %+v", got)
	}
}

func TestFTSStillWorks(t *testing.T) {
	s := openTestStore(t)
	feedID := addTestFeed(t, s)
	s.InsertItems([]model.Item{
		{FeedID: feedID, GUID: "a", Title: "强化学习规模扩展", Content: "正文"},
		{FeedID: feedID, GUID: "b", Title: "无关文章", Content: "别的"},
	})
	got, err := s.ListItems(ItemFilter{Query: "强化学习"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "强化学习规模扩展" {
		t.Fatalf("fts: %+v", got)
	}
	if got[0].Content != "" {
		t.Fatal("search result must not carry content")
	}
}

func BenchmarkListItems(b *testing.B) {
	s := openTestStore(&testing.T{})
	feedID := addTestFeed(&testing.T{}, s)
	now := time.Now()
	var batch []model.Item
	for i := 0; i < 5000; i++ {
		tm := now.Add(-time.Duration(i) * time.Minute)
		batch = append(batch, model.Item{FeedID: feedID, GUID: fmt.Sprint(i),
			Title: fmt.Sprintf("文章%d", i), Content: "x", PublishedAt: &tm})
	}
	if _, err := s.InsertItems(batch); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.ListItems(ItemFilter{Limit: 50}); err != nil {
			b.Fatal(err)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

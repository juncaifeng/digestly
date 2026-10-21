package store

import (
	"testing"

	"github.com/juncaifeng/digestly/core/model"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func addFeed(t *testing.T, st *Store, title string) int64 {
	t.Helper()
	f := &model.Feed{Title: title, URL: "http://x/" + title, Collector: "rss", Config: "{}", IntervalMin: 30}
	if err := st.AddFeed(f); err != nil {
		t.Fatal(err)
	}
	return f.ID
}

func addItem(t *testing.T, st *Store, feedID int64, guid string) int64 {
	t.Helper()
	if _, err := st.InsertItems([]model.Item{{FeedID: feedID, GUID: guid, Title: "t-" + guid}}); err != nil {
		t.Fatal(err)
	}
	items, _ := st.ListItems(ItemFilter{FeedID: feedID})
	for _, it := range items {
		if it.GUID == guid {
			return it.ID
		}
	}
	t.Fatal("item not found")
	return 0
}

func TestColumnCRUD(t *testing.T) {
	st := openTest(t)
	id, err := st.AddColumn("AI 前线")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameColumn(id, "AI"); err != nil {
		t.Fatal(err)
	}
	cols, _ := st.ListColumns()
	if len(cols) != 1 || cols[0].Name != "AI" {
		t.Fatalf("cols = %+v", cols)
	}
}

func TestColumnFilterInheritAndFavorite(t *testing.T) {
	st := openTest(t)
	feedA := addFeed(t, st, "a")
	feedB := addFeed(t, st, "b")
	itemA := addItem(t, st, feedA, "ga")
	itemB := addItem(t, st, feedB, "gb")

	colID, _ := st.AddColumn("c1")
	// feedA 移入专栏 -> itemA 继承命中;itemB 不命中
	if err := st.SetFeedColumn(feedA, &colID); err != nil {
		t.Fatal(err)
	}
	items, _ := st.ListItems(ItemFilter{ColumnID: colID})
	if len(items) != 1 || items[0].ID != itemA {
		t.Fatalf("inherit filter = %+v", items)
	}

	// 手动收藏 itemB -> 也命中
	if err := st.FavoriteItem(colID, itemB); err != nil {
		t.Fatal(err)
	}
	items, _ = st.ListItems(ItemFilter{ColumnID: colID})
	if len(items) != 2 {
		t.Fatalf("inherit+fav filter = %d items", len(items))
	}

	// GetItem 携带收藏专栏
	full, err := st.GetItem(itemB)
	if err != nil || len(full.ColumnIDs) != 1 || full.ColumnIDs[0] != colID {
		t.Fatalf("GetItem ColumnIDs = %+v err=%v", full.ColumnIDs, err)
	}

	// 取消收藏 -> 只剩继承
	if err := st.UnfavoriteItem(colID, itemB); err != nil {
		t.Fatal(err)
	}
	items, _ = st.ListItems(ItemFilter{ColumnID: colID})
	if len(items) != 1 || items[0].ID != itemA {
		t.Fatalf("after unfav = %+v", items)
	}

	// 全文搜索路径同样受专栏约束
	items, _ = st.ListItems(ItemFilter{ColumnID: colID, Query: "t-ga"})
	if len(items) != 1 || items[0].ID != itemA {
		t.Fatalf("fts column filter = %+v", items)
	}
	items, _ = st.ListItems(ItemFilter{ColumnID: colID, Query: "t-gb"})
	if len(items) != 0 {
		t.Fatalf("fts column leak = %+v", items)
	}
}

func TestDeleteColumnRestoresAndCascades(t *testing.T) {
	st := openTest(t)
	feedID := addFeed(t, st, "a")
	itemID := addItem(t, st, feedID, "g")
	colID, _ := st.AddColumn("c")
	_ = st.SetFeedColumn(feedID, &colID)
	_ = st.FavoriteItem(colID, itemID)

	if err := st.DeleteColumn(colID); err != nil {
		t.Fatal(err)
	}
	// 订阅回未分组
	feeds, _ := st.ListFeeds()
	if feeds[0].ColumnID != nil {
		t.Fatalf("feed should be ungrouped: %+v", feeds[0].ColumnID)
	}
	// 收藏清空,专栏过滤归零
	items, _ := st.ListItems(ItemFilter{ColumnID: colID})
	if len(items) != 0 {
		t.Fatalf("deleted column must match nothing: %d", len(items))
	}
}

func TestMoveFeedBetweenColumns(t *testing.T) {
	st := openTest(t)
	feedID := addFeed(t, st, "a")
	itemID := addItem(t, st, feedID, "g")
	c1, _ := st.AddColumn("c1")
	c2, _ := st.AddColumn("c2")
	_ = st.SetFeedColumn(feedID, &c1)
	_ = st.SetFeedColumn(feedID, &c2)
	items, _ := st.ListItems(ItemFilter{ColumnID: c1})
	if len(items) != 0 {
		t.Fatal("moved feed must leave c1")
	}
	items, _ = st.ListItems(ItemFilter{ColumnID: c2})
	if len(items) != 1 || items[0].ID != itemID {
		t.Fatal("moved feed must land in c2")
	}
	// 移回未分组
	_ = st.SetFeedColumn(feedID, nil)
	items, _ = st.ListItems(ItemFilter{ColumnID: c2})
	if len(items) != 0 {
		t.Fatal("ungrouped feed must leave c2")
	}
}

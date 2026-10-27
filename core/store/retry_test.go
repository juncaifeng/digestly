package store

import (
	"database/sql"
	"testing"

	"github.com/juncaifeng/digestly/core/model"
)

// seedFailed 插入一篇并置为 failed,返回其 id。
func seedFailed(t *testing.T, s *Store, guid string) int64 {
	t.Helper()
	feedID := addTestFeed(t, s)
	if _, err := s.InsertItems([]model.Item{{FeedID: feedID, GUID: guid, Title: "t", Content: "c"}}); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListItems(ItemFilter{FeedID: feedID, Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("ListItems: %v, len=%d", err, len(items))
	}
	id := items[0].ID
	if err := s.SetEnrichment(id, "摘要", "kw", "src", model.StatusFailed); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRetryFailed(t *testing.T) {
	s := openTestStore(t)
	id := seedFailed(t, s, "a")

	n, err := s.RetryFailed()
	if err != nil || n != 1 {
		t.Fatalf("RetryFailed = %d, %v; want 1, nil", n, err)
	}
	it, err := s.GetItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if it.Status != model.StatusPending || it.Summary != "" || it.Source != "" {
		t.Fatalf("重试后应回到 pending 且清空标注,得到 status=%s summary=%q source=%q", it.Status, it.Summary, it.Source)
	}
	// 没有 failed 时是幂等空操作
	if n, _ = s.RetryFailed(); n != 0 {
		t.Fatalf("无 failed 时应返回 0,得到 %d", n)
	}
}

func TestRetryFailedSkipsArchived(t *testing.T) {
	s := openTestStore(t)
	id := seedFailed(t, s, "a")
	if err := s.SetArchived(id, true); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.RetryFailed(); n != 0 {
		t.Fatalf("已归档的 failed 不应被重试,返回 %d", n)
	}
	if it, _ := s.GetItem(id); it.Status != model.StatusFailed {
		t.Fatalf("归档文章状态不应变化,得到 %s", it.Status)
	}
}

func TestRetryItem(t *testing.T) {
	s := openTestStore(t)
	id := seedFailed(t, s, "a")

	if err := s.RetryItem(id); err != nil {
		t.Fatal(err)
	}
	if it, _ := s.GetItem(id); it.Status != model.StatusPending {
		t.Fatalf("RetryItem 后应为 pending,得到 %s", it.Status)
	}
	if err := s.RetryItem(99999); err != sql.ErrNoRows {
		t.Fatalf("不存在的文章应返回 sql.ErrNoRows,得到 %v", err)
	}
}

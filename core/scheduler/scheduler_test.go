package scheduler

import (
	"testing"
	"time"

	"github.com/juncaifeng/digestly/core/model"
)

func day(offset int) *time.Time {
	t := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
	return &t
}

func TestFilterIngestAgeAndCount(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	items := []model.Item{
		{ID: 1, PublishedAt: day(-1)},  // 新
		{ID: 2, PublishedAt: day(-40)}, // 超龄
		{ID: 3, PublishedAt: nil},      // 无日期,保留
		{ID: 4, PublishedAt: day(-10)},
		{ID: 5, PublishedAt: day(-39)}, // 超龄
	}

	// 30 天上限: -40 / -39 被丢,剩 3 条;条数 50 不触发
	got := filterIngest(items, ingestLimits{maxItems: 50, maxAgeDays: 30}, now)
	if len(got) != 3 {
		t.Fatalf("age filter: got %d items, want 3: %+v", len(got), got)
	}
	for _, it := range got {
		if it.ID == 2 || it.ID == 5 {
			t.Fatalf("超龄条目未被过滤: %+v", it)
		}
	}

	// 条数上限 2: 按时间降序取 2 条(-1 与 -10),无日期排最后被截掉
	got = filterIngest(items, ingestLimits{maxItems: 2, maxAgeDays: 30}, now)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 4 {
		t.Fatalf("count filter: got %+v, want [1 4]", got)
	}

	// 0 = 不限: 全保留
	got = filterIngest(items, ingestLimits{maxItems: 0, maxAgeDays: 0}, now)
	if len(got) != 5 {
		t.Fatalf("unlimited: got %d, want 5", len(got))
	}

	// 无日期条目不因天数被丢(30 天过滤后仍保留)
	got = filterIngest(items, ingestLimits{maxItems: 3, maxAgeDays: 30}, now)
	if len(got) != 3 {
		t.Fatalf("nil date retention: got %+v, want 3 items", got)
	}
	seen := map[int64]bool{}
	for _, it := range got {
		seen[it.ID] = true
	}
	if !seen[1] || !seen[4] || !seen[3] {
		t.Fatalf("nil date retention: got ids %v, want {1 3 4}", seen)
	}
}

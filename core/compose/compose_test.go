package compose

import (
	"encoding/json"
	"testing"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// plan 必须是「前端可直接消费」的形状: 所有数组字段永不为 JSON null。
// 前端按 plan.items.map / plan.history_titles.length 渲染,null 会抛 TypeError 导致整页白屏。
func TestPlanJSONArraysNeverNull(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	f := &model.Feed{Title: "f", URL: "http://x/f", Collector: "rss", Config: "{}", IntervalMin: 30}
	if err := st.AddFeed(f); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertItems([]model.Item{
		{FeedID: f.ID, GUID: "g1", Title: "t1"},
		{FeedID: f.ID, GUID: "g2", Title: "t2"},
	}); err != nil {
		t.Fatal(err)
	}
	tpl := &model.ArticleTemplate{Name: "tpl", Prompt: "P {{items}}"}
	if err := st.AddTemplate(tpl); err != nil {
		t.Fatal(err)
	}
	cr := &model.Creation{
		Name:       "c",
		TemplateID: tpl.ID,
		Sources:    []model.CreationSource{{Kind: "feed", RefID: f.ID}},
	}
	if err := st.AddCreation(cr); err != nil {
		t.Fatal(err)
	}

	// 无历史文章: history_titles 为 nil 切片,必须仍序列化为 []
	p, err := BuildPlan(st, cr.ID, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"item_ids", "items", "history_titles", "feed_ids"} {
		v, ok := m[key]
		if !ok {
			t.Fatalf("plan JSON 缺少字段 %q: %s", key, b)
		}
		if v == nil {
			t.Fatalf("plan.%s 序列化为 null(前端渲染会崩): %s", key, b)
		}
		if _, isArr := v.([]any); !isArr {
			t.Fatalf("plan.%s 不是数组: %s", key, b)
		}
	}
	if n := len(m["history_titles"].([]any)); n != 0 {
		t.Fatalf("history_titles len = %d, want 0", n)
	}
	if n := len(m["items"].([]any)); n != 2 {
		t.Fatalf("items len = %d, want 2", n)
	}
}

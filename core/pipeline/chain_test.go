package pipeline_test

import (
	"errors"
	"testing"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/store"
)

// boom 一个必定失败的标注步骤。
type boom struct{}

func (boom) Name() string { return "boom" }
func (boom) Enrich(*model.Item) error { return errors.New("boom") }

func setup(t *testing.T) (*store.Store, *pipeline.Chain, int64) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := model.Feed{Title: "t", URL: "http://x/", Collector: "rss", Config: "{}", IntervalMin: 30}
	if err := st.AddFeed(&f); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertItems([]model.Item{{FeedID: f.ID, GUID: "a", Title: "标题", Content: "<p>正文内容</p>", Link: "http://x/a"}}); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListItems(store.ItemFilter{FeedID: f.ID, Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("ListItems: %v, len=%d", err, len(items))
	}
	chain := pipeline.NewChain(st,
		pipeline.Source{Rules: map[string]string{}},
		pipeline.Summary{MaxRunes: 200},
		pipeline.Keywords{TopN: 5},
	)
	return st, chain, items[0].ID
}

func TestRunOne(t *testing.T) {
	st, chain, id := setup(t)

	status, err := chain.RunOne(id)
	if err != nil {
		t.Fatal(err)
	}
	if status != model.StatusProcessed {
		t.Fatalf("RunOne 状态 = %s,想 processed", status)
	}
	it, err := st.GetItem(id)
	if err != nil {
		t.Fatal(err)
	}
	if it.Summary == "" || it.Status != model.StatusProcessed {
		t.Fatalf("RunOne 后应有摘要且 processed,得到 summary=%q status=%s", it.Summary, it.Status)
	}
}

func TestRunOneFailedStep(t *testing.T) {
	st, chain, id := setup(t)
	if _, err := chain.RunOne(id); err != nil {
		t.Fatal(err)
	}
	failing := pipeline.NewChain(st, boom{})
	status, err := failing.RunOne(id)
	if err != nil {
		t.Fatal(err)
	}
	if status != model.StatusFailed {
		t.Fatalf("失败步骤应得到 failed,得到 %s", status)
	}
	if it, _ := st.GetItem(id); it.Status != model.StatusFailed {
		t.Fatalf("落库状态应为 failed,得到 %s", it.Status)
	}
}

// skipper 返回跳过(不算失败),验证 skip 记入日志且不中断。
type skipper struct{}

func (skipper) Name() string                 { return "skipper" }
func (skipper) Enrich(*model.Item) error     { return pipeline.Skip("无配置,跳过") }

func TestPipelineLogsRecorded(t *testing.T) {
	st, _, itemID := setup(t)
	chain := pipeline.NewChain(st, skipper{}, boom{}, pipeline.Summary{MaxRunes: 50})
	// RunOne 只在取不到文章时报错;boom 失败由状态体现
	_, _ = chain.RunOne(itemID)
	logs, err := st.PipelineLogs(itemID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs = %+v, want 2(skipper + boom)", logs)
	}
	// 新→旧: [boom failed, skipper skipped]
	if logs[0].Step != "boom" || logs[0].Status != "failed" {
		t.Fatalf("logs[0] = %+v", logs[0])
	}
	if logs[1].Step != "skipper" || logs[1].Status != "skipped" || logs[1].Message != "无配置,跳过" {
		t.Fatalf("logs[1] = %+v", logs[1])
	}
}

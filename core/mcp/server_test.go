package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// 端到端: 内存传输连上 MCP server, 验证工具注册与查询/创作链路。
func TestToolsRoundTrip(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// 造数据: 订阅源 + 2 篇未读 + 模板 + 创作项目
	f := &model.Feed{Title: "f", URL: "http://x/f", Collector: "rss", Config: "{}", IntervalMin: 30}
	if err := st.AddFeed(f); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertItems([]model.Item{
		{FeedID: f.ID, GUID: "g1", Title: "t1", PublishedAt: &time.Time{}},
		{FeedID: f.ID, GUID: "g2", Title: "t2"},
	}); err != nil {
		t.Fatal(err)
	}
	tpl := &model.ArticleTemplate{Name: "tpl", Prompt: "P {{items}}"}
	if err := st.AddTemplate(tpl); err != nil {
		t.Fatal(err)
	}
	cr := &model.Creation{
		Name: "c", TemplateID: tpl.ID,
		Sources: []model.CreationSource{{Kind: "feed", RefID: f.ID}},
	}
	if err := st.AddCreation(cr); err != nil {
		t.Fatal(err)
	}

	// 内存传输: server 一端跑起来, client 一端连接
	server := NewServer(st)
	ct, stp := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	go func() { _ = server.Run(ctx, stp) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v0"}, nil)
	sess, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	// 工具全集
	lt, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range lt.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{
		"list_items", "get_item", "mark_item_read", "archive_item",
		"list_feeds", "list_columns", "list_templates",
		"list_creations", "create_creation", "update_creation", "delete_creation",
		"plan_creation", "generate_creation", "list_articles", "get_article",
	} {
		if !names[want] {
			t.Errorf("缺少工具 %q", want)
		}
	}

	call := func(name string, args any) map[string]any {
		t.Helper()
		res, err := sess.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s: tool error: %v", name, res.Content)
		}
		if res.StructuredContent == nil {
			t.Fatalf("%s: 无 structuredContent", name)
		}
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("%s: structuredContent 序列化失败: %v", name, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: structuredContent 非对象: %v", name, err)
		}
		return m
	}

	// 查询链路
	m := call("list_feeds", nil)
	if feeds, ok := m["feeds"].([]any); !ok || len(feeds) != 1 {
		t.Fatalf("list_feeds = %v", m)
	}
	m = call("list_creations", nil)
	if cs, ok := m["creations"].([]any); !ok || len(cs) != 1 {
		t.Fatalf("list_creations = %v", m)
	}
	m = call("list_items", map[string]any{"unread_only": true})
	items, ok := m["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("list_items = %v", m)
	}
	m = call("get_item", map[string]any{"id": int64(1)})
	if m["title"] != "t1" {
		t.Fatalf("get_item = %v", m)
	}

	// plan: 素材清单 + prompt, 无副作用
	m = call("plan_creation", map[string]any{"id": cr.ID})
	if _, ok := m["prompt"].(string); !ok {
		t.Fatalf("plan 缺 prompt: %v", m)
	}

	// 状态翻转
	call("mark_item_read", map[string]any{"id": int64(1), "read": true})
	m = call("list_items", map[string]any{"unread_only": true})
	if items, _ := m["items"].([]any); len(items) != 1 {
		t.Fatalf("标记已读后未读应剩 1: %v", m)
	}
	call("archive_item", map[string]any{"id": int64(1), "archived": true})
	m = call("list_items", map[string]any{"limit": 50})
	if items, _ := m["items"].([]any); len(items) != 1 {
		t.Fatalf("归档后默认列表应剩 1: %v", m)
	}

	// 创作 CRUD
	m = call("create_creation", map[string]any{"name": "c2", "group_name": "g"})
	id, _ := m["id"].(float64)
	if id != 2 {
		t.Fatalf("create_creation = %v", m)
	}
	call("update_creation", map[string]any{"id": int64(2), "name": "c2b"})
	m = call("list_creations", nil)
	if cs, _ := m["creations"].([]any); len(cs) != 2 {
		t.Fatalf("create 后应有 2 个创作: %v", m)
	}
	call("delete_creation", map[string]any{"id": int64(2)})
	m = call("list_creations", nil)
	if cs, _ := m["creations"].([]any); len(cs) != 1 {
		t.Fatalf("delete 后应剩 1 个创作: %v", m)
	}
}

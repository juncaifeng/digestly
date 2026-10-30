// Package mcp 把 digestly 的工具面暴露为 MCP(Model Context Protocol)服务。
// 与 web(HTTP API)、CLI 并列的第三种入口: 外部 agent(MCP 客户端)经 streamable HTTP
// 读取订阅源与文章(唯一 id + 原文),后续加工交给下游专门工具。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/juncaifeng/digestly/core/store"
)

// NewServer 构建注册好全部工具的 MCP server。
func NewServer(st *store.Store) *mcpsdk.Server {
	s := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "digestly", Version: "0.1.0"}, nil)
	registerTools(s, st)
	return s
}

// Handler 返回挂载用的 streamable HTTP handler(建议挂在 /mcp)。
// Stateless + JSON 响应: 无会话状态,便于本地工具场景与调试。
func Handler(st *store.Store) http.Handler {
	srv := NewServer(st)
	return mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return srv
	}, &mcpsdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

// toolOK 把任意结构转成 map 输出(nil slice 兜底为空数组,防 null)。
func toolOK(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func registerTools(s *mcpsdk.Server, st *store.Store) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_feeds", Description: "列出全部订阅源(id、名称、URL、所属专栏、未读数)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		fs, err := st.ListFeeds()
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(map[string]any{"feeds": fs}), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_columns", Description: "列出全部专栏(id、名称、订阅数、收藏数)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		cs, err := st.ListColumns()
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(map[string]any{"columns": cs}), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_items", Description: "按条件检索已采集文章(标题/链接/来源/时间,不含正文);支持关键词全文搜索、订阅源/专栏/标签/时间/未读筛选与分页"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in itemQuery) (*mcpsdk.CallToolResult, any, error) {
		since, err := parseTime(in.Since, true)
		if err != nil {
			return nil, nil, fmt.Errorf("since: %w", err)
		}
		until, err := parseTime(in.Until, false)
		if err != nil {
			return nil, nil, fmt.Errorf("until: %w", err)
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		items, err := st.ListItems(store.ItemFilter{
			FeedID: in.FeedID, ColumnID: in.ColumnID, Tag: in.Tag,
			Query: in.Query, Unread: in.UnreadOnly, Archived: in.Archived,
			Since: since, Until: until, Limit: limit, Offset: in.Offset,
		})
		if err != nil {
			return nil, nil, err
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			it.Content = "" // 列表不带正文,详情走 get_item
			out = append(out, toolOK(it))
		}
		return nil, map[string]any{"items": out, "limit": limit, "offset": in.Offset}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "get_item", Description: "按 id 读单篇文章全文(含 content 正文)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in idArg) (*mcpsdk.CallToolResult, any, error) {
		it, err := st.GetItem(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(it), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "mark_item_read", Description: "标记文章已读/未读"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in readArg) (*mcpsdk.CallToolResult, any, error) {
		if err := st.MarkRead(in.ID, in.Read); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"ok": true}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "archive_item", Description: "归档/恢复文章(归档后从列表与未读数隐藏,不删除)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in archiveArg) (*mcpsdk.CallToolResult, any, error) {
		if err := st.SetArchived(in.ID, in.Archived); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"ok": true}, nil
	})
}

// ---- 输入结构 ----

type idArg struct {
	ID int64 `json:"id" jsonschema:"id(用对应 list 接口查)"`
}

type readArg struct {
	ID   int64 `json:"id" jsonschema:"文章 id"`
	Read bool  `json:"read" jsonschema:"是否已读: true=已读 false=未读"`
}

type archiveArg struct {
	ID       int64 `json:"id" jsonschema:"文章 id"`
	Archived bool  `json:"archived" jsonschema:"是否归档: true=归档 false=恢复"`
}

type itemQuery struct {
	Query      string `json:"query,omitempty" jsonschema:"全文搜索关键词(标题/正文)"`
	FeedID     int64  `json:"feed_id,omitempty" jsonschema:"按订阅源 id 过滤"`
	ColumnID   int64  `json:"column_id,omitempty" jsonschema:"按专栏 id 过滤(含订阅继承与手动收藏)"`
	Tag        string `json:"tag,omitempty" jsonschema:"按标签过滤"`
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"只看未读"`
	Archived   bool   `json:"archived,omitempty" jsonschema:"是否看归档箱: true=只看归档,默认只看未归档"`
	Since      string `json:"since,omitempty" jsonschema:"起始时间: 2026-10-28 或 RFC3339"`
	Until      string `json:"until,omitempty" jsonschema:"截止时间(不含),格式同 since"`
	Limit      int    `json:"limit,omitempty" jsonschema:"返回条数上限,默认 20"`
	Offset     int    `json:"offset,omitempty" jsonschema:"分页偏移"`
}

// parseTime 解析时间参数。start 决定日期粒度取当天起点还是次日起点。
func parseTime(s string, start bool) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	now := time.Now()
	var t time.Time
	switch {
	case s == "today":
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if !start {
			t = t.Add(24 * time.Hour)
		}
	default:
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			t = ts
		} else if ts, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
			t = ts
			if !start {
				t = t.Add(24 * time.Hour)
			}
		} else if ts, err := time.ParseInLocation("2006-01-02 15:04", s, now.Location()); err == nil {
			t = ts
		} else {
			return nil, fmt.Errorf("无法识别的时间 %q", s)
		}
	}
	return &t, nil
}

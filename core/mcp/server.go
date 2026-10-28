// Package mcp 把 digestly 的工具面暴露为 MCP(Model Context Protocol)服务。
// 与 web(HTTP API)、CLI 并列的第三种入口: 外部 agent(MCP 客户端)经 streamable HTTP
// 直接调用 digestly 的查询/阅读/创作工具,工具面与 digestly-cli 保持一致。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/juncaifeng/digestly/core/compose"
	"github.com/juncaifeng/digestly/core/model"
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
	// ---------- 发现 ----------
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

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_templates", Description: "列出大纲模板(id、名称、prompt),创作生成时选用"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		ts, err := st.ListTemplates()
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(map[string]any{"templates": ts}), nil
	})

	// ---------- 资讯检索与阅读 ----------
	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_items", Description: "按条件检索已采集资讯(标题/链接/摘要/时间,不含正文);支持关键词全文搜索、订阅源/专栏/标签/时间/未读筛选与分页"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in itemQuery) (*mcpsdk.CallToolResult, any, error) {
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
			Status: model.ItemStatus(in.Status), Query: in.Query,
			Unread: in.UnreadOnly, Archived: in.Archived,
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

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "get_item", Description: "按 id 读单篇资讯全文(含 content 正文);列表接口只给摘要,深读素材用这个"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in idArg) (*mcpsdk.CallToolResult, any, error) {
		it, err := st.GetItem(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(it), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "mark_item_read", Description: "标记资讯已读/未读"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in readArg) (*mcpsdk.CallToolResult, any, error) {
		if err := st.MarkRead(in.ID, in.Read); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"ok": true}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "archive_item", Description: "归档/恢复资讯(归档后从列表与未读数隐藏,不删除)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in archiveArg) (*mcpsdk.CallToolResult, any, error) {
		if err := st.SetArchived(in.ID, in.Archived); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"ok": true}, nil
	})

	// ---------- 创作项目 CRUD ----------
	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_creations", Description: "列出创作项目(名称、文件夹、大纲模板、素材源)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		cs, err := st.ListCreations()
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(map[string]any{"creations": cs}), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "create_creation", Description: "新建创作项目;素材源可混合专栏(取其全部订阅源)与单个订阅源"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in creationInput) (*mcpsdk.CallToolResult, any, error) {
		c := &model.Creation{Name: in.Name, GroupName: in.GroupName, TemplateID: in.TemplateID, Sources: toSources(in.Sources)}
		if err := st.AddCreation(c); err != nil {
			return nil, nil, err
		}
		return nil, toolOK(c), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "update_creation", Description: "更新创作项目;只覆盖传入字段,sources 传了就是整体替换"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in creationUpdate) (*mcpsdk.CallToolResult, any, error) {
		cur, err := findCreation(st, in.ID)
		if err != nil {
			return nil, nil, err
		}
		if in.Name != "" {
			cur.Name = in.Name
		}
		if in.GroupName != nil {
			cur.GroupName = *in.GroupName
		}
		if in.TemplateID != nil {
			cur.TemplateID = *in.TemplateID
		}
		if in.Sources != nil {
			cur.Sources = toSources(*in.Sources)
		}
		if err := st.UpdateCreation(cur); err != nil {
			return nil, nil, err
		}
		return nil, toolOK(cur), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "delete_creation", Description: "删除创作项目,其历史产出一并删除"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in idArg) (*mcpsdk.CallToolResult, any, error) {
		if err := st.DeleteCreation(in.ID); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"ok": true}, nil
	})

	// ---------- 生成(预览 / 成文 / 历史) ----------
	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "plan_creation", Description: "预览创作生成输入(大纲模板 + 素材清单 + 完整 prompt);不调用 LLM、无副作用,适合先看会用什么素材"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in planArg) (*mcpsdk.CallToolResult, any, error) {
		p, err := compose.BuildPlan(st, in.ID, in.TemplateID, in.Force)
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(p), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "generate_creation", Description: "生成一篇创作文章并落库(调用配置的 LLM,消耗素材并标记已读,真实副作用,耗时可达数十秒)。生成前可先用 plan_creation 预览"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in planArg) (*mcpsdk.CallToolResult, any, error) {
		a, err := compose.Generate(ctx, st, in.ID, in.TemplateID, in.Force)
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(a), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "list_articles", Description: "列出某创作项目的历史产出文章(标题/时间/消耗素材,不含正文)"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in struct {
		CreationID int64 `json:"creation_id" jsonschema:"创作项目 id"`
	}) (*mcpsdk.CallToolResult, any, error) {
		as, err := st.ListArticles(in.CreationID)
		if err != nil {
			return nil, nil, err
		}
		out := make([]map[string]any, 0, len(as))
		for _, a := range as {
			a.Content = ""
			out = append(out, toolOK(a))
		}
		return nil, map[string]any{"articles": out}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{Name: "get_article", Description: "按 id 读历史产出文章全文"}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in idArg) (*mcpsdk.CallToolResult, any, error) {
		a, err := st.GetArticle(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, toolOK(a), nil
	})
}

// ---- 输入结构 ----

type idArg struct {
	ID int64 `json:"id" jsonschema:"id(用对应 list 接口查)"`
}

type readArg struct {
	ID   int64 `json:"id" jsonschema:"资讯 id"`
	Read bool  `json:"read" jsonschema:"是否已读: true=已读 false=未读"`
}

type archiveArg struct {
	ID       int64 `json:"id" jsonschema:"资讯 id"`
	Archived bool  `json:"archived" jsonschema:"是否归档: true=归档 false=恢复"`
}

type itemQuery struct {
	Query      string `json:"query,omitempty" jsonschema:"全文搜索关键词(标题/正文/摘要/关键词)"`
	FeedID     int64  `json:"feed_id,omitempty" jsonschema:"按订阅源 id 过滤"`
	ColumnID   int64  `json:"column_id,omitempty" jsonschema:"按专栏 id 过滤(含订阅继承与手动收藏)"`
	Tag        string `json:"tag,omitempty" jsonschema:"按标签过滤"`
	Status     string `json:"status,omitempty" jsonschema:"标注状态: pending | processed | failed"`
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"只看未读"`
	Archived   bool   `json:"archived,omitempty" jsonschema:"是否看归档箱: true=只看归档,默认只看未归档"`
	Since      string `json:"since,omitempty" jsonschema:"起始时间: 2026-10-28 或 RFC3339"`
	Until      string `json:"until,omitempty" jsonschema:"截止时间(不含),格式同 since"`
	Limit      int    `json:"limit,omitempty" jsonschema:"返回条数上限,默认 20"`
	Offset     int    `json:"offset,omitempty" jsonschema:"分页偏移"`
}

type sourceRef struct {
	Kind  string `json:"kind" jsonschema:"素材源类型: column 或 feed"`
	RefID int64  `json:"ref_id" jsonschema:"专栏/订阅源 id"`
}

type creationInput struct {
	Name       string      `json:"name" jsonschema:"创作项目名称"`
	GroupName  string      `json:"group_name,omitempty" jsonschema:"所属文件夹(留空为未分组)"`
	TemplateID int64       `json:"template_id,omitempty" jsonschema:"大纲模板 id(list_templates 查)"`
	Sources    []sourceRef `json:"sources,omitempty" jsonschema:"素材源列表(kind 为 column 表示整专栏; kind 为 feed 表示单订阅源)"`
}

type creationUpdate struct {
	ID         int64        `json:"id" jsonschema:"创作项目 id"`
	Name       string       `json:"name,omitempty" jsonschema:"新名称(不传则不变)"`
	GroupName  *string      `json:"group_name,omitempty" jsonschema:"新文件夹(不传则不变)"`
	TemplateID *int64       `json:"template_id,omitempty" jsonschema:"新大纲模板 id(不传则不变)"`
	Sources    *[]sourceRef `json:"sources,omitempty" jsonschema:"素材源整体替换(不传则不变)"`
}

type planArg struct {
	ID         int64 `json:"id" jsonschema:"创作项目 id"`
	TemplateID int64 `json:"template_id,omitempty" jsonschema:"临时覆盖默认大纲模板 id"`
	Force      bool  `json:"force,omitempty" jsonschema:"是否强制: true=不限未读取最新素材(未读耗尽后用)"`
}

func toSources(in []sourceRef) []model.CreationSource {
	out := make([]model.CreationSource, 0, len(in))
	for _, s := range in {
		out = append(out, model.CreationSource{Kind: s.Kind, RefID: s.RefID})
	}
	return out
}

func findCreation(st *store.Store, id int64) (*model.Creation, error) {
	cs, err := st.ListCreations()
	if err != nil {
		return nil, err
	}
	for i := range cs {
		if cs[i].ID == id {
			return &cs[i], nil
		}
	}
	return nil, fmt.Errorf("创作不存在(id=%d)", id)
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

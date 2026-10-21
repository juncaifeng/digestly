// digestly CLI: 查询已采集/标注的资讯,供人读或直接喂给 AI。
//
// 直接读 SQLite(只读),不需要启动服务端:
//
//	digestly --since today --q "AI" --format md
//	digestly --since 7d --tag 技术 --format json
//
// 创作单元(写操作走 WAL 并发):
//
//	digestly -cmd creation-list
//	digestly -cmd creation-add -name 日报1 -group "AI 日报" -sources "column:1,feed:5"
//	digestly -cmd plan -creation 2 [-force]          # 预览模板+素材+prompt,不生成
//	digestly -cmd plan -creation 2 -ids-only         # 只输出素材 id,配合 get 分页细读
//	digestly -cmd get -id 123                        # 按 id 读单篇文章全文
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/compose"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/store"
)

func main() {
	var (
		dataDir = flag.String("data", defaultDataDir(), "数据目录(默认自动探测桌面端数据目录,否则 ./data)")
		sinceS  = flag.String("since", "", "起始时间: today | yesterday | 7d | 30d | 2026-10-20 | RFC3339")
		untilS  = flag.String("until", "", "截止时间(不含),格式同 -since")
		query   = flag.String("q", "", "全文搜索(标题/正文/摘要/关键词)")
		tag     = flag.String("tag", "", "按标签筛选")
		feedS   = flag.String("feed", "", "按订阅源筛选(id 或名称)")
		columnS = flag.String("column", "", "按专栏筛选(id 或名称);含订阅继承与手动收藏")
		statusS = flag.String("status", "", "按标注状态筛选: pending | processed | failed")
		unread  = flag.Bool("unread", false, "只看未读")
		limit   = flag.Int("limit", 20, "返回条数上限")
		offset  = flag.Int("offset", 0, "分页偏移")
		full    = flag.Bool("full", false, "输出正文(默认只给摘要)")
		format  = flag.String("format", "md", "输出格式: md | text | json")
		cmd     = flag.String("cmd", "list", "命令: list | feeds | stats | get | creation-list | creation-add | creation-update | creation-del | plan")
		id      = flag.Int64("id", 0, "条目/创作 id(get / creation-update / creation-del)")
		// 创作单元参数
		creation = flag.String("creation", "", "创作单元(id 或名称),plan 用")
		name     = flag.String("name", "", "创作名称(creation-add/update)")
		group    = flag.String("group", "", "创作文件夹(creation-add/update)")
		template = flag.Int64("template", 0, "大纲模板 id(creation-add/update / plan 覆盖默认模板)")
		sources  = flag.String("sources", "", "素材源,逗号分隔: column:1,feed:5(creation-add/update)")
		force    = flag.Bool("force", false, "plan: 不限未读,取最新素材")
		idsOnly  = flag.Bool("ids-only", false, "plan: 只输出素材 id 列表(分页细读用)")
	)
	flag.Parse()

	// 写命令打开可写库;只读命令走只读(与运行中的服务端并发安全)
	var st *store.Store
	var err error
	switch *cmd {
	case "creation-add", "creation-update", "creation-del":
		st, err = store.Open(*dataDir + "/digestly.db")
	default:
		st, err = store.OpenReadOnly(*dataDir + "/digestly.db")
	}
	if err != nil {
		fatal("打开数据库失败: %v(确认 -data 目录正确)", err)
	}
	defer st.Close()

	switch *cmd {
	case "get":
		cmdGet(st, *id, *format)
	case "creation-list":
		cmdCreationList(st)
	case "creation-add":
		cmdCreationAdd(st, *name, *group, *template, *sources)
	case "creation-update":
		cmdCreationUpdate(st, *id, *name, *group, *template, *sources)
	case "creation-del":
		if *id <= 0 {
			fatal("creation-del 需要 -id")
		}
		if err := st.DeleteCreation(*id); err != nil {
			fatal("删除失败: %v", err)
		}
		fmt.Printf("已删除创作 %d\n", *id)
	case "plan":
		cmdPlan(st, *creation, *template, *force, *idsOnly, *format, *limit, *offset, *full)
	case "feeds":
		feeds, err := st.ListFeeds()
		if err != nil {
			fatal("%v", err)
		}
		cols, _ := st.ListColumns()
		colName := map[int64]string{}
		for _, c := range cols {
			colName[c.ID] = c.Name
		}
		for _, f := range feeds {
			col := "-"
			if f.ColumnID != nil {
				col = colName[*f.ColumnID]
			}
			fmt.Printf("%-4d %-24s %-12s %-10s %s\n", f.ID, trunc(f.Title, 24), trunc(col, 12), f.Collector, f.URL)
		}
	case "reprocess":
		// 重跑标注管线(管线升级后刷新存量摘要/关键词)
		w, err := store.Open(*dataDir + "/digestly.db")
		if err != nil {
			fatal("%v", err)
		}
		defer w.Close()
		n, err := w.ResetEnrichment(true)
		if err != nil {
			fatal("重置失败: %v", err)
		}
		fmt.Printf("已重置 %d 篇为待标注,开始重跑管线…", n)
		fmt.Println()
		chain := pipeline.NewChain(w,
			pipeline.Source{Rules: map[string]string{}},
			pipeline.Summary{MaxRunes: 200},
			pipeline.Keywords{TopN: 5},
		)
		done := 0
		for {
			c, err := chain.RunOnce(200)
			if err != nil {
				fatal("管线失败: %v", err)
			}
			done += c
			if c == 0 {
				break
			}
		}
		fmt.Printf("重新标注完成: %d 篇", done)
		fmt.Println()
	case "stats":
		items, entries, bytes := st.CacheStats()
		fmt.Printf("索引条目: %d\nL2 正文缓存: %d 条 / %d 字节\n", items, entries, bytes)
		for _, s := range []model.ItemStatus{model.StatusPending, model.StatusProcessed, model.StatusFailed} {
			n, err := st.ListItems(store.ItemFilter{Status: s, Limit: 100000})
			if err == nil {
				fmt.Printf("%s: %d\n", s, len(n))
			}
		}
	default:
		since, err := parseTime(*sinceS, true)
		if err != nil {
			fatal("起始时间格式不对 %q: %v", *sinceS, err)
		}
		until, err := parseTime(*untilS, false)
		if err != nil {
			fatal("截止时间格式不对 %q: %v", *untilS, err)
		}
		feedID, err := resolveFeed(st, *feedS)
		if err != nil {
			fatal("%v", err)
		}
		var columnID int64
		if *columnS != "" {
			columnID, err = st.FindColumn(*columnS)
			if err != nil {
				fatal("%v", err)
			}
		}
		items, err := st.ListItems(store.ItemFilter{
			FeedID:   feedID,
			ColumnID: columnID,
			Status:   model.ItemStatus(*statusS),
			Tag:      *tag,
			Query:    *query,
			Unread:   *unread,
			Since:    since,
			Until:    until,
			Limit:    *limit,
			Offset:   *offset,
		})
		if err != nil {
			fatal("查询失败: %v", err)
		}
		emit(items, *format, *full, *query, *sinceS)
	}
}

// parseTime 解析时间参数。start 决定日期粒度参数取当天起点还是次日起点。
func parseTime(s string, start bool) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	now := time.Now()
	var t time.Time
	switch strings.ToLower(s) {
	case "today":
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if !start {
			t = t.Add(24 * time.Hour)
		}
	case "yesterday":
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-24 * time.Hour)
		if !start {
			t = t.Add(24 * time.Hour)
		}
	default:
		if strings.HasSuffix(s, "d") {
			var n int
			if _, err := fmt.Sscanf(s, "%dd", &n); err != nil {
				return nil, fmt.Errorf("无法识别的相对时间 %q", s)
			}
			t = now.Add(-time.Duration(n) * 24 * time.Hour)
		} else if ts, err := time.Parse(time.RFC3339, s); err == nil {
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

// resolveFeed 把 -feed 的 id 或名称解析成 feed id(空则不限)。
func resolveFeed(st *store.Store, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	feeds, err := st.ListFeeds()
	if err != nil {
		return 0, err
	}
	for _, f := range feeds {
		if f.Title == s || strings.EqualFold(f.Title, s) {
			return f.ID, nil
		}
	}
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err == nil {
		return id, nil
	}
	return 0, fmt.Errorf("找不到订阅源 %q", s)
}

func emit(items []model.Item, format string, full bool, query, since string) {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if !full {
			for i := range items {
				items[i].Content = ""
			}
		}
		_ = enc.Encode(items)
	case "text":
		for i, it := range items {
			fmt.Printf("%d. %s\n   %s | %s\n   %s\n", i+1, it.Title, dateOf(it), sourceOf(it), it.Link)
			if it.Summary != "" {
				fmt.Printf("   %s\n", it.Summary)
			}
		}
		fmt.Printf("\n共 %d 篇\n", len(items))
	default: // md — 直接喂给 AI 的格式
		head := "digestly 资讯"
		if query != "" {
			head += " · 关键词「" + query + "」"
		}
		if since != "" {
			head += " · 自 " + since
		}
		fmt.Printf("# %s(共 %d 篇)\n\n", head, len(items))
		for i, it := range items {
			fmt.Printf("## %d. %s\n\n", i+1, it.Title)
			fmt.Printf("- 时间: %s\n", dateOf(it))
			if it.Source != "" {
				fmt.Printf("- 来源: %s\n", it.Source)
			}
			if it.Keywords != "" {
				fmt.Printf("- 关键词: %s\n", it.Keywords)
			}
			if len(it.Tags) > 0 {
				fmt.Printf("- 标签: %s\n", strings.Join(it.Tags, ", "))
			}
			fmt.Printf("- 链接: %s\n", it.Link)
			if it.Summary != "" {
				fmt.Printf("\n%s\n", it.Summary)
			}
			if full && it.Content != "" {
				fmt.Printf("\n<正文>\n%s\n</正文>\n", trunc(stripTags(it.Content), 2000))
			}
			fmt.Println()
		}
	}
}

func dateOf(it model.Item) string {
	if it.PublishedAt != nil {
		return it.PublishedAt.Local().Format("2006-01-02 15:04")
	}
	return it.CreatedAt.Local().Format("2006-01-02 15:04")
}

func sourceOf(it model.Item) string {
	if it.Source != "" {
		return it.Source
	}
	return it.Author
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// stripTags 极简去 HTML,只在 --full 时用(管线摘要已去标签)。
func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// cmdGet 按 id 读单篇文章(plan -ids-only 后分页细读用)。
func cmdGet(st *store.Store, id int64, format string) {
	if id <= 0 {
		fatal("get 需要 -id")
	}
	it, err := st.GetItem(id)
	if err != nil {
		fatal("读取失败: %v", err)
	}
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(it)
		return
	}
	fmt.Printf("# %s\n\n- id: %d\n- 时间: %s\n- 来源: %s\n- 链接: %s\n", it.Title, it.ID, dateOf(*it), sourceOf(*it), it.Link)
	if it.Summary != "" {
		fmt.Printf("\n%s\n", it.Summary)
	}
	if it.Content != "" {
		fmt.Printf("\n<正文>\n%s\n</正文>\n", stripTags(it.Content))
	}
}

// cmdCreationList 列出全部创作单元。
func cmdCreationList(st *store.Store) {
	cs, err := st.ListCreations()
	if err != nil {
		fatal("%v", err)
	}
	for _, c := range cs {
		var srcs []string
		for _, s := range c.Sources {
			srcs = append(srcs, fmt.Sprintf("%s:%d", s.Kind, s.RefID))
		}
		g := c.GroupName
		if g == "" {
			g = "-"
		}
		fmt.Printf("%-4d %-20s 文件夹:%-12s 模板:%-4d 源:[%s]\n", c.ID, trunc(c.Name, 20), trunc(g, 12), c.TemplateID, strings.Join(srcs, ","))
	}
}

// parseSources 解析 "column:1,feed:5" 为素材源列表。空 = 不变(仅 update 语义;add 时为空切片)。
func parseSources(s string) ([]model.CreationSource, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []model.CreationSource
	for _, part := range strings.Split(s, ",") {
		kind, idS, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || (kind != "column" && kind != "feed") {
			return nil, fmt.Errorf("素材源格式不对 %q,应为 column:<id> 或 feed:<id>", part)
		}
		var id int64
		if _, err := fmt.Sscanf(idS, "%d", &id); err != nil || id <= 0 {
			return nil, fmt.Errorf("素材源 id 不对 %q", part)
		}
		out = append(out, model.CreationSource{Kind: kind, RefID: id})
	}
	return out, nil
}

func cmdCreationAdd(st *store.Store, name, group string, templateID int64, sourcesS string) {
	if strings.TrimSpace(name) == "" {
		fatal("creation-add 需要 -name")
	}
	srcs, err := parseSources(sourcesS)
	if err != nil {
		fatal("%v", err)
	}
	c := &model.Creation{Name: name, GroupName: group, TemplateID: templateID, Sources: srcs}
	if err := st.AddCreation(c); err != nil {
		fatal("新建失败: %v", err)
	}
	fmt.Printf("已创建创作 %d %q\n", c.ID, c.Name)
}

func cmdCreationUpdate(st *store.Store, id int64, name, group string, templateID int64, sourcesS string) {
	if id <= 0 {
		fatal("creation-update 需要 -id")
	}
	var cur *model.Creation
	cs, err := st.ListCreations()
	if err != nil {
		fatal("%v", err)
	}
	for i := range cs {
		if cs[i].ID == id {
			cur = &cs[i]
			break
		}
	}
	if cur == nil {
		fatal("创作不存在(id=%d)", id)
	}
	// 只覆盖显式传入的字段
	if name != "" {
		cur.Name = name
	}
	if group != "" {
		cur.GroupName = group
	}
	if templateID > 0 {
		cur.TemplateID = templateID
	}
	if strings.TrimSpace(sourcesS) != "" {
		srcs, err := parseSources(sourcesS)
		if err != nil {
			fatal("%v", err)
		}
		cur.Sources = srcs
	}
	if err := st.UpdateCreation(cur); err != nil {
		fatal("更新失败: %v", err)
	}
	fmt.Printf("已更新创作 %d\n", id)
}

// cmdPlan 预览创作的生成输入: 模板 + 素材 + 完整 prompt,不调用 LLM、无副作用。
// -ids-only 只输出素材 id(配合 get -id 分页细读);否则按 -limit/-offset 分页展示素材。
func cmdPlan(st *store.Store, creationS string, templateID int64, force, idsOnly bool, format string, limit, offset int, full bool) {
	creationID, err := resolveCreation(st, creationS)
	if err != nil {
		fatal("%v", err)
	}
	p, err := compose.BuildPlan(st, creationID, templateID, force)
	if err != nil {
		fatal("预览失败: %v", err)
	}
	if idsOnly {
		for _, id := range p.ItemIDs {
			fmt.Println(id)
		}
		return
	}
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(p)
		return
	}
	fmt.Printf("# 创作预览: %s\n\n", p.Creation.Name)
	fmt.Printf("- 模板: %s (id=%d)\n", p.Template.Name, p.Template.ID)
	fmt.Printf("- 订阅源: %v\n", p.FeedIDs)
	fmt.Printf("- 强制模式: %v\n", p.Force)
	fmt.Printf("- 素材: %d 篇(分页 %d-%d)\n", len(p.ItemIDs), offset, offset+min(limit, len(p.Items)))
	if len(p.HistoryTitles) > 0 {
		fmt.Printf("- 历史上下文: %s 等 %d 篇\n", p.HistoryTitles[0], len(p.HistoryTitles))
	}
	fmt.Println()
	end := min(offset+limit, len(p.Items))
	for i := offset; i < end; i++ {
		it := p.Items[i]
		read := "未读"
		if it.Read {
			read = "已读"
		}
		fmt.Printf("%d. [#%d] %s (%s, %s)\n", i+1, it.ID, it.Title, sourceOf(it), read)
		if it.Summary != "" {
			fmt.Printf("   %s\n", it.Summary)
		}
		if full && it.Content != "" {
			fmt.Printf("   <正文>%s</正文>\n", trunc(stripTags(it.Content), 2000))
		}
	}
	fmt.Printf("\n## 完整 prompt(%d 字符)\n\n%s\n", len(p.Prompt), p.Prompt)
}

// resolveCreation 把 -creation 的 id 或名称解析成创作 id。
func resolveCreation(st *store.Store, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("plan 需要 -creation <id 或名称>")
	}
	cs, err := st.ListCreations()
	if err != nil {
		return 0, err
	}
	for _, c := range cs {
		if c.Name == s || strings.EqualFold(c.Name, s) {
			return c.ID, nil
		}
	}
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err == nil {
		for _, c := range cs {
			if c.ID == id {
				return id, nil
			}
		}
		return 0, fmt.Errorf("创作不存在(id=%d)", id)
	}
	return 0, fmt.Errorf("找不到创作 %q", s)
}

// defaultDataDir 自动探测桌面端(Tauri app_data_dir)的数据目录:
// Windows %APPDATA%\com.digestly.app, macOS ~/Library/Application Support/com.digestly.app,
// Linux ~/.local/share/com.digestly.app。找不到则退回 ./data。
func defaultDataDir() string {
	var dir string
	if runtime.GOOS == "linux" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".local", "share", "com.digestly.app")
		}
	} else if cfg, err := os.UserConfigDir(); err == nil {
		dir = filepath.Join(cfg, "com.digestly.app")
	}
	if dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "digestly.db")); err == nil {
			return dir
		}
	}
	return "data"
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

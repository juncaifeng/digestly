// digestly CLI: 查询已采集的文章,供人读或直接喂给 AI/下游工具。
//
// 直接读 SQLite(只读),不需要启动服务端:
//
//	digestly --since today --q "AI" --format md
//	digestly -cmd list --since 7d --tag 技术 --format json
//	digestly -cmd get -id 123                        # 按 id 读单篇文章全文
//	digestly -cmd read -id 123 / unread -id 123      # 标记已读/未读
//	digestly -cmd archive -id 123 / unarchive -id 123 # 归档/恢复
//	digestly -cmd feeds / columns / stats            # 订阅源/专栏/统计
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/model"
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
		unread  = flag.Bool("unread", false, "只看未读")
		limit   = flag.Int("limit", 20, "返回条数上限")
		offset  = flag.Int("offset", 0, "分页偏移")
		full    = flag.Bool("full", false, "输出正文(默认只给摘要)")
		format  = flag.String("format", "md", "输出格式: md | text | json")
		cmd     = flag.String("cmd", "list", "命令: list | feeds | columns | stats | get | read | unread | archive | unarchive")
		id      = flag.Int64("id", 0, "文章 id(get / read / unread / archive / unarchive)")
	)
	flag.Parse()

	// 写命令打开可写库;只读命令走只读(与运行中的服务端并发安全)
	var st *store.Store
	var err error
	switch *cmd {
	case "read", "unread", "archive", "unarchive":
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
	case "read", "unread":
		if *id <= 0 {
			fatal("%s 需要 -id", *cmd)
		}
		if err := st.MarkRead(*id, *cmd == "read"); err != nil {
			fatal("标记失败: %v", err)
		}
		fmt.Printf("已标记 %d 为%s\n", *id, map[bool]string{true: "已读", false: "未读"}[*cmd == "read"])
	case "archive", "unarchive":
		if *id <= 0 {
			fatal("%s 需要 -id", *cmd)
		}
		if err := st.SetArchived(*id, *cmd == "archive"); err != nil {
			fatal("归档失败: %v", err)
		}
		fmt.Printf("已%s %d\n", map[bool]string{true: "归档", false: "恢复"}[*cmd == "archive"], *id)
	case "columns":
		cols, err := st.ListColumns()
		if err != nil {
			fatal("%v", err)
		}
		if *format == "json" {
			printJSON(cols)
			return
		}
		for _, c := range cols {
			fmt.Printf("%-4d %-24s %d 源 / %d 收藏\n", c.ID, trunc(c.Name, 24), c.FeedCount, c.ItemCount)
		}
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
	case "stats":
		items, entries, bytes := st.CacheStats()
		fmt.Printf("索引条目: %d\nL2 正文缓存: %d 条 / %d 字节\n", items, entries, bytes)
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

// printJSON 统一 JSON 输出: 缩进两空格;nil slice/map 兜底为 [] / {} 而非 null(喂 AI 时的稳定契约)。
func printJSON(v any) {
	switch rv := reflect.ValueOf(v); {
	case rv.Kind() == reflect.Slice && rv.IsNil():
		v = reflect.MakeSlice(rv.Type(), 0, 0)
	case rv.Kind() == reflect.Map && rv.IsNil():
		v = reflect.MakeMap(rv.Type())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func emit(items []model.Item, format string, full bool, query, since string) {
	switch format {
	case "json":
		if !full {
			for i := range items {
				items[i].Content = ""
			}
		}
		printJSON(items)
	case "text":
		for i, it := range items {
			fmt.Printf("%d. %s\n   %s | %s\n   %s\n", i+1, it.Title, dateOf(it), sourceOf(it), it.Link)
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
			if len(it.Tags) > 0 {
				fmt.Printf("- 标签: %s\n", strings.Join(it.Tags, ", "))
			}
			fmt.Printf("- 链接: %s\n", it.Link)
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
		printJSON(it)
		return
	}
	fmt.Printf("# %s\n\n- id: %d\n- 时间: %s\n- 来源: %s\n- 链接: %s\n", it.Title, it.ID, dateOf(*it), sourceOf(*it), it.Link)
	if it.Content != "" {
		fmt.Printf("\n<正文>\n%s\n</正文>\n", stripTags(it.Content))
	}
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

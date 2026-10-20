// digestly CLI: 查询已采集/标注的资讯,供人读或直接喂给 AI。
//
// 直接读 SQLite(只读),不需要启动服务端:
//
//	digestly --since today --q "AI" --format md
//	digestly --since 7d --tag 技术 --format json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/store"
)

func main() {
	var (
		dataDir  = flag.String("data", "data", "数据目录(与服务端 -data 一致)")
		sinceS   = flag.String("since", "", "起始时间: today | yesterday | 7d | 30d | 2026-10-20 | RFC3339")
		untilS   = flag.String("until", "", "截止时间(不含),格式同 -since")
		query    = flag.String("q", "", "全文搜索(标题/正文/摘要/关键词)")
		tag      = flag.String("tag", "", "按标签筛选")
		feedS    = flag.String("feed", "", "按订阅源筛选(id 或名称)")
		statusS  = flag.String("status", "", "按标注状态筛选: pending | processed | failed")
		unread   = flag.Bool("unread", false, "只看未读")
		limit    = flag.Int("limit", 20, "返回条数上限")
		offset   = flag.Int("offset", 0, "分页偏移")
		full     = flag.Bool("full", false, "输出正文(默认只给摘要)")
		format   = flag.String("format", "md", "输出格式: md | text | json")
		cmd      = flag.String("cmd", "list", "命令: list | feeds | stats")
	)
	flag.Parse()

	st, err := store.OpenReadOnly(*dataDir + "/digestly.db")
	if err != nil {
		fatal("打开数据库失败: %v(确认 -data 目录正确)", err)
	}
	defer st.Close()

	switch *cmd {
	case "feeds":
		feeds, err := st.ListFeeds()
		if err != nil {
			fatal("%v", err)
		}
		for _, f := range feeds {
			fmt.Printf("%-4d %-24s %-10s %s\n", f.ID, trunc(f.Title, 24), f.Collector, f.URL)
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
		items, err := st.ListItems(store.ItemFilter{
			FeedID: feedID,
			Status: model.ItemStatus(*statusS),
			Tag:    *tag,
			Query:  *query,
			Unread: *unread,
			Since:  since,
			Until:  until,
			Limit:  *limit,
			Offset: *offset,
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

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

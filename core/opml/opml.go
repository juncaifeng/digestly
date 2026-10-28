// Package opml 实现 OPML 2.0 订阅清单的解析与生成,用于订阅的导入/导出。
// 结构约定: 顶层 outline(带 xmlUrl)为未分组订阅;嵌套 outline 组映射为专栏,
// 其内带 xmlUrl 的子 outline 映射为该专栏下的订阅源。
package opml

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Entry OPML 里的一条订阅(树拍平后的叶子)。
type Entry struct {
	Group  string // 所属分组(顶层 outline 文本);空 = 未分组
	Title  string
	XMLURL string // 订阅地址(RSS/Atom)
	HTMLURL string
}

type document struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr,omitempty"`
	Head    head     `xml:"head"`
	Body    body     `xml:"body"`
}

type head struct {
	Title       string `xml:"title,omitempty"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

type body struct {
	Outlines []outline `xml:"outline"`
}

type outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Children []outline `xml:"outline"`
}

// Parse 解析 OPML 文本,返回拍平的订阅列表(保留分组归属)。
// 递归遍历: 带 xmlUrl 的 outline 是订阅;不带的是分组。
func Parse(data []byte) ([]Entry, error) {
	var doc document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("OPML 解析失败: %w", err)
	}
	var out []Entry
	var walk func(nodes []outline, group string)
	walk = func(nodes []outline, group string) {
		for _, n := range nodes {
			url := strings.TrimSpace(n.XMLURL)
			title := strings.TrimSpace(n.Title)
			if title == "" {
				title = strings.TrimSpace(n.Text)
			}
			if url != "" {
				out = append(out, Entry{Group: group, Title: title, XMLURL: url, HTMLURL: strings.TrimSpace(n.HTMLURL)})
				continue
			}
			// 无 xmlUrl 的 outline 是分组;下探一层
			walk(n.Children, firstNonEmpty(strings.TrimSpace(n.Text), strings.TrimSpace(n.Title)))
		}
	}
	walk(doc.Body.Outlines, "")
	if len(out) == 0 {
		return nil, fmt.Errorf("OPML 里没有找到订阅源(带 xmlUrl 的 outline)")
	}
	return out, nil
}

// Build 生成 OPML 文本。groups: 分组名 -> 组内订阅;ungrouped: 未分组订阅。
func Build(title string, groups map[string][]Entry, ungrouped []Entry) ([]byte, error) {
	doc := document{
		Version: "2.0",
		Head: head{
			Title:       firstNonEmpty(title, "digestly subscriptions"),
			DateCreated: time.Now().UTC().Format(time.RFC1123Z),
		},
	}
	for _, e := range ungrouped {
		doc.Body.Outlines = append(doc.Body.Outlines, entryOutline(e))
	}
	// 分组名排序(map 无序)
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	for _, g := range names {
		entries := groups[g]
		if len(entries) == 0 {
			continue
		}
		grp := outline{Text: g, Title: g}
		for _, e := range entries {
			grp.Children = append(grp.Children, entryOutline(e))
		}
		doc.Body.Outlines = append(doc.Body.Outlines, grp)
	}
	b, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), b...), nil
}

func entryOutline(e Entry) outline {
	return outline{
		Text:    e.Title,
		Title:   e.Title,
		Type:    "rss",
		XMLURL:  e.XMLURL,
		HTMLURL: e.HTMLURL,
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

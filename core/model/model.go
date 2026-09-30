// Package model 定义 digestly 的核心数据模型。
package model

import "time"

// ItemStatus 表示文章在标注管线中的状态。
type ItemStatus string

const (
	StatusPending   ItemStatus = "pending"   // 已采集，等待标注管线处理
	StatusProcessed ItemStatus = "processed" // 已完成标注(来源/摘要/关键词)
	StatusFailed    ItemStatus = "failed"    // 标注失败，可重试
)

// Feed 一个订阅源。
type Feed struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Collector   string    `json:"collector"` // 采集器名: rss / atom / jsonfeed / js:<脚本名>
	Config      string    `json:"config"`    // JSON，采集器自定义参数
	IntervalMin int       `json:"interval_min"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Item 一篇文章。Raw 字段为采集器输出，Enriched 字段由标注管线填充。
type Item struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feed_id"`
	GUID        string     `json:"guid"`
	Title       string     `json:"title"`
	Link        string     `json:"link"`
	Author      string     `json:"author"`
	Content     string     `json:"content"`      // 原始正文(HTML 或纯文本)
	Summary     string     `json:"summary"`      // 管线产出: 摘要
	Keywords    string     `json:"keywords"`     // 管线产出: 逗号分隔关键词
	Source      string     `json:"source"`       // 管线产出: 来源归类
	PublishedAt *time.Time `json:"published_at"`
	Status      ItemStatus `json:"status"`
	Read        bool       `json:"read"`
	Tags        []string   `json:"tags,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Tag 一个标签，可由管线或用户附加到 Item。
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

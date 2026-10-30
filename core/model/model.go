// Package model 定义 digestly 的核心数据模型。
// 职责边界: 只做文章采集与阅读 —— 订阅源、文章(唯一 id + 原文 + 元数据)、专栏、标签。
package model

import "time"

// Feed 一个订阅源。
type Feed struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	ColumnID    *int64    `json:"column_id"` // 所属专栏;nil = 未分组
	URL         string    `json:"url"`
	Collector   string    `json:"collector"` // 采集器名: rss / atom / jsonfeed / js:<脚本名>
	Config      string    `json:"config"`    // JSON，采集器自定义参数
	IntervalMin int       `json:"interval_min"`
	MaxItems    int       `json:"max_items"`    // 历史上限: 仅保留最近 N 条,0 = 不限
	UnreadCount int       `json:"unread_count"` // 未读文章数(ListFeeds 时统计)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Item 一篇文章: 采集的唯一记录(id + 原文 + 元数据),不做内容加工。
type Item struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feed_id"`
	GUID        string     `json:"guid"`
	Title       string     `json:"title"`
	Link        string     `json:"link"`
	Author      string     `json:"author"`
	Content     string     `json:"content"`  // 原始正文(HTML 或纯文本)
	Source      string     `json:"source"`   // 来源(订阅源名/域名)
	PublishedAt *time.Time `json:"published_at"`
	Read        bool       `json:"read"`
	Archived    bool       `json:"archived"`            // 已归档: 从列表/未读数隐藏,不删除
	Tags        []string   `json:"tags,omitempty"`
	ColumnIDs   []int64    `json:"column_ids,omitempty"` // 手动收藏进的专栏(仅详情填充)
	CreatedAt   time.Time  `json:"created_at"`
}

// MarketSource 一个已挂载的第三方市场源(订阅源清单地址)。
type MarketSource struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Enabled   bool      `json:"enabled"`
	FeedCount int       `json:"feed_count"`
	LastFetch time.Time `json:"last_fetch"`
	CreatedAt time.Time `json:"created_at"`
}

// Column 一个专栏: 订阅源的文件夹式分组,文章可手动收藏进来。
type Column struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	FeedCount int       `json:"feed_count"` // 组内订阅数
	ItemCount int       `json:"item_count"` // 手动收藏的文章数
	CreatedAt time.Time `json:"created_at"`
}

// Tag 一个标签,可附加到 Item。
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

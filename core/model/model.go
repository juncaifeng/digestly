// Package model 定义 digestly 的核心数据模型。
package model

import "time"

// ItemStatus 表示文章在标注管线中的状态。
type ItemStatus string

const (
	StatusPending    ItemStatus = "pending"    // 已采集，等待标注管线处理
	StatusProcessing ItemStatus = "processing" // 处理中(已被管线认领)
	StatusProcessed  ItemStatus = "processed"  // 已完成标注(来源/摘要/关键词)
	StatusFailed     ItemStatus = "failed"     // 标注失败，可重试
)

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
	Archived    bool       `json:"archived"`   // 已归档: 从列表/未读数隐藏,不删除
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
// 作为创作组织单元,可关联默认大纲模板(TemplateID)。
type Column struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Sort       int       `json:"sort"`
	TemplateID int64     `json:"template_id"` // 关联的默认大纲模板;0 = 未设置
	FeedCount  int       `json:"feed_count"`  // 组内订阅数
	ItemCount  int       `json:"item_count"`  // 手动收藏的文章数
	CreatedAt  time.Time `json:"created_at"`
}

// Tag 一个标签，可由管线或用户附加到 Item。
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// LLMConfig 一个大模型供应商配置(OpenAI 兼容接口)。
// LLMConfig 一个大模型供应商配置(OpenAI 兼容接口)。
// kind: "chat"(默认,对话/生成) | "embedding"(向量模型,实体/文章向量化)。
type LLMConfig struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"` // "chat"(默认) | "embedding"
	BaseURL     string    `json:"base_url"`
	APIKey      string    `json:"api_key"` // 列表/详情接口返回掩码;写空串表示不修改
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	IsDefault   bool      `json:"is_default"` // 同 kind 内唯一
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ArticleTemplate 专栏文章生成模板,Prompt 中 {{items}} 为素材占位符。
type ArticleTemplate struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Prompt    string    `json:"prompt"`
	BuiltIn   bool      `json:"built_in"` // 内置模板不可删除,可编辑
	CreatedAt time.Time `json:"created_at"`
}

// CreationSource 创作的素材来源: 整个专栏 或 单个订阅源。
type CreationSource struct {
	Kind  string `json:"kind"` // "column" | "feed"
	RefID int64  `json:"ref_id"`
}

// Creation 一个创作单元(如 "AI 日报"),一等公民,不与专栏强绑定。
// 素材源可混合勾选专栏与单个订阅源;GroupName 为所属文件夹(空 = 未分组)。
type Creation struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	GroupName  string           `json:"group_name"`
	TemplateID int64            `json:"template_id"` // 关联的大纲模板;0 = 未设置
	Sources    []CreationSource `json:"sources,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// Article 一篇生成的创作文章。CreationID 指向创作单元;ItemIDs 记录消耗的素材文章 id。
type Article struct {
	ID         int64     `json:"id"`
	CreationID int64     `json:"creation_id"`
	TemplateID int64     `json:"template_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	ItemIDs    []int64   `json:"item_ids,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

package market

import (
	"context"
	"sync"
	"time"
)

// SourceFeeds 一个已挂载市场源解析出的订阅源目录(带缓存)。
type SourceFeeds struct {
	SourceURL string       `json:"source_url"`
	Title     string       `json:"title"`
	Feeds     []MarketFeed `json:"feeds"`
	FetchedAt time.Time    `json:"fetched_at"`
	Err       string       `json:"err,omitempty"` // 上次解析失败原因(仍保留旧目录)
}

// FeedMarket 多个市场源的订阅源目录缓存。
// 读缓存瞬时返回;过期或 force 才回源解析——和采集器市场同一套策略。
type FeedMarket struct {
	// FeedTTL 目录过期时长。
	FeedTTL time.Duration

	mu    sync.Mutex
	cache map[string]*SourceFeeds
}

func NewFeedMarket() *FeedMarket {
	return &FeedMarket{FeedTTL: time.Hour, cache: map[string]*SourceFeeds{}}
}

// Sync 解析一个市场源并缓存目录(挂载/手动刷新时调用)。
func (m *FeedMarket) Sync(ctx context.Context, sourceURL, title string) *SourceFeeds {
	feeds, err := ParseFeeds(ctx, nil, sourceURL)
	sf := &SourceFeeds{SourceURL: sourceURL, Title: title, FetchedAt: time.Now()}
	if err != nil {
		sf.Err = err.Error()
	} else {
		sf.Feeds = feeds
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// 解析失败时保留旧目录(离线降级),只记录失败原因
	if old, ok := m.cache[sourceURL]; ok && err != nil {
		sf.Feeds = old.Feeds
	}
	m.cache[sourceURL] = sf
	return sf
}

// Get 取缓存目录;过期时后台刷新,当前请求返回旧目录(不阻塞)。
func (m *FeedMarket) Get(ctx context.Context, sourceURL, title string) *SourceFeeds {
	m.mu.Lock()
	sf, ok := m.cache[sourceURL]
	stale := !ok || time.Since(sf.FetchedAt) > m.FeedTTL
	m.mu.Unlock()

	if !ok {
		// 首次: 同步解析一次(挂载后的首次列表要能看到内容)
		return m.Sync(ctx, sourceURL, title)
	}
	if stale {
		go m.Sync(context.Background(), sourceURL, title)
	}
	return sf
}

// Evict 丢弃某市场源的缓存(卸载时调用)。
func (m *FeedMarket) Evict(sourceURL string) {
	m.mu.Lock()
	delete(m.cache, sourceURL)
	m.mu.Unlock()
}

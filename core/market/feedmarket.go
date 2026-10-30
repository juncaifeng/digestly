package market

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

	// HTTP 用于解析清单;带短超时,避免远端挂起把后台刷新 goroutine 吊住
	HTTP *http.Client

	// CachePath 目录缓存落盘路径(空 = 仅内存)。启动即载入,
	// 市场页打开先出缓存内容,过期才后台刷新——不必每次打开都回源。
	CachePath string

	mu         sync.Mutex
	cache      map[string]*SourceFeeds
	refreshing map[string]time.Time // 正在后台刷新的地址 → 开始时刻(超时即视为失效)
}

// refreshBudget 后台刷新允许占用 refreshing 标记的最长时间;
// 超过即视为失效,后续 Get 可重新排队刷新。
// 变量便于测试缩短。
var refreshBudget = 30 * time.Second

func NewFeedMarket(cachePath string) *FeedMarket {
	m := &FeedMarket{
		FeedTTL:    time.Hour,
		HTTP:       &http.Client{Timeout: 15 * time.Second},
		CachePath:  cachePath,
		cache:      map[string]*SourceFeeds{},
		refreshing: map[string]time.Time{},
	}
	m.load()
	return m
}

// load 启动时从磁盘载入目录缓存(损坏/缺失时静默跳过)。
func (m *FeedMarket) load() {
	if m.CachePath == "" {
		return
	}
	b, err := os.ReadFile(m.CachePath)
	if err != nil {
		return
	}
	var saved map[string]*SourceFeeds
	if err := json.Unmarshal(b, &saved); err != nil {
		return
	}
	for k, v := range saved {
		if v != nil {
			m.cache[k] = v
		}
	}
}

// persistLocked 落盘目录缓存(调用方需持有 mu)。原子写,失败不影响内存态。
func (m *FeedMarket) persistLocked() {
	if m.CachePath == "" {
		return
	}
	b, err := json.Marshal(m.cache)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.CachePath), 0o755); err != nil {
		return
	}
	tmp := m.CachePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, m.CachePath)
}

// Sync 解析一个市场源并缓存目录(挂载/手动刷新时调用)。
// 解析失败时保留旧目录(离线降级);失败不更新 FetchedAt,
// 否则目录刚失败就被当成"新鲜",过期重试/红标会长期残留。
func (m *FeedMarket) Sync(ctx context.Context, sourceURL, title string) *SourceFeeds {
	feeds, err := ParseFeeds(ctx, m.HTTP, sourceURL)

	m.mu.Lock()
	defer m.mu.Unlock()
	old, hadOld := m.cache[sourceURL]
	sf := &SourceFeeds{SourceURL: sourceURL, Title: title, FetchedAt: time.Now()}
	if err != nil {
		sf.Err = err.Error()
		if hadOld {
			// 离线降级: 继续用上次成功的目录,时间戳也算上次成功的
			sf.Feeds = old.Feeds
			sf.FetchedAt = old.FetchedAt
		} else {
			// 从未成功过: 留着零值时间戳,让下次 Get 继续重试,
			// 而不是把失败当下"新鲜"从而永久卡在空目录
			sf.FetchedAt = time.Time{}
		}
	} else {
		sf.Feeds = feeds // 成功即清空旧的失败原因
	}
	m.cache[sourceURL] = sf
	m.persistLocked()
	return sf
}

// Get 取缓存目录;过期时后台刷新,当前请求返回旧目录(不阻塞)。
// 同一地址的后台刷新用 refreshing 去重,避免并发打爆远端。
func (m *FeedMarket) Get(ctx context.Context, sourceURL, title string) *SourceFeeds {
	m.mu.Lock()
	sf, ok := m.cache[sourceURL]
	stale := !ok || time.Since(sf.FetchedAt) > m.FeedTTL
	kick := false
	if stale {
		busy := false
		if started, ok := m.refreshing[sourceURL]; ok {
			// 上一轮卡太久(远端挂起)就视为失效,允许重新排队,否则永远不再刷新
			busy = time.Since(started) <= refreshBudget
		}
		if !busy {
			m.refreshing[sourceURL] = time.Now()
			kick = true
		}
	}
	m.mu.Unlock()

	if !ok {
		// 首次: 同步解析一次(挂载后的首次列表要能看到内容)。
		// 之前这里在 kick 时就把 refreshing 置位,而同步分支不走后台 goroutine
		// 的 defer 清理,标记会永久残留 → 后续所有后台刷新被跳过。
		if kick {
			m.mu.Lock()
			delete(m.refreshing, sourceURL)
			m.mu.Unlock()
		}
		return m.Sync(ctx, sourceURL, title)
	}
	if kick {
		go func() {
			defer func() {
				m.mu.Lock()
				delete(m.refreshing, sourceURL)
				m.mu.Unlock()
			}()
			// 后台刷新带独立超时: 远端挂起时也必定返回,
			// 否则 refreshing 会被长期占住、后续刷新全部被跳过
			ctx, cancel := context.WithTimeout(context.Background(), refreshBudget)
			defer cancel()
			m.Sync(ctx, sourceURL, title)
		}()
	}
	return sf
}

// Evict 丢弃某市场源的缓存(卸载时调用)。
func (m *FeedMarket) Evict(sourceURL string) {
	m.mu.Lock()
	delete(m.cache, sourceURL)
	delete(m.refreshing, sourceURL)
	m.persistLocked()
	m.mu.Unlock()
}

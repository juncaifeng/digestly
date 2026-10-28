// Package scheduler 按 feed 配置的间隔周期采集，并驱动标注管线。
package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/store"
)

type Scheduler struct {
	store    *store.Store
	pipeline *pipeline.Chain
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func New(st *store.Store, pl *pipeline.Chain) *Scheduler {
	return &Scheduler{store: st, pipeline: pl}
}

// Start 启动调度循环：每 30s 扫描一次到期 feed。
func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		s.cycle(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.cycle(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) cycle(ctx context.Context) {
	feeds, err := s.store.ListFeeds()
	if err != nil {
		log.Printf("scheduler: list feeds: %v", err)
		return
	}
	for _, f := range feeds {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := s.RefreshFeed(ctx, f.ID); err != nil {
			log.Printf("scheduler: refresh feed %d: %v", f.ID, err)
		}
	}
}

// RefreshFeed 立即采集一个 feed 并触发标注，供调度与 API 手动刷新共用。
// TODO: 按 feed.UpdatedAt + IntervalMin 判断到期，目前简化为全量刷新。
func (s *Scheduler) RefreshFeed(ctx context.Context, feedID int64) error {
	feeds, err := s.store.ListFeeds()
	if err != nil {
		return err
	}
	var feed *model.Feed
	for i := range feeds {
		if feeds[i].ID == feedID {
			feed = &feeds[i]
			break
		}
	}
	if feed == nil {
		return nil
	}
	c, err := collector.Get(feed.Collector)
	if err != nil {
		return err
	}
	cfg := map[string]string{}
	_ = json.Unmarshal([]byte(feed.Config), &cfg)
	items, err := c.Collect(ctx, collector.Source{FeedID: feed.ID, URL: feed.URL, Config: cfg})
	if err != nil {
		return err
	}
	// 入库前过滤: 超龄不收,超量只收最近 N 条(每源 max_items 优先于全局默认)
	items = filterIngest(items, s.ingestLimits(feed), time.Now())
	n, err := s.store.InsertItems(items)
	if err != nil {
		return err
	}
	if n > 0 {
		if _, err := s.pipeline.RunOnce(200); err != nil {
			log.Printf("scheduler: pipeline: %v", err)
		}
	}
	log.Printf("feed %d(%s): %d new items", feed.ID, feed.Title, n)
	return nil
}

// 采集入库默认限制(可在设置页调整,0 = 不限)。
// 首次订阅整站 RSS 时往往带回全量历史,不设限会一次灌入上千条(如 OpenAI Blog)。
const (
	DefaultIngestMaxItems   = 50 // 每次采集最多收最近 N 条
	DefaultIngestMaxAgeDays = 30 // 只收最近 X 天发布的
)

type ingestLimits struct {
	maxItems   int
	maxAgeDays int
}

// ingestLimits 解析生效的入库限制: 每源 max_items 优先,未设则用全局设置,再缺省用默认值。
func (s *Scheduler) ingestLimits(feed *model.Feed) ingestLimits {
	l := ingestLimits{maxItems: DefaultIngestMaxItems, maxAgeDays: DefaultIngestMaxAgeDays}
	if v, ok, err := s.store.GetSetting("ingest_max_items"); err == nil && ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			l.maxItems = n
		}
	}
	if v, ok, err := s.store.GetSetting("ingest_max_age_days"); err == nil && ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			l.maxAgeDays = n
		}
	}
	if feed.MaxItems > 0 {
		l.maxItems = feed.MaxItems
	}
	return l
}

// filterIngest 入库前过滤(纯函数,不改输入顺序):
//   - 超龄丢弃(maxAgeDays > 0 时,发布时间早于 now-N 天的不收;无发布时间的保留);
//   - 按发布时间降序取前 maxItems 条(maxItems <= 0 不限)。
//
// 只影响新拉取,已入库数据不动(存量用归档管理)。
func filterIngest(items []model.Item, l ingestLimits, now time.Time) []model.Item {
	var cutoff time.Time
	if l.maxAgeDays > 0 {
		cutoff = now.AddDate(0, 0, -l.maxAgeDays)
	}
	kept := make([]model.Item, 0, len(items))
	for _, it := range items {
		if l.maxAgeDays > 0 && it.PublishedAt != nil && it.PublishedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, it)
	}
	if l.maxItems > 0 && len(kept) > l.maxItems {
		sort.SliceStable(kept, func(i, j int) bool {
			pi, pj := kept[i].PublishedAt, kept[j].PublishedAt
			if pi == nil {
				return false // 无发布时间的排最后
			}
			if pj == nil {
				return true
			}
			return pi.After(*pj)
		})
		kept = kept[:l.maxItems]
	}
	return kept
}

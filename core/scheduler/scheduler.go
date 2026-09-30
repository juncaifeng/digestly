// Package scheduler 按 feed 配置的间隔周期采集，并驱动标注管线。
package scheduler

import (
	"context"
	"encoding/json"
	"log"
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

package store

import (
	"time"

	"github.com/juncaifeng/digestly/core/model"
)

// AddSource 挂载一个第三方市场源(按 url 去重),返回其 id。
func (s *Store) AddSource(url, title string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`INSERT INTO market_sources(url,title) VALUES(?,?)
		ON CONFLICT(url) DO UPDATE SET title=excluded.title RETURNING id`, url, title).Scan(&id)
	return id, err
}

// ListSources 列出已挂载市场源。
func (s *Store) ListSources() ([]model.MarketSource, error) {
	rows, err := s.db.Query(`SELECT id,url,title,enabled,feed_count,last_fetch,created_at
		FROM market_sources ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MarketSource{}
	for rows.Next() {
		var m model.MarketSource
		var enabled int
		var lastFetch *time.Time
		if err := rows.Scan(&m.ID, &m.URL, &m.Title, &enabled, &m.FeedCount, &lastFetch, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Enabled = enabled == 1
		if lastFetch != nil {
			m.LastFetch = *lastFetch
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteSource 卸载市场源。
func (s *Store) DeleteSource(id int64) error {
	_, err := s.db.Exec(`DELETE FROM market_sources WHERE id=?`, id)
	return err
}

// TouchSource 回写市场源解析结果(条目数与时间)。
func (s *Store) TouchSource(id int64, feedCount int) error {
	_, err := s.db.Exec(`UPDATE market_sources SET feed_count=?,last_fetch=? WHERE id=?`,
		feedCount, time.Now().UTC(), id)
	return err
}

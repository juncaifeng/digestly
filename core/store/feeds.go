package store

import "github.com/juncaifeng/digestly/core/model"

func (s *Store) AddFeed(f *model.Feed) error {
	res, err := s.db.Exec(
		`INSERT INTO feeds(title,url,collector,config,interval_min) VALUES(?,?,?,?,?)`,
		f.Title, f.URL, f.Collector, f.Config, f.IntervalMin)
	if err != nil {
		return err
	}
	f.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) DeleteFeed(id int64) error {
	_, err := s.db.Exec(`DELETE FROM feeds WHERE id=?`, id)
	return err
}

func (s *Store) ListFeeds() ([]model.Feed, error) {
	rows, err := s.db.Query(
		`SELECT id,title,url,collector,config,interval_min,created_at,updated_at FROM feeds ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Feed
	for rows.Next() {
		var f model.Feed
		if err := rows.Scan(&f.ID, &f.Title, &f.URL, &f.Collector, &f.Config,
			&f.IntervalMin, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SetMarketVersion 记录市场脚本安装版本。
func (s *Store) SetMarketVersion(name string, version int) error {
	_, err := s.db.Exec(`INSERT INTO market_state(name,script_version) VALUES(?,?)
		ON CONFLICT(name) DO UPDATE SET script_version=excluded.script_version,
		installed_at=CURRENT_TIMESTAMP`, name, version)
	return err
}

// MarketVersions 取所有已安装脚本版本。
func (s *Store) MarketVersions() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT name,script_version FROM market_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var n string
		var v int
		if err := rows.Scan(&n, &v); err != nil {
			return nil, err
		}
		out[n] = v
	}
	return out, rows.Err()
}

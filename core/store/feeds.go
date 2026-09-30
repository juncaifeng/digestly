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

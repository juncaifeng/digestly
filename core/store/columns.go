package store

import (
	"fmt"

	"github.com/juncaifeng/digestly/core/model"
)

// AddColumn 新建专栏,返回 id。
func (s *Store) AddColumn(name string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO columns(name) VALUES(?)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RenameColumn 重命名专栏。
func (s *Store) RenameColumn(id int64, name string) error {
	res, err := s.db.Exec(`UPDATE columns SET name=? WHERE id=?`, name, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("专栏 %d 不存在", id)
	}
	return nil
}

// DeleteColumn 删除专栏: 组内订阅回未分组(ON DELETE SET NULL),
// 手动收藏级联清除并同步 L1。
func (s *Store) DeleteColumn(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM columns WHERE id=?`, id); err != nil {
		return err
	}
	s.index.mu.Lock()
	for fid, cid := range s.index.feedCols {
		if cid == id {
			delete(s.index.feedCols, fid)
		}
	}
	for _, m := range s.index.items {
		out := m.favCols[:0]
		for _, c := range m.favCols {
			if c != id {
				out = append(out, c)
			}
		}
		m.favCols = out
	}
	s.index.mu.Unlock()
	return nil
}

// ListColumns 列出全部专栏(含组内订阅数与收藏文章数)。
func (s *Store) ListColumns() ([]model.Column, error) {
	rows, err := s.db.Query(`SELECT c.id,c.name,c.sort,c.created_at,
		(SELECT COUNT(*) FROM feeds f WHERE f.column_id=c.id),
		(SELECT COUNT(*) FROM column_items ci WHERE ci.column_id=c.id)
		FROM columns c ORDER BY c.sort, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Column{}
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt, &c.FeedCount, &c.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindColumn 按名称或 id 找到专栏 id(CLI 用);0 表示不限。
func (s *Store) FindColumn(nameOrID string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(nameOrID, "%d", &id); err == nil && id > 0 {
		return id, nil
	}
	cols, err := s.ListColumns()
	if err != nil {
		return 0, err
	}
	for _, c := range cols {
		if c.Name == nameOrID {
			return c.ID, nil
		}
	}
	return 0, fmt.Errorf("找不到专栏 %q", nameOrID)
}

// SetFeedColumn 把订阅移入专栏(columnID 为 nil 表示移回未分组)。
func (s *Store) SetFeedColumn(feedID int64, columnID *int64) error {
	res, err := s.db.Exec(`UPDATE feeds SET column_id=? WHERE id=?`, columnID, feedID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("订阅源 %d 不存在", feedID)
	}
	s.index.mu.Lock()
	if columnID == nil {
		delete(s.index.feedCols, feedID)
	} else {
		s.index.feedCols[feedID] = *columnID
	}
	s.index.mu.Unlock()
	return nil
}

// FavoriteItem 把文章手动收藏进专栏(write-through)。
func (s *Store) FavoriteItem(columnID, itemID int64) error {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO column_items(column_id,item_id) VALUES(?,?)`,
		columnID, itemID); err != nil {
		return err
	}
	s.index.update(itemID, func(m *itemMeta) {
		if !hasCol(m.favCols, columnID) {
			m.favCols = append(m.favCols, columnID)
		}
	})
	return nil
}

// UnfavoriteItem 取消收藏(write-through)。
func (s *Store) UnfavoriteItem(columnID, itemID int64) error {
	if _, err := s.db.Exec(`DELETE FROM column_items WHERE column_id=? AND item_id=?`,
		columnID, itemID); err != nil {
		return err
	}
	s.index.update(itemID, func(m *itemMeta) {
		out := m.favCols[:0]
		for _, c := range m.favCols {
			if c != columnID {
				out = append(out, c)
			}
		}
		m.favCols = out
	})
	return nil
}

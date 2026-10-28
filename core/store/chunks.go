package store

// ItemChunk 文章的一个滑动窗口块及其块摘要。
type ItemChunk struct {
	ID      int64  `json:"id"`
	ItemID  int64  `json:"item_id"`
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Summary string `json:"summary"`
}

// ReplaceItemChunks 覆盖写一篇文章的全部分块摘要(重跑标注时先清后写)。
func (s *Store) ReplaceItemChunks(itemID int64, chunks []ItemChunk) error {
	if _, err := s.db.Exec(`DELETE FROM item_chunks WHERE item_id=?`, itemID); err != nil {
		return err
	}
	for i, c := range chunks {
		if _, err := s.db.Exec(`INSERT INTO item_chunks(item_id,idx,text,summary) VALUES(?,?,?,?)`,
			itemID, i, c.Text, c.Summary); err != nil {
			return err
		}
	}
	return nil
}

// ItemChunks 取一篇文章的分块摘要(按序)。
func (s *Store) ItemChunks(itemID int64) ([]ItemChunk, error) {
	rows, err := s.db.Query(`SELECT id,item_id,idx,text,summary FROM item_chunks WHERE item_id=? ORDER BY idx`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ItemChunk{}
	for rows.Next() {
		var c ItemChunk
		if err := rows.Scan(&c.ID, &c.ItemID, &c.Index, &c.Text, &c.Summary); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

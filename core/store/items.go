package store

import (
	"database/sql"
	"sort"
	"strings"

	"github.com/juncaifeng/digestly/core/model"
)

// InsertItems 批量写入，按 (feed_id,guid) 去重，返回新插入条数。
// write-through: 落库成功后同步 L1 索引。
func (s *Store) InsertItems(items []model.Item) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO items
		(feed_id,guid,title,link,author,content,published_at,status) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var inserted []model.Item
	for _, it := range items {
		res, err := stmt.Exec(it.FeedID, it.GUID, it.Title, it.Link, it.Author,
			it.Content, it.PublishedAt, model.StatusPending)
		if err != nil {
			return len(inserted), err
		}
		if aff, _ := res.RowsAffected(); aff > 0 {
			it.ID, _ = res.LastInsertId()
			it.Status = model.StatusPending
			it.Content = "" // L1 不持有正文
			inserted = append(inserted, it)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	for _, it := range inserted {
		s.index.add(&itemMeta{item: it})
	}
	return len(inserted), nil
}

// ItemFilter 列表查询条件。
type ItemFilter struct {
	FeedID int64
	Status model.ItemStatus
	Tag    string
	Query  string // FTS5 全文搜索(走 SQL;其余条件走 L1 内存索引)
	Unread bool
	Limit  int
	Offset int
}

// ListItems 列表查询。不返回正文(详情走 GetItem / L2 LRU)。
// 无 Query 时纯内存完成;有 Query 时走 FTS5,但同样不取正文。
func (s *Store) ListItems(f ItemFilter) ([]model.Item, error) {
	if f.Query == "" {
		return s.index.filter(f), nil
	}
	return s.searchItems(f)
}

// searchItems FTS5 搜索路径(无法走内存索引的唯一场景)。
func (s *Store) searchItems(f ItemFilter) ([]model.Item, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT i.id,i.feed_id,i.guid,i.title,i.link,i.author,
		i.summary,i.keywords,i.source,i.published_at,i.status,i.read,i.created_at
		FROM items i JOIN items_fts ft ON ft.rowid = i.id AND items_fts MATCH ?`)
	args = append(args, quoteFTS(f.Query))
	if f.Tag != "" {
		sb.WriteString(` JOIN item_tags it ON it.item_id=i.id
			JOIN tags t ON t.id=it.tag_id AND t.name=?`)
		args = append(args, f.Tag)
	}
	sb.WriteString(` WHERE 1=1`)
	if f.FeedID > 0 {
		sb.WriteString(` AND i.feed_id=?`)
		args = append(args, f.FeedID)
	}
	if f.Status != "" {
		sb.WriteString(` AND i.status=?`)
		args = append(args, f.Status)
	}
	if f.Unread {
		sb.WriteString(` AND i.read=0`)
	}
	sb.WriteString(` ORDER BY i.published_at DESC, i.id DESC`)
	if f.Limit <= 0 {
		f.Limit = 50
	}
	sb.WriteString(` LIMIT ? OFFSET ?`)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link,
			&it.Author, &it.Summary, &it.Keywords, &it.Source,
			&it.PublishedAt, &it.Status, &it.Read, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 标签从 L1 索引取,避免 N+1
	s.index.mu.RLock()
	for i := range out {
		if m, ok := s.index.items[out[i].ID]; ok {
			out[i].Tags = append([]string{}, m.tags...)
		}
	}
	s.index.mu.RUnlock()
	return out, nil
}

// GetItem 取单篇完整条目(含正文)。正文走 L2 LRU,miss 时回源 SQLite 并回填。
func (s *Store) GetItem(id int64) (*model.Item, error) {
	s.index.mu.RLock()
	m, ok := s.index.items[id]
	if !ok {
		s.index.mu.RUnlock()
		return nil, sql.ErrNoRows
	}
	it := m.item
	it.Tags = append([]string{}, m.tags...)
	s.index.mu.RUnlock()

	if content, hit := s.content.Get(id); hit {
		it.Content = content
		return &it, nil
	}
	var content string
	if err := s.db.QueryRow(`SELECT content FROM items WHERE id=?`, id).Scan(&content); err != nil {
		return nil, err
	}
	s.content.Add(id, content)
	it.Content = content
	return &it, nil
}

// SetEnrichment 标注管线回写：摘要/关键词/来源 + 状态推进。
func (s *Store) SetEnrichment(id int64, summary, keywords, source string, status model.ItemStatus) error {
	if _, err := s.db.Exec(
		`UPDATE items SET summary=?,keywords=?,source=?,status=? WHERE id=?`,
		summary, keywords, source, status, id); err != nil {
		return err
	}
	s.index.update(id, func(m *itemMeta) {
		m.item.Summary = summary
		m.item.Keywords = keywords
		m.item.Source = source
		m.item.Status = status
	})
	return nil
}

// PendingItems 取待标注文章(含正文,管线需要;批量作业不走缓存)。
func (s *Store) PendingItems(limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,feed_id,guid,title,link,author,content,
		summary,keywords,source,published_at,status,read,created_at
		FROM items WHERE status=? ORDER BY id LIMIT ?`, model.StatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link,
			&it.Author, &it.Content, &it.Summary, &it.Keywords, &it.Source,
			&it.PublishedAt, &it.Status, &it.Read, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) MarkRead(id int64, read bool) error {
	if _, err := s.db.Exec(`UPDATE items SET read=? WHERE id=?`, read, id); err != nil {
		return err
	}
	s.index.update(id, func(m *itemMeta) { m.item.Read = read })
	return nil
}

// AddTag 给文章打标签(用户手工或管线规则)。
func (s *Store) AddTag(itemID int64, name string) error {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO tags(name) VALUES(?)`, name); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO item_tags(item_id,tag_id)
		SELECT ?, id FROM tags WHERE name=?`, itemID, name); err != nil {
		return err
	}
	s.index.update(itemID, func(m *itemMeta) {
		if !hasTag(m.tags, name) {
			m.tags = append(m.tags, name)
			sort.Strings(m.tags) // 与 SQL ORDER BY name 一致
		}
	})
	return nil
}

// ItemTags 取文章标签(优先 L1;索引未命中时回源 SQL)。
func (s *Store) ItemTags(itemID int64) ([]string, error) {
	s.index.mu.RLock()
	if m, ok := s.index.items[itemID]; ok {
		out := append([]string{}, m.tags...)
		s.index.mu.RUnlock()
		return out, nil
	}
	s.index.mu.RUnlock()
	rows, err := s.db.Query(`SELECT t.name FROM tags t
		JOIN item_tags it ON it.tag_id=t.id WHERE it.item_id=? ORDER BY t.name`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// quoteFTS 把用户输入包成 FTS5 短语查询，避免语法字符注入。
func quoteFTS(q string) string {
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

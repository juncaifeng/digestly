package store

import (
	"database/sql"
	"strings"

	"github.com/juncaifeng/digestly/core/model"
)

// InsertItems 批量写入，按 (feed_id,guid) 去重，返回新插入条数。
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
	n := 0
	for _, it := range items {
		res, err := stmt.Exec(it.FeedID, it.GUID, it.Title, it.Link, it.Author,
			it.Content, it.PublishedAt, model.StatusPending)
		if err != nil {
			return n, err
		}
		if aff, _ := res.RowsAffected(); aff > 0 {
			n++
		}
	}
	return n, tx.Commit()
}

// ItemFilter 列表查询条件。
type ItemFilter struct {
	FeedID int64
	Status model.ItemStatus
	Tag    string
	Query  string // FTS5 全文搜索
	Unread bool
	Limit  int
	Offset int
}

func (s *Store) ListItems(f ItemFilter) ([]model.Item, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT i.id,i.feed_id,i.guid,i.title,i.link,i.author,i.content,
		i.summary,i.keywords,i.source,i.published_at,i.status,i.read,i.created_at
		FROM items i`)
	if f.Query != "" {
		sb.WriteString(` JOIN items_fts ft ON ft.rowid = i.id AND items_fts MATCH ?`)
		args = append(args, quoteFTS(f.Query))
	}
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
	return scanItems(rows)
}

func scanItems(rows *sql.Rows) ([]model.Item, error) {
	var out []model.Item
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

// SetEnrichment 标注管线回写：摘要/关键词/来源 + 状态推进。
func (s *Store) SetEnrichment(id int64, summary, keywords, source string, status model.ItemStatus) error {
	_, err := s.db.Exec(
		`UPDATE items SET summary=?,keywords=?,source=?,status=? WHERE id=?`,
		summary, keywords, source, status, id)
	return err
}

// PendingItems 取待标注文章。
func (s *Store) PendingItems(limit int) ([]model.Item, error) {
	return s.ListItems(ItemFilter{Status: model.StatusPending, Limit: limit})
}

func (s *Store) MarkRead(id int64, read bool) error {
	_, err := s.db.Exec(`UPDATE items SET read=? WHERE id=?`, read, id)
	return err
}

// AddTag 给文章打标签(用户手工或管线规则)。
func (s *Store) AddTag(itemID int64, name string) error {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO tags(name) VALUES(?)`, name); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO item_tags(item_id,tag_id)
		SELECT ?, id FROM tags WHERE name=?`, itemID, name)
	return err
}

func (s *Store) ItemTags(itemID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT t.name FROM tags t
		JOIN item_tags it ON it.tag_id=t.id WHERE it.item_id=? ORDER BY t.name`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
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

package store

import (
	"database/sql"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	FeedID   int64
	ColumnID int64 // 专栏: 订阅继承 ∪ 手动收藏
	Status   model.ItemStatus
	Tag      string
	Query    string // FTS5 全文搜索(走 SQL;其余条件走 L1 内存索引)
	Unread   bool
	Archived bool // false = 只看未归档(默认); true = 只看已归档
	Since    *time.Time // 发布时间下限(含)
	Until    *time.Time // 发布时间上限(不含);无发布时间的条目按 created_at 兜底
	Limit    int
	Offset   int
}

// inRange 发布时间区间判断;无 published_at 的条目用 created_at 参与比较。
func inRange(it *model.Item, since, until *time.Time) bool {
	t := it.PublishedAt
	if t == nil {
		tt := it.CreatedAt
		t = &tt
	}
	if since != nil && t.Before(*since) {
		return false
	}
	if until != nil && !t.Before(*until) {
		return false
	}
	return true
}

// ListItems 列表查询。不返回正文(详情走 GetItem / L2 LRU)。
// 无 Query 时纯内存完成;有 Query 时走 FTS5,但同样不取正文。
// 短查询(<3 字符,如 "AI"、"Go")trigram 分词无效,自动降级为 LIKE 子串匹配。
func (s *Store) ListItems(f ItemFilter) ([]model.Item, error) {
	if f.Query == "" {
		return s.index.filter(f), nil
	}
	if utf8.RuneCountInString(strings.TrimSpace(f.Query)) < 3 {
		return s.likeItems(f)
	}
	return s.searchItems(f)
}

// likeItems 短查询降级路径: LIKE 匹配标题/摘要/关键词(避开 FTS trigram 的 3 字符限制)。
func (s *Store) likeItems(f ItemFilter) ([]model.Item, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT id,feed_id,guid,title,link,author,summary,keywords,source,
		published_at,status,read,archived,created_at FROM items WHERE archived=? AND (title LIKE ? OR summary LIKE ? OR keywords LIKE ?)`)
	like := "%" + f.Query + "%"
	args = append(args, f.Archived, like, like, like)
	if f.FeedID > 0 {
		sb.WriteString(` AND feed_id=?`)
		args = append(args, f.FeedID)
	}
	if f.ColumnID > 0 {
		sb.WriteString(` AND (feed_id IN (SELECT id FROM feeds WHERE column_id=?)
			OR id IN (SELECT item_id FROM column_items WHERE column_id=?))`)
		args = append(args, f.ColumnID, f.ColumnID)
	}
	if f.Status != "" {
		sb.WriteString(` AND status=?`)
		args = append(args, f.Status)
	}
	if f.Unread {
		sb.WriteString(` AND read=0`)
	}
	if f.Since != nil {
		sb.WriteString(` AND COALESCE(published_at,created_at) >= ?`)
		args = append(args, f.Since.UTC())
	}
	if f.Until != nil {
		sb.WriteString(` AND COALESCE(published_at,created_at) < ?`)
		args = append(args, f.Until.UTC())
	}
	sb.WriteString(` ORDER BY published_at DESC, id DESC LIMIT ? OFFSET ?`)
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link, &it.Author,
			&it.Summary, &it.Keywords, &it.Source, &it.PublishedAt, &it.Status,
			&it.Read, &it.Archived, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.index.mu.RLock()
	for i := range out {
		if m, ok := s.index.items[out[i].ID]; ok {
			out[i].Tags = append([]string{}, m.tags...)
		}
	}
	s.index.mu.RUnlock()
	return out, nil
}

// searchItems FTS5 搜索路径(无法走内存索引的唯一场景)。
func (s *Store) searchItems(f ItemFilter) ([]model.Item, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT i.id,i.feed_id,i.guid,i.title,i.link,i.author,
		i.summary,i.keywords,i.source,i.published_at,i.status,i.read,i.archived,i.created_at
		FROM items i JOIN items_fts ft ON ft.rowid = i.id AND items_fts MATCH ?`)
	args = append(args, quoteFTS(f.Query))
	if f.Tag != "" {
		sb.WriteString(` JOIN item_tags it ON it.item_id=i.id
			JOIN tags t ON t.id=it.tag_id AND t.name=?`)
		args = append(args, f.Tag)
	}
	sb.WriteString(` WHERE i.archived=?`)
	args = append(args, f.Archived)
	if f.FeedID > 0 {
		sb.WriteString(` AND i.feed_id=?`)
		args = append(args, f.FeedID)
	}
	if f.ColumnID > 0 {
		sb.WriteString(` AND (i.feed_id IN (SELECT id FROM feeds WHERE column_id=?)
			OR i.id IN (SELECT item_id FROM column_items WHERE column_id=?))`)
		args = append(args, f.ColumnID, f.ColumnID)
	}
	if f.Status != "" {
		sb.WriteString(` AND i.status=?`)
		args = append(args, f.Status)
	}
	if f.Unread {
		sb.WriteString(` AND i.read=0`)
	}
	if f.Since != nil {
		sb.WriteString(` AND COALESCE(i.published_at,i.created_at) >= ?`)
		args = append(args, f.Since.UTC())
	}
	if f.Until != nil {
		sb.WriteString(` AND COALESCE(i.published_at,i.created_at) < ?`)
		args = append(args, f.Until.UTC())
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
			&it.PublishedAt, &it.Status, &it.Read, &it.Archived, &it.CreatedAt); err != nil {
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
	it.ColumnIDs = append([]int64{}, m.favCols...)
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

// SetEnrichment 标注管线回写：摘要/关键词/来源 + 状态推进(同时清除认领标记)。
func (s *Store) SetEnrichment(id int64, summary, keywords, source string, status model.ItemStatus) error {
	if _, err := s.db.Exec(
		`UPDATE items SET summary=?,keywords=?,source=?,status=?,claimed_at=NULL WHERE id=?`,
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

// ResetEnrichment 把已标注文章退回 pending(管线升级后重跑用),返回重置条数。
func (s *Store) ResetEnrichment(all bool) (int, error) {
	q := `UPDATE items SET status='pending',summary='',keywords='',source='',claimed_at=NULL WHERE status='processed'`
	if all {
		q = `UPDATE items SET status='pending',summary='',keywords='',source='',claimed_at=NULL WHERE status!='pending'`
	}
	res, err := s.db.Exec(q)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	// 同步 L1 索引
	s.index.mu.Lock()
	for _, m := range s.index.items {
		if m.item.Status != model.StatusPending {
			m.item.Status = model.StatusPending
			m.item.Summary, m.item.Keywords, m.item.Source = "", "", ""
		}
	}
	s.index.mu.Unlock()
	return int(n), nil
}

// RetryFailed 把标注失败的文章退回 pending(已归档的除外),返回重置条数。
func (s *Store) RetryFailed() (int, error) {
	res, err := s.db.Exec(
		`UPDATE items SET status='pending',summary='',keywords='',source='',claimed_at=NULL WHERE status='failed' AND archived=0`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	// 同步 L1 索引(与 SQL 同条件: 跳过已归档)
	s.index.mu.Lock()
	for _, m := range s.index.items {
		if m.item.Status == model.StatusFailed && !m.item.Archived {
			m.item.Status = model.StatusPending
			m.item.Summary, m.item.Keywords, m.item.Source = "", "", ""
		}
	}
	s.index.mu.Unlock()
	return int(n), nil
}

// RetryItem 把单篇退回 pending(单篇重跑标注用)。文章不存在时返回 sql.ErrNoRows。
func (s *Store) RetryItem(id int64) error {
	res, err := s.db.Exec(
		`UPDATE items SET status='pending',summary='',keywords='',source='',claimed_at=NULL WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	s.index.update(id, func(m *itemMeta) {
		m.item.Status = model.StatusPending
		m.item.Summary, m.item.Keywords, m.item.Source = "", "", ""
	})
	return nil
}

// PendingItems 取待标注文章(含正文,管线需要;批量作业不走缓存)。已归档的不参与标注。
func (s *Store) PendingItems(limit int) ([]model.Item, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,feed_id,guid,title,link,author,content,
		summary,keywords,source,published_at,status,read,archived,created_at
		FROM items WHERE status=? AND archived=0 ORDER BY id LIMIT ?`, model.StatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link,
			&it.Author, &it.Content, &it.Summary, &it.Keywords, &it.Source,
			&it.PublishedAt, &it.Status, &it.Read, &it.Archived, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ClaimPending 认领一批待处理文章并置为 processing(带 claimed_at 时间戳):
// 取 status=pending,以及 status=processing 但认领超时(疑似处理端崩溃)的。
// 返回认领到的文章;调用方处理完用 SetEnrichment 落终态(会清认领标记)。
func (s *Store) ClaimPending(limit int, staleAfter time.Duration) ([]model.Item, error) {
	if limit <= 0 {
		limit = 50
	}
	cutoff := time.Now().Add(-staleAfter)
	rows, err := s.db.Query(`UPDATE items SET status=?, claimed_at=CURRENT_TIMESTAMP
		WHERE id IN (SELECT id FROM items WHERE archived=0 AND (status=? OR (status=? AND (claimed_at IS NULL OR claimed_at < ?)))
			ORDER BY id LIMIT ?)
		RETURNING id,feed_id,guid,title,link,author,content,
		summary,keywords,source,published_at,status,read,archived,created_at`,
		model.StatusProcessing, model.StatusPending, model.StatusProcessing, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link,
			&it.Author, &it.Content, &it.Summary, &it.Keywords, &it.Source,
			&it.PublishedAt, &it.Status, &it.Read, &it.Archived, &it.CreatedAt); err != nil {
			return nil, err
		}
		s.index.update(it.ID, func(m *itemMeta) { m.item.Status = model.StatusProcessing })
		out = append(out, it)
	}
	return out, rows.Err()
}

// SetArchived 归档/取消归档单篇文章。
func (s *Store) SetArchived(id int64, archived bool) error {
	if _, err := s.db.Exec(`UPDATE items SET archived=? WHERE id=?`, archived, id); err != nil {
		return err
	}
	s.index.update(id, func(m *itemMeta) { m.item.Archived = archived })
	return nil
}

// ArchiveFeedItems 批量归档一个 feed 的文章;onlyRead 为 true 时只归档已读。
// 返回归档条数。
func (s *Store) ArchiveFeedItems(feedID int64, onlyRead bool) (int, error) {
	q := `UPDATE items SET archived=1 WHERE feed_id=? AND archived=0`
	args := []any{feedID}
	if onlyRead {
		q += ` AND read=1`
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	s.index.mu.Lock()
	for _, m := range s.index.items {
		if m.item.FeedID == feedID && !m.item.Archived && (!onlyRead || m.item.Read) {
			m.item.Archived = true
		}
	}
	s.index.mu.Unlock()
	return int(n), nil
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

// ItemStatusCounts 未归档文章按标注状态计数(pending/processing/processed/failed)。
func (s *Store) ItemStatusCounts() (map[model.ItemStatus]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM items WHERE archived=0 GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[model.ItemStatus]int{}
	for rows.Next() {
		var st model.ItemStatus
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

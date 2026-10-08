// Package store 提供 SQLite 持久化，含 FTS5 全文索引与标签。
// 使用 modernc.org/sqlite(pure Go)，便于桌面 sidecar 与 gomobile 交叉编译。
//
// 缓存分层: L1 条目元数据常驻内存(列表/过滤零 SQL) + L2 正文 LRU(按字节限额)。
// 所有写操作 write-through: 先落 SQLite(唯一真相源),再同步内存结构。
package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// DefaultContentCacheBytes 正文 LRU 默认限额 64MB。
const DefaultContentCacheBytes = 64 << 20

type Store struct {
	db      *sql.DB
	index   *itemIndex
	content *contentCache
}

// Open 打开(必要时创建)数据库并执行迁移,然后预热 L1 索引。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, index: newItemIndex(), content: newContentCache(DefaultContentCacheBytes)}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.warmIndex(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// warmIndex 启动时把全部条目元数据(不含正文)与标签载入 L1。
func (s *Store) warmIndex() error {
	rows, err := s.db.Query(`SELECT id,feed_id,guid,title,link,author,
		summary,keywords,source,published_at,status,read,created_at FROM items`)
	if err != nil {
		return err
	}
	var metas []*itemMeta
	for rows.Next() {
		m := &itemMeta{}
		if err := rows.Scan(&m.item.ID, &m.item.FeedID, &m.item.GUID, &m.item.Title,
			&m.item.Link, &m.item.Author, &m.item.Summary, &m.item.Keywords,
			&m.item.Source, &m.item.PublishedAt, &m.item.Status, &m.item.Read,
			&m.item.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	tagRows, err := s.db.Query(`SELECT it.item_id, t.name FROM item_tags it
		JOIN tags t ON t.id=it.tag_id ORDER BY t.name`)
	if err != nil {
		return err
	}
	tagsByItem := map[int64][]string{}
	for tagRows.Next() {
		var id int64
		var name string
		if err := tagRows.Scan(&id, &name); err != nil {
			tagRows.Close()
			return err
		}
		tagsByItem[id] = append(tagsByItem[id], name)
	}
	tagRows.Close()

	// 直接按排序规则插入(add 内部有序)
	for _, m := range metas {
		m.tags = tagsByItem[m.item.ID]
		s.index.add(m)
	}
	return nil
}

// CacheStats 返回缓存观测指标: L1 条目数, L2 缓存条数与字节数。
func (s *Store) CacheStats() (items int, contentEntries int, contentBytes int64) {
	s.index.mu.RLock()
	items = len(s.index.order)
	s.index.mu.RUnlock()
	contentEntries, contentBytes = s.content.Stats()
	return
}

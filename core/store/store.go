// Package store 提供 SQLite 持久化，含 FTS5 全文索引与标签。
// 使用 modernc.org/sqlite(pure Go)，便于桌面 sidecar 与 gomobile 交叉编译。
//
// 缓存分层: L1 条目元数据常驻内存(列表/过滤零 SQL) + L2 正文 LRU(按字节限额)。
// 所有写操作 write-through: 先落 SQLite(唯一真相源),再同步内存结构。
package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// DefaultContentCacheBytes 正文 LRU 默认限额 64MB。
const DefaultContentCacheBytes = 64 << 20

type Store struct {
	db      *sql.DB
	index   *itemIndex
	content *contentCache
	Path    string // 数据库文件路径(供 CLI/工具定位)
}

// Open 打开(必要时创建)数据库并执行迁移,然后预热 L1 索引。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, index: newItemIndex(), content: newContentCache(DefaultContentCacheBytes), Path: path}
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

// OpenReadOnly 以只读方式打开(不跑迁移),CLI 查询用。
// 与服务端并发读安全(SQLite WAL)。
func OpenReadOnly(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("数据库不存在: %s", path)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, index: newItemIndex(), content: newContentCache(DefaultContentCacheBytes), Path: path}
	if err := s.warmIndex(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrate 基础建表 + 旧库增量补列(CREATE-IF-NOT-EXISTS 不会给旧表加列)。
func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	var n int
	// feeds.column_id(专栏分组)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name='column_id'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE feeds ADD COLUMN column_id INTEGER REFERENCES columns(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	// feeds.max_items(单源历史上限)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name='max_items'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE feeds ADD COLUMN max_items INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// items.archived(归档)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name='archived'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE items ADD COLUMN archived INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// items.source(来源元数据;旧库可能缺)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name='source'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE items ADD COLUMN source TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// warmIndex 启动时把全部条目元数据(不含正文)与标签载入 L1。
func (s *Store) warmIndex() error {
	rows, err := s.db.Query(`SELECT id,feed_id,guid,title,link,author,
		source,published_at,read,archived,created_at FROM items`)
	if err != nil {
		return err
	}
	var metas []*itemMeta
	for rows.Next() {
		m := &itemMeta{}
		if err := rows.Scan(&m.item.ID, &m.item.FeedID, &m.item.GUID, &m.item.Title,
			&m.item.Link, &m.item.Author, &m.item.Source, &m.item.PublishedAt,
			&m.item.Read, &m.item.Archived, &m.item.CreatedAt); err != nil {
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

	// 订阅 -> 专栏 映射(专栏过滤的继承通道)。
	// 只读打开的旧库可能还没有 column_id 列,探测后缺失则跳过(继承通道为空)。
	var hasCol int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name='column_id'`).Scan(&hasCol); err != nil {
		return err
	}
	if hasCol > 0 {
		fcRows, err := s.db.Query(`SELECT id,column_id FROM feeds WHERE column_id IS NOT NULL`)
		if err != nil {
			return err
		}
		s.index.mu.Lock()
		for fcRows.Next() {
			var fid, cid int64
			if err := fcRows.Scan(&fid, &cid); err != nil {
				fcRows.Close()
				s.index.mu.Unlock()
				return err
			}
			s.index.feedCols[fid] = cid
		}
		s.index.mu.Unlock()
		fcRows.Close()
	}

	// 手动收藏(文章 -> 专栏);只读打开的旧库可能还没有该表,缺失则跳过。
	favByItem := map[int64][]int64{}
	var hasCI int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='column_items'`).Scan(&hasCI); err != nil {
		return err
	}
	if hasCI > 0 {
		ciRows, err := s.db.Query(`SELECT item_id,column_id FROM column_items`)
		if err != nil {
			return err
		}
		for ciRows.Next() {
			var iid, cid int64
			if err := ciRows.Scan(&iid, &cid); err != nil {
				ciRows.Close()
				return err
			}
			favByItem[iid] = append(favByItem[iid], cid)
		}
		ciRows.Close()
	}

	// 直接按排序规则插入(add 内部有序)
	for _, m := range metas {
		m.tags = tagsByItem[m.item.ID]
		m.favCols = favByItem[m.item.ID]
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

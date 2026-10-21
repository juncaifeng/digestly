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
}

// Open 打开(必要时创建)数据库并执行迁移,然后预热 L1 索引。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
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
	s := &Store{db: db, index: newItemIndex(), content: newContentCache(DefaultContentCacheBytes)}
	if err := s.warmIndex(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// CREATE-IF-NOT-EXISTS 不会给旧表加列,探测后补 feeds.column_id
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name='column_id'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE feeds ADD COLUMN column_id INTEGER REFERENCES columns(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	// 同样探测补 feeds.max_items(历史上限)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name='max_items'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE feeds ADD COLUMN max_items INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// 同样探测补 items.archived(归档)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name='archived'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE items ADD COLUMN archived INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// 同样探测补 columns.template_id(专栏默认大纲模板)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('columns') WHERE name='template_id'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE columns ADD COLUMN template_id INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// 同样探测补 llm_configs.kind / command(生成后端: 直连 API 或外部 Agent 进程)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('llm_configs') WHERE name='kind'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE llm_configs ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat'`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`ALTER TABLE llm_configs ADD COLUMN command TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	// 同样探测补 articles.user_message(对话式生成的用户指示)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('articles') WHERE name='user_message'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE articles ADD COLUMN user_message TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if err := s.migrateArticlesToCreations(); err != nil {
		return err
	}
	return s.seedBuiltinTemplates()
}

// migrateArticlesToCreations articles 从「属于专栏」升级为「属于创作单元」。
// 旧表(带 column_id 外键)重建为新结构,并为每个有历史文章的专栏自动建同名创作,
// 素材源指向该专栏,模板沿用专栏默认,历史文章保留。
func (s *Store) migrateArticlesToCreations() error {
	var hasOld, hasNew int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('articles') WHERE name='column_id'`).Scan(&hasOld); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('articles') WHERE name='creation_id'`).Scan(&hasNew); err != nil {
		return err
	}
	if hasOld == 0 || hasNew > 0 {
		return nil // 新库或已迁移
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`ALTER TABLE articles RENAME TO articles_old`,
		`CREATE TABLE articles (
		  id          INTEGER PRIMARY KEY AUTOINCREMENT,
		  creation_id INTEGER NOT NULL DEFAULT 0 REFERENCES creations(id) ON DELETE CASCADE,
		  template_id INTEGER NOT NULL DEFAULT 0,
		  title       TEXT NOT NULL DEFAULT '',
		  content     TEXT NOT NULL DEFAULT '',
		  item_ids    TEXT NOT NULL DEFAULT '[]',
		  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	// 每个有历史文章的专栏 -> 同名创作(素材源=该专栏,模板沿用专栏默认)
	colRows, err := tx.Query(`SELECT DISTINCT column_id FROM articles_old WHERE column_id > 0`)
	if err != nil {
		return err
	}
	var colIDs []int64
	for colRows.Next() {
		var id int64
		if err := colRows.Scan(&id); err != nil {
			colRows.Close()
			return err
		}
		colIDs = append(colIDs, id)
	}
	colRows.Close()
	for _, colID := range colIDs {
		var name string
		var tplID int64
		if err := tx.QueryRow(`SELECT name, COALESCE(template_id,0) FROM columns WHERE id=?`, colID).
			Scan(&name, &tplID); err != nil {
			name, tplID = fmt.Sprintf("专栏%d", colID), 0
		}
		res, err := tx.Exec(`INSERT INTO creations(name,template_id) VALUES(?,?)`, name, tplID)
		if err != nil {
			return err
		}
		creationID, _ := res.LastInsertId()
		if _, err := tx.Exec(`INSERT INTO creation_sources(creation_id,kind,ref_id) VALUES(?,'column',?)`,
			creationID, colID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO articles(id,creation_id,template_id,title,content,item_ids,created_at,updated_at)
			SELECT id,?,template_id,title,content,item_ids,created_at,updated_at
			FROM articles_old WHERE column_id=?`, creationID, colID); err != nil {
			return err
		}
	}
	// column_id=0 的历史(不应存在,兜底)直接丢弃
	if _, err := tx.Exec(`DROP TABLE articles_old`); err != nil {
		return err
	}
	return tx.Commit()
}

// seedBuiltinTemplates 写入内置专栏文章模板(INSERT OR IGNORE,用户可自行编辑内容)。
func (s *Store) seedBuiltinTemplates() error {
	seeds := []struct{ name, prompt string }{
		{"话题综述", `你是一名科技编辑。以下是专栏最近未读的文章素材清单(标题 / 摘要 / 来源):
{{items}}

请围绕这些素材写一篇话题综述专栏文章,要求:
1. 自拟一个概括本期主题的标题(第一行,以 "# " 开头)
2. 归纳出 2-4 个共同话题或趋势,分别展开
3. 每个观点注明来自哪些素材(用素材标题指代)
4. 中文撰写,800 字以内`},
		{"科普解读", `你是一名科普作者。以下是专栏最近未读的文章素材清单(标题 / 摘要 / 来源):
{{items}}

请挑选其中最有价值的 1-3 个主题,写一篇科普解读专栏文章,要求:
1. 自拟标题(第一行,以 "# " 开头)
2. 用通俗语言解释核心概念,避免堆砌术语
3. 说明这些内容对普通读者的意义
4. 中文撰写,600 字以内`},
	}
	for _, seed := range seeds {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO article_templates(name,prompt,built_in) VALUES(?,?,1)`,
			seed.name, seed.prompt); err != nil {
			return err
		}
	}
	return nil
}

// warmIndex 启动时把全部条目元数据(不含正文)与标签载入 L1。
func (s *Store) warmIndex() error {
	rows, err := s.db.Query(`SELECT id,feed_id,guid,title,link,author,
		summary,keywords,source,published_at,status,read,archived,created_at FROM items`)
	if err != nil {
		return err
	}
	var metas []*itemMeta
	for rows.Next() {
		m := &itemMeta{}
		if err := rows.Scan(&m.item.ID, &m.item.FeedID, &m.item.GUID, &m.item.Title,
			&m.item.Link, &m.item.Author, &m.item.Summary, &m.item.Keywords,
			&m.item.Source, &m.item.PublishedAt, &m.item.Status, &m.item.Read,
			&m.item.Archived, &m.item.CreatedAt); err != nil {
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

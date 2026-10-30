package store

// schema 只保留职责域内的表: 订阅源、文章、全文索引、标签、专栏、市场源、应用设置。
// 历史库中可能残留已移除功能(创作/管线/向量)的表与列,不再读写,无副作用。
const schema = `
CREATE TABLE IF NOT EXISTS feeds (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL,
  url          TEXT NOT NULL UNIQUE,
  collector    TEXT NOT NULL DEFAULT 'rss',
  config       TEXT NOT NULL DEFAULT '{}',
  interval_min INTEGER NOT NULL DEFAULT 30,
  max_items    INTEGER NOT NULL DEFAULT 0,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- summary/keywords/status/claimed_at 为历史列,保留以兼容旧库与 FTS 触发器,不再写入业务值
CREATE TABLE IF NOT EXISTS items (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  feed_id      INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid         TEXT NOT NULL,
  title        TEXT NOT NULL DEFAULT '',
  link         TEXT NOT NULL DEFAULT '',
  author       TEXT NOT NULL DEFAULT '',
  content      TEXT NOT NULL DEFAULT '',
  summary      TEXT NOT NULL DEFAULT '',
  keywords     TEXT NOT NULL DEFAULT '',
  source       TEXT NOT NULL DEFAULT '',
  published_at DATETIME,
  status       TEXT NOT NULL DEFAULT 'pending',
  claimed_at   DATETIME,
  read         INTEGER NOT NULL DEFAULT 0,
  archived     INTEGER NOT NULL DEFAULT 0,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(feed_id, guid)
);
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
  title, content, summary, keywords,
  content='items', content_rowid='id', tokenize='trigram'
);
CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
  INSERT INTO items_fts(rowid, title, content, summary, keywords)
  VALUES (new.id, new.title, new.content, new.summary, new.keywords);
END;
CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
  INSERT INTO items_fts(items_fts, rowid, title, content, summary, keywords)
  VALUES ('delete', old.id, old.title, old.content, old.summary, old.keywords);
END;
CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
  INSERT INTO items_fts(items_fts, rowid, title, content, summary, keywords)
  VALUES ('delete', old.id, old.title, old.content, old.summary, old.keywords);
  INSERT INTO items_fts(rowid, title, content, summary, keywords)
  VALUES (new.id, new.title, new.content, new.summary, new.keywords);
END;
CREATE TABLE IF NOT EXISTS tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
-- 已挂载的第三方市场源(订阅源清单地址)
CREATE TABLE IF NOT EXISTS market_sources (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  url        TEXT NOT NULL UNIQUE,
  title      TEXT NOT NULL DEFAULT '',
  enabled    INTEGER NOT NULL DEFAULT 1,
  feed_count INTEGER NOT NULL DEFAULT 0,
  last_fetch DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 市场已安装脚本的版本,用于更新检测
CREATE TABLE IF NOT EXISTS market_state (
  name           TEXT PRIMARY KEY,
  script_version INTEGER NOT NULL DEFAULT 1,
  installed_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 通用应用设置(kv)
CREATE TABLE IF NOT EXISTS app_settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS item_tags (
  item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (item_id, tag_id)
);
-- 专栏: 订阅源的文件夹式分组(feeds.column_id 指向,一个订阅只属于一个专栏)
CREATE TABLE IF NOT EXISTS columns (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL UNIQUE,
  sort        INTEGER NOT NULL DEFAULT 0,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 手动收藏: 文章 <-> 专栏(多对多);订阅继承关系不落表,经 feeds.column_id 推导
CREATE TABLE IF NOT EXISTS column_items (
  column_id  INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (column_id, item_id)
);
`

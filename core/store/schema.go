package store

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
CREATE TABLE IF NOT EXISTS item_tags (
  item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (item_id, tag_id)
);
-- 专栏: 订阅源的文件夹式分组(feeds.column_id 指向,一个订阅只属于一个专栏)
-- template_id 关联默认大纲模板(创作中心用)
CREATE TABLE IF NOT EXISTS columns (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL UNIQUE,
  sort        INTEGER NOT NULL DEFAULT 0,
  template_id INTEGER NOT NULL DEFAULT 0,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 手动收藏: 文章 <-> 专栏(多对多);订阅继承关系不落表,经 feeds.column_id 推导
CREATE TABLE IF NOT EXISTS column_items (
  column_id  INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (column_id, item_id)
);
-- LLM 供应商配置(OpenAI 兼容接口)
CREATE TABLE IF NOT EXISTS llm_configs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'chat',
  command     TEXT NOT NULL DEFAULT '',
  base_url    TEXT NOT NULL,
  api_key     TEXT NOT NULL DEFAULT '',
  model       TEXT NOT NULL,
  temperature REAL NOT NULL DEFAULT 0.7,
  max_tokens  INTEGER NOT NULL DEFAULT 4096,
  is_default  INTEGER NOT NULL DEFAULT 0,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 专栏文章生成模板,prompt 中 {{items}} 为素材占位符
CREATE TABLE IF NOT EXISTS article_templates (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL UNIQUE,
  prompt     TEXT NOT NULL,
  built_in   INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- 创作单元(一等公民): 素材源可混合专栏与单个订阅源;group_name 为所属文件夹
CREATE TABLE IF NOT EXISTS creations (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  group_name  TEXT NOT NULL DEFAULT '',
  template_id INTEGER NOT NULL DEFAULT 0,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS creation_sources (
  creation_id INTEGER NOT NULL REFERENCES creations(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL, -- "column" | "feed"
  ref_id      INTEGER NOT NULL,
  PRIMARY KEY (creation_id, kind, ref_id)
);
-- 生成的创作文章历史;creation_id 指向创作单元,item_ids 为消耗的素材 id(JSON 数组)
CREATE TABLE IF NOT EXISTS articles (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  creation_id INTEGER NOT NULL DEFAULT 0 REFERENCES creations(id) ON DELETE CASCADE,
  template_id INTEGER NOT NULL DEFAULT 0,
  title       TEXT NOT NULL DEFAULT '',
  content     TEXT NOT NULL DEFAULT '',
  item_ids    TEXT NOT NULL DEFAULT '[]',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

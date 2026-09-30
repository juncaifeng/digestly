package store

const schema = `
CREATE TABLE IF NOT EXISTS feeds (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL,
  url          TEXT NOT NULL UNIQUE,
  collector    TEXT NOT NULL DEFAULT 'rss',
  config       TEXT NOT NULL DEFAULT '{}',
  interval_min INTEGER NOT NULL DEFAULT 30,
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
CREATE TABLE IF NOT EXISTS item_tags (
  item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (item_id, tag_id)
);
`

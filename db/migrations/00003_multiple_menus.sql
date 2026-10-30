-- +goose Up
-- Many menus, each with a name, a schedule (JSON: weekly hours or a one-off
-- event) and a published flag. The existing menu becomes the Regular Menu,
-- published with no hours set, so it stays on the TV until it's scheduled.
-- (Copied through a temporary table, not renamed, so the schema reads as
-- written.)
CREATE TEMP TABLE menus_before AS SELECT id, content, revision FROM menus;
DROP TABLE menus;
CREATE TABLE menus (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  content TEXT NOT NULL,
  schedule TEXT NOT NULL,
  published INTEGER NOT NULL DEFAULT 1,
  revision INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
INSERT INTO menus (id, name, content, schedule, published, revision, created_at, updated_at)
  SELECT id, 'Regular Menu', content, '{"kind":"weekly","hours":[]}', 1, revision, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM menus_before;
DROP TABLE menus_before;
-- +goose Down
CREATE TEMP TABLE menus_after AS SELECT content, revision FROM menus ORDER BY id LIMIT 1;
DROP TABLE menus;
CREATE TABLE menus (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  content TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1
);
INSERT INTO menus (id, content, revision) SELECT 1, content, revision FROM menus_after;
DROP TABLE menus_after;

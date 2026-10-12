-- +goose Up
CREATE TABLE menus (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  content TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email_address TEXT NOT NULL UNIQUE,
  password_digest TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_digest TEXT NOT NULL UNIQUE,
  ip_address TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  last_seen_at DATETIME NOT NULL
);
CREATE INDEX sessions_user_id ON sessions(user_id);
CREATE TABLE invitations (
  token_digest TEXT PRIMARY KEY,
  expires_at INTEGER NOT NULL
);
-- +goose Down
DROP TABLE invitations;
DROP TABLE sessions;
DROP TABLE users;
DROP TABLE menus;

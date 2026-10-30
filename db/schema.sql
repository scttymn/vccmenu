-- The schema as the migrations leave it, for sqlc and for reading. It's
-- written from the migrations; don't edit it by hand.

CREATE TABLE invitations (
  token_digest TEXT PRIMARY KEY,
  expires_at INTEGER NOT NULL
);
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
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_digest TEXT NOT NULL UNIQUE,
  ip_address TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  last_seen_at DATETIME NOT NULL
);
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email_address TEXT NOT NULL UNIQUE,
  password_digest TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX sessions_user_id ON sessions(user_id);

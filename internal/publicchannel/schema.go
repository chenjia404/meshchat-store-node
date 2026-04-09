package publicchannel

const schemaVersion = 1

const migrateSQL = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 15000;

CREATE TABLE IF NOT EXISTS schema_meta (
  key TEXT PRIMARY KEY,
  value INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS public_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id TEXT NOT NULL UNIQUE,
  owner_peer_id TEXT NOT NULL,
  owner_version INTEGER NOT NULL,
  name TEXT NOT NULL,
  avatar_json TEXT NOT NULL DEFAULT '',
  bio TEXT NOT NULL DEFAULT '',
  message_retention_minutes INTEGER NOT NULL DEFAULT 0,
  profile_version INTEGER NOT NULL,
  last_message_id INTEGER NOT NULL DEFAULT 0,
  last_seq INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  head_updated_at INTEGER NOT NULL DEFAULT 0,
  profile_signature TEXT NOT NULL,
  head_signature TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS public_channel_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_db_id INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  version INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  owner_version INTEGER NOT NULL,
  creator_peer_id TEXT NOT NULL,
  author_peer_id TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  is_deleted INTEGER NOT NULL DEFAULT 0,
  message_type TEXT NOT NULL,
  content_json TEXT NOT NULL DEFAULT '',
  signature TEXT NOT NULL,
  UNIQUE(channel_db_id, message_id),
  UNIQUE(channel_db_id, seq)
);

CREATE TABLE IF NOT EXISTS public_channel_changes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_db_id INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  change_type TEXT NOT NULL,
  message_id INTEGER,
  version INTEGER,
  is_deleted INTEGER,
  profile_version INTEGER,
  created_at INTEGER NOT NULL,
  UNIQUE(channel_db_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_public_channels_owner ON public_channels(owner_peer_id);
CREATE INDEX IF NOT EXISTS idx_public_channel_messages_channel_msg ON public_channel_messages(channel_db_id, message_id DESC);
CREATE INDEX IF NOT EXISTS idx_public_channel_messages_channel_seq ON public_channel_messages(channel_db_id, seq ASC);
CREATE INDEX IF NOT EXISTS idx_public_channel_changes_channel_seq ON public_channel_changes(channel_db_id, seq ASC);
`

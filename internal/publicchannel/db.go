package publicchannel

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open 打开或创建 public_channels.db（纯 Go SQLite，适合 Alpine 镜像）。
func Open(dbPath string) (*sql.DB, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("abs db path: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?_pragma=busy_timeout(15000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(migrateSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	var ver int
	if err := db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&ver); err != nil {
		if err == sql.ErrNoRows {
			if _, err := db.Exec(`INSERT INTO schema_meta(key, value) VALUES ('version', ?)`, schemaVersion); err != nil {
				_ = db.Close()
				return nil, fmt.Errorf("set schema version: %w", err)
			}
		} else {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

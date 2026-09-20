package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver registration
)

// GenesisHash is the root hash for the first operation in an empty ledger.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// schemaSQL defines the SQLite relational tables and indexes.
const schemaSQL = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

-- Tracks distinct organizing sessions (one-shot run or watch session)
CREATE TABLE IF NOT EXISTS runs (
    run_id TEXT PRIMARY KEY,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    trigger_type TEXT NOT NULL,          -- 'run' or 'watch'
    status TEXT NOT NULL DEFAULT 'active' -- 'active' or 'rolled_back'
);

-- Tracks every individual file move with cryptographic hash linking
CREATE TABLE IF NOT EXISTS operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    source_path TEXT NOT NULL,
    dest_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    prev_record_hash TEXT NOT NULL,
    record_hash TEXT NOT NULL,
    is_undone BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (run_id) REFERENCES runs(run_id) ON DELETE CASCADE
);

-- Indexes for rapid lookup during undo and log inspection
CREATE INDEX IF NOT EXISTS idx_ops_run_id ON operations(run_id);
CREATE INDEX IF NOT EXISTS idx_ops_source ON operations(source_path);
CREATE INDEX IF NOT EXISTS idx_ops_dest ON operations(dest_path);
`

// InitDB ensures the directory exists, opens the SQLite file, and applies the schema.
func InitDB(dbPath string) (*sql.DB, error) {
	// 1. Ensure directory exists (e.g. ~/.config/tidy/)
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	// 2. Open SQLite connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// 3. Verify connection by pinging
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}

	// 4. Apply schema and performance pragmas
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return db, nil
}

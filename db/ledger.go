package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// Operation represents a single recorded file move in the ledger.
type Operation struct {
	ID             int64
	RunID          string
	Timestamp      string
	SourcePath     string
	DestPath       string
	FileSize       int64
	PrevRecordHash string
	RecordHash     string
	IsUndone       bool
}

// Ledger provides high-level methods for recording and auditing file movements.
type Ledger struct {
	db *sql.DB
}

// NewLedger wraps an existing sql.DB into a Ledger.
func NewLedger(db *sql.DB) *Ledger {
	return &Ledger{db: db}
}

// OpenLedger initializes the database and returns an active Ledger.
func OpenLedger(dbPath string) (*Ledger, error) {
	database, err := InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	return NewLedger(database), nil
}

// Close closes the underlying SQLite database connection.
func (l *Ledger) Close() error {
	return l.db.Close()
}

// StartRun inserts a new run record into the database.
func (l *Ledger) StartRun(runID, triggerType string) error {
	query := `INSERT INTO runs (run_id, trigger_type, status) VALUES (?, ?, 'active')`
	_, err := l.db.Exec(query, runID, triggerType)
	if err != nil {
		return fmt.Errorf("failed to start run %s: %w", runID, err)
	}
	return nil
}

// ComputeHash generates a deterministic SHA-256 hash linking the operation to its predecessor.
func ComputeHash(prevHash, runID, source, dest string, fileSize int64) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%d", prevHash, runID, source, dest, fileSize)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

// RecordMove records a file move into the operations table with cryptographic hash linking.
func (l *Ledger) RecordMove(runID, source, dest string, fileSize int64) (*Operation, error) {
	// 1. Fetch the hash of the latest operation (GenesisHash if this is the first entry)
	var prevHash string
	err := l.db.QueryRow(`SELECT record_hash FROM operations ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
	if err == sql.ErrNoRows {
		prevHash = GenesisHash
	} else if err != nil {
		return nil, fmt.Errorf("failed to query previous record hash: %w", err)
	}

	// 2. Compute the cryptographic hash for this new record
	recordHash := ComputeHash(prevHash, runID, source, dest, fileSize)

	// 3. Insert the operation into the ledger
	query := `
	INSERT INTO operations (run_id, source_path, dest_path, file_size, prev_record_hash, record_hash, is_undone)
	VALUES (?, ?, ?, ?, ?, ?, 0)
	`
	res, err := l.db.Exec(query, runID, source, dest, fileSize, prevHash, recordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to record operation: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch operation ID: %w", err)
	}

	return &Operation{
		ID:             id,
		RunID:          runID,
		SourcePath:     source,
		DestPath:       dest,
		FileSize:       fileSize,
		PrevRecordHash: prevHash,
		RecordHash:     recordHash,
		IsUndone:       false,
	}, nil
}

// GetOperationsForRun retrieves all active (not undone) operations for a given run in reverse order.
func (l *Ledger) GetOperationsForRun(runID string) ([]Operation, error) {
	query := `
	SELECT id, run_id, timestamp, source_path, dest_path, file_size, prev_record_hash, record_hash, is_undone
	FROM operations
	WHERE run_id = ? AND is_undone = 0
	ORDER BY id DESC
	`
	rows, err := l.db.Query(query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query operations for run %s: %w", runID, err)
	}
	defer rows.Close()

	var ops []Operation
	for rows.Next() {
		var op Operation
		if err := rows.Scan(&op.ID, &op.RunID, &op.Timestamp, &op.SourcePath, &op.DestPath,
			&op.FileSize, &op.PrevRecordHash, &op.RecordHash, &op.IsUndone); err != nil {
			return nil, fmt.Errorf("failed to scan operation row: %w", err)
		}
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for run %s: %w", runID, err)
	}

	return ops, nil
}

// MarkOperationUndone flags an operation as reversed in the ledger.
func (l *Ledger) MarkOperationUndone(opID int64) error {
	query := `UPDATE operations SET is_undone = 1 WHERE id = ?`
	_, err := l.db.Exec(query, opID)
	if err != nil {
		return fmt.Errorf("failed to mark operation %d as undone: %w", opID, err)
	}
	return nil
}

// MarkRunUndone flags an entire run as rolled back.
func (l *Ledger) MarkRunUndone(runID string) error {
	query := `UPDATE runs SET status = 'rolled_back' WHERE run_id = ?`
	_, err := l.db.Exec(query, runID)
	if err != nil {
		return fmt.Errorf("failed to mark run %s as rolled back: %w", runID, err)
	}
	return nil
}

// VerifyLedgerIntegrity audits the full hash chain from start to finish.
// Returns false if any record has been modified, tampered with, or out of sequence.
func (l *Ledger) VerifyLedgerIntegrity() (bool, error) {
	query := `
	SELECT id, run_id, source_path, dest_path, file_size, prev_record_hash, record_hash
	FROM operations
	ORDER BY id ASC
	`
	rows, err := l.db.Query(query)
	if err != nil {
		return false, fmt.Errorf("failed to query operations for audit: %w", err)
	}
	defer rows.Close()

	expectedPrev := GenesisHash

	for rows.Next() {
		var id, fileSize int64
		var runID, source, dest, prevHash, recHash string

		if err := rows.Scan(&id, &runID, &source, &dest, &fileSize, &prevHash, &recHash); err != nil {
			return false, fmt.Errorf("failed to scan row during audit: %w", err)
		}

		// 1. Verify link to predecessor
		if prevHash != expectedPrev {
			return false, fmt.Errorf("integrity violation at operation #%d: broken link (prevHash %s != expected %s)", id, prevHash, expectedPrev)
		}

		// 2. Recalculate hash and verify it matches
		recalculated := ComputeHash(prevHash, runID, source, dest, fileSize)
		if recHash != recalculated {
			return false, fmt.Errorf("integrity violation at operation #%d: tampered content (recordHash %s != computed %s)", id, recHash, recalculated)
		}

		// Move to next link in the chain
		expectedPrev = recHash
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("row iteration error during audit: %w", err)
	}

	return true, nil
}

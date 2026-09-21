package engine

import (
	"database/sql"
	"fmt"
	"os"

	"tidy/db"
	"tidy/fs"
)

// UndoResult summarizes the outcome of rolling back a run.
type UndoResult struct {
	RunID         string
	FilesRestored int
	FilesSkipped  int
	Errors        []string
}

// UndoRun reverses the file moves recorded in the specified run (or latest run if runID is empty).
func UndoRun(ledger *db.Ledger, runID string) (*UndoResult, error) {
	// 1. If runID is not specified, query the most recent active run
	if runID == "" {
		latest, err := ledger.GetLatestActiveRun()
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active runs found to undo")
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch latest run: %w", err)
		}
		runID = latest
	}

	// 2. Fetch all active operations for this run in reverse order (LIFO)
	ops, err := ledger.GetOperationsForRun(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch operations for run %s: %w", runID, err)
	}

	if len(ops) == 0 {
		return nil, fmt.Errorf("no reversible operations found for run %s", runID)
	}

	result := &UndoResult{
		RunID: runID,
	}

	// 3. Safely move each file back to its original location
	for _, op := range ops {
		// A. Verify file still exists at destination
		if _, err := os.Stat(op.DestPath); os.IsNotExist(err) {
			result.FilesSkipped++
			result.Errors = append(result.Errors, fmt.Sprintf("cannot restore %s: file no longer exists at destination %s", op.SourcePath, op.DestPath))
			continue
		}

		// B. Handle collision at original source path
		restoredPath, shouldSkip, err := fs.ResolveCollision(op.SourcePath, fs.StrategyRename)
		if err != nil {
			result.FilesSkipped++
			result.Errors = append(result.Errors, fmt.Sprintf("collision check failed for %s: %v", op.SourcePath, err))
			continue
		}
		if shouldSkip {
			result.FilesSkipped++
			continue
		}

		// C. Move the file back atomically (Destination -> Source)
		if err := fs.MoveFile(op.DestPath, restoredPath); err != nil {
			result.FilesSkipped++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to restore %s to %s: %v", op.DestPath, restoredPath, err))
			continue
		}

		// D. Mark operation as undone in the SQLite ledger
		if err := ledger.MarkOperationUndone(op.ID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("restored %s but failed to update ledger: %v", op.DestPath, err))
		}

		result.FilesRestored++
	}

	// 4. Mark run as rolled back if at least one file was restored
	if result.FilesRestored > 0 {
		if err := ledger.MarkRunUndone(runID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to mark run as rolled back: %v", err))
		}
	}

	return result, nil
}

// UndoOperation reverses a single file move identified by its operation ID.
func UndoOperation(ledger *db.Ledger, opID int64) (*UndoResult, error) {
	op, err := ledger.GetOperationByID(opID)
	if err != nil {
		return nil, fmt.Errorf("operation #%d not found: %w", opID, err)
	}

	if op.IsUndone {
		return nil, fmt.Errorf("operation #%d is already marked as undone", opID)
	}

	result := &UndoResult{
		RunID: op.RunID,
	}

	// 1. Verify file still exists at destination
	if _, err := os.Stat(op.DestPath); os.IsNotExist(err) {
		result.FilesSkipped++
		result.Errors = append(result.Errors, fmt.Sprintf("cannot restore %s: file no longer exists at destination %s", op.SourcePath, op.DestPath))
		return result, nil
	}

	// 2. Handle collision at original source path
	restoredPath, shouldSkip, err := fs.ResolveCollision(op.SourcePath, fs.StrategyRename)
	if err != nil {
		return nil, fmt.Errorf("collision check failed for %s: %w", op.SourcePath, err)
	}
	if shouldSkip {
		result.FilesSkipped++
		return result, nil
	}

	// 3. Move the file back atomically (Destination -> Source)
	if err := fs.MoveFile(op.DestPath, restoredPath); err != nil {
		return nil, fmt.Errorf("failed to restore %s to %s: %w", op.DestPath, restoredPath, err)
	}

	// 4. Mark operation as undone in the SQLite ledger
	if err := ledger.MarkOperationUndone(op.ID); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("restored %s but failed to update ledger: %v", op.DestPath, err))
	}

	result.FilesRestored++
	return result, nil
}


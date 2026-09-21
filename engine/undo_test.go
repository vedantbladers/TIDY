package engine

import (
	"os"
	"path/filepath"
	"testing"

	"tidy/db"
	"tidy/fs"
)

func TestUndoRun_Success(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "Downloads")
	destDir := filepath.Join(tempDir, "Documents")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(destDir, 0755)

	// 1. Initialize test ledger
	dbPath := filepath.Join(tempDir, "history.db")
	ledger, err := db.OpenLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to open test ledger: %v", err)
	}
	defer ledger.Close()

	// 2. Create source file and simulate organizing it to destDir
	sourceFile := filepath.Join(sourceDir, "tax_report.pdf")
	destFile := filepath.Join(destDir, "tax_report.pdf")
	content := []byte("confidential tax data")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Move file and record in ledger
	runID := "run-undo-001"
	ledger.StartRun(runID, "run")
	if err := fs.MoveFile(sourceFile, destFile); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.RecordMove(runID, sourceFile, destFile, int64(len(content))); err != nil {
		t.Fatal(err)
	}

	// 3. Execute Undo
	res, err := UndoRun(ledger, runID)
	if err != nil {
		t.Fatalf("UndoRun failed: %v", err)
	}

	if res.FilesRestored != 1 {
		t.Errorf("expected 1 file restored, got %d", res.FilesRestored)
	}

	// 4. Verify file is back in sourceDir and removed from destDir
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Errorf("expected file to be restored to %s, but it's missing", sourceFile)
	}
	if _, err := os.Stat(destFile); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed from %s, but it still exists", destFile)
	}

	// 5. Verify database marks it undone (0 active operations remaining)
	remaining, err := ledger.GetOperationsForRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 active operations left after undo, got %d", len(remaining))
	}
}

func TestUndoRun_SourceCollision(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "Downloads")
	destDir := filepath.Join(tempDir, "Images")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(destDir, 0755)

	dbPath := filepath.Join(tempDir, "history.db")
	ledger, err := db.OpenLedger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	sourceFile := filepath.Join(sourceDir, "photo.png")
	destFile := filepath.Join(destDir, "photo.png")
	os.WriteFile(destFile, []byte("original photo"), 0644)

	runID := "run-collision-002"
	ledger.StartRun(runID, "run")
	ledger.RecordMove(runID, sourceFile, destFile, 14)

	// SIMULATE COLLISION: User created a NEW photo.png in Downloads before undoing!
	newDownload := filepath.Join(sourceDir, "photo.png")
	os.WriteFile(newDownload, []byte("brand new download"), 0644)

	// Execute Undo: Must NOT overwrite the new download; should rename to photo (1).png!
	res, err := UndoRun(ledger, runID)
	if err != nil {
		t.Fatalf("UndoRun failed: %v", err)
	}
	if res.FilesRestored != 1 {
		t.Fatalf("expected 1 file restored, got %d", res.FilesRestored)
	}

	expectedRestored := filepath.Join(sourceDir, "photo (1).png")
	if _, err := os.Stat(expectedRestored); os.IsNotExist(err) {
		t.Errorf("expected restored file at %s, but it does not exist", expectedRestored)
	}
}

func TestUndoRun_MissingDestinationFile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "history.db")
	ledger, err := db.OpenLedger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	runID := "run-missing-003"
	ledger.StartRun(runID, "run")
	ledger.RecordMove(runID, "/tmp/src.txt", "/tmp/dest_does_not_exist.txt", 100)

	// File doesn't exist at destination (user deleted it). Should skip gracefully.
	res, err := UndoRun(ledger, runID)
	if err != nil {
		t.Fatalf("expected graceful skip, got error: %v", err)
	}
	if res.FilesSkipped != 1 || res.FilesRestored != 0 {
		t.Errorf("expected 1 skipped and 0 restored, got skipped=%d, restored=%d", res.FilesSkipped, res.FilesRestored)
	}
}

func TestUndoOperation_Success(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "Downloads")
	destDir := filepath.Join(tempDir, "Documents")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(destDir, 0755)

	dbPath := filepath.Join(tempDir, "history.db")
	ledger, err := db.OpenLedger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	sourceFile := filepath.Join(sourceDir, "single_file.pdf")
	destFile := filepath.Join(destDir, "single_file.pdf")
	content := []byte("hello world")
	os.WriteFile(destFile, content, 0644)

	runID := "run-single-op"
	ledger.StartRun(runID, "run")
	op, err := ledger.RecordMove(runID, sourceFile, destFile, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}

	// Undo single operation by ID
	res, err := UndoOperation(ledger, op.ID)
	if err != nil {
		t.Fatalf("UndoOperation failed: %v", err)
	}
	if res.FilesRestored != 1 {
		t.Errorf("expected 1 file restored, got %d", res.FilesRestored)
	}

	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Errorf("expected restored file at %s", sourceFile)
	}
	if _, err := os.Stat(destFile); !os.IsNotExist(err) {
		t.Errorf("expected dest file %s to be removed", destFile)
	}

	// Undoing again should return error (already undone)
	_, err = UndoOperation(ledger, op.ID)
	if err == nil {
		t.Error("expected error when undoing already-undone operation, got nil")
	}
}


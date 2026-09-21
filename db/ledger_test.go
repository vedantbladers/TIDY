package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLedger_HashChainingAndAudit(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "history.db")

	// 1. Open ledger
	ledger, err := OpenLedger(dbPath)
	if err != nil {
		t.Fatalf("OpenLedger failed: %v", err)
	}
	defer ledger.Close()

	// 2. Start a run
	runID := "test-run-001"
	if err := ledger.StartRun(runID, "run"); err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	// 3. Record first move
	op1, err := ledger.RecordMove(runID, "/Downloads/file1.txt", "/Documents/file1.txt", 1024)
	if err != nil {
		t.Fatalf("RecordMove 1 failed: %v", err)
	}
	if op1.PrevRecordHash != GenesisHash {
		t.Errorf("expected op1 prevHash to be GenesisHash, got: %s", op1.PrevRecordHash)
	}

	// 4. Record second move
	op2, err := ledger.RecordMove(runID, "/Downloads/photo.png", "/Images/photo.png", 2048)
	if err != nil {
		t.Fatalf("RecordMove 2 failed: %v", err)
	}
	// Verify cryptographic link: op2's prevHash MUST match op1's recordHash!
	if op2.PrevRecordHash != op1.RecordHash {
		t.Errorf("broken chain link: op2 prevHash (%s) != op1 recordHash (%s)", op2.PrevRecordHash, op1.RecordHash)
	}

	// 5. Verify integrity passes on clean data
	valid, err := ledger.VerifyLedgerIntegrity()
	if err != nil || !valid {
		t.Errorf("expected clean ledger to be valid, got valid=%v, err=%v", valid, err)
	}

	// 6. Test Reverse Retrieval for Undo (LIFO order: op2 should come before op1)
	ops, err := ledger.GetOperationsForRun(runID)
	if err != nil {
		t.Fatalf("GetOperationsForRun failed: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	if ops[0].ID != op2.ID || ops[1].ID != op1.ID {
		t.Errorf("expected reverse LIFO order for undo: got IDs [%d, %d], expected [%d, %d]",
			ops[0].ID, ops[1].ID, op2.ID, op1.ID)
	}

	// 7. Mark op2 as undone and verify it is filtered out of active ops
	if err := ledger.MarkOperationUndone(op2.ID); err != nil {
		t.Fatalf("MarkOperationUndone failed: %v", err)
	}
	remainingOps, err := ledger.GetOperationsForRun(runID)
	if err != nil {
		t.Fatalf("GetOperationsForRun failed: %v", err)
	}
	if len(remainingOps) != 1 || remainingOps[0].ID != op1.ID {
		t.Errorf("expected only op1 remaining, got: %v", remainingOps)
	}
}

func TestLedger_DetectTampering(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "tamper_test.db")

	ledger, err := OpenLedger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	runID := "tamper-run"
	ledger.StartRun(runID, "run")
	ledger.RecordMove(runID, "/Downloads/report.pdf", "/Documents/report.pdf", 500)
	ledger.RecordMove(runID, "/Downloads/song.mp3", "/Music/song.mp3", 1500)

	// SIMULATE A MALICIOUS TAMPER:
	// Manually execute an SQL UPDATE behind the application's back
	_, err = ledger.db.Exec(`UPDATE operations SET dest_path = '/Hacked/malicious.pdf' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}

	// Audit the ledger: The cryptographic hash chain MUST catch the tamper!
	valid, err := ledger.VerifyLedgerIntegrity()
	if valid {
		t.Error("CRITICAL FAILURE: Tampered ledger passed validation!")
	}
	if err == nil || !strings.Contains(err.Error(), "tampered content") {
		t.Errorf("expected error mentioning 'tampered content', got: %v", err)
	}
}

func TestLedger_GetOperationByIDAndAll(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "query_test.db")

	ledger, err := OpenLedger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	runID := "run-query"
	ledger.StartRun(runID, "run")
	op1, err := ledger.RecordMove(runID, "/a.txt", "/b.txt", 100)
	if err != nil {
		t.Fatal(err)
	}
	op2, err := ledger.RecordMove(runID, "/c.txt", "/d.txt", 200)
	if err != nil {
		t.Fatal(err)
	}

	// Query by ID
	fetched, err := ledger.GetOperationByID(op1.ID)
	if err != nil {
		t.Fatalf("GetOperationByID failed: %v", err)
	}
	if fetched.SourcePath != "/a.txt" || fetched.DestPath != "/b.txt" {
		t.Errorf("unexpected op data: %+v", fetched)
	}

	// Query all operations
	allOps, err := ledger.GetAllOperations(0)
	if err != nil {
		t.Fatalf("GetAllOperations failed: %v", err)
	}
	if len(allOps) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(allOps))
	}
	if allOps[0].ID != op1.ID || allOps[1].ID != op2.ID {
		t.Errorf("unexpected ordering: %+v", allOps)
	}
}


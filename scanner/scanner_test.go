package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTopLevel_OnlyRegularFiles(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create top-level regular files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.png")
	if err := os.WriteFile(file1, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("image_data"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	// 2. Create subdirectories (e.g. organized destination folders)
	subDir1 := filepath.Join(tempDir, "Documents")
	subDir2 := filepath.Join(tempDir, "Images")
	if err := os.Mkdir(subDir1, 0755); err != nil {
		t.Fatalf("failed to create subDir1: %v", err)
	}
	if err := os.Mkdir(subDir2, 0755); err != nil {
		t.Fatalf("failed to create subDir2: %v", err)
	}

	// 3. Create a nested file inside a subdirectory (must NEVER be scanned!)
	nestedFile := filepath.Join(subDir1, "nested_doc.pdf")
	if err := os.WriteFile(nestedFile, []byte("nested"), 0644); err != nil {
		t.Fatalf("failed to create nestedFile: %v", err)
	}

	// 4. Run ScanTopLevel
	scanned, err := ScanTopLevel(tempDir)
	if err != nil {
		t.Fatalf("ScanTopLevel failed: %v", err)
	}

	// 5. Assertions: We expect EXACTLY 2 files (file1.txt and file2.png)
	if len(scanned) != 2 {
		t.Fatalf("expected exactly 2 files, got %d", len(scanned))
	}

	names := make(map[string]bool)
	for _, f := range scanned {
		names[f.Name] = true
	}

	if !names["file1.txt"] || !names["file2.png"] {
		t.Errorf("expected file1.txt and file2.png, got: %v", scanned)
	}
	if names["nested_doc.pdf"] {
		t.Errorf("CRITICAL VIOLATION: nested file was scanned! Scanner is not top-level only.")
	}
}

func TestScanTopLevel_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	scanned, err := ScanTopLevel(tempDir)
	if err != nil {
		t.Fatalf("unexpected error on empty directory: %v", err)
	}
	if len(scanned) != 0 {
		t.Errorf("expected 0 files, got %d", len(scanned))
	}
}

func TestScanTopLevel_NonExistentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	missingDir := filepath.Join(tempDir, "not_here")

	_, err := ScanTopLevel(missingDir)
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

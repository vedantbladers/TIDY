package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile_SamePartitionAtomic(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "original.txt")
	content := []byte("hello tidy safe mover")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tempDir, "nested", "folder", "moved.txt")

	// MoveFile should automatically create "nested/folder" and move the file
	if err := MoveFile(src, dest); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// 1. Verify original is gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected source file to be gone, but it still exists")
	}

	// 2. Verify destination exists with exact content
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, expected %q", string(data), string(content))
	}
}

func TestMoveFile_SameSourceAndDestination(t *testing.T) {
	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "stay_put.txt")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Moving to self should be a safe no-op
	if err := MoveFile(file, file); err != nil {
		t.Fatalf("expected nil for same source and dest, got: %v", err)
	}
}

func TestCopyAndRemove_FallbackDirect(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "source_copy.txt")
	content := []byte("fallback copy and remove verification")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tempDir, "dest_copy.txt")

	// Directly test the copyAndRemove fallback implementation
	if err := copyAndRemove(src, dest); err != nil {
		t.Fatalf("copyAndRemove failed: %v", err)
	}

	// Verify source was removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected source file to be removed after copy")
	}

	// Verify destination has correct data
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, expected %q", string(data), string(content))
	}
}

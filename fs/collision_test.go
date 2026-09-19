package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitNameAndExt(t *testing.T) {
	tests := []struct {
		filename     string
		expectedBase string
		expectedExt  string
	}{
		{"report.pdf", "report", ".pdf"},
		{"archive.tar.gz", "archive", ".tar.gz"},
		{"backup.tar.bz2", "backup", ".tar.bz2"},
		{"photo", "photo", ""},
		{".hiddenfile", ".hiddenfile", ""},
		{"my.notes.txt", "my.notes", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			base, ext := SplitNameAndExt(tt.filename)
			if base != tt.expectedBase || ext != tt.expectedExt {
				t.Errorf("SplitNameAndExt(%q) = (%q, %q); expected (%q, %q)",
					tt.filename, base, ext, tt.expectedBase, tt.expectedExt)
			}
		})
	}
}

func TestResolveCollision(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Non-colliding target returns original path
	t.Run("no collision returns target directly", func(t *testing.T) {
		target := filepath.Join(tempDir, "fresh_file.txt")
		resolved, skip, err := ResolveCollision(target, StrategyRename)
		if err != nil || skip || resolved != target {
			t.Errorf("expected %s, got %s (skip=%v, err=%v)", target, resolved, skip, err)
		}
	})

	// 2. Collision with StrategySkip
	t.Run("skip strategy flags skip", func(t *testing.T) {
		existing := filepath.Join(tempDir, "existing_file.txt")
		if err := os.WriteFile(existing, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		resolved, skip, err := ResolveCollision(existing, StrategySkip)
		if err != nil || !skip || resolved != "" {
			t.Errorf("expected skip=true, got resolved=%q, skip=%v, err=%v", resolved, skip, err)
		}
	})

	// 3. Collision with StrategyOverwrite
	t.Run("overwrite strategy returns same path", func(t *testing.T) {
		existing := filepath.Join(tempDir, "overwrite_me.txt")
		if err := os.WriteFile(existing, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		resolved, skip, err := ResolveCollision(existing, StrategyOverwrite)
		if err != nil || skip || resolved != existing {
			t.Errorf("expected %s, got %s (skip=%v, err=%v)", existing, resolved, skip, err)
		}
	})

	// 4. Collision with StrategyRename (generates "file (1).ext", "file (2).ext")
	t.Run("rename strategy increments filename cleanly", func(t *testing.T) {
		baseFile := filepath.Join(tempDir, "document.pdf")
		file1 := filepath.Join(tempDir, "document (1).pdf")

		// Create document.pdf and document (1).pdf
		if err := os.WriteFile(baseFile, []byte("base"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file1, []byte("copy1"), 0644); err != nil {
			t.Fatal(err)
		}

		// Should resolve to document (2).pdf!
		resolved, skip, err := ResolveCollision(baseFile, StrategyRename)
		if err != nil || skip {
			t.Fatalf("unexpected error or skip: %v", err)
		}

		expected := filepath.Join(tempDir, "document (2).pdf")
		if resolved != expected {
			t.Errorf("expected %s, got %s", expected, resolved)
		}
	})

	// 5. Collision with compound extension (.tar.gz)
	t.Run("rename strategy respects compound extension", func(t *testing.T) {
		tarFile := filepath.Join(tempDir, "archive.tar.gz")
		if err := os.WriteFile(tarFile, []byte("tar"), 0644); err != nil {
			t.Fatal(err)
		}

		resolved, _, err := ResolveCollision(tarFile, StrategyRename)
		if err != nil {
			t.Fatal(err)
		}

		expected := filepath.Join(tempDir, "archive (1).tar.gz")
		if resolved != expected {
			t.Errorf("expected %s, got %s", expected, resolved)
		}
	})
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 1: Verifies that a clean, valid configuration passes with zero errors and zero warnings.
func TestValidateConfig_Valid(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &Config{
		WatchDirs: []string{tempDir},
		Rules: []Rule{
			{
				Name:       "Documents",
				Extensions: []string{".pdf", ".docx"},
				Dest:       filepath.Join(tempDir, "Docs"),
			},
		},
	}

	res := ValidateConfig(cfg)
	if !res.IsValid() {
		t.Fatalf("expected config to be valid, got errors: %v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(res.Warnings), res.Warnings)
	}
}

// Test 2: Verifies that missing directories and invalid file types under watch_dirs are caught.
func TestValidateConfig_WatchDirs(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		res := ValidateConfig(nil)
		if res.IsValid() {
			t.Error("expected nil config to fail validation")
		}
	})

	t.Run("empty watch_dirs returns error", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{},
			Rules: []Rule{
				{Name: "Docs", Extensions: []string{".pdf"}, Dest: "/tmp"},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected empty watch_dirs to fail validation")
		}
	})

	t.Run("non-existent watch_dir returns error", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{"/path/that/really/does/not/exist/12345"},
			Rules: []Rule{
				{Name: "Docs", Extensions: []string{".pdf"}, Dest: "/tmp"},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected non-existent watch_dir to fail validation")
		}
		if len(res.Errors) == 0 || !strings.Contains(res.Errors[0], "does not exist") {
			t.Errorf("expected 'does not exist' error, got: %v", res.Errors)
		}
	})

	t.Run("watch_dir that is a file returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "regular_file.txt")
		if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cfg := &Config{
			WatchDirs: []string{filePath}, // OOPS! Passing a file instead of a folder!
			Rules: []Rule{
				{Name: "Docs", Extensions: []string{".pdf"}, Dest: "/tmp"},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected file watch_dir to fail validation")
		}
		if len(res.Errors) == 0 || !strings.Contains(res.Errors[0], "is a file, not a directory") {
			t.Errorf("expected 'is a file' error, got: %v", res.Errors)
		}
	})
}

// Test 3: Verifies rule structural requirements (name, dest, conditions).
func TestValidateConfig_RuleStructure(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("rule missing name returns error", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "", Extensions: []string{".pdf"}, Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected missing rule name to fail validation")
		}
	})

	t.Run("rule missing dest returns error", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "Docs", Extensions: []string{".pdf"}, Dest: ""},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected missing dest to fail validation")
		}
	})

	t.Run("rule missing both extensions and pattern returns error", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "Docs", Extensions: []string{}, Pattern: "", Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if res.IsValid() {
			t.Error("expected rule with no conditions to fail validation")
		}
	})
}

// Test 4: Verifies rule shadowing detection across extensions and patterns.
func TestValidateConfig_RuleShadowing(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("complete extension shadowing triggers warning", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "Images", Extensions: []string{".png", ".jpg"}, Dest: tempDir},
				{Name: "Screenshots", Extensions: []string{".png"}, Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if !res.IsValid() {
			t.Errorf("shadowed rule should produce a warning, NOT a fatal error!")
		}
		if len(res.Warnings) == 0 {
			t.Fatal("expected at least 1 warning for shadowed rule, got 0")
		}
		if !strings.Contains(res.Warnings[0], "completely shadowed") {
			t.Errorf("expected warning to mention 'completely shadowed', got: %s", res.Warnings[0])
		}
	})

	t.Run("partial overlap does not trigger complete shadowing", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "Images", Extensions: []string{".png", ".jpg"}, Dest: tempDir},
				{Name: "Graphics", Extensions: []string{".png", ".gif"}, Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if len(res.Warnings) != 0 {
			t.Errorf("expected 0 warnings because .gif is not shadowed, got: %v", res.Warnings)
		}
	})

	t.Run("universal wildcard pattern shadows subsequent rules", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "CatchAll", Pattern: "*", Dest: tempDir},
				{Name: "Images", Extensions: []string{".png"}, Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "pattern '*'") {
			t.Errorf("expected wildcard shadowing warning, got: %v", res.Warnings)
		}
	})

	t.Run("identical patterns trigger duplicate warning", func(t *testing.T) {
		cfg := &Config{
			WatchDirs: []string{tempDir},
			Rules: []Rule{
				{Name: "Screenshots1", Pattern: "Screenshot*", Dest: tempDir},
				{Name: "Screenshots2", Pattern: "Screenshot*", Dest: tempDir},
			},
		}
		res := ValidateConfig(cfg)
		if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "identical to earlier rule") {
			t.Errorf("expected duplicate pattern warning, got: %v", res.Warnings)
		}
	})
}

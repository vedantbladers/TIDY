package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 1: Verifies that ExpandPath properly expands tilde (~) into full home paths.
func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir for testing: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "absolute path is unchanged",
			input:    "/var/log",
			expected: "/var/log",
		},
		{
			name:     "tilde only returns home directory",
			input:    "~",
			expected: homeDir,
		},
		{
			name:     "tilde with subfolder expands properly",
			input:    "~/Downloads/Documents",
			expected: filepath.Join(homeDir, "Downloads/Documents"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test 2: Verifies that DefaultConfigPath returns the canonical path.
func TestDefaultConfigPath(t *testing.T) {
	expected := "~/.config/tidy/config.yaml"
	if got := DefaultConfigPath(); got != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, expected)
	}
}

// Test 3: Verifies that LoadConfig correctly reads, parses, and validates YAML.
func TestLoadConfig(t *testing.T) {
	// Subtest A: Missing file should return an error
	t.Run("missing file returns error", func(t *testing.T) {
		_, err := LoadConfig("/path/to/nonexistent/file.yaml")
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	// Subtest B: Valid YAML config should parse and expand paths
	t.Run("valid config parses and expands paths", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")

		yamlContent := `
watch_dirs:
  - ~/Downloads
ignore:
  - "*.tmp"
sniff_content: true
rules:
  - name: Documents
    extensions: [".pdf", ".txt"]
    dest: ~/Downloads/Documents
`
		if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := LoadConfig(configFile)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if len(cfg.WatchDirs) != 1 {
			t.Fatalf("expected 1 watch_dir, got %d", len(cfg.WatchDirs))
		}

		// Verify tilde was expanded in watch_dirs
		if strings.HasPrefix(cfg.WatchDirs[0], "~") {
			t.Errorf("watch_dir was not expanded: %s", cfg.WatchDirs[0])
		}

		if !cfg.SniffContent {
			t.Error("expected SniffContent to be true")
		}

		if len(cfg.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
		}

		if cfg.Rules[0].Name != "Documents" {
			t.Errorf("got rule name %q, want 'Documents'", cfg.Rules[0].Name)
		}

		// Verify tilde was expanded in rule destination
		if strings.HasPrefix(cfg.Rules[0].Dest, "~") {
			t.Errorf("rule dest was not expanded: %s", cfg.Rules[0].Dest)
		}
	})

	// Subtest C: Broken YAML syntax should return a parsing error
	t.Run("invalid yaml syntax returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "broken.yaml")

		brokenYAML := `
watch_dirs: [unclosed bracket
`
		if err := os.WriteFile(configFile, []byte(brokenYAML), 0644); err != nil {
			t.Fatalf("failed to write broken config: %v", err)
		}

		_, err := LoadConfig(configFile)
		if err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})

	// Subtest D: Empty watch_dirs should fail validation
	t.Run("empty watch_dirs returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "empty_watch.yaml")

		emptyWatchYAML := `
watch_dirs: []
rules:
  - name: Docs
    extensions: [".pdf"]
    dest: ~/Documents
`
		if err := os.WriteFile(configFile, []byte(emptyWatchYAML), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := LoadConfig(configFile)
		if err == nil {
			t.Error("expected error for empty watch_dirs, got nil")
		}
	})
}

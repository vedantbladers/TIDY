package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSniffBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PNG magic bytes",
			data:     []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00},
			expected: ".png",
		},
		{
			name:     "JPEG magic bytes",
			data:     []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
			expected: ".jpg",
		},
		{
			name:     "PDF magic bytes",
			data:     []byte("%PDF-1.4 header contents"),
			expected: ".pdf",
		},
		{
			name:     "Plain text",
			data:     []byte("This is just regular plain text content"),
			expected: ".txt",
		},
		{
			name:     "Empty buffer returns empty string",
			data:     []byte{},
			expected: "",
		},
		{
			name:     "Arbitrary binary returns empty string",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SniffBytes(tt.data)
			if result != tt.expected {
				t.Errorf("SniffBytes() = %q; expected %q", result, tt.expected)
			}
		})
	}
}

func TestSniffFileExtension(t *testing.T) {
	tempDir := t.TempDir()

	// Create an extensionless file containing PNG magic bytes
	testFile := filepath.Join(tempDir, "mystery_blob")
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00}
	if err := os.WriteFile(testFile, pngHeader, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ext, err := SniffFileExtension(testFile)
	if err != nil {
		t.Fatalf("SniffFileExtension returned unexpected error: %v", err)
	}
	if ext != ".png" {
		t.Errorf("expected extension '.png', got %q", ext)
	}

	// Test non-existent file returns an error
	_, err = SniffFileExtension(filepath.Join(tempDir, "does_not_exist"))
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

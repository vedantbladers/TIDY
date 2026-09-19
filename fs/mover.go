package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFile moves a file from src to dest safely.
// It first attempts an atomic os.Rename. If that fails (e.g. across drives/partitions),
// it falls back to a verified copy-fsync-delete sequence.
func MoveFile(src, dest string) error {
	if src == dest {
		return nil
	}

	// 1. Ensure the destination parent directory exists
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// 2. Fast Path: Try atomic os.Rename
	err := os.Rename(src, dest)
	if err == nil {
		return nil // Successfully moved in 0ms via filesystem metadata!
	}

	// 3. Fallback: If rename failed with a LinkError (cross-device EXDEV), copy and remove
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return copyAndRemove(src, dest)
	}

	// Other unexpected filesystem error
	return fmt.Errorf("failed to rename %s to %s: %w", src, dest, err)
}

// copyAndRemove safely copies src to dest, forces hardware flush (fsync),
// verifies file size, and only then deletes src.
func copyAndRemove(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create destination preserving original permissions
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	// Copy data stream
	bytesWritten, err := io.Copy(destFile, srcFile)
	if err != nil {
		destFile.Close()
		os.Remove(dest) // Clean up partial destination file on copy error
		return fmt.Errorf("failed during data copy: %w", err)
	}

	// Force OS to flush buffer cache to physical disk before proceeding
	if err := destFile.Sync(); err != nil {
		destFile.Close()
		return fmt.Errorf("failed to sync destination file to disk: %w", err)
	}

	if err := destFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}

	// Verification check: Ensure written byte count matches original size
	if bytesWritten != srcInfo.Size() {
		os.Remove(dest)
		return fmt.Errorf("byte mismatch: wrote %d bytes, expected %d", bytesWritten, srcInfo.Size())
	}

	// Close source file before removing
	srcFile.Close()

	// Safe to remove source file now that destination is verified on disk
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("file copied but failed to remove source %s: %w", src, err)
	}

	return nil
}

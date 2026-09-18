package scanner

import (
	"os"
	"path/filepath"
	"time"
)

// ScannedFile contains metadata for a regular file discovered in a watch directory.
type ScannedFile struct {
	Name    string    // e.g. "report.pdf"
	Path    string    // e.g. "/home/vedant/Downloads/report.pdf"
	Size    int64     // Size in bytes
	ModTime time.Time // Last modification time
}

// ScanTopLevel reads ONLY the immediate children of watchDir.
// It strictly skips subdirectories, sockets, and non-regular files.
func ScanTopLevel(watchDir string) ([]ScannedFile, error) {
	entries, err := os.ReadDir(watchDir)
	if err != nil {
		return nil, err
	}

	var files []ScannedFile
	for _, entry := range entries {
		// 1. Skip all subdirectories (Core safety rule: NEVER recursive)
		if entry.IsDir() {
			continue
		}

		// 2. Fetch file metadata
		info, err := entry.Info()
		if err != nil {
			// If file was deleted or unreadable mid-scan, skip gracefully
			continue
		}

		// 3. Skip non-regular files (e.g. sockets, pipes, special device files)
		if !info.Mode().IsRegular() {
			continue
		}

		files = append(files, ScannedFile{
			Name:    entry.Name(),
			Path:    filepath.Join(watchDir, entry.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	return files, nil
}

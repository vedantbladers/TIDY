package engine

import (
	"io"
	"net/http"
	"os"
	"strings"
)

// mimeToExt maps detected MIME types to standardized file extensions.
var mimeToExt = map[string]string{
	"image/png":                ".png",
	"image/jpeg":               ".jpg",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"application/pdf":          ".pdf",
	"application/zip":          ".zip",
	"application/x-tar":        ".tar",
	"application/gzip":         ".gz",
	"text/plain":               ".txt",
	"text/html":                ".html",
	"application/json":         ".json",
	"application/octet-stream": "", // Generic binary; cannot reliably deduce
}

// SniffBytes inspects a byte buffer (typically the first 512 bytes)
// and returns the deduced file extension (e.g. ".png", ".pdf") or empty string if unknown.
func SniffBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// http.DetectContentType inspects up to 512 bytes
	mime := http.DetectContentType(data)

	// Strip parameters like '; charset=utf-8' if present
	if idx := strings.Index(mime, ";"); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}

	return mimeToExt[mime]
}

// SniffFileExtension opens a file on disk, reads the first 512 bytes,
// and returns the inferred file extension based on magic bytes.
func SniffFileExtension(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 512 bytes is the standard sample size for MIME sniffing
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	return SniffBytes(buffer[:n]), nil
}

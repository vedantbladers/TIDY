package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Strategy defines how to handle a file move when the destination path already exists.
type Strategy string

const (
	StrategyRename    Strategy = "rename"    // Default: generate "file (1).ext"
	StrategySkip      Strategy = "skip"      // Leave source file untouched
	StrategyOverwrite Strategy = "overwrite" // Overwrite destination file
)

// SplitNameAndExt splits a filename into its base name and extension.
// It recognizes compound extensions like ".tar.gz", ".tar.bz2", and ".tar.xz".
func SplitNameAndExt(filename string) (string, string) {
	// Dotfiles like ".gitignore" or ".hiddenfile" have NO extension
	if strings.HasPrefix(filename, ".") && strings.Count(filename, ".") == 1 {
		return filename, ""
	}

	lower := strings.ToLower(filename)
	compoundExts := []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"}

	for _, cExt := range compoundExts {
		if strings.HasSuffix(lower, cExt) {
			base := filename[:len(filename)-len(cExt)]
			return base, filename[len(filename)-len(cExt):]
		}
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return base, ext
}

// ResolveCollision determines the final destination path based on the chosen strategy.
// If the destination file does not exist, it returns targetPath immediately.
func ResolveCollision(targetPath string, strategy Strategy) (string, bool, error) {
	// If the file does not exist at destination, no collision!
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, false, nil
	} else if err != nil {
		return "", false, err
	}

	// Collision detected! Handle according to strategy:
	switch strategy {
	case StrategySkip:
		// Return empty path and shouldSkip = true
		return "", true, nil

	case StrategyOverwrite:
		// Target path remains the same; caller will overwrite
		return targetPath, false, nil

	case StrategyRename, "": // "rename" is default
		dir := filepath.Dir(targetPath)
		filename := filepath.Base(targetPath)
		base, ext := SplitNameAndExt(filename)

		counter := 1
		for {
			candidateName := fmt.Sprintf("%s (%d)%s", base, counter, ext)
			candidatePath := filepath.Join(dir, candidateName)

			if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
				return candidatePath, false, nil
			} else if err != nil {
				return "", false, err
			}
			counter++
		}

	default:
		return "", false, fmt.Errorf("unknown collision strategy: %s", strategy)
	}
}

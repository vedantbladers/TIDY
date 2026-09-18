package engine

import (
	"path/filepath"
	"strings"

	"tidy/config"
)

// MatchResult represents the outcome of evaluating a file against configuration rules.
type MatchResult struct {
	Matched  bool   // True if the file matched an organization rule
	Ignored  bool   // True if the file was matched by an ignore pattern
	RuleName string // Name of the winning rule (e.g. "Screenshots")
	DestDir  string // Destination directory for the file
}

// IsIgnored checks whether a filename matches any pattern in the ignore list.
// Matching is case-insensitive (e.g. '*.TMP' matches 'file.tmp').
func IsIgnored(filename string, ignorePatterns []string) bool {
	baseName := strings.ToLower(filepath.Base(filename))

	for _, pattern := range ignorePatterns {
		lowerPattern := strings.ToLower(pattern)
		matched, err := filepath.Match(lowerPattern, baseName)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// MatchFile evaluates a file against the configuration rules top-to-bottom.
// The first matching rule wins.
func MatchFile(filename string, cfg *config.Config) MatchResult {
	baseName := filepath.Base(filename)
	lowerName := strings.ToLower(baseName)

	// 1. Check if the file is in the ignore list
	if IsIgnored(baseName, cfg.Ignore) {
		return MatchResult{
			Ignored: true,
		}
	}

	// 2. Evaluate rules sequentially (First matching rule wins)
	for _, rule := range cfg.Rules {
		// A. Check glob pattern (e.g. "Screenshot*")
		if rule.Pattern != "" {
			lowerPattern := strings.ToLower(rule.Pattern)
			matched, err := filepath.Match(lowerPattern, lowerName)
			if err == nil && matched {
				return MatchResult{
					Matched:  true,
					RuleName: rule.Name,
					DestDir:  rule.Dest,
				}
			}
		}

		// B. Check extensions (e.g. [".png", ".jpg", ".tar.gz"])
		for _, ext := range rule.Extensions {
			lowerExt := strings.ToLower(ext)
			// Ensure extension starts with a dot
			if !strings.HasPrefix(lowerExt, ".") {
				lowerExt = "." + lowerExt
			}

			// strings.HasSuffix handles both simple (.png) and compound (.tar.gz) extensions
			if strings.HasSuffix(lowerName, lowerExt) {
				return MatchResult{
					Matched:  true,
					RuleName: rule.Name,
					DestDir:  rule.Dest,
				}
			}
		}
	}

	// No rule matched this file
	return MatchResult{
		Matched: false,
	}
}

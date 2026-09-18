package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidationResult holds all fatal errors and non-fatal warnings.
type ValidationResult struct {
	Errors   []string
	Warnings []string
}

// IsValid returns true if there are zero fatal errors.
func (v *ValidationResult) IsValid() bool {
	return len(v.Errors) == 0
}

// ValidateConfig audits the configuration for missing directories, invalid rules,
// and detects rule shadowing (when an earlier rule starves a later rule).
// ValidateConfig audits the configuration for missing directories, invalid rules,
// and detects rule shadowing (when an earlier rule starves a later rule).
func ValidateConfig(cfg *Config) *ValidationResult {
	res := &ValidationResult{
		Errors:   []string{},
		Warnings: []string{},
	}

	if cfg == nil {
		res.Errors = append(res.Errors, "configuration is nil")
		return res
	}

	// 1. Validate watch_dirs exist on disk and are directories
	if len(cfg.WatchDirs) == 0 {
		res.Errors = append(res.Errors, "no 'watch_dirs' defined in configuration")
	} else {
		for i, dir := range cfg.WatchDirs {
			if strings.TrimSpace(dir) == "" {
				res.Errors = append(res.Errors, fmt.Sprintf("watch_dir at index %d is empty", i))
				continue
			}

			// Verify directory actually exists on the filesystem
			info, err := os.Stat(dir)
			if err != nil {
				if os.IsNotExist(err) {
					res.Errors = append(res.Errors, fmt.Sprintf("watch directory does not exist: %s", dir))
				} else {
					res.Errors = append(res.Errors, fmt.Sprintf("cannot access watch directory %s: %v", dir, err))
				}
			} else if !info.IsDir() {
				res.Errors = append(res.Errors, fmt.Sprintf("watch directory is a file, not a directory: %s", dir))
			}
		}
	}

	if len(cfg.Rules) == 0 {
		res.Errors = append(res.Errors, "No rules defined in config.tidy.yaml")
		return res
	}

	for i, rule := range cfg.Rules {
		ruleName := rule.Name
		if strings.TrimSpace(ruleName) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("rule at index %d has no name", i))
			continue
		}

		if strings.TrimSpace(rule.Dest) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("rule %q has empty destination", ruleName))
			continue
		}
		hasExtensions := len(rule.Extensions) > 0
		hasPatterns := strings.TrimSpace(rule.Pattern) != ""

		if !hasExtensions && !hasPatterns {
			res.Errors = append(res.Errors, fmt.Sprintf("Rule %q has no match criteria (no extensions and empty pattern)", ruleName))
		}
	}
	detectShadowing(cfg.Rules, res)
	return res
}

// detectShadowing analyzes the rule chain and emits warnings when an earlier rule
// completely prevents a later rule from ever matching files.
func detectShadowing(rules []Rule, res *ValidationResult) {
	// Map to track extensions already claimed by earlier rules: extension -> ruleName
	seenExtensions := make(map[string]string)
	// Tracks if an earlier rule has a universal wildcard pattern (like "*")
	universalPatternRule := ""

	for _, rule := range rules {
		ruleName := rule.Name

		// If an earlier rule had pattern: "*", every single rule after it is dead
		if universalPatternRule != "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"rule %q is completely shadowed by earlier rule %q (which matches all files with pattern '*')",
				ruleName, universalPatternRule,
			))
			continue
		}

		// Check if this rule is a universal wildcard
		trimmedPat := strings.TrimSpace(rule.Pattern)
		if trimmedPat == "*" || trimmedPat == "*.*" {
			universalPatternRule = ruleName
			continue
		}

		// Check extension-based shadowing
		if len(rule.Extensions) > 0 {
			allShadowed := true
			var shadowedExts []string
			var shadowedByRules []string

			for _, ext := range rule.Extensions {
				normalizedExt := strings.ToLower(strings.TrimSpace(ext))
				if !strings.HasPrefix(normalizedExt, ".") {
					normalizedExt = "." + normalizedExt
				}

				if earlierRule, ok := seenExtensions[normalizedExt]; ok {
					shadowedExts = append(shadowedExts, normalizedExt)
					if !containsString(shadowedByRules, earlierRule) {
						shadowedByRules = append(shadowedByRules, earlierRule)
					}
				} else {
					// Found at least one extension that was NOT seen before!
					allShadowed = false
					seenExtensions[normalizedExt] = ruleName
				}
			}

			// If EVERY extension in this rule was already captured earlier, warn the user!
			if allShadowed && len(rule.Extensions) > 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"rule %q is completely shadowed: all its extensions (%s) are already caught by earlier rule(s): %s",
					ruleName, strings.Join(shadowedExts, ", "), strings.Join(shadowedByRules, ", "),
				))
			}
		}

		// Check identical pattern shadowing
		if trimmedPat != "" {
			for _, earlierRule := range rules {
				if earlierRule.Name == rule.Name {
					break // reached self
				}
				if strings.TrimSpace(earlierRule.Pattern) == trimmedPat {
					res.Warnings = append(res.Warnings, fmt.Sprintf(
						"rule %q is completely shadowed: pattern %q is identical to earlier rule %q",
						ruleName, trimmedPat, earlierRule.Name,
					))
					break
				}
			}
		}
	}
}

// containsString is a small helper checking if a string slice contains a value.
func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

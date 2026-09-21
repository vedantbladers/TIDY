package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tidy/config"
	"tidy/engine"

	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain <file>",
	Short: "Trace rule evaluation for a specific file without moving it",
	Long: `explain performs a dry simulation of how a specific file or path is evaluated against your configured rules.

It shows which ignore patterns or rules were checked, why a rule matched (by extension, glob pattern, or magic-byte sniffing fallback), and the planned destination directory. Zero filesystem modifications occur.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		baseName := filepath.Base(filePath)

		// 1. Load configuration
		expandedConfigPath, err := config.ExpandPath(CfgFile)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}

		cfg, err := config.LoadConfig(expandedConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config %s: %w", expandedConfigPath, err)
		}

		fmt.Printf("%s=== TIDY RULE TRACE FOR: %s%s\n\n", colorBold, baseName, colorReset)
		fmt.Printf("Input Path: %s\n", filePath)

		// 2. Check Ignore Patterns
		fmt.Printf("\n%s[Step 1] Ignore Filter Check:%s\n", colorBold, colorReset)
		ignored := false
		for _, pattern := range cfg.Ignore {
			matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(baseName))
			if matched {
				fmt.Printf("  %s✖ MATCHED IGNORE PATTERN: %q%s\n", colorRed, pattern, colorReset)
				fmt.Printf("  ➔ File is ignored and will never be moved.\n\n")
				ignored = true
				break
			}
		}

		if ignored {
			return nil
		}
		fmt.Printf("  %s✓ Not ignored%s (did not match any of %d ignore pattern(s))\n", colorGreen, colorReset, len(cfg.Ignore))

		// 3. Evaluate Rules in Sequence (First Match Wins)
		fmt.Printf("\n%s[Step 2] Rule Chain Evaluation (First match wins):%s\n", colorBold, colorReset)
		matchedRuleIndex := -1
		var matchedReason string

		for i, rule := range cfg.Rules {
			fmt.Printf("  Rule %d: %s%s%s\n", i+1, colorBold, rule.Name, colorReset)

			// Check pattern first
			if rule.Pattern != "" {
				patMatched, _ := filepath.Match(strings.ToLower(rule.Pattern), strings.ToLower(baseName))
				if patMatched {
					matchedRuleIndex = i
					matchedReason = fmt.Sprintf("Pattern glob %q matched filename", rule.Pattern)
					fmt.Printf("    %s✓ MATCH!%s %s\n", colorGreen, colorReset, matchedReason)
					break
				} else {
					fmt.Printf("    - Pattern %q did not match\n", rule.Pattern)
				}
			}

			// Check extensions
			if len(rule.Extensions) > 0 {
				extMatched := false
				lowerBase := strings.ToLower(baseName)
				for _, ext := range rule.Extensions {
					cleanExt := strings.ToLower(ext)
					if !strings.HasPrefix(cleanExt, ".") {
						cleanExt = "." + cleanExt
					}
					if strings.HasSuffix(lowerBase, cleanExt) {
						matchedRuleIndex = i
						matchedReason = fmt.Sprintf("Extension %q matched filename suffix", ext)
						fmt.Printf("    %s✓ MATCH!%s %s\n", colorGreen, colorReset, matchedReason)
						extMatched = true
						break
					}
				}
				if extMatched {
					break
				} else {
					fmt.Printf("    - Extensions %v did not match\n", rule.Extensions)
				}
			}
		}

		// 4. Content Sniffing Fallback (if no extension/pattern matched)
		if matchedRuleIndex == -1 && cfg.SniffContent {
			fmt.Printf("\n%s[Step 3] Magic-Byte Content Sniffing Fallback:%s\n", colorBold, colorReset)
			// Check if file actually exists on disk to sniff
			if _, err := os.Stat(filePath); err == nil {
				detectedExt, err := engine.SniffFileExtension(filePath)
				if err != nil {
					fmt.Printf("    %s⚠ Failed to read file header: %v%s\n", colorYellow, err, colorReset)
				} else if detectedExt != "" {
					fmt.Printf("    Detected true file type via magic bytes: %s%s%s\n", colorBold, detectedExt, colorReset)
					// Try matching detected extension against rules
					for i, rule := range cfg.Rules {
						for _, ext := range rule.Extensions {
							cleanExt := strings.ToLower(ext)
							if !strings.HasPrefix(cleanExt, ".") {
								cleanExt = "." + cleanExt
							}
							if cleanExt == detectedExt {
								matchedRuleIndex = i
								matchedReason = fmt.Sprintf("Magic bytes sniffed as %q matching rule extension", detectedExt)
								fmt.Printf("    %s✓ MATCH!%s Rule %d (%s): %s\n", colorGreen, colorReset, i+1, rule.Name, matchedReason)
								break
							}
						}
						if matchedRuleIndex != -1 {
							break
						}
					}
				} else {
					fmt.Printf("    Magic bytes could not identify a known file signature.\n")
				}
			} else {
				fmt.Printf("    (File not found on disk — skipping live magic-byte sniffing test)\n")
			}
		}

		// 5. Final Result Summary
		fmt.Printf("\n%s=== VERDICT ===%s\n", colorBold, colorReset)
		if matchedRuleIndex >= 0 {
			rule := cfg.Rules[matchedRuleIndex]
			expandedDest, err := config.ExpandPath(rule.Dest)
			if err != nil {
				expandedDest = rule.Dest
			}
			targetPath := filepath.Join(expandedDest, baseName)

			fmt.Printf("%sMatched Rule:%s %s\n", colorGreen, colorReset, rule.Name)
			fmt.Printf("%sReason:%s       %s\n", colorGreen, colorReset, matchedReason)
			fmt.Printf("%sDestination:%s  %s\n", colorGreen, colorReset, targetPath)

			// Check if file exists and is already in destination
			if absInput, err := filepath.Abs(filePath); err == nil {
				if absDest, err := filepath.Abs(targetPath); err == nil && absInput == absDest {
					fmt.Printf("%sNote:%s         File is already located in the destination folder. tidy will skip it during organizing.\n",
						colorYellow, colorReset)
				}
			}
		} else {
			fmt.Printf("%sNo rules matched.%s\n", colorYellow, colorReset)
			fmt.Printf("The file would remain in its current location.\n")
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(explainCmd)
}

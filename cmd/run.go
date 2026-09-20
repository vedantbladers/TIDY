package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"tidy/config"
	"tidy/db"
	"tidy/engine"
	"tidy/fs"
	"tidy/scanner"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Organize files in watch directories according to configured rules",
	Long: `run scans all configured watch directories (top-level only) and moves
matching files to their designated destination folders.

Use --dry-run to preview all planned operations without modifying the filesystem.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// 1. Resolve and load configuration
		targetPath := CfgFile
		if targetPath == "" {
			targetPath = config.DefaultConfigPath()
		}
		expandedConfigPath, err := config.ExpandPath(targetPath)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}

		cfg, err := config.LoadConfig(expandedConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// 2. Validate configuration before running
		validation := config.ValidateConfig(cfg)
		if !validation.IsValid() {
			return fmt.Errorf("configuration has fatal errors; run 'tidy validate' for details")
		}

		// 3. Initialize history ledger if not in dry-run mode
		var ledger *db.Ledger
		runID := uuid.New().String()

		if !DryRun {
			dbPath, err := config.ExpandPath("~/.config/tidy/history.db")
			if err != nil {
				return fmt.Errorf("invalid history db path: %w", err)
			}

			ledger, err = db.OpenLedger(dbPath)
			if err != nil {
				return fmt.Errorf("failed to open history ledger: %w", err)
			}
			defer ledger.Close()

			if err := ledger.StartRun(runID, "run"); err != nil {
				return fmt.Errorf("failed to initialize run in ledger: %w", err)
			}
		}

		// 4. Print run header
		if DryRun {
			fmt.Printf("%s=== DRY RUN MODE (no files will be moved) ===%s\n\n", colorYellow, colorReset)
		} else {
			fmt.Printf("%sStarting tidy run [%s]...%s\n\n", colorBold, runID[:8], colorReset)
		}

		totalScanned := 0
		totalMoved := 0

		// 5. Scan and organize each watch directory
		for _, watchDir := range cfg.WatchDirs {
			files, err := scanner.ScanTopLevel(watchDir)
			if err != nil {
				fmt.Printf("%s[ERROR] Failed to scan %s: %v%s\n", colorRed, watchDir, err, colorReset)
				continue
			}

			totalScanned += len(files)

			for _, file := range files {
				// Evaluate matching rules
				match := engine.MatchFile(file.Name, cfg)

				// Fallback: If no match and sniffing is enabled, try magic bytes
				if !match.Matched && !match.Ignored && cfg.SniffContent {
					sniffedExt, err := engine.SniffFileExtension(file.Path)
					if err == nil && sniffedExt != "" {
						match = engine.MatchFile(file.Name+sniffedExt, cfg)
					}
				}

				// Skip if ignored or no matching rule
				if match.Ignored || !match.Matched {
					continue
				}

				// Resolve target path and collision handling
				targetDest := filepath.Join(match.DestDir, file.Name)
				resolvedDest, shouldSkip, err := fs.ResolveCollision(targetDest, fs.StrategyRename)
				if err != nil {
					fmt.Printf("%s[ERROR] Collision check failed for %s: %v%s\n", colorRed, file.Name, err, colorReset)
					continue
				}
				if shouldSkip {
					fmt.Printf("%s[SKIP] %s already exists at destination%s\n", colorYellow, file.Name, colorReset)
					continue
				}

				// Execute move or simulate
				if DryRun {
					fmt.Printf("%s[DRY RUN]%s Would move: %s%s%s ➔ %s%s%s (%s)\n",
						colorYellow, colorReset,
						colorBold, file.Name, colorReset,
						colorGreen, resolvedDest, colorReset,
						match.RuleName,
					)
					totalMoved++
				} else {
					if err := fs.MoveFile(file.Path, resolvedDest); err != nil {
						fmt.Printf("%s[ERROR] Failed to move %s: %v%s\n", colorRed, file.Name, err, colorReset)
						continue
					}

					// Record move in ledger with cryptographic hash chaining
					if _, err := ledger.RecordMove(runID, file.Path, resolvedDest, file.Size); err != nil {
						fmt.Printf("%s[WARNING] Moved %s but failed to record in ledger: %v%s\n", colorYellow, file.Name, err, colorReset)
					}

					fmt.Printf("%s✓ Moved:%s %s ➔ %s%s%s (%s)\n",
						colorGreen, colorReset,
						file.Name,
						colorBold, filepath.Base(resolvedDest), colorReset,
						match.RuleName,
					)
					totalMoved++
				}
			}
		}

		// 6. Summary Footer
		elapsed := time.Since(startTime).Round(time.Millisecond)
		fmt.Println()
		if DryRun {
			fmt.Printf("%sDry run complete:%s %d of %d file(s) would be organized in %v\n",
				colorYellow, colorReset, totalMoved, totalScanned, elapsed)
		} else {
			fmt.Printf("%sRun complete:%s %d of %d file(s) organized in %v (Run ID: %s)\n",
				colorGreen, colorReset, totalMoved, totalScanned, elapsed, runID[:8])
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
}

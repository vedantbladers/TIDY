package cmd

import (
	"fmt"
	"path/filepath"

	"tidy/config"
	"tidy/db"

	"github.com/spf13/cobra"
)

var (
	logLimit  int
	logVerify bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Inspect organizing history and verify cryptographic ledger integrity",
	Long: `log displays chronological records of organized files from the SQLite ledger.

Use --verify to audit the SHA-256 hash chain and ensure no records have been tampered with.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Open the history ledger
		dbPath, err := config.ExpandPath("~/.config/tidy/history.db")
		if err != nil {
			return fmt.Errorf("invalid history db path: %w", err)
		}

		ledger, err := db.OpenLedger(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open history ledger: %w", err)
		}
		defer ledger.Close()

		// 2. Cryptographic Audit Mode (--verify)
		if logVerify {
			fmt.Printf("%sAuditing ledger cryptographic integrity...%s\n\n", colorBold, colorReset)
			valid, err := ledger.VerifyLedgerIntegrity()
			if err != nil || !valid {
				fmt.Printf("%s✖ INTEGRITY VIOLATION DETECTED!%s\n", colorRed, colorReset)
				fmt.Printf("  %v\n\n", err)
				return fmt.Errorf("ledger verification failed: database has been tampered with or corrupted")
			}

			fmt.Printf("%s✓ Ledger integrity verified: All cryptographic SHA-256 hash links are valid.%s\n",
				colorGreen, colorReset)
			fmt.Printf("%sZero tampering detected.%s\n", colorGreen, colorReset)
			return nil
		}

		// 3. Normal Log Mode: Display recent runs
		runs, err := ledger.GetRecentRuns(logLimit)
		if err != nil {
			return fmt.Errorf("failed to retrieve history: %w", err)
		}

		if len(runs) == 0 {
			fmt.Println("No organizing runs recorded in history yet.")
			return nil
		}

		fmt.Printf("%s=== TIDY HISTORY LOG (Showing last %d run(s)) ===%s\n\n", colorBold, len(runs), colorReset)

		for _, r := range runs {
			statusColor := colorGreen
			if r.Status == "rolled_back" {
				statusColor = colorYellow
			}

			runTag := r.RunID
			if len(runTag) > 8 {
				runTag = runTag[:8]
			}

			fmt.Printf("%sRun [%s]%s • %s • Trigger: %s • Status: %s%s%s\n",
				colorBold, runTag, colorReset,
				r.Timestamp, r.TriggerType,
				statusColor, r.Status, colorReset,
			)

			// Fetch operations for this run
			ops, err := ledger.GetAllOperationsForRun(r.RunID)
			if err != nil {
				fmt.Printf("  %s[ERROR] Failed to load operations: %v%s\n", colorRed, err, colorReset)
				continue
			}

			if len(ops) == 0 {
				fmt.Println("  (No files moved in this run)")
			}

			for _, op := range ops {
				stateIcon := "✓"
				if op.IsUndone {
					stateIcon = "↺ (undone)"
				}

				fmt.Printf("  %s %s ➔ %s%s%s (%d bytes)\n",
					stateIcon,
					filepath.Base(op.SourcePath),
					colorBold, op.DestPath, colorReset,
					op.FileSize,
				)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 10, "maximum number of recent runs to display")
	logCmd.Flags().BoolVar(&logVerify, "verify", false, "audit the cryptographic hash chain for tamper-evidence")
	RootCmd.AddCommand(logCmd)
}

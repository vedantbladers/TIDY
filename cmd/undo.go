package cmd

import (
	"fmt"

	"tidy/config"
	"tidy/db"
	"tidy/engine"

	"github.com/spf13/cobra"
)

var undoRunID string

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Reverse the latest organizing run or a specified run ID",
	Long: `undo restores organized files back to their original locations.

If --run-id is omitted, tidy automatically finds and rolls back the most recent active run.`,
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

		fmt.Printf("%sRolling back operations...%s\n\n", colorBold, colorReset)

		// 2. Execute rollback
		res, err := engine.UndoRun(ledger, undoRunID)
		if err != nil {
			return fmt.Errorf("%s[ERROR]%s %w", colorRed, colorReset, err)
		}

		// 3. Print any warnings encountered (e.g. files missing from destination)
		if len(res.Errors) > 0 {
			for _, e := range res.Errors {
				fmt.Printf("  %s⚠ %s%s\n", colorYellow, e, colorReset)
			}
			fmt.Println()
		}

		// 4. Print summary
		if res.FilesRestored > 0 {
			runTag := res.RunID
			if len(runTag) > 8 {
				runTag = runTag[:8]
			}
			fmt.Printf("%s✓ Successfully restored %d file(s) for run [%s]%s\n",
				colorGreen, res.FilesRestored, runTag, colorReset)
		} else {
			fmt.Printf("%sNo files were restored.%s\n", colorYellow, colorReset)
		}

		return nil
	},
}

func init() {
	undoCmd.Flags().StringVar(&undoRunID, "run-id", "", "specific run ID to undo (defaults to latest)")
	RootCmd.AddCommand(undoCmd)
}

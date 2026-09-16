package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// CfgFile stores the path to the YAML configuration file.
	// Can be overridden via the global --config flag.
	CfgFile string

	// DryRun indicates whether operations should simulate changes without touching the filesystem.
	// Can be enabled via the global --dry-run flag.
	DryRun bool
)

var RootCmd = &cobra.Command{
	Use:   "tidy",
	Short: "A deterministic, safe CLI folder organizer",
	Long: `tidy is a lightweight, zero-dependency CLI tool that keeps folders
organized using simple, auditable YAML rules.

It features first-class undo, a tamper-evident history ledger,
and guarantees top-level-only folder scanning so organized files
are never inadvertently touched.`,
}

func Execute() error {
	return RootCmd.Execute()
}

func init() {
	// PersistentFlags are available to 'tidy' itself and every child subcommand
	// (e.g., tidy run --dry-run, tidy watch --config=custom.yaml).
	RootCmd.PersistentFlags().StringVar(&CfgFile, "config", "~/.config/tidy/config.yaml", "path to YAML configuration file")
	RootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false, "preview planned moves without touching the filesystem")
}

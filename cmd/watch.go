package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tidy/config"
	"tidy/daemon"
	"tidy/db"

	"github.com/spf13/cobra"
)

var debounceDuration time.Duration

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Run tidy as a background daemon watching folders in real time",
	Long: `watch continuously monitors configured watch directories using kernel events.
Files are automatically moved to designated destinations as soon as they finish downloading/writing.

Press Ctrl+C or send SIGTERM to stop the daemon safely.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// 2. Validate configuration before launching daemon
		validation := config.ValidateConfig(cfg)
		if !validation.IsValid() {
			return fmt.Errorf("configuration has fatal errors; run 'tidy validate' for details")
		}

		// 3. Open history ledger
		dbPath, err := config.ExpandPath("~/.config/tidy/history.db")
		if err != nil {
			return fmt.Errorf("invalid history db path: %w", err)
		}

		ledger, err := db.OpenLedger(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open history ledger: %w", err)
		}
		defer ledger.Close()

		// 4. Create the filesystem watcher
		w, err := daemon.NewWatcher(cfg, ledger, debounceDuration)
		if err != nil {
			return fmt.Errorf("failed to start watcher: %w", err)
		}

		// 5. Setup graceful shutdown listener for Ctrl+C (SIGINT) and systemd (SIGTERM)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("%s[TIDY DAEMON ACTIVE]%s Watching %d director(y/ies) with %v debounce...\n",
			colorGreen, colorReset, len(cfg.WatchDirs), debounceDuration)
		for _, dir := range cfg.WatchDirs {
			fmt.Printf("  • %s\n", dir)
		}
		fmt.Printf("%sPress Ctrl+C to stop.%s\n\n", colorBold, colorReset)

		// 6. Run watcher (blocks until ctx is canceled)
		if err := w.Start(ctx); err != nil {
			return fmt.Errorf("watcher encountered error: %w", err)
		}

		fmt.Printf("\n%sDaemon stopped gracefully.%s\n", colorYellow, colorReset)
		return nil
	},
}

func init() {
	watchCmd.Flags().DurationVar(&debounceDuration, "debounce", 500*time.Millisecond, "debounce duration before organizing a modified file")
	RootCmd.AddCommand(watchCmd)
}

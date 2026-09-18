package cmd

import (
	"fmt"

	"tidy/config"

	"github.com/spf13/cobra"
)

// ANSI color escape codes for clean terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file and check for rule shadowing",
	Long: `validate loads the active configuration file and checks for:
  - Missing or non-existent watch directories
  - Invalid rule structures (missing names, missing destinations)
  - Inaccessible destination paths
  - Shadowed or duplicate rules that will never be reached`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Determine target config path
		targetPath := CfgFile
		if targetPath == "" {
			targetPath = config.DefaultConfigPath()
		}

		expandedPath, err := config.ExpandPath(targetPath)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		fmt.Printf("Validating configuration: %s%s%s\n\n", colorBold, expandedPath, colorReset)

		// 2. Load the configuration
		cfg, err := config.LoadConfig(expandedPath)
		if err != nil {
			return fmt.Errorf("%s[ERROR] Failed to load config:%s %w", colorRed, colorReset, err)
		}

		// 3. Run semantic validation and rule shadowing detection
		result := config.ValidateConfig(cfg)

		// 4. Print warnings (if any)
		if len(result.Warnings) > 0 {
			fmt.Printf("%sWarnings (%d):%s\n", colorYellow, len(result.Warnings), colorReset)
			for _, w := range result.Warnings {
				fmt.Printf("  %s⚠ %s%s\n", colorYellow, w, colorReset)
			}
			fmt.Println()
		}

		// 5. Print fatal errors (if any) and exit with failure
		if !result.IsValid() {
			fmt.Printf("%sErrors (%d):%s\n", colorRed, len(result.Errors), colorReset)
			for _, e := range result.Errors {
				fmt.Printf("  %s✖ %s%s\n", colorRed, e, colorReset)
			}
			fmt.Println()
			return fmt.Errorf("configuration validation failed with %d error(s)", len(result.Errors))
		}

		// 6. Success message
		if len(result.Warnings) > 0 {
			fmt.Printf("%s✓ Configuration is valid (with warnings).%s\n", colorGreen, colorReset)
		} else {
			fmt.Printf("%s✓ Configuration is valid and ready to use!%s\n", colorGreen, colorReset)
		}

		return nil
	},
}

func init() {
	// Register 'tidy validate' with the root command
	RootCmd.AddCommand(validateCmd)
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"tidy/config"

	"github.com/spf13/cobra"
)

// The default starter configuration template written by 'tidy init'.
const defaultConfigTemplate = `# tidy default configuration
# Default location: ~/.config/tidy/config.yaml

# Directories to scan and organize (Top-level only, never recursive)
watch_dirs:
  - ~/Downloads

# Optional content-sniffing fallback using magic bytes (true/false)
sniff_content: false

# Patterns to ignore during scans and active debounce windows
ignore:
  - "*.crdownload"

  - "*.part"

  - "*.tmp"

  - ".DS_Store"

  - "desktop.ini"
  
  - "*.download"

# Ordered rule definitions (First matching rule wins!)
rules:
  - name: Documents
    extensions: [".pdf", ".doc", ".docx", ".txt", ".md", ".csv"]
    dest: ~/Downloads/Documents

  - name: Screenshots
    pattern: "Screenshot*"
    dest: ~/Downloads/Images/Screenshots

  - name: Images
    extensions: [".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"]
    dest: ~/Downloads/Images

  - name: Archives
    extensions: [".zip", ".rar", ".7z", ".tar", ".tar.gz", ".tgz"]
    dest: ~/Downloads/Compressed

  - name: Torrents
    extensions: [".torrent"]
    dest: ~/Downloads/Torrents
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter configuration file",
	Long: `init writes a starter configuration file to ~/.config/tidy/config.yaml.
If a configuration file already exists at that location, it will not be overwritten.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Determine destination path (from --config flag or default)
		targetPath := CfgFile
		if targetPath == "" {
			targetPath = config.DefaultConfigPath()
		}

		expandedPath, err := config.ExpandPath(targetPath)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		// 2. Check if file already exists (non-destructive safety guard)
		if _, err := os.Stat(expandedPath); err == nil {
			fmt.Printf("Configuration file already exists at: %s\nDoing nothing.\n", expandedPath)
			return nil
		}

		// 3. Create parent directories if missing (e.g. ~/.config/tidy)
		dir := filepath.Dir(expandedPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// 4. Write starter config template
		if err := os.WriteFile(expandedPath, []byte(defaultConfigTemplate), 0644); err != nil {
			return fmt.Errorf("failed to write config to %s: %w", expandedPath, err)
		}

		fmt.Printf("Successfully created starter configuration at: %s\n", expandedPath)
		return nil
	},
}

func init() {
	// Register 'tidy init' with the root command
	RootCmd.AddCommand(initCmd)
}

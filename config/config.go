package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	Name       string   `yaml:"name"`
	Extensions []string `yaml:"extensions,omitempty"`
	Pattern    string   `yaml:"pattern,omitempty"`
	Dest       string   `yaml:"dest"`
}

type Config struct {
	WatchDirs    []string `yaml:"watch_dirs"`
	Ignore       []string `yaml:"ignore,omitempty"`
	SniffContent bool     `yaml:"sniff_content,omitempty"`
	Rules        []Rule   `yaml:"rules"`
}

func DefaultConfigPath() string {
	return "~/.config/tidy/config.yaml"
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()

		if err != nil {
			return "", fmt.Errorf("failed to get user home directory %w", err)
		}

		if path == "~" {
			return homeDir, nil
		}

		path = filepath.Join(homeDir, path[2:])
	}
	return filepath.Clean(path), nil
}

// LoadConfig reads, expands, and unmarshals the YAML configuration from the given path.
// If path is empty, it falls back to DefaultConfigPath().
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate that we have at least one watch directory
	if len(cfg.WatchDirs) == 0 {
		return nil, fmt.Errorf("no watch directories defined")
	}

	// Ensure all rules have at least one condition
	for _, rule := range cfg.Rules {
		if len(rule.Extensions) == 0 && rule.Pattern == "" {
			return nil, fmt.Errorf("rule '%s' has no conditions (no extensions or pattern)", rule.Name)
		}
	}

	for i, dir := range cfg.WatchDirs {
		expandedDir, err := ExpandPath(dir)
		if err != nil {
			return nil, fmt.Errorf("invalid watch directory '%s' at index %d: %w", dir, i, err)
		}
		cfg.WatchDirs[i] = expandedDir
	}

	// Expand tilde in all rule destination paths
	for i, rule := range cfg.Rules {
		expandedDest, err := ExpandPath(rule.Dest)
		if err != nil {
			return nil, fmt.Errorf("invalid destination path '%s' in rule '%s': %w", rule.Dest, rule.Name, err)
		}
		cfg.Rules[i].Dest = expandedDest
	}

	return &cfg, nil
}

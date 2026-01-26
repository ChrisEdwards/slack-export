package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/spf13/cobra"
)

// Version information, injected at build time via ldflags.
var (
	Version   = "dev"
	Build     = "unknown"
	BuildTime = "unknown"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "slack-export",
	Short: "Export Slack channel logs to dated markdown files",
	Long: `slack-export is a CLI tool that exports Slack channel logs to dated markdown files.

It uses the Slack Edge API for fast channel detection and slackdump for message export.
Configuration is via YAML file with glob-based channel include/exclude patterns.`,
	Version: fmt.Sprintf("%s (build %s, %s)", Version, Build, BuildTime),
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Long:  `Display all configuration settings, their current values, and the source config file.`,
	RunE:  runConfig,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./slack-export.yaml)")
	rootCmd.AddCommand(configCmd)
}

func runConfig(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Configuration:")
	fmt.Printf("  Output Directory: %s\n", cfg.OutputDir)
	fmt.Printf("  Timezone:         %s\n", cfg.Timezone)
	fmt.Printf("  Include patterns: %s\n", formatPatterns(cfg.Include))
	fmt.Printf("  Exclude patterns: %s\n", formatPatterns(cfg.Exclude))
	if cfg.SlackdumpPath != "" {
		fmt.Printf("  Slackdump path:   %s\n", cfg.SlackdumpPath)
	} else {
		fmt.Println("  Slackdump path:   (not set, will use PATH)")
	}
	fmt.Println()
	if cfg.ConfigFile() != "" {
		fmt.Printf("Config file: %s\n", cfg.ConfigFile())
	} else {
		fmt.Println("Config file: (none - using defaults)")
	}

	return nil
}

func formatPatterns(patterns []string) string {
	if len(patterns) == 0 {
		return "(none)"
	}
	return "[" + strings.Join(patterns, ", ") + "]"
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

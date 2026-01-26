package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information, injected at build time via ldflags.
var (
	Version   = "dev"
	Build     = "unknown"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "slack-export",
	Short: "Export Slack channel logs to dated markdown files",
	Long: `slack-export is a CLI tool that exports Slack channel logs to dated markdown files.

It uses the Slack Edge API for fast channel detection and slackdump for message export.
Configuration is via YAML file with glob-based channel include/exclude patterns.`,
	Version: fmt.Sprintf("%s (build %s, %s)", Version, Build, BuildTime),
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

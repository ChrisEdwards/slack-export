package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/export"
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

var exportCmd = &cobra.Command{
	Use:   "export [date]",
	Short: "Export Slack logs for a date or date range",
	Long: `Export Slack channel logs for a specific date or date range.

Examples:
  slack-export export 2026-01-22               # Export single date
  slack-export export --from 2026-01-15        # From date to today
  slack-export export --from 2026-01-15 --to 2026-01-20  # Date range`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExport,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync Slack logs from last export to today",
	Long: `Automatically detect the most recent export date and sync from there to today.

The command scans the output directory for dated folders (YYYY-MM-DD pattern),
finds the most recent date, and re-exports from that date through today.
If no previous exports exist, it defaults to yesterday.

The last export date is re-exported because it may have been incomplete.`,
	RunE: runSync,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./slack-export.yaml)")
	rootCmd.AddCommand(configCmd)

	exportCmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	exportCmd.Flags().String("to", "", "End date (YYYY-MM-DD), defaults to today")
	rootCmd.AddCommand(exportCmd)

	rootCmd.AddCommand(syncCmd)
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

func runExport(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	exporter, err := export.NewExporter(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize exporter: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(args) == 1 {
		return exporter.ExportDate(ctx, args[0])
	}

	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	if from == "" {
		return errors.New("specify a date argument or use --from flag")
	}

	if to == "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
		to = time.Now().In(loc).Format("2006-01-02")
	}

	return exporter.ExportRange(ctx, from, to)
}

func runSync(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}

	lastDate, err := findLastExportDate(cfg.OutputDir)
	if err != nil {
		return fmt.Errorf("scanning output directory: %w", err)
	}

	if lastDate == "" {
		lastDate = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
		fmt.Printf("No previous exports found, starting from %s\n", lastDate)
	} else {
		fmt.Printf("Last export: %s\n", lastDate)
	}

	today := time.Now().In(loc).Format("2006-01-02")
	fmt.Printf("Syncing from %s to %s\n", lastDate, today)

	exporter, err := export.NewExporter(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize exporter: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return exporter.ExportRange(ctx, lastDate, today)
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func findLastExportDate(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() && datePattern.MatchString(entry.Name()) {
			dates = append(dates, entry.Name())
		}
	}

	if len(dates) == 0 {
		return "", nil
	}

	sort.Strings(dates)
	return dates[len(dates)-1], nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

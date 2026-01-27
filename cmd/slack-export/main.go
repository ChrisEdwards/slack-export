package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/chrisedwards/slack-export/internal/channels"
	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/export"
	"github.com/chrisedwards/slack-export/internal/slack"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List active Slack channels",
	Long: `List active Slack channels for debugging and pattern discovery.

This command helps discover channel names to configure include/exclude patterns.
Include and exclude patterns from the configuration are applied to the output.

Examples:
  slack-export channels                      # All channels
  slack-export channels --since 2026-01-20   # Channels with recent activity`,
	RunE: runChannels,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up slack-export with guided wizard",
	Long: `Interactive setup wizard for first-time configuration.

Walks through:
  - Installing slackdump (if needed)
  - Authenticating with Slack (if needed)
  - Creating configuration file
  - Verifying the setup works`,
	RunE: runInit,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.config/slack-export/slack-export.yaml)")
	rootCmd.AddCommand(configCmd)

	exportCmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	exportCmd.Flags().String("to", "", "End date (YYYY-MM-DD), defaults to today")
	rootCmd.AddCommand(exportCmd)

	rootCmd.AddCommand(syncCmd)

	channelsCmd.Flags().String("since", "", "Only show channels with activity since this date (YYYY-MM-DD)")
	rootCmd.AddCommand(channelsCmd)

	initCmd.Flags().Bool("force", false, "Skip config exists warning, still shows form with current values")
	rootCmd.AddCommand(initCmd)
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

func runChannels(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	creds, err := slack.LoadCredentials()
	if err != nil {
		if credErr := slack.GetCredentialError(err); credErr != nil {
			fmt.Fprintln(os.Stderr, credErr.UserMessage())
			os.Exit(1)
		}
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	if err := creds.Validate(); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	client := slack.NewEdgeClient(creds)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// AuthTest verifies credentials and sets the TeamID needed for Edge API calls
	if _, err := client.AuthTest(ctx); err != nil {
		return fmt.Errorf("verifying credentials: %w", err)
	}

	var since time.Time
	sinceStr, _ := cmd.Flags().GetString("since")
	if sinceStr != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
		since, err = time.ParseInLocation("2006-01-02", sinceStr, loc)
		if err != nil {
			return fmt.Errorf("invalid since date: %w", err)
		}
	}

	userIndex, err := client.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	// Set up external user cache for Slack Connect users
	cache := slack.NewUserCache(slack.DefaultCachePath())
	if err := cache.Load(); err != nil {
		return fmt.Errorf("loading user cache: %w", err)
	}

	resolver := slack.NewUserResolver(userIndex, cache, client)

	chans, err := client.GetActiveChannelsWithResolver(ctx, since, resolver)
	if err != nil {
		return fmt.Errorf("getting channels: %w", err)
	}

	// Save cache after successful fetch (may have new external users)
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save user cache: %v\n", err)
	}

	chans = channels.FilterChannels(chans, cfg.Include, cfg.Exclude)

	sort.Slice(chans, func(i, j int) bool {
		return chans[i].Name < chans[j].Name
	})

	for _, ch := range chans {
		fmt.Printf("%-12s  %s\n", ch.ID, ch.Name)
	}
	fmt.Printf("\n%d channels\n", len(chans))

	return nil
}

func runInit(_ *cobra.Command, _ []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("init requires an interactive terminal")
	}

	// Step 1: Check for slackdump
	if err := initStepSlackdump(); err != nil {
		return err
	}

	// Step 2: Check authentication
	authSkipped, workspace, err := initStepAuth()
	if err != nil {
		return err
	}

	// Step 3: Configuration form
	cfg, configPath, err := initStepConfig()
	if err != nil {
		return err
	}

	// Step 4: Verification and summary
	return initStepVerify(cfg, configPath, authSkipped, workspace)
}

func initStepSlackdump() error {
	fmt.Println("Step 1/4: Checking for slackdump...")

	path, err := export.FindSlackdump()
	if err == nil {
		fmt.Printf("✓ Found slackdump at %s\n\n", path)
		return nil
	}

	// slackdump not found, prompt to install
	fmt.Println("slackdump not found")
	fmt.Println()
	fmt.Println("slackdump is required to export Slack data.")
	fmt.Println()

	var install bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Install slackdump now?").
				Affirmative("Yes, install").
				Negative("No, I'll install manually").
				Value(&install),
		),
	)

	if err := form.Run(); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if !install {
		fmt.Println()
		fmt.Println("To install slackdump manually, run:")
		fmt.Println("  go install github.com/rusq/slackdump/v3/cmd/slackdump@latest")
		fmt.Println()
		fmt.Println("Make sure $GOPATH/bin is in your PATH.")
		return errors.New("slackdump required but not installed")
	}

	// Install slackdump
	fmt.Println()
	fmt.Println("Installing slackdump...")

	cmd := exec.Command("go", "install", "github.com/rusq/slackdump/v3/cmd/slackdump@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println()
		fmt.Println("Installation failed. Make sure Go is installed and $GOPATH/bin is in your PATH.")
		return fmt.Errorf("failed to install slackdump: %w", err)
	}

	// Verify installation
	path, err = exec.LookPath("slackdump")
	if err != nil {
		fmt.Println()
		fmt.Println("slackdump was installed but not found in PATH.")
		fmt.Println("Add $GOPATH/bin to your PATH and try again.")
		return errors.New("slackdump not in PATH after installation")
	}

	fmt.Printf("✓ Installed slackdump at %s\n\n", path)
	return nil
}

// initStepAuth checks for valid Slack authentication.
// Returns (authSkipped, workspace, error).
func initStepAuth() (bool, string, error) {
	fmt.Println("Step 2/4: Checking Slack authentication...")

	creds, err := slack.LoadCredentials()
	if err == nil {
		if err := creds.Validate(); err == nil {
			fmt.Printf("✓ Authenticated to workspace: %s\n\n", creds.Workspace)
			return false, creds.Workspace, nil
		}
	}

	// Auth not valid, prompt user
	fmt.Println("Slack authentication required")
	fmt.Println()
	fmt.Println("slackdump needs to authenticate with your Slack workspace.")
	fmt.Println("This will open a browser for you to sign in.")
	fmt.Println()

	var authenticate bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Authenticate now?").
				Affirmative("Authenticate now").
				Negative("Skip for now").
				Value(&authenticate),
		),
	)

	if err := form.Run(); err != nil {
		return false, "", fmt.Errorf("prompt failed: %w", err)
	}

	if !authenticate {
		fmt.Println()
		fmt.Println("You can authenticate later with: slackdump auth")
		fmt.Println()
		return true, "", nil
	}

	// Run slackdump auth
	fmt.Println()
	fmt.Println("Running slackdump auth... (follow the prompts)")
	fmt.Println()

	slackdumpPath, err := export.FindSlackdump()
	if err != nil {
		return false, "", fmt.Errorf("slackdump not found: %w", err)
	}

	// #nosec G204 -- slackdumpPath comes from FindSlackdump, not untrusted input
	cmd := exec.Command(slackdumpPath, "auth")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, "", fmt.Errorf("slackdump auth failed: %w", err)
	}

	// Verify credentials now work
	creds, err = slack.LoadCredentials()
	if err != nil {
		fmt.Println()
		fmt.Println("Authentication completed but credentials could not be loaded.")
		fmt.Println("Try running 'slackdump auth' manually.")
		return false, "", fmt.Errorf("credentials not found after auth: %w", err)
	}

	if err := creds.Validate(); err != nil {
		return false, "", fmt.Errorf("credentials invalid after auth: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Authenticated to workspace: %s\n\n", creds.Workspace)
	return false, creds.Workspace, nil
}

// initStepConfig prompts for configuration and saves it.
// Returns (config, configPath, error).
func initStepConfig() (*config.Config, string, error) {
	fmt.Println("Step 3/4: Configuring slack-export...")

	configPath := config.DefaultConfigPath()
	fmt.Printf("Config will be saved to: %s\n\n", configPath)

	// Load existing config for defaults
	existingCfg, _ := config.Load("")

	// Default values
	outputDir := "./slack-logs"
	timezone := "America/New_York"

	if existingCfg != nil {
		if existingCfg.OutputDir != "" {
			outputDir = existingCfg.OutputDir
		}
		if existingCfg.Timezone != "" {
			timezone = existingCfg.Timezone
		}
	}

	// Detect system timezone
	detectedTZ := detectTimezone()
	if detectedTZ != "" {
		timezone = detectedTZ
	}

	// Prompt for output directory
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Output directory").
				Description("Where to save exported Slack logs").
				Placeholder(outputDir).
				Value(&outputDir),
		),
	)

	if err := form.Run(); err != nil {
		return nil, "", fmt.Errorf("prompt failed: %w", err)
	}

	// Expand to absolute path
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, "", fmt.Errorf("invalid path: %w", err)
	}
	outputDir = absOutputDir

	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		fmt.Printf("\nDirectory doesn't exist: %s\n", outputDir)

		var createDir bool
		createForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Create directory now?").
					Affirmative("Yes, create it").
					Negative("No, I'll create it later").
					Value(&createDir),
			),
		)

		if err := createForm.Run(); err != nil {
			return nil, "", fmt.Errorf("prompt failed: %w", err)
		}

		if createDir {
			if err := os.MkdirAll(outputDir, 0750); err != nil {
				return nil, "", fmt.Errorf("failed to create directory: %w", err)
			}
			fmt.Printf("✓ Created %s\n", outputDir)
		}
	}

	// Prompt for timezone
	var useDetectedTZ bool
	tzForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Use timezone %s?", timezone)).
				Affirmative("Yes").
				Negative("No, choose another").
				Value(&useDetectedTZ),
		),
	)

	if err := tzForm.Run(); err != nil {
		return nil, "", fmt.Errorf("prompt failed: %w", err)
	}

	if !useDetectedTZ {
		timezones := []string{
			"America/New_York",
			"America/Chicago",
			"America/Denver",
			"America/Los_Angeles",
			"Europe/London",
			"Europe/Paris",
			"Asia/Tokyo",
			"UTC",
		}

		options := make([]huh.Option[string], len(timezones))
		for i, tz := range timezones {
			options[i] = huh.NewOption(tz, tz)
		}

		selectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select timezone").
					Options(options...).
					Value(&timezone),
			),
		)

		if err := selectForm.Run(); err != nil {
			return nil, "", fmt.Errorf("prompt failed: %w", err)
		}
	}

	// Create and save config
	cfg := &config.Config{
		OutputDir: outputDir,
		Timezone:  timezone,
	}

	if err := cfg.Save(configPath); err != nil {
		return nil, "", fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Config saved to %s\n\n", configPath)
	return cfg, configPath, nil
}

// detectTimezone attempts to detect the system timezone.
func detectTimezone() string {
	// Try TZ environment variable first
	if tz := os.Getenv("TZ"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}

	// On Unix systems, check /etc/localtime symlink
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		// Extract timezone from path like /usr/share/zoneinfo/America/New_York
		if _, tz, found := strings.Cut(target, "zoneinfo/"); found {
			if _, err := time.LoadLocation(tz); err == nil {
				return tz
			}
		}
	}

	return ""
}

// initStepVerify verifies the setup and prints a summary.
func initStepVerify(cfg *config.Config, configPath string, authSkipped bool, workspace string) error {
	fmt.Println("Step 4/4: Verifying setup...")

	// Try to verify connection if auth wasn't skipped
	if !authSkipped {
		creds, err := slack.LoadCredentials()
		if err == nil {
			if err := creds.Validate(); err == nil {
				client := slack.NewEdgeClient(creds)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// AuthTest verifies credentials and sets TeamID
				if _, err := client.AuthTest(ctx); err == nil {
					workspace = creds.Workspace

					// Fetch users for DM name resolution
					userIndex, _ := client.FetchUsers(ctx)

					// Set up external user cache for Slack Connect users
					cache := slack.NewUserCache(slack.DefaultCachePath())
					_ = cache.Load() // Ignore error - verification only

					resolver := slack.NewUserResolver(userIndex, cache, client)

					// Fetch channels
					chans, err := client.GetActiveChannelsWithResolver(ctx, time.Time{}, resolver)
					if err == nil {
						// Save cache after successful fetch
						_ = cache.Save()
						fmt.Printf("✓ Connected to workspace: %s\n", workspace)
						fmt.Printf("✓ Found %d channels", len(chans))

						// Show first 5 channel names
						limit := min(5, len(chans))
						if limit > 0 {
							fmt.Printf(" (showing first %d):\n", limit)
							for i := range limit {
								fmt.Printf("    #%s\n", chans[i].Name)
							}
						} else {
							fmt.Println()
						}
					}
				}
			}
		}
	}

	// Print summary
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()

	if authSkipped {
		fmt.Println("Setup partially complete.")
		fmt.Println()
		fmt.Println("⚠ Authentication skipped - run 'slackdump auth' before exporting")
	} else {
		fmt.Println("Setup complete!")
	}

	fmt.Println()
	fmt.Printf("Config saved to: %s\n", configPath)
	fmt.Printf("Output directory: %s\n", cfg.OutputDir)
	fmt.Printf("Timezone: %s\n", cfg.Timezone)
	if workspace != "" {
		fmt.Printf("Workspace: %s\n", workspace)
	}

	fmt.Println()
	fmt.Println("To customize include/exclude patterns, edit the config file.")
	fmt.Println()
	fmt.Println("Try these commands:")
	fmt.Println("  slack-export channels          List your Slack channels")
	fmt.Println("  slack-export export 2026-01-25 Export a specific date")
	fmt.Println("  slack-export sync              Sync recent activity")
	fmt.Println()
	fmt.Println("Run 'slack-export --help' to see all available commands.")
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

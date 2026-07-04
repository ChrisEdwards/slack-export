package main

import (
	"os"
	"testing"
)

func TestExportCmd_Flags(t *testing.T) {
	// Verify the export command has the expected flags
	fromFlag := exportCmd.Flags().Lookup("from")
	if fromFlag == nil {
		t.Error("export command should have --from flag")
	}

	toFlag := exportCmd.Flags().Lookup("to")
	if toFlag == nil {
		t.Error("export command should have --to flag")
	}
}

func TestExportCmd_Args(t *testing.T) {
	// Verify maximum args is 1
	if err := exportCmd.Args(exportCmd, []string{"2026-01-22"}); err != nil {
		t.Errorf("export command should accept 1 arg: %v", err)
	}

	if err := exportCmd.Args(exportCmd, []string{}); err != nil {
		t.Errorf("export command should accept 0 args: %v", err)
	}

	if err := exportCmd.Args(exportCmd, []string{"2026-01-22", "2026-01-23"}); err == nil {
		t.Error("export command should reject more than 1 arg")
	}
}

func TestConfigCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("config command should be registered with root")
	}
}

func TestExportCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "export" {
			found = true
			break
		}
	}
	if !found {
		t.Error("export command should be registered with root")
	}
}

func TestRootCmd_GlobalConfigFlag(t *testing.T) {
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("root command should have --config persistent flag")
	}

	if configFlag.Shorthand != "c" {
		t.Errorf("config flag shorthand = %q, want 'c'", configFlag.Shorthand)
	}
}

func TestFormatPatterns_Empty(t *testing.T) {
	result := formatPatterns(nil)
	if result != "(none)" {
		t.Errorf("formatPatterns(nil) = %q, want (none)", result)
	}

	result = formatPatterns([]string{})
	if result != "(none)" {
		t.Errorf("formatPatterns([]) = %q, want (none)", result)
	}
}

func TestFormatPatterns_Single(t *testing.T) {
	result := formatPatterns([]string{"general"})
	if result != "[general]" {
		t.Errorf("formatPatterns([general]) = %q, want [general]", result)
	}
}

func TestFormatPatterns_Multiple(t *testing.T) {
	result := formatPatterns([]string{"general", "random", "team-*"})
	expected := "[general, random, team-*]"
	if result != expected {
		t.Errorf("formatPatterns() = %q, want %q", result, expected)
	}
}

func TestExportCmd_UsageAndHelp(t *testing.T) {
	if exportCmd.Use != "export [date]" {
		t.Errorf("export Use = %q, want 'export [date]'", exportCmd.Use)
	}

	if exportCmd.Short == "" {
		t.Error("export command should have Short description")
	}

	if exportCmd.Long == "" {
		t.Error("export command should have Long description")
	}
}

func TestSyncCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sync command should be registered with root")
	}
}

func TestSyncCmd_UsageAndHelp(t *testing.T) {
	if syncCmd.Use != "sync" {
		t.Errorf("sync Use = %q, want 'sync'", syncCmd.Use)
	}

	if syncCmd.Short == "" {
		t.Error("sync command should have Short description")
	}

	if syncCmd.Long == "" {
		t.Error("sync command should have Long description")
	}
}

func TestRenderCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "render" {
			found = true
			break
		}
	}
	if !found {
		t.Error("render command should be registered with root")
	}
}

func TestRenderCmd_FullFlag(t *testing.T) {
	fullFlag := renderCmd.Flags().Lookup("full")
	if fullFlag == nil {
		t.Error("render command should have --full flag")
	}
}

func TestChannelsCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "channels" {
			found = true
			break
		}
	}
	if !found {
		t.Error("channels command should be registered with root")
	}
}

func TestChannelsCmd_UsageAndHelp(t *testing.T) {
	if channelsCmd.Use != "channels" {
		t.Errorf("channels Use = %q, want 'channels'", channelsCmd.Use)
	}

	if channelsCmd.Short == "" {
		t.Error("channels command should have Short description")
	}

	if channelsCmd.Long == "" {
		t.Error("channels command should have Long description")
	}
}

func TestChannelsCmd_SinceFlag(t *testing.T) {
	sinceFlag := channelsCmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Error("channels command should have --since flag")
	}
}

func TestInitCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("init command should be registered with root")
	}
}

func TestInitCmd_UsageAndHelp(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("init Use = %q, want 'init'", initCmd.Use)
	}

	if initCmd.Short == "" {
		t.Error("init command should have Short description")
	}

	if initCmd.Long == "" {
		t.Error("init command should have Long description")
	}
}

func TestInitCmd_ForceFlag(t *testing.T) {
	forceFlag := initCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("init command should have --force flag")
	}
}

func TestDetectTimezone_TZEnvVar(t *testing.T) {
	t.Setenv("TZ", "Europe/Paris")

	tz := detectTimezone()
	if tz != "Europe/Paris" {
		t.Errorf("detectTimezone() = %q, want Europe/Paris when TZ is set", tz)
	}
}

func TestDetectTimezone_InvalidTZ(t *testing.T) {
	t.Setenv("TZ", "Invalid/NotATimezone")

	// Should not return the invalid timezone
	tz := detectTimezone()
	if tz == "Invalid/NotATimezone" {
		t.Error("detectTimezone() should not return invalid timezone")
	}
}

func TestDetectTimezone_EmptyTZ(t *testing.T) {
	t.Setenv("TZ", "")

	// Should fallback to other detection methods or return empty
	tz := detectTimezone()
	// Either returns a valid timezone from /etc/localtime or empty string
	if tz != "" {
		// Verify it's a valid timezone
		_, err := os.Stat("/etc/localtime")
		if err != nil {
			t.Errorf("detectTimezone() = %q but /etc/localtime doesn't exist", tz)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
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

func TestFindLastExportDate_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	date, err := findLastExportDate(tmpDir)
	if err != nil {
		t.Errorf("findLastExportDate() error = %v", err)
	}
	if date != "" {
		t.Errorf("findLastExportDate() = %q, want empty string for empty dir", date)
	}
}

func TestFindLastExportDate_NonExistentDir(t *testing.T) {
	date, err := findLastExportDate("/nonexistent/dir/path")
	if err != nil {
		t.Errorf("findLastExportDate() error = %v, want nil for non-existent dir", err)
	}
	if date != "" {
		t.Errorf("findLastExportDate() = %q, want empty string for non-existent dir", date)
	}
}

func TestFindLastExportDate_NoDatedFolders(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "random-folder"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "another-folder"), 0750); err != nil {
		t.Fatal(err)
	}

	date, err := findLastExportDate(tmpDir)
	if err != nil {
		t.Errorf("findLastExportDate() error = %v", err)
	}
	if date != "" {
		t.Errorf("findLastExportDate() = %q, want empty string for no dated folders", date)
	}
}

func TestFindLastExportDate_SingleDate(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "2026-01-22"), 0750); err != nil {
		t.Fatal(err)
	}

	date, err := findLastExportDate(tmpDir)
	if err != nil {
		t.Errorf("findLastExportDate() error = %v", err)
	}
	if date != "2026-01-22" {
		t.Errorf("findLastExportDate() = %q, want 2026-01-22", date)
	}
}

func TestFindLastExportDate_MultipleDates(t *testing.T) {
	tmpDir := t.TempDir()
	dates := []string{"2026-01-15", "2026-01-22", "2026-01-18"}
	for _, d := range dates {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0750); err != nil {
			t.Fatal(err)
		}
	}

	date, err := findLastExportDate(tmpDir)
	if err != nil {
		t.Errorf("findLastExportDate() error = %v", err)
	}
	if date != "2026-01-22" {
		t.Errorf("findLastExportDate() = %q, want 2026-01-22 (most recent)", date)
	}
}

func TestFindLastExportDate_MixedContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Dated folders
	if err := os.MkdirAll(filepath.Join(tmpDir, "2026-01-20"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "2026-01-22"), 0750); err != nil {
		t.Fatal(err)
	}

	// Non-dated folders
	if err := os.MkdirAll(filepath.Join(tmpDir, "other-folder"), 0750); err != nil {
		t.Fatal(err)
	}

	// Files (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "2026-01-25"), []byte("file"), 0640); err != nil {
		t.Fatal(err)
	}

	// Invalid date format folders
	if err := os.MkdirAll(filepath.Join(tmpDir, "26-01-22"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "2026-1-22"), 0750); err != nil {
		t.Fatal(err)
	}

	date, err := findLastExportDate(tmpDir)
	if err != nil {
		t.Errorf("findLastExportDate() error = %v", err)
	}
	if date != "2026-01-22" {
		t.Errorf("findLastExportDate() = %q, want 2026-01-22", date)
	}
}

func TestDatePattern(t *testing.T) {
	valid := []string{
		"2026-01-22",
		"2025-12-31",
		"2000-01-01",
		"1999-06-15",
	}
	for _, d := range valid {
		if !datePattern.MatchString(d) {
			t.Errorf("datePattern should match %q", d)
		}
	}

	invalid := []string{
		"26-01-22",
		"2026-1-22",
		"2026-01-2",
		"2026/01/22",
		"20260122",
		"2026-01-22-extra",
		"prefix-2026-01-22",
	}
	for _, d := range invalid {
		if datePattern.MatchString(d) {
			t.Errorf("datePattern should NOT match %q", d)
		}
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

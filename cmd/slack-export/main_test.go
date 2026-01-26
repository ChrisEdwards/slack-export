package main

import (
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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OutputDir != "./slack-logs" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "./slack-logs")
	}
	if cfg.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "America/New_York")
	}
}

func TestLoad_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-config.yaml")

	content := `output_dir: "/custom/path"
timezone: "Europe/London"
include:
  - "eng-*"
  - "team-*"
exclude:
  - "*-archive"
slackdump_path: "/usr/local/bin/slackdump"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OutputDir != "/custom/path" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "/custom/path")
	}
	if cfg.Timezone != "Europe/London" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "Europe/London")
	}
	if len(cfg.Include) != 2 {
		t.Errorf("len(Include) = %d, want 2", len(cfg.Include))
	}
	if len(cfg.Exclude) != 1 {
		t.Errorf("len(Exclude) = %d, want 1", len(cfg.Exclude))
	}
	if cfg.SlackdumpPath != "/usr/local/bin/slackdump" {
		t.Errorf("SlackdumpPath = %q, want %q", cfg.SlackdumpPath, "/usr/local/bin/slackdump")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("SLACK_EXPORT_OUTPUT_DIR", "/env/override/path")
	t.Setenv("SLACK_EXPORT_TIMEZONE", "UTC")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OutputDir != "/env/override/path" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "/env/override/path")
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "UTC")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-config.yaml")

	content := `output_dir: "/file/path"
timezone: "Europe/London"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("SLACK_EXPORT_OUTPUT_DIR", "/env/override/path")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OutputDir != "/env/override/path" {
		t.Errorf("OutputDir = %q, want %q (env should override file)", cfg.OutputDir, "/env/override/path")
	}
	if cfg.Timezone != "Europe/London" {
		t.Errorf("Timezone = %q, want %q (file value should remain)", cfg.Timezone, "Europe/London")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.yaml")

	content := `output_dir: [invalid yaml`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}

func TestLoad_NonExistentExplicitPath(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for non-existent explicit path, got nil")
	}
}

func TestLoad_SearchPathConfig(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "slack-export.yaml")

	content := `output_dir: "/found/in/search/path"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OutputDir != "/found/in/search/path" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "/found/in/search/path")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		OutputDir: filepath.Join(dir, "output"),
		Timezone:  "America/New_York",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	info, err := os.Stat(cfg.OutputDir)
	if err != nil {
		t.Errorf("OutputDir not created: %v", err)
	} else if !info.IsDir() {
		t.Error("OutputDir is not a directory")
	}
}

func TestValidate_InvalidTimezone(t *testing.T) {
	cfg := &Config{
		OutputDir: t.TempDir(),
		Timezone:  "Invalid/Timezone",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid timezone, got nil")
	}
}

func TestValidate_CreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	cfg := &Config{
		OutputDir: nested,
		Timezone:  "UTC",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	info, err := os.Stat(nested)
	if err != nil {
		t.Errorf("Nested OutputDir not created: %v", err)
	} else if !info.IsDir() {
		t.Error("OutputDir is not a directory")
	}
}

func TestValidate_CommonTimezones(t *testing.T) {
	timezones := []string{
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Asia/Tokyo",
		"UTC",
		"Local",
	}

	for _, tz := range timezones {
		t.Run(tz, func(t *testing.T) {
			cfg := &Config{
				OutputDir: t.TempDir(),
				Timezone:  tz,
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error for timezone %q: %v", tz, err)
			}
		})
	}
}

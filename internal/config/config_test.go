package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Use a temp HOME to avoid reading the user's actual config file
	t.Setenv("HOME", t.TempDir())

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

func TestSave_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "slack-export.yaml")

	cfg := &Config{
		OutputDir: "/custom/output",
		Timezone:  "Europe/London",
		Include:   []string{"eng-*"},
		Exclude:   []string{"*-archive"},
	}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read back and verify
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.OutputDir != cfg.OutputDir {
		t.Errorf("OutputDir = %q, want %q", loaded.OutputDir, cfg.OutputDir)
	}
	if loaded.Timezone != cfg.Timezone {
		t.Errorf("Timezone = %q, want %q", loaded.Timezone, cfg.Timezone)
	}
	if len(loaded.Include) != 1 || loaded.Include[0] != "eng-*" {
		t.Errorf("Include = %v, want [eng-*]", loaded.Include)
	}
	if len(loaded.Exclude) != 1 || loaded.Exclude[0] != "*-archive" {
		t.Errorf("Exclude = %v, want [*-archive]", loaded.Exclude)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nested", "dir", "slack-export.yaml")

	cfg := &Config{
		OutputDir: "/test/path",
		Timezone:  "UTC",
	}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file not created: %v", err)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()

	// Should be non-empty
	if path == "" {
		t.Error("DefaultConfigPath() returned empty string")
	}

	// Should end with expected filename
	if filepath.Base(path) != "slack-export.yaml" {
		t.Errorf("DefaultConfigPath() = %q, should end with slack-export.yaml", path)
	}

	// Should contain .config/slack-export
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultConfigPath() = %q, should be absolute path", path)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "slack-export.yaml")

	cfg := &Config{
		OutputDir: "/test/path",
		Timezone:  "UTC",
	}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Check file permissions (should be 0600)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("File permissions = %o, want 0600", perm)
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

func TestConfigFile_ReturnsUsedPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-config.yaml")

	content := `output_dir: "/test/path"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ConfigFile() != configPath {
		t.Errorf("ConfigFile() = %q, want %q", cfg.ConfigFile(), configPath)
	}
}

func TestConfigFile_EmptyWhenDefaultsUsed(t *testing.T) {
	// Use a temp HOME to avoid reading the user's actual config file
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ConfigFile() != "" {
		t.Errorf("ConfigFile() = %q, want empty string when no config file found", cfg.ConfigFile())
	}
}

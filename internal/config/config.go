package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds application configuration loaded from YAML.
type Config struct {
	OutputDir string   `yaml:"output_dir" mapstructure:"output_dir"`
	Timezone  string   `yaml:"timezone" mapstructure:"timezone"`
	Include   []string `yaml:"include" mapstructure:"include"`
	Exclude   []string `yaml:"exclude" mapstructure:"exclude"`

	configFile string // path to the config file used (if any)
}

// ConfigFile returns the path to the config file used, or empty string if defaults were used.
func (c *Config) ConfigFile() string {
	return c.configFile
}

// Load reads configuration from YAML file and environment variables.
// Search order: explicit path > ~/.config/slack-export/slack-export.yaml
// Environment variables with SLACK_EXPORT_ prefix override file values.
func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("output_dir", "./slack-logs")
	v.SetDefault("timezone", "America/New_York")

	v.SetEnvPrefix("SLACK_EXPORT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("slack-export")
		v.SetConfigType("yaml")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "slack-export"))
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	cfg.configFile = v.ConfigFileUsed()
	return &cfg, nil
}

// Validate checks that the configuration is valid.
// It validates the timezone and ensures the output directory exists (creating it if needed).
func (c *Config) Validate() error {
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
	}
	if err := os.MkdirAll(c.OutputDir, 0750); err != nil {
		return fmt.Errorf("cannot create output directory %q: %w", c.OutputDir, err)
	}
	return nil
}

// Save writes the configuration to a YAML file.
// If path is empty, uses the default user config location (~/.config/slack-export/slack-export.yaml).
func (c *Config) Save(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, ".config", "slack-export", "slack-export.yaml")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	return nil
}

// DefaultConfigPath returns the default user config path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "slack-export", "slack-export.yaml")
}

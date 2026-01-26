package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds application configuration loaded from YAML.
type Config struct {
	OutputDir     string   `yaml:"output_dir" mapstructure:"output_dir"`
	Timezone      string   `yaml:"timezone" mapstructure:"timezone"`
	Include       []string `yaml:"include" mapstructure:"include"`
	Exclude       []string `yaml:"exclude" mapstructure:"exclude"`
	SlackdumpPath string   `yaml:"slackdump_path" mapstructure:"slackdump_path"`
}

// Load reads configuration from YAML file and environment variables.
// Search order: explicit path > ./slack-export.yaml > ~/.config/slack-export/
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
		v.AddConfigPath(".")
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

	return &cfg, nil
}

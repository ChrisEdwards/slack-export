package config

// Config holds application configuration loaded from YAML.
type Config struct {
	OutputDir     string   `yaml:"output_dir" mapstructure:"output_dir"`
	Timezone      string   `yaml:"timezone" mapstructure:"timezone"`
	Include       []string `yaml:"include" mapstructure:"include"`
	Exclude       []string `yaml:"exclude" mapstructure:"exclude"`
	SlackdumpPath string   `yaml:"slackdump_path" mapstructure:"slackdump_path"`
}

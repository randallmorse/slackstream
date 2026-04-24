package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token    string   `yaml:"token"`
	Channels []string `yaml:"channels"`
	DMs      bool     `yaml:"dms"`
	Settings Settings `yaml:"settings"`
	Sounds   Sounds   `yaml:"sounds"`
	Status   Status   `yaml:"status"`
}

type Settings struct {
	PollInterval     time.Duration `yaml:"poll_interval"`
	MaxMessageLength int           `yaml:"max_message_length"`
	ShowThreads      bool          `yaml:"show_threads"`
	ShowBots         bool          `yaml:"show_bots"`
}

type Sounds struct {
	Enabled  bool          `yaml:"enabled"`
	Channels SoundConfig   `yaml:"channels"`
	DMs      SoundConfig   `yaml:"dms"`
	Debounce time.Duration `yaml:"debounce"`
}

type SoundConfig struct {
	Enabled bool   `yaml:"enabled"`
	Sound   string `yaml:"sound"`
}

type Status struct {
	Enabled bool   `yaml:"enabled"`
	Text    string `yaml:"text"`
	Emoji   string `yaml:"emoji"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if envToken := os.Getenv("SLACK_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required (set in config or SLACK_TOKEN env var)")
	}
	if len(c.Channels) == 0 && !c.DMs {
		return fmt.Errorf("at least one channel or dms: true required")
	}
	if c.Settings.PollInterval == 0 {
		c.Settings.PollInterval = 3 * time.Second
	}
	if c.Settings.MaxMessageLength == 0 {
		c.Settings.MaxMessageLength = 120
	}
	if c.Sounds.Debounce == 0 {
		c.Sounds.Debounce = 3 * time.Second
	}
	return nil
}

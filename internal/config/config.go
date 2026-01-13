package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the entire application configuration
type Config struct {
	Settings  Settings            `yaml:"settings"`
	Notifiers map[string]Notifier `yaml:"notifiers"`
	Websites  []Website           `yaml:"websites"`
}

// Settings defines global application settings
type Settings struct {
	CheckInterval string `yaml:"check_interval"`
	Retry         Retry  `yaml:"retry"`
}

// Retry defines the retry mechanism for checking websites
type Retry struct {
	Attempts      int     `yaml:"attempts"`
	InitialDelay  string  `yaml:"initial_delay"`
	BackoffFactor float64 `yaml:"backoff_factor"`
}

// Notifier is a generic configuration for any notifier type
type Notifier struct {
	Type string `yaml:"type"`
	// Telegram
	BotToken string `yaml:"bot_token,omitempty"`
	ChatID   string `yaml:"chat_id,omitempty"`
	// Email
	Host     string   `yaml:"host,omitempty"`
	Port     int      `yaml:"port,omitempty"`
	Username string   `yaml:"username,omitempty"`
	Password string   `yaml:"password,omitempty"`
	From     string   `yaml:"from,omitempty"`
	To       []string `yaml:"to,omitempty"`
	// HTTP
	URL string `yaml:"url,omitempty"`
}

// Website defines a site to be monitored
type Website struct {
	URL         string   `yaml:"url"`
	WarningDays []int    `yaml:"warning_days"`
	Notifiers   []string `yaml:"notifiers"`
	Retry       *Retry   `yaml:"retry,omitempty"` // Pointer to allow for nil when not overridden
}

// Load reads a configuration file from the given path and parses it.
// It also expands environment variables in the configuration.
func Load(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedFile := os.ExpandEnv(string(file))

	var cfg Config
	err = yaml.Unmarshal([]byte(expandedFile), &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// GetCheckIntervalDuration returns the check interval as a time.Duration
func (s *Settings) GetCheckIntervalDuration() (time.Duration, error) {
	duration, err := time.ParseDuration(s.CheckInterval)
	if err != nil {
		return 0, err
	}
	if duration < 0 {
		return 0, fmt.Errorf("check_interval must be a non-negative duration")
	}
	return duration, nil
}

// GetInitialDelayDuration returns the initial retry delay as a time.Duration
func (r *Retry) GetInitialDelayDuration() (time.Duration, error) {
	duration, err := time.ParseDuration(r.InitialDelay)
	if err != nil {
		return 0, err
	}
	if duration < 0 {
		return 0, fmt.Errorf("initial_delay must be a non-negative duration")
	}
	return duration, nil
}

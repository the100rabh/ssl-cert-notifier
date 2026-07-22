package config

import (
	"fmt"
	"os"
	"strings"
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
	CheckTime     string `yaml:"check_time,omitempty"`
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
	URL             string   `yaml:"url"`
	DaysUntilExpiry int      `yaml:"days_until_expiry"`
	Notifiers       []string `yaml:"notifiers"`
	Retry           *Retry   `yaml:"retry,omitempty"` // Pointer to allow for nil when not overridden
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

// GetNextCheckTime calculates the next time the check should run based on CheckTime and a reference time.
// Supported formats include "15:04", "15:04:05", "3:04PM", "3:04 PM", "03:04PM", "03:04 PM".
func (s *Settings) GetNextCheckTime(now time.Time) (time.Time, error) {
	timeStr := strings.TrimSpace(s.CheckTime)
	if timeStr == "" {
		return time.Time{}, nil
	}

	formats := []string{
		"15:04",
		"15:04:05",
		"3:04PM",
		"3:04 PM",
		"03:04PM",
		"03:04 PM",
		"3:04pm",
		"3:04 pm",
		"03:04pm",
		"03:04 pm",
	}

	timeStrUpper := strings.ToUpper(timeStr)

	var parsedTime time.Time
	var matched bool

	for _, format := range formats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			parsedTime = t
			matched = true
			break
		}
		t, err = time.Parse(format, timeStrUpper)
		if err == nil {
			parsedTime = t
			matched = true
			break
		}
	}

	if !matched {
		return time.Time{}, fmt.Errorf("invalid check_time format '%s': expected format like '09:00' or '14:30:00'", s.CheckTime)
	}

	hour, min, sec := parsedTime.Clock()

	// Construct target time today in the same timezone location as `now`
	targetToday := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())

	if targetToday.After(now) {
		return targetToday, nil
	}

	// If target time today has passed or is equal, schedule for tomorrow
	return targetToday.AddDate(0, 0, 1), nil
}

// GetCheckTimeDuration returns the duration from now until the next scheduled CheckTime.
func (s *Settings) GetCheckTimeDuration(now time.Time) (time.Duration, error) {
	if strings.TrimSpace(s.CheckTime) == "" {
		return 0, nil
	}
	nextTime, err := s.GetNextCheckTime(now)
	if err != nil {
		return 0, err
	}
	return nextTime.Sub(now), nil
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

package config

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Helper to create a temporary config file for testing
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return tmpFile.Name()
}

func TestLoadValidConfig(t *testing.T) {
	// This test remains largely the same, confirming the happy path.
	testConfigContent := `
settings:
  check_interval: "1h"
  retry:
    attempts: 3
    initial_delay: "30s"
    backoff_factor: 2.0
notifiers:
  telegram_test:
    type: telegram
    bot_token: "test_token"
    chat_id: "test_chat_id"
websites:
  - url: "example.com:443"
    warning_days: [30, 7]
    notifiers: ["telegram_test"]
    retry:
      attempts: 5
      initial_delay: "10s"
      backoff_factor: 1.5
`
	configPath := createTempConfigFile(t, testConfigContent)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed for valid config: %v", err)
	}
	if cfg.Settings.Retry.Attempts != 3 {
		t.Errorf("Expected global retry attempts 3, got %d", cfg.Settings.Retry.Attempts)
	}
	if len(cfg.Websites) != 1 {
		t.Errorf("Expected 1 website, got %d", len(cfg.Websites))
	}
}

func TestLoadEdgeCases(t *testing.T) {
	t.Run("Missing 'settings' section", func(t *testing.T) {
		configContent := `
websites: []
notifiers: {}
`
		configPath := createTempConfigFile(t, configContent)
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() failed for missing settings: %v", err)
		}
		// Check that defaults are zero-values
		if cfg.Settings.CheckInterval != "" {
			t.Errorf("Expected empty CheckInterval, got %s", cfg.Settings.CheckInterval)
		}
	})

	t.Run("Empty 'websites' section", func(t *testing.T) {
		configContent := `
settings:
  check_interval: "1h"
websites: []
`
		configPath := createTempConfigFile(t, configContent)
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() failed for empty websites: %v", err)
		}
		if len(cfg.Websites) != 0 {
			t.Errorf("Expected 0 websites, got %d", len(cfg.Websites))
		}
	})

	t.Run("Type mismatch in config", func(t *testing.T) {
		configContent := `
settings:
  retry:
    attempts: "three" # This should be a number
`
		configPath := createTempConfigFile(t, configContent)
		_, err := Load(configPath)
		if err == nil {
			t.Fatal("Load() should have failed for type mismatch, but succeeded")
		}
		if !strings.Contains(err.Error(), "cannot unmarshal !!str `three` into int") {
			t.Errorf("Expected type mismatch error, got: %v", err)
		}
	})
}

func TestLoadInvalidYAMLStructure(t *testing.T) {
	malformedConfigContent := `
settings:
  check_interval: "1h"
  retry:
    attempts: 3
notifiers:
  - this is not a map
`
	configPath := createTempConfigFile(t, malformedConfigContent)
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() expected to fail for malformed YAML, but succeeded")
	}
	if !strings.Contains(err.Error(), "failed to parse config file: yaml: unmarshal errors") {
		t.Errorf("Expected malformed YAML error containing 'unmarshal errors', got: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("non_existent_config.yaml")
	if err == nil {
		t.Fatal("Load() expected to fail for missing file, but succeeded")
	}
	// Use errors.Is for robust error chain inspection
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected an os.ErrNotExist error, but got a different error type: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	t.Run("Valid duration", func(t *testing.T) {
		settings := Settings{CheckInterval: "2h30m"}
		duration, err := settings.GetCheckIntervalDuration()
		if err != nil {
			t.Fatalf("GetCheckIntervalDuration() failed: %v", err)
		}
		if duration != (2*time.Hour + 30*time.Minute) {
			t.Errorf("Expected duration %v, got %v", (2*time.Hour + 30*time.Minute), duration)
		}
	})

	t.Run("Invalid duration unit", func(t *testing.T) {
		settings := Settings{CheckInterval: "10z"}
		_, err := settings.GetCheckIntervalDuration()
		if err == nil {
			t.Fatal("Expected error for invalid unit, but got nil")
		}
	})

	t.Run("Zero duration", func(t *testing.T) {
		settings := Settings{CheckInterval: "0s"}
		duration, err := settings.GetCheckIntervalDuration()
		if err != nil {
			t.Fatalf("Got unexpected error for zero duration: %v", err)
		}
		if duration != 0 {
			t.Errorf("Expected 0 duration, got %v", duration)
		}
	})

	t.Run("Negative duration", func(t *testing.T) {
		retry := Retry{InitialDelay: "-5s"}
		_, err := retry.GetInitialDelayDuration()
		if err == nil {
			t.Fatal("Expected error for negative duration, but got nil")
		}
	})

	t.Run("Negative check interval", func(t *testing.T) {
		settings := Settings{CheckInterval: "-1h"}
		_, err := settings.GetCheckIntervalDuration()
		if err == nil {
			t.Fatal("Expected error for negative check interval, but got nil")
		}
		if !strings.Contains(err.Error(), "check_interval must be a non-negative duration") {
			t.Errorf("Expected 'non-negative duration' error, got: %v", err)
		}
	})
}

func TestEnvVarExpansion(t *testing.T) {
	t.Run("Set environment variable", func(t *testing.T) {
		os.Setenv("TEST_BOT_TOKEN", "env_token_123")
		defer os.Unsetenv("TEST_BOT_TOKEN")

		configContent := `notifiers: { test: { bot_token: "${TEST_BOT_TOKEN}" } }`
		configPath := createTempConfigFile(t, configContent)
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		if cfg.Notifiers["test"].BotToken != "env_token_123" {
			t.Errorf("Expected token 'env_token_123', got '%s'", cfg.Notifiers["test"].BotToken)
		}
	})

	t.Run("Unset environment variable", func(t *testing.T) {
		// Ensure the variable is not set
		os.Unsetenv("UNSET_TEST_VAR")
		configContent := `notifiers: { test: { bot_token: "${UNSET_TEST_VAR}" } }`
		configPath := createTempConfigFile(t, configContent)
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		if cfg.Notifiers["test"].BotToken != "" {
			t.Errorf("Expected empty string for unset env var, got '%s'", cfg.Notifiers["test"].BotToken)
		}
	})
}

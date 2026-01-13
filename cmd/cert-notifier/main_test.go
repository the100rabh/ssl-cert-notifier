package main

import (
	"os"
	"testing"

	"github.com/the100rabh/ssl-cert-notifier/internal/app"
	"github.com/the100rabh/ssl-cert-notifier/internal/checker"
	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// TestMainLogic tests the core logic of the main function by mocking the config loading
func TestMainLogic(t *testing.T) {
	// Create a temporary config file for testing
	tempConfigFile := "test_config.yaml"
	configContent := `settings:
  check_interval: "100ms"  # Very short interval for testing
  retry:
    attempts: 1
    initial_delay: "1ms"
    backoff_factor: 1.0
notifiers:
  log_test:
    type: log
websites:
  - url: "example.com:443"
    warning_days: [30, 7]
    notifiers: ["log_test"]
`

	err := os.WriteFile(tempConfigFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempConfigFile) // Clean up

	// Test config loading
	cfg, err := config.Load(tempConfigFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test notifier initialization
	initializedNotifiers, initErrors := app.InitializeNotifiers(cfg.Notifiers)
	if len(initErrors) > 0 {
		t.Errorf("Unexpected initialization errors: %v", initErrors)
	}
	if len(initializedNotifiers) == 0 {
		t.Error("Expected at least one initialized notifier")
	}

	// Test check interval parsing
	checkInterval, err := cfg.Settings.GetCheckIntervalDuration()
	if err != nil {
		t.Fatalf("Failed to parse check interval: %v", err)
	}
	if checkInterval <= 0 {
		t.Error("Expected positive check interval")
	}

	// Test that RunChecks can be called without panicking
	app.RunChecks(cfg, initializedNotifiers, checker.Check)
}

func TestMainLogic_ZeroInterval(t *testing.T) {
	// Create a temporary config file for testing with zero interval
	tempConfigFile := "test_config_zero.yaml"
	configContent := `settings:
  check_interval: "0s"  # Zero interval - should run once
  retry:
    attempts: 1
    initial_delay: "1ms"
    backoff_factor: 1.0
notifiers:
  log_test:
    type: log
websites:
  - url: "example.com:443"
    warning_days: [30, 7]
    notifiers: ["log_test"]
`

	err := os.WriteFile(tempConfigFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempConfigFile) // Clean up

	// Test config loading
	cfg, err := config.Load(tempConfigFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test check interval parsing
	checkInterval, err := cfg.Settings.GetCheckIntervalDuration()
	if err != nil {
		t.Fatalf("Failed to parse check interval: %v", err)
	}
	if checkInterval != 0 {
		t.Errorf("Expected zero check interval, got %v", checkInterval)
	}

	// Test notifier initialization
	initializedNotifiers, _ := app.InitializeNotifiers(cfg.Notifiers)

	// Test that RunChecks can be called without panicking
	app.RunChecks(cfg, initializedNotifiers, checker.Check)
}

func TestMainLogic_InvalidConfig(t *testing.T) {
	// Test loading non-existent config file
	_, err := config.Load("non_existent_config.yaml")
	if err == nil {
		t.Fatal("Expected error when loading non-existent config file")
	}
}

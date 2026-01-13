package notifiers

import (
	"strings"
	"testing"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

func TestGetNotifierFactory(t *testing.T) {
	t.Run("Get known notifier", func(t *testing.T) {
		cfg := config.Notifier{Type: "log"}
		notifier, err := GetNotifier("test_log", cfg)
		if err != nil {
			t.Fatalf("Expected no error for 'log' notifier, but got %v", err)
		}
		if _, ok := notifier.(*LogNotifier); !ok {
			t.Errorf("Expected a *LogNotifier, but got %T", notifier)
		}
	})

	t.Run("Get unknown notifier", func(t *testing.T) {
		cfg := config.Notifier{Type: "sms"}
		_, err := GetNotifier("test_sms", cfg)
		if err == nil {
			t.Fatal("Expected an error for unknown notifier type, but got nil")
		}
		if !strings.Contains(err.Error(), "unknown notifier type 'sms'") {
			t.Errorf("Error message did not contain expected substring. Got: %v", err)
		}
	})
}

func TestGetNotifier_MissingConfigFields(t *testing.T) {
	t.Run("Telegram missing bot_token", func(t *testing.T) {
		cfg := config.Notifier{
			Type:   "telegram",
			ChatID: "12345",
		}
		_, err := GetNotifier("test_telegram", cfg)
		if err == nil {
			t.Fatal("Expected an error for missing bot_token, but got nil")
		}
		if !strings.Contains(err.Error(), "bot_token is required") {
			t.Errorf("Error message did not contain expected substring. Got: %v", err)
		}
	})

	t.Run("Telegram missing chat_id", func(t *testing.T) {
		cfg := config.Notifier{
			Type:     "telegram",
			BotToken: "my-token",
		}
		_, err := GetNotifier("test_telegram", cfg)
		if err == nil {
			t.Fatal("Expected an error for missing chat_id, but got nil")
		}
		if !strings.Contains(err.Error(), "chat_id is required") {
			t.Errorf("Error message did not contain expected substring. Got: %v", err)
		}
	})

	t.Run("Email missing host", func(t *testing.T) {
		cfg := config.Notifier{
			Type:     "email",
			Port:     587,
			Username: "user",
			Password: "pass",
			From:     "from@example.com",
			To:       []string{"to@example.com"},
		}
		_, err := GetNotifier("test_email", cfg)
		if err == nil {
			t.Fatal("Expected an error for missing host, but got nil")
		}
		if !strings.Contains(err.Error(), "host is required") {
			t.Errorf("Error message did not contain expected substring. Got: %v", err)
		}
	})

	t.Run("Email missing 'to' address", func(t *testing.T) {
		cfg := config.Notifier{
			Type:     "email",
			Host:     "smtp.example.com",
			Port:     587,
			Username: "user",
			Password: "pass",
			From:     "from@example.com",
			To:       []string{}, // Empty list
		}
		_, err := GetNotifier("test_email", cfg)
		if err == nil {
			t.Fatal("Expected an error for missing 'to' address, but got nil")
		}
		if !strings.Contains(err.Error(), "at least one 'to' address is required") {
			t.Errorf("Error message did not contain expected substring. Got: %v", err)
		}
	})
}

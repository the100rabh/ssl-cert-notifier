package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// TelegramNotifier sends notifications to a Telegram chat.
type TelegramNotifier struct {
	BotToken string
	ChatID   string
	Client   *http.Client // Add an HTTP client for making requests
	// telegramAPIBaseURL allows overriding the Telegram API base URL for testing
	telegramAPIBaseURL string
}

// NewTelegramNotifier creates a new TelegramNotifier, validating its configuration.
func NewTelegramNotifier(cfg config.Notifier) (*TelegramNotifier, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("bot_token is required for telegram notifier")
	}
	if cfg.ChatID == "" {
		return nil, fmt.Errorf("chat_id is required for telegram notifier")
	}

	// Default Telegram API base URL
	apiBaseURL := "https://api.telegram.org"
	// Allow overriding via config if needed (e.g., for self-hosted Telegram Bot API)
	// For testing, we'll set this directly on the struct.

	return &TelegramNotifier{
		BotToken: cfg.BotToken,
		ChatID:   cfg.ChatID,
		Client: &http.Client{
			Timeout: 10 * time.Second, // Configure a timeout for the HTTP client
		},
		telegramAPIBaseURL: apiBaseURL,
	}, nil
}

// Send sends the message to Telegram.
func (n *TelegramNotifier) Send(subject, body string) error {
	telegramAPIURL := fmt.Sprintf("%s/bot%s/sendMessage", n.telegramAPIBaseURL, n.BotToken)

	// Combine subject and body for the Telegram message
	messageText := fmt.Sprintf("Subject: %s\n\n%s", subject, body)

	// Telegram message payload
	payload := map[string]string{
		"chat_id": n.ChatID,
		"text":    messageText,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: TelegramNotifier failed to marshal JSON payload: %v", err)
		return fmt.Errorf("failed to marshal Telegram payload: %w", err)
	}

	req, err := http.NewRequest("POST", telegramAPIURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("ERROR: TelegramNotifier failed to create HTTP request: %v", err)
		return fmt.Errorf("failed to create Telegram HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		log.Printf("ERROR: TelegramNotifier failed to send HTTP request to Telegram API: %v", err)
		return fmt.Errorf("failed to send Telegram notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) // Read response body for more details
		log.Printf("ERROR: TelegramNotifier received non-OK status from Telegram API. Status: %s, Body: %s", resp.Status, respBody)
		return fmt.Errorf("Telegram API returned non-OK status: %s (Body: %s)", resp.Status, respBody)
	}

	log.Printf("INFO: Telegram notification sent successfully to chat ID %s", n.ChatID)
	return nil
}

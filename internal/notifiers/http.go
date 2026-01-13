package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// HTTPNotifier sends notifications to a generic HTTP endpoint.
type HTTPNotifier struct {
	URL    string
	Client *http.Client
}

// NotificationPayload is the JSON structure sent to the HTTP endpoint.
type NotificationPayload struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// NewHTTPNotifier creates a new HTTPNotifier, validating its configuration.
func NewHTTPNotifier(cfg config.Notifier) (*HTTPNotifier, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("url is required for http notifier")
	}
	return &HTTPNotifier{
		URL: cfg.URL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Send sends the message to the configured HTTP endpoint.
func (n *HTTPNotifier) Send(subject, body string) error {
	payload := NotificationPayload{
		Subject: subject,
		Body:    body,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal http notification payload: %w", err)
	}

	req, err := http.NewRequest("POST", n.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send http notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http notifier received non-2xx status code: %d", resp.StatusCode)
	}

	return nil
}

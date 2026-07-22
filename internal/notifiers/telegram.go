package notifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// TelegramNotifier sends notifications to a Telegram chat with automatic retries and failed message queueing.
type TelegramNotifier struct {
	BotToken           string
	ChatID             string
	Client             *http.Client // HTTP client for making requests
	telegramAPIBaseURL string
	queueFile          string

	MaxAttempts   int
	InitialDelay  time.Duration
	BackoffFactor float64

	mu              sync.Mutex
	pendingMessages []PendingMessage
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

	n := &TelegramNotifier{
		BotToken: cfg.BotToken,
		ChatID:   cfg.ChatID,
		Client: &http.Client{
			Timeout: 10 * time.Second, // Configure a timeout for the HTTP client
		},
		telegramAPIBaseURL: apiBaseURL,
		MaxAttempts:        3,
		InitialDelay:       100 * time.Millisecond,
		BackoffFactor:      2.0,
		queueFile:          ".pending_telegram_queue.json",
	}
	n.loadQueueUnlocked()
	return n, nil
}

// SetQueueFilePath sets a custom file path for persistent queue storage.
func (n *TelegramNotifier) SetQueueFilePath(path string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.queueFile = path
	n.loadQueueUnlocked()
}

func (n *TelegramNotifier) loadQueueUnlocked() {
	if n.queueFile == "" {
		return
	}
	data, err := os.ReadFile(n.queueFile)
	if err != nil {
		return
	}
	var loaded []PendingMessage
	if err := json.Unmarshal(data, &loaded); err == nil && len(loaded) > 0 {
		n.pendingMessages = loaded
		log.Printf("INFO: Loaded %d pending Telegram notifications from persistent storage (%s)", len(loaded), n.queueFile)
	}
}

func (n *TelegramNotifier) saveQueueUnlocked() {
	if n.queueFile == "" {
		return
	}
	if len(n.pendingMessages) == 0 {
		_ = os.Remove(n.queueFile)
		return
	}
	data, err := json.MarshalIndent(n.pendingMessages, "", "  ")
	if err == nil {
		_ = os.WriteFile(n.queueFile, data, 0644)
	}
}

// Send sends the message to Telegram with retry, backoff, and queued delivery for failed messages.
func (n *TelegramNotifier) Send(ctx context.Context, subject, body string) error {
	n.mu.Lock()
	n.flushPendingUnlocked(ctx)
	n.mu.Unlock()

	err := n.sendWithRetry(ctx, subject, body)
	if err != nil {
		n.mu.Lock()
		n.pendingMessages = append(n.pendingMessages, PendingMessage{
			Subject:   subject,
			Body:      body,
			Timestamp: time.Now(),
		})
		n.saveQueueUnlocked()
		n.mu.Unlock()
		log.Printf("WARN: TelegramNotifier failed to send alert after %d attempts; message queued for later delivery: %v", n.MaxAttempts, err)
		return fmt.Errorf("failed to send Telegram notification (queued for later): %w", err)
	}

	n.mu.Lock()
	n.flushPendingUnlocked(ctx)
	n.mu.Unlock()

	return nil
}

func (n *TelegramNotifier) sendWithRetry(ctx context.Context, subject, body string) error {
	attempts := n.MaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	initialDelay := n.InitialDelay
	if initialDelay <= 0 {
		initialDelay = 100 * time.Millisecond
	}
	backoffFactor := n.BackoffFactor
	if backoffFactor <= 0 {
		backoffFactor = 2.0
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := n.sendSingle(ctx, subject, body)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Attempt %d/%d to send Telegram notification failed: %v", i+1, attempts, err)

		if i == attempts-1 {
			break
		}

		waitDuration := time.Duration(float64(initialDelay) * math.Pow(backoffFactor, float64(i)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}
	}
	return lastErr
}

func (n *TelegramNotifier) flushPendingUnlocked(ctx context.Context) {
	if len(n.pendingMessages) == 0 {
		return
	}

	var remaining []PendingMessage
	for _, msg := range n.pendingMessages {
		if ctx.Err() != nil {
			remaining = append(remaining, msg)
			continue
		}
		err := n.sendWithRetry(ctx, msg.Subject, msg.Body)
		if err != nil {
			log.Printf("WARN: Failed to flush queued Telegram message (subject: %s): %v", msg.Subject, err)
			remaining = append(remaining, msg)
		} else {
			log.Printf("INFO: Successfully sent queued Telegram message (subject: %s)", msg.Subject)
		}
	}
	n.pendingMessages = remaining
	n.saveQueueUnlocked()
}

// Flush attempts to send any stored failed messages.
func (n *TelegramNotifier) Flush(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.flushPendingUnlocked(ctx)
	if len(n.pendingMessages) > 0 {
		return fmt.Errorf("%d pending Telegram notifications could not be flushed", len(n.pendingMessages))
	}
	return nil
}

// GetPendingMessages returns a copy of stored pending messages.
func (n *TelegramNotifier) GetPendingMessages() []PendingMessage {
	n.mu.Lock()
	defer n.mu.Unlock()

	cp := make([]PendingMessage, len(n.pendingMessages))
	copy(cp, n.pendingMessages)
	return cp
}

func (n *TelegramNotifier) sendSingle(ctx context.Context, subject, body string) error {
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

	req, err := http.NewRequestWithContext(ctx, "POST", telegramAPIURL, bytes.NewBuffer(jsonPayload))
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

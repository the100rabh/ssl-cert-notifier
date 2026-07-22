package notifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"
)

// PendingMessage stores an alert message that failed to send and is queued for retry.
type PendingMessage struct {
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
}

// Flusher is an interface for notifiers that support flushing queued messages.
type Flusher interface {
	Flush(ctx context.Context) error
	GetPendingMessages() []PendingMessage
}

// RetryingNotifier wraps any Notifier with retry, exponential backoff logic,
// context cancellation support, and persistent disk queueing for failed alert messages.
type RetryingNotifier struct {
	underlying    Notifier
	maxAttempts   int
	initialDelay  time.Duration
	backoffFactor float64
	queueFile     string

	mu              sync.Mutex
	pendingMessages []PendingMessage
}

// NewRetryingNotifier wraps an existing Notifier with retry and queuing behavior.
func NewRetryingNotifier(underlying Notifier, maxAttempts int, initialDelay time.Duration, backoffFactor float64) *RetryingNotifier {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if initialDelay <= 0 {
		initialDelay = 100 * time.Millisecond
	}
	if backoffFactor <= 0 {
		backoffFactor = 2.0
	}
	r := &RetryingNotifier{
		underlying:    underlying,
		maxAttempts:   maxAttempts,
		initialDelay:  initialDelay,
		backoffFactor: backoffFactor,
		queueFile:     ".pending_queue.json",
	}
	r.loadQueueUnlocked()
	return r
}

// SetQueueFilePath sets a custom file path for persistent queue storage.
func (r *RetryingNotifier) SetQueueFilePath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queueFile = path
	r.loadQueueUnlocked()
}

func (r *RetryingNotifier) loadQueueUnlocked() {
	if r.queueFile == "" {
		return
	}
	data, err := os.ReadFile(r.queueFile)
	if err != nil {
		return // File does not exist yet or cannot be read
	}
	var loaded []PendingMessage
	if err := json.Unmarshal(data, &loaded); err == nil && len(loaded) > 0 {
		r.pendingMessages = loaded
		log.Printf("INFO: Loaded %d pending notifications from persistent storage (%s)", len(loaded), r.queueFile)
	}
}

func (r *RetryingNotifier) saveQueueUnlocked() {
	if r.queueFile == "" {
		return
	}
	if len(r.pendingMessages) == 0 {
		_ = os.Remove(r.queueFile)
		return
	}
	data, err := json.MarshalIndent(r.pendingMessages, "", "  ")
	if err == nil {
		_ = os.WriteFile(r.queueFile, data, 0644)
	}
}

// Send attempts to send the alert message using exponential backoff retry.
// Before sending, it attempts to flush any previously stored failed messages.
// If sending fails after all retries, the message is stored in the persistent queue.
func (r *RetryingNotifier) Send(ctx context.Context, subject, body string) error {
	r.mu.Lock()
	r.flushPendingUnlocked(ctx)
	r.mu.Unlock()

	err := r.sendWithRetry(ctx, subject, body)
	if err != nil {
		r.mu.Lock()
		r.pendingMessages = append(r.pendingMessages, PendingMessage{
			Subject:   subject,
			Body:      body,
			Timestamp: time.Now(),
		})
		r.saveQueueUnlocked()
		r.mu.Unlock()
		log.Printf("WARN: RetryingNotifier failed to send alert after %d attempts; queued message (subject: %s) for later delivery: %v",
			r.maxAttempts, subject, err)
		return fmt.Errorf("failed to send notification after retries (queued for later): %w", err)
	}

	r.mu.Lock()
	r.flushPendingUnlocked(ctx)
	r.mu.Unlock()

	return nil
}

func (r *RetryingNotifier) sendWithRetry(ctx context.Context, subject, body string) error {
	var lastErr error
	for i := 0; i < r.maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := r.underlying.Send(ctx, subject, body)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Attempt %d/%d to send notification failed: %v", i+1, r.maxAttempts, err)

		if i == r.maxAttempts-1 {
			break
		}

		waitDuration := time.Duration(float64(r.initialDelay) * math.Pow(r.backoffFactor, float64(i)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}
	}
	return lastErr
}

func (r *RetryingNotifier) flushPendingUnlocked(ctx context.Context) {
	if len(r.pendingMessages) == 0 {
		return
	}

	var remaining []PendingMessage
	for _, msg := range r.pendingMessages {
		if ctx.Err() != nil {
			remaining = append(remaining, msg)
			continue
		}
		err := r.sendWithRetry(ctx, msg.Subject, msg.Body)
		if err != nil {
			log.Printf("WARN: Failed to flush pending notification (subject: %s): %v", msg.Subject, err)
			remaining = append(remaining, msg)
		} else {
			log.Printf("INFO: Successfully flushed pending notification (subject: %s)", msg.Subject)
		}
	}
	r.pendingMessages = remaining
	r.saveQueueUnlocked()
}

// Flush attempts to resend all pending failed messages.
func (r *RetryingNotifier) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.flushPendingUnlocked(ctx)
	if len(r.pendingMessages) > 0 {
		return fmt.Errorf("%d pending notifications could not be flushed", len(r.pendingMessages))
	}
	return nil
}

// GetPendingMessages returns a slice copy of currently stored failed messages.
func (r *RetryingNotifier) GetPendingMessages() []PendingMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := make([]PendingMessage, len(r.pendingMessages))
	copy(cp, r.pendingMessages)
	return cp
}

package notifiers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockNotifier struct {
	failCount  int
	callsCount int
	sentMsgs   []string
}

func (m *mockNotifier) Send(ctx context.Context, subject, body string) error {
	m.callsCount++
	if m.failCount > 0 {
		m.failCount--
		return errors.New("network connection down")
	}
	m.sentMsgs = append(m.sentMsgs, subject)
	return nil
}

func (m *mockNotifier) flushPendingUnlocked(ctx context.Context) {
	// No-op for mock
}

func TestRetryingNotifier(t *testing.T) {
	t.Run("Retry success after temporary failure", func(t *testing.T) {
		mock := &mockNotifier{failCount: 2} // Fails twice, succeeds on 3rd attempt
		retrying := NewRetryingNotifier(mock, 3, 1*time.Millisecond, 2.0)

		err := retrying.Send(context.Background(), "Subject", "Body")
		if err != nil {
			t.Fatalf("Expected Send to succeed after retries, got %v", err)
		}

		if mock.callsCount != 3 {
			t.Errorf("Expected 3 calls to Send, got %d", mock.callsCount)
		}

		if len(retrying.GetPendingMessages()) != 0 {
			t.Errorf("Expected 0 pending messages, got %d", len(retrying.GetPendingMessages()))
		}
	})

	t.Run("Queue on persistent failure and flush when recovered", func(t *testing.T) {
		mock := &mockNotifier{failCount: 100} // Keeps failing
		retrying := NewRetryingNotifier(mock, 2, 1*time.Millisecond, 1.5)

		err := retrying.Send(context.Background(), "Alert 1", "Body 1")
		if err == nil {
			t.Fatal("Expected error when retries exhausted, got nil")
		}
		if !strings.Contains(err.Error(), "queued for later") {
			t.Errorf("Expected error to mention queued for later, got %v", err)
		}

		if len(retrying.GetPendingMessages()) != 1 {
			t.Fatalf("Expected 1 pending message queued, got %d", len(retrying.GetPendingMessages()))
		}

		// Network recovers: reset failCount to 0
		mock.failCount = 0

		err = retrying.Flush(context.Background())
		if err != nil {
			t.Fatalf("Expected Flush to succeed after network recovery, got %v", err)
		}

		if len(retrying.GetPendingMessages()) != 0 {
			t.Errorf("Expected 0 pending messages after flush, got %d", len(retrying.GetPendingMessages()))
		}

		if len(mock.sentMsgs) != 1 || mock.sentMsgs[0] != "Alert 1" {
			t.Errorf("Expected Alert 1 to be sent during flush, got %v", mock.sentMsgs)
		}
	})
}

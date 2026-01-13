package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/app"
	"github.com/the100rabh/ssl-cert-notifier/internal/checker"
	"github.com/the100rabh/ssl-cert-notifier/internal/config"
	"github.com/the100rabh/ssl-cert-notifier/internal/notifiers"
)

// mockNotificationReceiver is a simple HTTP server that collects notifications.
type mockNotificationReceiver struct {
	mu       sync.Mutex
	server   *httptest.Server
	payloads []notifiers.NotificationPayload
}

func newMockNotificationReceiver() *mockNotificationReceiver {
	receiver := &mockNotificationReceiver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}

		var payload notifiers.NotificationPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Error unmarshaling JSON", http.StatusBadRequest)
			return
		}

		receiver.mu.Lock()
		receiver.payloads = append(receiver.payloads, payload)
		receiver.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	})
	receiver.server = httptest.NewServer(handler)
	return receiver
}

func (r *mockNotificationReceiver) URL() string {
	return r.server.URL
}

func (r *mockNotificationReceiver) Close() {
	r.server.Close()
}

func (r *mockNotificationReceiver) Payloads() []notifiers.NotificationPayload {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Return a copy
	result := make([]notifiers.NotificationPayload, len(r.payloads))
	copy(result, r.payloads)
	return result
}

func TestE2E_CertificateChecks(t *testing.T) {
	// 1. Start the mock notification receiver
	receiver := newMockNotificationReceiver()
	defer receiver.Close()

	// 2. Create a configuration for the E2E test
	cfg := &config.Config{
		Settings: config.Settings{
			Retry: config.Retry{Attempts: 2, InitialDelay: "1ms"},
		},
		Notifiers: map[string]config.Notifier{
			"e2e_http": {
				Type: "http",
				URL:  receiver.URL(),
			},
		},
		Websites: []config.Website{
			{
				URL:         "google.com", // This should succeed without notification
				WarningDays: []int{1},
				Notifiers:   []string{"e2e_http"},
			},
			{
				URL:         "expired.badssl.com", // This should trigger an "expired" notification
				WarningDays: []int{1},
				Notifiers:   []string{"e2e_http"},
			},
			{
				URL:         "192.0.2.2:443", // This host should fail, triggering a "failed" notification
				WarningDays: []int{1},
				Notifiers:   []string{"e2e_http"},
			},
		},
	}

	// 3. Initialize the real notifiers using the factory from the app package
	initializedNotifiers, initErrors := app.InitializeNotifiers(cfg.Notifiers)
	if len(initErrors) > 0 {
		t.Fatalf("Failed to initialize notifiers: %v", initErrors)
	}

	// 4. Run the application's core logic directly from the app package
	// We pass the real checker.Check function to get as close to reality as possible
	app.RunChecks(cfg, initializedNotifiers, checker.Check)

	// 5. Assert the results
	// Allow some time for http requests to be processed
	time.Sleep(100 * time.Millisecond)

	receivedPayloads := receiver.Payloads()
	if len(receivedPayloads) != 2 {
		t.Fatalf("Expected 2 notifications, but got %d", len(receivedPayloads))
	}

	// Check for specific notifications
	var foundExpired, foundFailed bool
	for _, p := range receivedPayloads {
		t.Logf("Received notification: Subject: %s", p.Subject)
		if strings.Contains(p.Subject, "expired.badssl.com") && strings.Contains(p.Subject, "Expired") {
			foundExpired = true
		}
		if strings.Contains(p.Subject, "192.0.2.2:443") && strings.Contains(p.Subject, "Check Failed") {
			foundFailed = true
		}
	}

	if !foundExpired {
		t.Error("Did not receive 'Expired' notification for expired.badssl.com")
	}
	if !foundFailed {
		t.Error("Did not receive 'Check Failed' notification for 192.0.2.2:443")
	}
}

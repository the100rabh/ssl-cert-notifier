package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/checker"
	"github.com/the100rabh/ssl-cert-notifier/internal/config"
	"github.com/the100rabh/ssl-cert-notifier/internal/notifiers"
)

// mockChecker satisfies the app.CheckerFunc type for testing.
type mockChecker struct {
	mu         sync.Mutex
	callCount  int
	failNTimes int
	details    *checker.CertDetails
	err        error
}

func (m *mockChecker) Check(ctx context.Context, url string) (*checker.CertDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.callCount <= m.failNTimes {
		return nil, m.err
	}
	return m.details, nil
}

func (m *mockChecker) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockChecker) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
	m.failNTimes = 0
	m.details = nil
	m.err = nil
}

func TestPerformCheckWithRetries(t *testing.T) {
	site := config.Website{URL: "test.com"}
	defaultRetry := config.Retry{
		Attempts:      3,
		InitialDelay:  "1ms", // Use a short delay for fast tests
		BackoffFactor: 2.0,
	}

	t.Run("Success on first attempt", func(t *testing.T) {
		checkerMock := &mockChecker{
			details: &checker.CertDetails{DaysRemaining: 30},
		}

		details, err := performCheckWithRetries(context.Background(), site, defaultRetry, checkerMock.Check)
		if err != nil {
			t.Fatalf("Expected no error, but got %v", err)
		}
		if details.DaysRemaining != 30 {
			t.Errorf("Expected 30 days remaining, got %f", details.DaysRemaining)
		}
		if checkerMock.CallCount() != 1 {
			t.Errorf("Expected checker to be called 1 time, but was called %d times", checkerMock.CallCount())
		}
	})

	t.Run("Success after two failures", func(t *testing.T) {
		checkerMock := &mockChecker{
			failNTimes: 2,
			details:    &checker.CertDetails{DaysRemaining: 30},
			err:        fmt.Errorf("mock check failed"),
		}

		details, err := performCheckWithRetries(context.Background(), site, defaultRetry, checkerMock.Check)
		if err != nil {
			t.Fatalf("Expected no error, but got %v", err)
		}
		if details.DaysRemaining != 30 {
			t.Errorf("Expected 30 days remaining, got %f", details.DaysRemaining)
		}
		if checkerMock.CallCount() != 3 {
			t.Errorf("Expected checker to be called 3 times, but was called %d times", checkerMock.CallCount())
		}
	})

	t.Run("Failure on all attempts", func(t *testing.T) {
		checkerMock := &mockChecker{
			failNTimes: 3,
			err:        fmt.Errorf("final error"),
		}

		_, err := performCheckWithRetries(context.Background(), site, defaultRetry, checkerMock.Check)
		if err == nil {
			t.Fatal("Expected an error, but got nil")
		}
		if !strings.Contains(err.Error(), "final error") {
			t.Errorf("Expected error to contain 'final error', but got %v", err)
		}
		if checkerMock.CallCount() != 3 {
			t.Errorf("Expected checker to be called 3 times, but was called %d times", checkerMock.CallCount())
		}
	})

	t.Run("Site-specific retry override", func(t *testing.T) {
		siteWithRetry := config.Website{
			URL: "test.com",
			Retry: &config.Retry{
				Attempts:      2,
				InitialDelay:  "1ms",
				BackoffFactor: 2.0,
			},
		}
		checkerMock := &mockChecker{
			failNTimes: 2,
			err:        fmt.Errorf("final error"),
		}

		_, err := performCheckWithRetries(context.Background(), siteWithRetry, defaultRetry, checkerMock.Check)
		if err == nil {
			t.Fatal("Expected an error, but got nil")
		}
		if checkerMock.CallCount() != 2 {
			t.Errorf("Expected checker to be called 2 times (from site config), but was called %d times", checkerMock.CallCount())
		}
	})
}

// Mock Notifier for InitializeNotifiers tests
type mockNotifier struct {
	fail      bool
	mu        sync.Mutex
	callCount int
}

func (m *mockNotifier) Send(ctx context.Context, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.fail {
		return fmt.Errorf("mock send failed")
	}
	return nil
}

func (m *mockNotifier) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestInitializeNotifiers(t *testing.T) {
	t.Run("All notifiers valid", func(t *testing.T) {
		notifierConfigs := map[string]config.Notifier{
			"log_good":      {Type: "log"},
			"telegram_good": {Type: "telegram", BotToken: "good_token", ChatID: "good_chat"},
		}

		initialized, initErrors := InitializeNotifiers(notifierConfigs)

		if len(initErrors) != 0 {
			t.Fatalf("Expected 0 initErrors, got %d: %v", len(initErrors), initErrors)
		}
		if len(initialized) != 2 {
			t.Fatalf("Expected 2 initialized notifiers, got %d", len(initialized))
		}
		if _, ok := initialized["log_good"]; !ok {
			t.Error("Expected 'log_good' to be initialized")
		}
		if _, ok := initialized["telegram_good"]; !ok {
			t.Error("Expected 'telegram_good' to be initialized")
		}
	})

	t.Run("One notifier invalid and unused", func(t *testing.T) {
		notifierConfigs := map[string]config.Notifier{
			"log_good":     {Type: "log"},
			"telegram_bad": {Type: "telegram", BotToken: "", ChatID: "bad_chat"}, // Invalid
		}

		initialized, initErrors := InitializeNotifiers(notifierConfigs)

		if len(initErrors) != 1 {
			t.Fatalf("Expected 1 initError, got %d: %v", len(initErrors), initErrors)
		}
		if _, ok := initErrors["telegram_bad"]; !ok {
			t.Error("Expected error for 'telegram_bad'")
		}
		if !strings.Contains(initErrors["telegram_bad"].Error(), "bot_token is required") {
			t.Errorf("Expected 'bot_token is required' error, got: %v", initErrors["telegram_bad"])
		}
		if len(initialized) != 1 {
			t.Fatalf("Expected 1 initialized notifier, got %d", len(initialized))
		}
		if _, ok := initialized["log_good"]; !ok {
			t.Error("Expected 'log_good' to be initialized")
		}
	})

	t.Run("All notifiers invalid", func(t *testing.T) {
		notifierConfigs := map[string]config.Notifier{
			"telegram_bad": {Type: "telegram", BotToken: "", ChatID: "bad_chat"}, // Invalid
			"email_bad":    {Type: "email", Host: "", Port: 0},                   // Invalid
		}

		initialized, initErrors := InitializeNotifiers(notifierConfigs)

		if len(initErrors) != 2 {
			t.Fatalf("Expected 2 initErrors, got %d: %v", len(initErrors), initErrors)
		}
		if len(initialized) != 0 {
			t.Fatalf("Expected 0 initialized notifiers, got %d", len(initialized))
		}
	})
}

// Tests for RunChecks function
func TestRunChecks(t *testing.T) {
	t.Run("Check failure triggers notification", func(t *testing.T) {
		// Create a mock checker that always fails
		failingChecker := func(ctx context.Context, url string) (*checker.CertDetails, error) {
			return nil, fmt.Errorf("connection failed")
		}

		notifier := &mockNotifier{fail: false}
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": notifier,
		}

		// Create config with one website
		cfg := &config.Config{
			Websites: []config.Website{
				{
					URL:       "example.com",
					Notifiers: []string{"test_notifier"},
				},
			},
			Settings: config.Settings{
				Retry: config.Retry{
					Attempts:      1,
					InitialDelay:  "1ms",
					BackoffFactor: 1.0,
				},
			},
		}

		// Run checks
		RunChecks(context.Background(), cfg, mockNotifiers, failingChecker)

		// Assert that a notification was sent
		if notifier.CallCount() != 1 {
			t.Errorf("Expected 1 notification to be sent, but got %d", notifier.CallCount())
		}
	})

	t.Run("Expired certificate triggers notification", func(t *testing.T) {
		// Create a mock checker that returns expired cert
		expiredChecker := func(ctx context.Context, url string) (*checker.CertDetails, error) {
			return &checker.CertDetails{
				DaysRemaining: -5.0, // Expired 5 days ago
				ExpiryDate:    time.Now().Add(-5 * 24 * time.Hour),
			}, nil
		}

		notifier := &mockNotifier{fail: false}
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": notifier,
		}

		// Create config with one website
		cfg := &config.Config{
			Websites: []config.Website{
				{
					URL:       "example.com",
					Notifiers: []string{"test_notifier"},
				},
			},
			Settings: config.Settings{
				Retry: config.Retry{
					Attempts:      1,
					InitialDelay:  "1ms",
					BackoffFactor: 1.0,
				},
			},
		}

		// Run checks
		RunChecks(context.Background(), cfg, mockNotifiers, expiredChecker)

		// Assert that a notification was sent
		if notifier.CallCount() != 1 {
			t.Errorf("Expected 1 notification to be sent, but got %d", notifier.CallCount())
		}
	})

	t.Run("Certificate expiring soon triggers notification", func(t *testing.T) {
		// Create a mock checker that returns cert expiring in 7 days
		expiringChecker := func(ctx context.Context, url string) (*checker.CertDetails, error) {
			return &checker.CertDetails{
				DaysRemaining: 7.0, // Expiring in 7 days
				ExpiryDate:    time.Now().Add(7 * 24 * time.Hour),
			}, nil
		}

		notifier := &mockNotifier{fail: false}
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": notifier,
		}

		// Create config with one website that wants warning at 7 days
		cfg := &config.Config{
			Websites: []config.Website{
				{
					URL:             "example.com",
					DaysUntilExpiry: 7, // Warn when 7 days remain
					Notifiers:       []string{"test_notifier"},
				},
			},
			Settings: config.Settings{
				Retry: config.Retry{
					Attempts:      1,
					InitialDelay:  "1ms",
					BackoffFactor: 1.0,
				},
			},
		}

		// Run checks
		RunChecks(context.Background(), cfg, mockNotifiers, expiringChecker)

		// Assert that a notification was sent
		if notifier.CallCount() != 1 {
			t.Errorf("Expected 1 notification to be sent, but got %d", notifier.CallCount())
		}
	})

	t.Run("Valid certificate does not trigger warning notification", func(t *testing.T) {
		// Create a mock checker that returns valid cert with many days remaining
		validChecker := func(ctx context.Context, url string) (*checker.CertDetails, error) {
			return &checker.CertDetails{
				DaysRemaining: 60.0, // Valid for 60 more days
				ExpiryDate:    time.Now().Add(60 * 24 * time.Hour),
			}, nil
		}

		notifier := &mockNotifier{fail: false}
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": notifier,
		}

		// Create config with one website
		cfg := &config.Config{
			Websites: []config.Website{
				{
					URL:             "example.com",
					DaysUntilExpiry: 30, // Only warn at 30 days or less
					Notifiers:       []string{"test_notifier"},
				},
			},
			Settings: config.Settings{
				Retry: config.Retry{
					Attempts:      1,
					InitialDelay:  "1ms",
					BackoffFactor: 1.0,
				},
			},
		}

		// Run checks
		RunChecks(context.Background(), cfg, mockNotifiers, validChecker)

		// Assert that no notification was sent
		if notifier.CallCount() != 0 {
			t.Errorf("Expected 0 notifications to be sent, but got %d", notifier.CallCount())
		}
	})
}

// Tests for dispatchNotifications function
func TestDispatchNotifications(t *testing.T) {
	t.Run("Valid notifier sends notification", func(t *testing.T) {
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": &mockNotifier{fail: false},
		}

		site := config.Website{
			URL:       "example.com",
			Notifiers: []string{"test_notifier"},
		}

		dispatchNotifications(context.Background(), site, "Test Subject", "Test Body", mockNotifiers)
		// This test verifies that the function runs without panicking
	})

	t.Run("Invalid notifier is skipped", func(t *testing.T) {
		mockNotifiers := map[string]notifiers.Notifier{
			"other_notifier": &mockNotifier{fail: false},
		}

		site := config.Website{
			URL:       "example.com",
			Notifiers: []string{"nonexistent_notifier"},
		}

		dispatchNotifications(context.Background(), site, "Test Subject", "Test Body", mockNotifiers)
		// This test verifies that the function runs without panicking when notifier doesn't exist
	})

	t.Run("Failing notifier logs error", func(t *testing.T) {
		mockNotifiers := map[string]notifiers.Notifier{
			"test_notifier": &mockNotifier{fail: true},
		}

		site := config.Website{
			URL:       "example.com",
			Notifiers: []string{"test_notifier"},
		}

		dispatchNotifications(context.Background(), site, "Test Subject", "Test Body", mockNotifiers)
		// This test verifies that the function runs without panicking when notifier fails
	})
}

type mockFlusherNotifier struct {
	flushed bool
}

func (m *mockFlusherNotifier) Send(ctx context.Context, subject, body string) error {
	return nil
}

func (m *mockFlusherNotifier) Flush() error {
	m.flushed = true
	return nil
}

func (m *mockFlusherNotifier) GetPendingMessages() []notifiers.PendingMessage {
	return nil
}

func TestFlushPendingNotifications(t *testing.T) {
	mockFlusher := &mockFlusherNotifier{flushed: true}
	initializedNotifiers := map[string]notifiers.Notifier{
		"flusher": mockFlusher,
	}

	FlushPendingNotifications(context.Background(), initializedNotifiers)

	if !mockFlusher.flushed {
		t.Error("Expected flusher notifier Flush method to be called")
	}
}

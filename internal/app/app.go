package app

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/checker"
	"github.com/the100rabh/ssl-cert-notifier/internal/config"
	"github.com/the100rabh/ssl-cert-notifier/internal/notifiers"
)

// CheckerFunc defines the signature for a function that can check a URL's SSL certificate.
type CheckerFunc func(url string) (*checker.CertDetails, error)

// RunChecks iterates through websites and checks their SSL certificates
func RunChecks(cfg *config.Config, initializedNotifiers map[string]notifiers.Notifier, checkFunc CheckerFunc) {
	log.Printf("Found %d websites to check.", len(cfg.Websites))
	for _, site := range cfg.Websites {
		log.Printf("Checking SSL certificate for %s...", site.URL)

		details, err := performCheckWithRetries(site, cfg.Settings.Retry, checkFunc)

		// Handle check failure
		if err != nil {
			log.Printf("Dispatching notifications for %s...", site.URL)
			subject := fmt.Sprintf("SSL Check Failed for %s", site.URL)
			body := fmt.Sprintf("Failed to check SSL certificate for %s after multiple retries.\n\nError: %v", site.URL, err)
			dispatchNotifications(site, subject, body, initializedNotifiers)
			continue
		}

		// Handle expired certificates
		if details.DaysRemaining <= 0 {
			log.Printf("Dispatching notifications for %s...", site.URL)
			subject := fmt.Sprintf("SSL Certificate Expired for %s", site.URL)
			body := fmt.Sprintf("The SSL certificate for %s expired %.2f days ago.\nExpiry Date: %s",
				site.URL, -details.DaysRemaining, details.ExpiryDate.Format("2006-01-02"))
			dispatchNotifications(site, subject, body, initializedNotifiers)
			continue
		}

		// Handle certificates nearing expiry
		var notified bool
		if details.DaysRemaining <= float64(site.DaysUntilExpiry) {
			log.Printf("Dispatching notifications for %s...", site.URL)
			daysRemaining := int(math.Floor(details.DaysRemaining))
			subject := fmt.Sprintf("SSL Certificate for %s is expiring soon", site.URL)
			body := fmt.Sprintf("The SSL certificate for %s is expiring in %d days.\nExpiry Date: %s",
				site.URL, daysRemaining, details.ExpiryDate.Format("2006-01-02"))
			dispatchNotifications(site, subject, body, initializedNotifiers)
			notified = true
		}

		if !notified {
			log.Printf("SUCCESS: Certificate for %s is valid. Expires in %.2f days (Expiry Date: %s)",
				site.URL, details.DaysRemaining, details.ExpiryDate.Format("2006-01-02"))
		}
	}
}

// InitializeNotifiers creates instances of all notifiers defined in the config.
// It returns a map of successfully initialized notifiers and a map of errors
// for notifiers that failed to initialize.
func InitializeNotifiers(notifierConfigs map[string]config.Notifier) (map[string]notifiers.Notifier, map[string]error) {
	initialized := make(map[string]notifiers.Notifier)
	errors := make(map[string]error)
	for name, conf := range notifierConfigs {
		notifier, err := notifiers.GetNotifier(name, conf)
		if err != nil {
			errors[name] = fmt.Errorf("could not initialize notifier '%s': %w", name, err)
			log.Printf("WARN: Notifier '%s' failed to initialize: %v", name, err)
			continue
		}
		initialized[name] = notifier
	}
	return initialized, errors
}

func dispatchNotifications(site config.Website, subject, body string, initializedNotifiers map[string]notifiers.Notifier) {
	for _, notifierName := range site.Notifiers {
		notifier, ok := initializedNotifiers[notifierName]
		if !ok {
			log.Printf("ERROR: Notifier '%s' for site %s is not defined or initialized. Skipping notification.", notifierName, site.URL)
			continue
		}
		err := notifier.Send(subject, body)
		if err != nil {
			log.Printf("ERROR: Failed to send notification via '%s' for site %s: %v", notifierName, site.URL, err)
		}
	}
}

// performCheckWithRetries wraps the check function with retry logic.
func performCheckWithRetries(site config.Website, defaultRetry config.Retry, checkFunc CheckerFunc) (*checker.CertDetails, error) {
	var attempts int
	var initialDelay time.Duration
	var backoffFactor float64

	retryConfig := site.Retry
	if retryConfig == nil {
		retryConfig = &defaultRetry
	}

	attempts = retryConfig.Attempts
	initialDelay, err := retryConfig.GetInitialDelayDuration()
	if err != nil {
		log.Printf("WARN: Invalid initial_delay for %s, using 30s default: %v", site.URL, err)
		initialDelay = 30 * time.Second
	}
	backoffFactor = retryConfig.BackoffFactor

	var lastErr error
	for i := 0; i < attempts; i++ {
		details, err := checkFunc(site.URL) // Use the passed-in check function
		if err == nil {
			return details, nil
		}

		lastErr = err
		log.Printf("Attempt %d/%d for %s failed: %v", i+1, attempts, site.URL, err)

		if i == attempts-1 {
			break
		}

		waitDuration := float64(initialDelay) * math.Pow(backoffFactor, float64(i))
		log.Printf("Waiting for %v before next retry...", time.Duration(waitDuration))
		time.Sleep(time.Duration(waitDuration))
	}

	return nil, lastErr
}

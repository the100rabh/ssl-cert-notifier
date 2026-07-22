package notifiers

import (
	"context"
	"fmt"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// Notifier is the interface that all notification channel implementations must satisfy.
type Notifier interface {
	Send(ctx context.Context, subject, body string) error
}

// GetNotifier is a factory function that returns the appropriate notifier
// based on the configuration.
func GetNotifier(notifierName string, notifierConfig config.Notifier) (Notifier, error) {
	switch notifierConfig.Type {
	case "log":
		return NewLogNotifier()
	case "telegram":
		return NewTelegramNotifier(notifierConfig)
	case "email":
		return NewEmailNotifier(notifierConfig)
	case "http":
		return NewHTTPNotifier(notifierConfig)
	default:
		return nil, fmt.Errorf("unknown notifier type '%s' for notifier '%s'", notifierConfig.Type, notifierName)
	}
}

package notifiers

import "log"

// LogNotifier is a simple notifier that writes messages to the log.
type LogNotifier struct{}

// NewLogNotifier creates a new LogNotifier.
func NewLogNotifier() (*LogNotifier, error) {
	return &LogNotifier{}, nil
}

// Send prints the notification message to the standard log.
func (n *LogNotifier) Send(subject, body string) error {
	log.Printf("--- NOTIFICATION ---")
	log.Printf("Subject: %s", subject)
	log.Printf("Body: %s", body)
	log.Printf("--------------------")
	return nil
}

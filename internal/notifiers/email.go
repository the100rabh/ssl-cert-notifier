package notifiers

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

// EmailNotifier sends notifications via SMTP.
type EmailNotifier struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

// NewEmailNotifier creates a new EmailNotifier, validating its configuration.
func NewEmailNotifier(cfg config.Notifier) (*EmailNotifier, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required for email notifier")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("port is required for email notifier")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("username is required for email notifier")
	}
	// Password can sometimes be optional for SMTP relays on the same network
	// but for public SMTP servers, it's almost always required.
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required for email notifier")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("from address is required for email notifier")
	}
	if len(cfg.To) == 0 {
		return nil, fmt.Errorf("at least one 'to' address is required for email notifier")
	}
	return &EmailNotifier{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		To:       cfg.To,
	}, nil
}

// SMTPClient interface to allow mocking for tests
type SMTPClient interface {
	Extension(string) (bool, string)
	StartTLS(*tls.Config) error
	Auth(smtp.Auth) error
	Mail(string) error
	Rcpt(string) error
	Data() (io.WriteCloser, error)
	Quit() error
}

// Dialer interface to allow mocking for tests
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// SMTPClientFactory interface to allow mocking for tests
type SMTPClientFactory interface {
	NewClient(conn net.Conn, host string) (SMTPClient, error)
}

// RealSMTPClientFactory implements SMTPClientFactory using the real smtp package
type RealSMTPClientFactory struct{}

func (f *RealSMTPClientFactory) NewClient(conn net.Conn, host string) (SMTPClient, error) {
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, err
	}
	return &smtpClientWrapper{client}, nil
}

// DefaultDialer implements Dialer using net.Dialer
type DefaultDialer struct {
	*net.Dialer
}

func (d *DefaultDialer) Dial(network, address string) (net.Conn, error) {
	return d.Dialer.Dial(network, address)
}

// Send sends the message via SMTP.
func (n *EmailNotifier) Send(subject, body string) error {
	return n.SendWithDeps(subject, body, &DefaultDialer{&net.Dialer{Timeout: 10 * time.Second}}, &RealSMTPClientFactory{})
}

// SendWithDeps allows dependency injection for testing
func (n *EmailNotifier) SendWithDeps(subject, body string, dialer Dialer, clientFactory SMTPClientFactory) error {
	// Create the email message
	headers := make(map[string]string)
	headers["From"] = n.From
	headers["To"] = strings.Join(n.To, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=utf-8"
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n") // End of headers
	message.WriteString(body)

	// Connect to the SMTP server
	auth := smtp.PlainAuth("", n.Username, n.Password, n.Host)

	// Establish connection
	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", n.Host, n.Port))
	if err != nil {
		log.Printf("ERROR: Failed to connect to SMTP server %s:%d: %v", n.Host, n.Port, err)
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create a new client using the factory
	client, err := clientFactory.NewClient(conn, n.Host)
	if err != nil {
		log.Printf("ERROR: Failed to create SMTP client: %v", err)
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// Always attempt STARTTLS if supported by the server, regardless of port
	// The client.Extension("STARTTLS") checks if the server advertises STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName: n.Host,
			// Skip verification for flexibility with self-signed certificates in testing
			// In production, consider setting InsecureSkipVerify to false and providing trusted CAs.
			InsecureSkipVerify: true,
		}
		if err = client.StartTLS(config); err != nil {
			log.Printf("ERROR: Failed to start TLS: %v", err)
			client.Quit()
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err = client.Auth(auth); err != nil {
		log.Printf("ERROR: Failed to authenticate with SMTP server: %v", err)
		client.Quit()
		return fmt.Errorf("failed to authenticate with SMTP server: %w", err)
	}

	// Set the sender
	if err = client.Mail(n.From); err != nil {
		log.Printf("ERROR: Failed to set sender (%s): %v", n.From, err)
		client.Quit()
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set the recipients
	for _, recipient := range n.To {
		if err = client.Rcpt(recipient); err != nil {
			log.Printf("ERROR: Failed to set recipient (%s): %v", recipient, err)
			client.Quit()
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send the email body
	writer, err := client.Data()
	if err != nil {
		log.Printf("ERROR: Failed to create data writer: %v", err)
		client.Quit()
		return fmt.Errorf("failed to create data writer: %w", err)
	}

	_, err = writer.Write([]byte(message.String()))
	writer.Close() // Close writer before quitting client
	if err != nil {
		log.Printf("ERROR: Failed to write email data: %v", err)
		client.Quit()
		return fmt.Errorf("failed to write email data: %w", err)
	}

	err = client.Quit()
	if err != nil {
		log.Printf("ERROR: Error closing SMTP connection: %v", err)
		// Don't return this error as the email was sent successfully
	}

	log.Printf("INFO: Email notification sent successfully to %v from %s via %s:%d", n.To, n.From, n.Host, n.Port)
	return nil
}

// smtpClientWrapper wraps the real smtp.Client to implement our SMTPClient interface
type smtpClientWrapper struct {
	*smtp.Client
}

func (w *smtpClientWrapper) StartTLS(config *tls.Config) error {
	return w.Client.StartTLS(config)
}

func (w *smtpClientWrapper) Quit() error {
	return w.Client.Quit()
}

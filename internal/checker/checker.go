package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

// CertDetails holds the relevant information about a certificate.
type CertDetails struct {
	ExpiryDate    time.Time
	DaysRemaining float64
}

// Check performs a TLS connection to the given URL and returns the certificate details.
func Check(ctx context.Context, url string) (*CertDetails, error) {
	// Remove scheme if present (e.g., "https://")
	if idx := strings.Index(url, "://"); idx != -1 {
		url = url[idx+3:]
	}
	// Also strip any leading/trailing whitespace
	url = strings.TrimSpace(url)

	// Ensure the URL has a port. Default to 443 if not present.
	host, port, err := net.SplitHostPort(url)
	if err != nil {
		// If SplitHostPort fails, it might be because there's no port.
		// We'll assume port 443 for TLS.
		host = url
		port = "443"
	}
	dialAddr := net.JoinHostPort(host, port)

	// We use tls.Dialer to set a timeout and context for the connection attempt.
	netDialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	tlsDialer := &tls.Dialer{
		NetDialer: netDialer,
		Config: &tls.Config{
			// We don't need to verify the certificate chain here because we are
			// interested in the certificate itself, even if it's invalid.
			InsecureSkipVerify: true,
		},
	}

	conn, err := tlsDialer.DialContext(ctx, "tcp", dialAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", dialAddr, err)
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("connection is not a TLS connection")
	}
	defer tlsConn.Close()

	// Get the peer certificates from the connection state.
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found for %s", dialAddr)
	}

	// The first certificate in the chain is the leaf certificate.
	leafCert := certs[0]

	// Calculate the number of days remaining until expiry.
	now := time.Now()
	daysRemaining := leafCert.NotAfter.Sub(now).Hours() / 24

	details := &CertDetails{
		ExpiryDate:    leafCert.NotAfter,
		DaysRemaining: daysRemaining,
	}

	return details, nil
}

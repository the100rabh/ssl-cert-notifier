package checker

import (
	"context"
	"strings"
	"testing"
)

// TestMain runs tests that require network access.
func TestCheckValidCertificate(t *testing.T) {
	t.Parallel()
	// Use a reliable domain with a valid SSL certificate
	url := "google.com:443"
	details, err := Check(context.Background(), url)
	if err != nil {
		t.Fatalf("Check(%s) failed: %v", url, err)
	}

	if details == nil {
		t.Fatal("Check() returned nil details for a valid certificate")
	}

	if details.ExpiryDate.IsZero() {
		t.Error("ExpiryDate is zero")
	}
	if details.DaysRemaining <= 0 {
		t.Errorf("DaysRemaining is %f, expected > 0 for a valid certificate", details.DaysRemaining)
	}
}

func TestBadSSLCertificates(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		url         string
		expectError bool
		check       func(t *testing.T, details *CertDetails)
	}{
		{
			name:        "Expired",
			url:         "expired.badssl.com:443",
			expectError: false,
			check: func(t *testing.T, details *CertDetails) {
				if details.DaysRemaining >= 0 {
					t.Errorf("Expected DaysRemaining < 0 for expired cert, got %f", details.DaysRemaining)
				}
			},
		},
		{
			name:        "Wrong Host",
			url:         "wrong.host.badssl.com:443",
			expectError: false,
			check: func(t *testing.T, details *CertDetails) {
				if details.DaysRemaining <= 0 {
					t.Errorf("Expected DaysRemaining > 0 for wrong host cert, got %f", details.DaysRemaining)
				}
			},
		},
		{
			name:        "Self Signed",
			url:         "self-signed.badssl.com:443",
			expectError: false,
			check: func(t *testing.T, details *CertDetails) {
				if details.DaysRemaining <= 0 {
					t.Errorf("Expected DaysRemaining > 0 for self-signed cert, got %f", details.DaysRemaining)
				}
			},
		},
		{
			name:        "Incomplete Chain",
			url:         "incomplete-chain.badssl.com:443",
			expectError: false,
			check: func(t *testing.T, details *CertDetails) {
				if details.DaysRemaining <= 0 {
					t.Errorf("Expected DaysRemaining > 0 for incomplete chain cert, got %f", details.DaysRemaining)
				}
			},
		},
		{
			name:        "Wildcard (Expired)",
			url:         "wildcard.badssl.com:443",
			expectError: false,
			check: func(t *testing.T, details *CertDetails) {
				if details.DaysRemaining >= 0 {
					t.Errorf("Expected DaysRemaining < 0 for expired wildcard cert, got %f", details.DaysRemaining)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			details, err := Check(context.Background(), tc.url)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
			if details == nil {
				t.Fatal("Expected details but got nil")
			}
			tc.check(t, details)
		})
	}
}

func TestNetworkErrors(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name              string
		url               string
		expectedErrSubstr []string
	}{
		{
			name:              "Unreachable Host",
			url:               "192.0.2.1:443", // TEST-NET-1 as per RFC 5737
			expectedErrSubstr: []string{"timeout", "connection refused", "no route to host"},
		},
		{
			name:              "Non-TLS Port",
			url:               "neverssl.com:80",
			expectedErrSubstr: []string{"no certificates found", "tls: first record does not look like a TLS handshake", "timeout"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Check(context.Background(), tc.url)
			if err == nil {
				t.Fatalf("Expected an error but got nil")
			}

			match := false
			for _, substr := range tc.expectedErrSubstr {
				if strings.Contains(err.Error(), substr) {
					match = true
					break
				}
			}
			if !match {
				t.Errorf("Error message '%v' did not contain any of the expected substrings: %v", err, tc.expectedErrSubstr)
			}
		})
	}
}

func TestCheckSplitHostPort(t *testing.T) {
	t.Parallel()
	// Test a URL without a port specified
	url := "example.com"
	details, err := Check(context.Background(), url)
	if err != nil {
		t.Fatalf("Check(%s) failed: %v", url, err)
	}
	if details == nil {
		t.Fatal("Check() returned nil details for a valid certificate without explicit port")
	}
	if details.DaysRemaining <= 0 {
		t.Errorf("DaysRemaining is %f, expected > 0 for a valid certificate", details.DaysRemaining)
	}
}

package netclient

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestWindows7CompatibilityTransportIsBounded(t *testing.T) {
	transport := NewTransport()
	if transport.ForceAttemptHTTP2 {
		t.Fatal("HTTP/2 must remain disabled for the Go 1.20 compatibility client")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatal("TLS 1.2 minimum is required")
	}
	if !transport.TLSClientConfig.SessionTicketsDisabled {
		t.Fatal("TLS session resumption must remain disabled")
	}
	if transport.ResponseHeaderTimeout <= 0 || transport.MaxResponseHeaderBytes <= 0 {
		t.Fatal("response headers must be time/size bounded")
	}
	client := New(15 * time.Second)
	if client.Timeout != 15*time.Second || client.Transport == nil {
		t.Fatal("whole-call timeout and custom transport are required")
	}
}

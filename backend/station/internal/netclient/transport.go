// Package netclient centralizes the constrained outbound HTTP transport used
// by the Windows 7 compatibility agent.
package netclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewTransport returns a bounded HTTP/1.1 transport. Go 1.20 is intentionally
// retained for Windows 7, so HTTP/2 and TLS session resumption are disabled to
// reduce exposure to protocol/parser advisories in that end-of-life stdlib.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:      false,
		MaxIdleConns:           32,
		MaxIdleConnsPerHost:    8,
		MaxConnsPerHost:        32,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		MaxResponseHeaderBytes: 1 << 20,
		TLSClientConfig: &tls.Config{
			MinVersion:             tls.VersionTLS12,
			SessionTicketsDisabled: true,
		},
	}
}

// New returns a client with both per-phase transport bounds and a whole-call
// timeout. Callers own the returned transport and its idle connections.
func New(timeout time.Duration) *http.Client {
	return &http.Client{Transport: NewTransport(), Timeout: timeout}
}

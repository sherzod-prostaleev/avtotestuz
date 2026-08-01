package push

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type goneError struct{ err error }

func (e goneError) Error() string { return e.err.Error() }
func (e goneError) Unwrap() error { return e.err }

func isGone(err error) bool {
	var g goneError
	return errors.As(err, &g)
}

// VAPIDSender delivers via the Web Push protocol.
type VAPIDSender struct {
	Cfg Config
}

func NewVAPIDSender(cfg Config) *VAPIDSender {
	return &VAPIDSender{Cfg: cfg}
}

func (s *VAPIDSender) Send(ctx context.Context, sub Subscription, payload []byte) error {
	subject := strings.TrimSpace(s.Cfg.Subject)
	if subject == "" {
		subject = "mailto:ops@avtotest.uz"
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      subject,
		VAPIDPublicKey:  strings.TrimSpace(s.Cfg.PublicKey),
		VAPIDPrivateKey: strings.TrimSpace(s.Cfg.PrivateKey),
		TTL:             60,
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		return goneError{err: fmt.Errorf("subscription gone: HTTP %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("web push HTTP %d", resp.StatusCode)
	}
	return nil
}

// FakeSender records payloads for tests.
type FakeSender struct {
	mu    sync.Mutex
	Calls []FakeSend
	Err   error
}

type FakeSend struct {
	Sub     Subscription
	Payload []byte
}

func (f *FakeSender) Send(_ context.Context, sub Subscription, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, FakeSend{Sub: sub, Payload: append([]byte(nil), payload...)})
	return f.Err
}

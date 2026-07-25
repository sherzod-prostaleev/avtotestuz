// Package push implements M4-08 web-push subscription storage and delivery.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrUnconfigured    = errors.New("web push is not configured")
	ErrBadEndpoint     = errors.New("invalid push endpoint")
	ErrBadKeys         = errors.New("invalid push keys")
	ErrNoSubs          = errors.New("no push subscriptions")
	ErrRateLimited     = errors.New("push test rate limited")
	ErrFeatureDisabled = errors.New("feature_disabled")
)

// TestCooldown is the minimum gap between learner self-test pushes.
const TestCooldown = 60 * time.Second

const (
	maxTitleLen = 120
	maxBodyLen  = 500
	defaultURL  = "/uz-Latn/dashboard"
)

// Sender delivers a payload to a single browser subscription.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) error
}

// Subscription is the browser PushSubscription shape we persist.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Config holds VAPID material. Empty public+private means disabled.
type Config struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

func (c Config) Configured() bool {
	return strings.TrimSpace(c.PublicKey) != "" && strings.TrimSpace(c.PrivateKey) != ""
}

// Service stores subscriptions and sends web-push notifications.
type Service struct {
	Q      *sqlc.Queries
	Pool   *pgxpool.Pool
	Cfg    Config
	Sender Sender
}

func NewService(pool *pgxpool.Pool, q *sqlc.Queries, cfg Config, sender Sender) *Service {
	if sender == nil && cfg.Configured() {
		sender = NewVAPIDSender(cfg)
	}
	return &Service{Q: q, Pool: pool, Cfg: cfg, Sender: sender}
}

// Status is the learner-facing GET /me/push payload.
type Status struct {
	Configured        bool   `json:"configured"`
	Subscribed        bool   `json:"subscribed"`
	SubscriptionCount int    `json:"subscription_count"`
	VAPIDPublicKey    string `json:"vapid_public_key,omitempty"`
}

func (s *Service) Status(ctx context.Context, profileID uuid.UUID) (Status, error) {
	n, err := s.Q.CountPushSubscriptions(ctx, profileID)
	if err != nil {
		return Status{}, err
	}
	out := Status{
		Configured:        s.Cfg.Configured(),
		Subscribed:        n > 0,
		SubscriptionCount: int(n),
	}
	if out.Configured {
		out.VAPIDPublicKey = strings.TrimSpace(s.Cfg.PublicKey)
	}
	return out, nil
}

type SubscribeInput struct {
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
}

func (s *Service) Subscribe(ctx context.Context, profileID uuid.UUID, in SubscribeInput) error {
	if !s.Cfg.Configured() {
		return ErrUnconfigured
	}
	endpoint := strings.TrimSpace(in.Endpoint)
	p256dh := strings.TrimSpace(in.P256dh)
	auth := strings.TrimSpace(in.Auth)
	if endpoint == "" || !strings.HasPrefix(endpoint, "https://") {
		return ErrBadEndpoint
	}
	if p256dh == "" || auth == "" {
		return ErrBadKeys
	}
	ua := strings.TrimSpace(in.UserAgent)
	if len(ua) > 512 {
		ua = ua[:512]
	}
	_, err := s.Q.UpsertPushSubscription(ctx, sqlc.UpsertPushSubscriptionParams{
		ProfileID: profileID,
		Endpoint:  endpoint,
		P256dh:    p256dh,
		Auth:      auth,
		UserAgent: ua,
	})
	return err
}

func (s *Service) Unsubscribe(ctx context.Context, profileID uuid.UUID, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ErrBadEndpoint
	}
	_, err := s.Q.DeletePushSubscription(ctx, sqlc.DeletePushSubscriptionParams{
		ProfileID: profileID,
		Endpoint:  endpoint,
	})
	return err
}

// NotifyPayload is the JSON body delivered to the service worker.
type NotifyPayload struct {
	Title string         `json:"title"`
	Body  string         `json:"body"`
	URL   string         `json:"url,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

func sanitizePayload(p NotifyPayload) NotifyPayload {
	p.Title = strings.TrimSpace(p.Title)
	p.Body = strings.TrimSpace(p.Body)
	if p.Title == "" {
		p.Title = "Driver Go"
	}
	if len(p.Title) > maxTitleLen {
		p.Title = p.Title[:maxTitleLen]
	}
	if len(p.Body) > maxBodyLen {
		p.Body = p.Body[:maxBodyLen]
	}
	url := strings.TrimSpace(p.URL)
	if url == "" || !strings.HasPrefix(url, "/") || strings.HasPrefix(url, "//") || strings.Contains(url, "://") {
		url = defaultURL
	}
	p.URL = url
	return p
}

// Notify writes a notification row and attempts delivery to every subscription.
func (s *Service) Notify(ctx context.Context, profileID uuid.UUID, kind string, payload NotifyPayload) (sent int, err error) {
	if !s.Cfg.Configured() {
		return 0, ErrUnconfigured
	}
	if s.Sender == nil {
		return 0, ErrUnconfigured
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "generic"
	}
	payload = sanitizePayload(payload)
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	row, err := s.Q.InsertNotification(ctx, sqlc.InsertNotificationParams{
		ProfileID: profileID,
		Kind:      kind,
		Payload:   raw,
		Channel:   "webpush",
	})
	if err != nil {
		return 0, err
	}

	subs, err := s.Q.ListPushSubscriptions(ctx, profileID)
	if err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, ErrNoSubs
	}

	var lastErr error
	for _, sub := range subs {
		sendErr := s.Sender.Send(ctx, Subscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.P256dh,
			Auth:     sub.Auth,
		}, raw)
		if sendErr != nil {
			if isGone(sendErr) {
				_, _ = s.Q.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint)
			}
			lastErr = sendErr
			continue
		}
		sent++
	}
	if sent > 0 {
		_ = s.Q.MarkNotificationSent(ctx, row.ID)
	}
	if sent == 0 && lastErr != nil {
		return 0, fmt.Errorf("web push delivery failed: %w", lastErr)
	}
	return sent, nil
}

// SendTest delivers a short self-test notification to the caller's devices.
// Repeated clicks within TestCooldown return ErrRateLimited (no second delivery).
func (s *Service) SendTest(ctx context.Context, profileID uuid.UUID) (int, error) {
	if s.Pool != nil {
		var last time.Time
		err := s.Pool.QueryRow(ctx, `
			SELECT created_at
			FROM notification
			WHERE profile_id = $1 AND kind = 'push_test' AND channel = 'webpush'
			ORDER BY created_at DESC
			LIMIT 1`, profileID).Scan(&last)
		if err == nil && time.Since(last) < TestCooldown {
			return 0, ErrRateLimited
		}
	}
	return s.Notify(ctx, profileID, "push_test", NotifyPayload{
		Title: "Driver Go",
		Body:  "Web push ishga tushdi / Push works",
		URL:   defaultURL,
		Data:  map[string]any{"kind": "push_test", "at": time.Now().UTC().Format(time.RFC3339)},
	})
}

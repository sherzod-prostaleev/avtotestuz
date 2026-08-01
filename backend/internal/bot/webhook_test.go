package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

func newTestWebhookHandler(t *testing.T) (*WebhookHandler, *fakeTelegram) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	fake, client := newFakeTelegram(t)
	b := &Bot{
		Link:     NewLinkService(pool, q),
		Billing:  billing.Service{Q: q},
		Progress: progress.NewService(q),
		TG:       client,
	}
	return &WebhookHandler{Bot: b, Secret: "top-secret"}, fake
}

func TestWebhook_RejectsMissingSecret(t *testing.T) {
	h, fake := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if fake.lastMessage() != "" {
		t.Error("dispatcher must not run without a valid secret")
	}
}

func TestWebhook_RejectsEmptyConfiguredSecret(t *testing.T) {
	h, fake := newTestWebhookHandler(t)
	h.Secret = ""
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if fake.lastMessage() != "" {
		t.Error("empty secret must not authenticate omitted headers")
	}
}

func TestWebhook_RejectsWrongSecret(t *testing.T) {
	h, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestWebhook_ValidUpdateDispatchesAndReturns200(t *testing.T) {
	h, fake := newTestWebhookHandler(t)
	body := `{"update_id":1,"message":{"message_id":1,"text":"/start","chat":{"id":50},"from":{"id":50,"username":"h"}}}`
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "top-secret")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if fake.lastMessage() == "" {
		t.Error("expected the dispatcher to have sent a reply")
	}
}

func TestWebhook_MalformedBodyStillReturns200(t *testing.T) {
	h, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{not-json`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "top-secret")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (must not trigger Telegram retries)", rr.Code)
	}
}

func TestWebhook_TransientDispatchFailureReturns503ForRetry(t *testing.T) {
	h, _ := newTestWebhookHandler(t)
	body := `{"update_id":2,"message":{"message_id":2,"text":"/start","chat":{"id":51},"from":{"id":51,"username":"h"}}}`
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "top-secret")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 so Telegram retries", rr.Code)
	}
}

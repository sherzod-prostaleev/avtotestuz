package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

// baseTestConfig mirrors config.Load()'s dev defaults closely enough for
// server.New to wire every route (a non-empty JWT secret, sandbox OTP).
func baseTestConfig() config.Config {
	return config.Config{
		Env:             "dev",
		JWTSecret:       "test-secret-at-least-32-bytes!!",
		OTPChannel:      "sandbox",
		PublicBaseURL:   "http://localhost:3000",
		TelegramBotMode: "off",
	}
}

func TestBotRoutes_NotMountedWhenModeOff(t *testing.T) {
	pool := testdb.New(t)
	rdb := redisx.NewTest(t)
	h := New(baseTestConfig(), Deps{Queries: sqlc.New(pool), Pool: pool, Redis: rdb})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/telegram/webhook", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("webhook route with mode=off: status = %d, want 404", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/me/telegram/link-token", nil)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 404 {
		t.Errorf("link-token route with mode=off: status = %d, want 404", rec2.Code)
	}
}

func TestBotRoutes_WebhookModeMountsBothRoutes(t *testing.T) {
	pool := testdb.New(t)
	rdb := redisx.NewTest(t)
	cfg := baseTestConfig()
	cfg.TelegramBotMode = "webhook"
	cfg.TelegramBotToken = "test-bot-token"
	cfg.TelegramWebhookSecret = "test-webhook-secret"
	cfg.TelegramBotAPIBaseURL = "http://127.0.0.1:0" // never actually dialed in this test
	cfg.TelegramBotUsername = "AvtoTestBot"
	q := sqlc.New(pool)
	h := New(cfg, Deps{Queries: q, Pool: pool, Redis: rdb})

	// Webhook route exists and enforces the secret header.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/telegram/webhook", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("webhook without secret: status = %d, want 401", rec.Code)
	}

	// The authenticated link-token route exists and requires auth.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/me/telegram/link-token", nil)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 401 {
		t.Errorf("link-token without auth: status = %d, want 401", rec2.Code)
	}

	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901140001"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(cfg.JWTSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/api/v1/me/telegram/link-token", nil)
	req3.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec3, req3)
	if rec3.Code != 200 {
		t.Errorf("link-token with auth: status = %d, want 200; body: %s", rec3.Code, rec3.Body.String())
	}
}

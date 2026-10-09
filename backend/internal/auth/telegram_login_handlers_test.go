package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func telegramLoginServer(t *testing.T) (*httptest.Server, *Service) {
	t.Helper()
	svc, _ := resetTestService(t)
	svc.TelegramBotToken = testBotToken
	svc.TelegramWebAppURL = testWebAppURL
	r := chi.NewRouter()
	(&Handler{Svc: svc, BotUsername: testLoginBot}).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, svc
}

func TestTelegramLoginOverHTTP(t *testing.T) {
	ts, svc := telegramLoginServer(t)
	status, env := postJSON(t, ts, "/auth/telegram-login/start", map[string]string{"user_agent": chromeAndroidUA})
	if status != http.StatusOK {
		t.Fatalf("start status=%d %+v", status, env.Error)
	}
	var st TelegramLoginStart
	if err := json.Unmarshal(env.Data, &st); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(st.BotURL, "https://t.me/"+testLoginBot+"?start=login_") || st.BrowserSecret == "" {
		t.Fatalf("start = %+v", st)
	}
	body := map[string]string{"token": st.Token, "browser_secret": st.BrowserSecret}

	_, env = postJSON(t, ts, "/auth/telegram-login/status", body)
	if string(env.Data) != `{"state":"pending"}` {
		t.Fatalf("status = %s", env.Data)
	}
	status, env = postJSON(t, ts, "/auth/telegram-login/complete", body)
	if status != http.StatusConflict || env.Error == nil || env.Error.Code != "login_not_approved" {
		t.Fatalf("early complete = %d %+v", status, env.Error)
	}

	ctx := context.Background()
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(8800)); err != nil {
		t.Fatal(err)
	}
	if r := shareAndApprove(t, svc, tgWho(8800), "998901119900"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	// The approved status says only "approved": no phone, no name.
	_, env = postJSON(t, ts, "/auth/telegram-login/status", body)
	if string(env.Data) != `{"state":"approved"}` {
		t.Fatalf("approved status = %s", env.Data)
	}
	status, env = postJSON(t, ts, "/auth/telegram-login/complete", map[string]string{"token": st.Token, "browser_secret": "other"})
	if status != http.StatusBadRequest || env.Error.Code != "invalid_login_request" {
		t.Fatalf("wrong secret = %d %+v", status, env.Error)
	}
	status, env = postJSON(t, ts, "/auth/telegram-login/complete", body)
	if status != http.StatusOK {
		t.Fatalf("complete = %d %+v", status, env.Error)
	}
	var toks telegramLoginCompleteResponse
	if err := json.Unmarshal(env.Data, &toks); err != nil || toks.AccessToken == "" || toks.RefreshToken == "" || !toks.Created {
		t.Fatalf("tokens = %+v %v", toks, err)
	}
	// Which account the browser now holds, masked: shown once on the website.
	if toks.PhoneMasked != "+998 90 ••• •• 00" || strings.Contains(string(env.Data), "998901119900") {
		t.Fatalf("phone_masked = %q in %s", toks.PhoneMasked, env.Data)
	}
	status, _ = postJSON(t, ts, "/auth/telegram-login/complete", body)
	if status != http.StatusBadRequest {
		t.Fatalf("replay status=%d", status)
	}
}

func TestTelegramLoginStartWithoutBotIs503(t *testing.T) {
	svc, _ := resetTestService(t)
	r := chi.NewRouter()
	(&Handler{Svc: svc}).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	status, env := postJSON(t, ts, "/auth/telegram-login/start", map[string]string{})
	if status != http.StatusServiceUnavailable || env.Error.Code != "telegram_bot_unconfigured" {
		t.Fatalf("status=%d %+v", status, env.Error)
	}
}

func TestTelegramWebAppPhoneOverHTTP(t *testing.T) {
	ts, _ := telegramLoginServer(t)
	initData, contact := webAppPhoneProof(t, 8801, "998901119911", "", time.Now())
	status, env := postJSON(t, ts, "/auth/telegram/webapp/phone", map[string]string{"init_data": initData, "contact": contact})
	if status != http.StatusOK {
		t.Fatalf("status=%d %+v", status, env.Error)
	}
	var toks tokensResponse
	if err := json.Unmarshal(env.Data, &toks); err != nil || toks.AccessToken == "" || !toks.TelegramLinked || !toks.Created {
		t.Fatalf("tokens = %+v", toks)
	}
	_, foreign := webAppPhoneProof(t, 8801, "79161234567", "", time.Now())
	status, env = postJSON(t, ts, "/auth/telegram/webapp/phone", map[string]string{"init_data": initData, "contact": foreign})
	if status != http.StatusBadRequest || env.Error.Code != "invalid_phone" {
		t.Fatalf("foreign = %d %+v", status, env.Error)
	}
}

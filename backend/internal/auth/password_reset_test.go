package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func resetTestService(t *testing.T) (*Service, *sqlc.Queries) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	c := redisx.NewTest(t)
	svc := NewService(q, pool, Limiter{R: c}, SandboxSender{Log: zap.NewNop()}, []byte(handlerSecret), "test")
	return svc, q
}

func parseResetRaw(t *testing.T, botURL string) string {
	t.Helper()
	const marker = "?start=" + PasswordResetStartPrefix
	i := strings.Index(botURL, marker)
	if i < 0 {
		t.Fatalf("bot_url %q missing %s", botURL, marker)
	}
	raw := botURL[i+len(marker):]
	if raw == "" {
		t.Fatal("empty reset token")
	}
	return raw
}

func TestStartPasswordReset_UnknownPhoneLooksLikeKnown(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()

	unknown, err := svc.StartPasswordReset(ctx, "901000001", "1.1.1.1", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unknown.BotURL, "https://t.me/AvtoTestBot?start=pwr_") {
		t.Fatalf("unknown bot_url=%q", unknown.BotURL)
	}

	var n int
	if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM password_reset_token`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("unknown phone stored %d tokens", n)
	}

	if _, err := svc.Register(ctx, RegisterInput{Phone: "901000002", Password: "secret123", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	known, err := svc.StartPasswordReset(ctx, "901000002", "1.1.1.2", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	if unknown.ExpiresInSec != known.ExpiresInSec {
		t.Fatalf("expires_in_sec unknown=%d known=%d", unknown.ExpiresInSec, known.ExpiresInSec)
	}
	if !strings.HasPrefix(known.BotURL, "https://t.me/AvtoTestBot?start=pwr_") {
		t.Fatalf("known bot_url=%q", known.BotURL)
	}
	if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM password_reset_token`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("known phone tokens=%d", n)
	}

	unknownRaw := parseResetRaw(t, unknown.BotURL)
	knownRaw := parseResetRaw(t, known.BotURL)
	if svc.PasswordResetStatus(ctx, unknownRaw).State != ResetStatePending {
		t.Fatal("unknown phone status must look pending (no enumeration)")
	}
	if svc.PasswordResetStatus(ctx, knownRaw).State != ResetStatePending {
		t.Fatal("known phone status must be pending")
	}
	if svc.PasswordResetStatus(ctx, "not-issued-token").State != ResetStateInvalid {
		t.Fatal("never-issued token must be invalid")
	}
	_ = q
}

func TestPasswordReset_LinkedTelegramThenComplete(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	const phone = "901000010"
	const oldPass = "oldpass12"
	const newPass = "newpass12"

	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: oldPass, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: reg.Profile.ID,
		TgUserID:  4242,
		Username:  "alice",
	}); err != nil {
		t.Fatal(err)
	}

	start, err := svc.StartPasswordReset(ctx, phone, "2.2.2.2", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)

	st := svc.PasswordResetStatus(ctx, raw)
	if st.State != ResetStatePending {
		t.Fatalf("status=%s want pending", st.State)
	}
	if err := svc.CompletePasswordReset(ctx, raw, newPass, "2.2.2.2"); !errors.Is(err, ErrResetNotVerified) {
		t.Fatalf("complete before verify err=%v", err)
	}

	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramResetVerified {
		t.Fatalf("begin=%s want verified", begin.Outcome)
	}
	if svc.PasswordResetStatus(ctx, raw).State != ResetStateVerified {
		t.Fatal("expected verified status")
	}

	if err := svc.CompletePasswordReset(ctx, raw, newPass, "2.2.2.2"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: oldPass}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("old password still worked: %v", err)
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: newPass}); err != nil {
		t.Fatalf("new password login: %v", err)
	}
	if err := svc.CompletePasswordReset(ctx, raw, "another12", "2.2.2.2"); !errors.Is(err, ErrResetInvalid) {
		t.Fatalf("reuse err=%v", err)
	}
}

func TestPasswordReset_ContactMustMatchAccountPhone(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const phone = "901000011"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "secret123", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, phone, "3.3.3.3", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)

	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, 77)
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%s want need_contact", begin.Outcome)
	}

	mismatch, err := svc.ConfirmTelegramPasswordResetContact(ctx, 77, 77, "901000099")
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Outcome != TelegramResetInvalid {
		t.Fatalf("mismatch=%s", mismatch.Outcome)
	}
	spoof, err := svc.ConfirmTelegramPasswordResetContact(ctx, 77, 88, "+998"+phone)
	if err != nil {
		t.Fatal(err)
	}
	if spoof.Outcome != TelegramResetInvalid {
		t.Fatalf("spoofed contact user_id accepted: %s", spoof.Outcome)
	}

	ok, err := svc.ConfirmTelegramPasswordResetContact(ctx, 77, 77, "998"+phone)
	if err != nil {
		t.Fatal(err)
	}
	if ok.Outcome != TelegramResetVerified {
		t.Fatalf("matching contact=%s", ok.Outcome)
	}
}

func TestPasswordReset_MissingBotUsername(t *testing.T) {
	svc, _ := resetTestService(t)
	_, err := svc.StartPasswordReset(context.Background(), "901000012", "", "")
	if !errors.Is(err, ErrTelegramBotUnconfigured) {
		t.Fatalf("err=%v", err)
	}
}

func TestPasswordResetHTTP_StartStatusComplete(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	const phone = "901000013"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "secret123", Name: "C"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: reg.Profile.ID,
		TgUserID:  91,
		Username:  "cara",
	}); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	(&Handler{Svc: svc, BotUsername: "AvtoTestBot"}).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	status, env := postJSON(t, ts, "/auth/password-reset/start", map[string]string{"phone": phone})
	if status != http.StatusOK {
		t.Fatalf("start status=%d env=%+v", status, env)
	}
	var start PasswordResetStart
	if err := json.Unmarshal(env.Data, &start); err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)
	if strings.Contains(string(env.Data), phone) || strings.Contains(string(env.Data), "+998") {
		t.Fatalf("start response leaked phone: %s", env.Data)
	}

	resp, err := ts.Client().Get(ts.URL + "/auth/password-reset/status?token=" + raw)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var stEnv respEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&stEnv); err != nil {
		t.Fatal(err)
	}
	var st PasswordResetStatus
	if err := json.Unmarshal(stEnv.Data, &st); err != nil {
		t.Fatal(err)
	}
	if st.State != ResetStatePending {
		t.Fatalf("http status=%s", st.State)
	}

	if _, err := svc.BeginTelegramPasswordReset(ctx, raw, 91); err != nil {
		t.Fatal(err)
	}
	status, env = postJSON(t, ts, "/auth/password-reset/complete", map[string]string{
		"token":    raw,
		"password": "brandnew1",
	})
	if status != http.StatusOK {
		t.Fatalf("complete status=%d env=%+v", status, env)
	}
	if strings.Contains(string(env.Data), "brandnew1") {
		t.Fatal("complete echoed password")
	}
}

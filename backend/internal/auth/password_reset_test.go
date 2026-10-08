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

	// A linked account still has to answer the explicit «Ha, men» question:
	// opening someone else's deep link must never verify on its own.
	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramResetNeedConfirm || begin.ConfirmNonce == "" {
		t.Fatalf("begin=%+v want need_confirm with nonce", begin)
	}
	if begin.MaskedPhone != "+998 90 ••• •• 10" {
		t.Fatalf("masked phone=%q", begin.MaskedPhone)
	}
	if svc.PasswordResetStatus(ctx, raw).State != ResetStatePending {
		t.Fatal("linked /start alone must leave the reset pending")
	}
	yes, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 4242, begin.ConfirmNonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if yes.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%s want verified", yes.Outcome)
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

	// Telegram always reports an international number; a bare 9-digit one is
	// a foreign number and must not match the national digits of a UZ phone.
	bare, err := svc.ConfirmTelegramPasswordResetContact(ctx, 77, 77, phone)
	if err != nil {
		t.Fatal(err)
	}
	if bare.Outcome != TelegramResetInvalid {
		t.Fatalf("9-digit contact accepted: %s", bare.Outcome)
	}

	ok, err := svc.ConfirmTelegramPasswordResetContact(ctx, 77, 77, "998"+phone)
	if err != nil {
		t.Fatal(err)
	}
	if ok.Outcome != TelegramResetNeedConfirm || ok.ConfirmNonce == "" {
		t.Fatalf("matching contact=%+v want need_confirm", ok)
	}
	if svc.PasswordResetStatus(ctx, raw).State != ResetStatePending {
		t.Fatal("a matching contact alone must not verify the reset")
	}
}

// The Mini App's requestContact also drops the learner's contact into the
// bot chat. With no reset waiting for it, that is not a failed reset.
func TestPasswordReset_ContactWithoutPendingResetIsNone(t *testing.T) {
	svc, _ := resetTestService(t)
	res, err := svc.ConfirmTelegramPasswordResetContact(context.Background(), 78, 78, "998901000012")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetNone {
		t.Fatalf("outcome=%s want none", res.Outcome)
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

	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, 91)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 91, begin.ConfirmNonce, true); err != nil {
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

// contactMatchedReset registers phone, starts a reset, has tgUserID open the
// deep link and share a matching contact. It returns the raw reset token and
// the confirm nonce the «Ha, men» / «Yo'q» buttons carry.
func contactMatchedReset(t *testing.T, svc *Service, phone string, tgUserID int64, contactPhone string) (raw, nonce string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "oldpass12", Name: "R"}); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, phone, "4.4.4.4", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw = parseResetRaw(t, start.BotURL)
	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, tgUserID)
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%s want need_contact", begin.Outcome)
	}
	res, err := svc.ConfirmTelegramPasswordResetContact(ctx, tgUserID, tgUserID, contactPhone)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetNeedConfirm || res.ConfirmNonce == "" {
		t.Fatalf("contact=%+v want need_confirm", res)
	}
	return raw, res.ConfirmNonce
}

func assertResetState(t *testing.T, svc *Service, raw, want string) {
	t.Helper()
	if got := svc.PasswordResetStatus(context.Background(), raw).State; got != want {
		t.Fatalf("status=%s want %s", got, want)
	}
}

func TestPasswordResetConfirm_YesFromSameUserVerifies(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901000030", 301, "+998901000030")
	assertResetState(t, svc, raw, ResetStatePending)

	res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 301, nonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%s want verified", res.Outcome)
	}
	assertResetState(t, svc, raw, ResetStateVerified)

	// Replay of the same «Ha, men» tap changes nothing and says so.
	again, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 301, nonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if again.Outcome != TelegramResetStale {
		t.Fatalf("replay=%s want stale", again.Outcome)
	}
	// Replaying «Yo'q» after verification must not cancel it either.
	no, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 301, nonce, false)
	if err != nil {
		t.Fatal(err)
	}
	if no.Outcome != TelegramResetStale {
		t.Fatalf("late no=%s want stale", no.Outcome)
	}
	assertResetState(t, svc, raw, ResetStateVerified)
}

func TestPasswordResetConfirm_YesFromAnotherUserChangesNothing(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901000031", 311, "+998901000031")

	res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 999, nonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetStale {
		t.Fatalf("foreign yes=%s want stale", res.Outcome)
	}
	if res, err = svc.AnswerTelegramPasswordResetConfirm(ctx, 999, nonce, false); err != nil || res.Outcome != TelegramResetStale {
		t.Fatalf("foreign no=%+v err=%v want stale", res, err)
	}
	assertResetState(t, svc, raw, ResetStatePending)

	// The rightful user can still confirm afterwards.
	if res, err = svc.AnswerTelegramPasswordResetConfirm(ctx, 311, nonce, true); err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("owner yes=%+v err=%v", res, err)
	}
}

func TestPasswordResetConfirm_NoCancelsReset(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901000032", 321, "+998901000032")

	res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 321, nonce, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetCancelled {
		t.Fatalf("no=%s want cancelled", res.Outcome)
	}
	assertResetState(t, svc, raw, ResetStateInvalid)
	if err := svc.CompletePasswordReset(ctx, raw, "brandnew1", "4.4.4.4"); !errors.Is(err, ErrResetInvalid) {
		t.Fatalf("complete after cancel err=%v", err)
	}
	if res, err = svc.AnswerTelegramPasswordResetConfirm(ctx, 321, nonce, true); err != nil || res.Outcome != TelegramResetStale {
		t.Fatalf("yes after cancel=%+v err=%v want stale", res, err)
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: "901000032", Password: "oldpass12"}); err != nil {
		t.Fatalf("old password must still work: %v", err)
	}
}

func TestPasswordResetConfirm_ExpiredChangesNothing(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901000033", 331, "+998901000033")
	if _, err := svc.Pool.Exec(ctx, `UPDATE password_reset_token SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	for _, accept := range []bool{true, false} {
		res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 331, nonce, accept)
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcome != TelegramResetStale {
			t.Fatalf("expired accept=%v outcome=%s want stale", accept, res.Outcome)
		}
	}
	var verified, used bool
	if err := svc.Pool.QueryRow(ctx, `SELECT verified_at IS NOT NULL, used_at IS NOT NULL FROM password_reset_token`).Scan(&verified, &used); err != nil {
		t.Fatal(err)
	}
	if verified || used {
		t.Fatalf("expired reset changed: verified=%v used=%v", verified, used)
	}
	if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 331, "not-a-nonce", true); err != nil || res.Outcome != TelegramResetStale {
		t.Fatalf("unknown nonce=%+v err=%v", res, err)
	}
	_ = raw
}

// The Mini App's «Raqamni Telegram'dan olish» posts the learner's own contact
// into the bot chat. While a reset someone else started is pending for this
// Telegram user, that contact must only produce the question, never verify.
func TestPasswordResetConfirm_MiniAppShareAloneStaysUnverified(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, _ := contactMatchedReset(t, svc, "901000034", 341, "+998901000034")
	// A second share (the Mini App button pressed again) re-asks, still no verify.
	res, err := svc.ConfirmTelegramPasswordResetContact(ctx, 341, 341, "+998901000034")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("second share=%s", res.Outcome)
	}
	assertResetState(t, svc, raw, ResetStatePending)
	if err := svc.CompletePasswordReset(ctx, raw, "brandnew1", "4.4.4.4"); !errors.Is(err, ErrResetNotVerified) {
		t.Fatalf("complete without confirmation err=%v", err)
	}
}

// Telegram reports contact phones in several shapes; each must complete the
// whole reset, not just pass normalisation.
func TestPasswordReset_ContactPhoneFormatsCompleteEndToEnd(t *testing.T) {
	cases := []struct {
		name, phone, contact string
		tg                   int64
	}{
		{"plus prefix", "901000035", "+998901000035", 351},
		{"formatted card", "901234567", "+998 (90) 123-45-67", 352},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := resetTestService(t)
			ctx := context.Background()
			raw, nonce := contactMatchedReset(t, svc, tc.phone, tc.tg, tc.contact)
			if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, tc.tg, nonce, true); err != nil || res.Outcome != TelegramResetVerified {
				t.Fatalf("yes=%+v err=%v", res, err)
			}
			if err := svc.CompletePasswordReset(ctx, raw, "brandnew1", "4.4.4.4"); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Login(ctx, LoginInput{Phone: tc.phone, Password: "brandnew1"}); err != nil {
				t.Fatalf("new password login: %v", err)
			}
		})
	}
}

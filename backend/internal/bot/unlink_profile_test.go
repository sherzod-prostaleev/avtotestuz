package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

func phoneVerified(t *testing.T, q *sqlc.Queries, profileID uuid.UUID) bool {
	t.Helper()
	acc, err := q.GetTelegramAccountByProfileID(context.Background(), profileID)
	if err != nil {
		t.Fatal(err)
	}
	return acc.PhoneVerifiedAt.Valid
}

// The legacy /start <token> link carries no phone proof, so it never counts
// as one for the Mini App (audit-2 C1); re-pointing a proven row to another
// Telegram user drops the proof, which was about the previous account.
func TestRedeemLinkTokenNeverMarksPhoneVerified(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901140001")
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: profileID, TgUserID: 7101, PhoneVerified: true,
	}); err != nil {
		t.Fatal(err)
	}

	tok, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RedeemLinkToken(ctx, tok.Token, 7102, "other"); err != nil {
		t.Fatal(err)
	}
	acc, err := q.GetTelegramAccountByProfileID(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.TgUserID != 7102 || acc.PhoneVerifiedAt.Valid {
		t.Fatalf("re-pointed link = %+v, want tg 7102 unverified", acc)
	}

	fresh := createProfile(t, q, "+998901140002")
	tok, err = svc.GenerateLinkToken(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RedeemLinkToken(ctx, tok.Token, 7103, "new"); err != nil {
		t.Fatal(err)
	}
	if phoneVerified(t, q, fresh) {
		t.Fatal("legacy redeem marked a new link phone-verified")
	}
}

func doDelete(t *testing.T, baseURL, token, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

// DELETE /me/telegram is the website's unlink: the learner removes their own
// profile's link (only theirs), idempotently.
func TestDeleteMeTelegramUnlinksOwnProfileOnly(t *testing.T) {
	ts, tok, link := setupLinkHandlerServer(t)
	ctx := context.Background()
	claims, err := auth.ParseAccess([]byte(handlerTestSecret), tok)
	if err != nil {
		t.Fatal(err)
	}
	other := createProfile(t, link.Q, "+998901140003")
	for _, p := range []struct {
		id uuid.UUID
		tg int64
	}{{claims.ProfileID, 7201}, {other, 7202}} {
		if err := link.Q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: p.id, TgUserID: p.tg, PhoneVerified: true}); err != nil {
			t.Fatal(err)
		}
	}

	if status, _ := doDelete(t, ts.URL, "", "/me/telegram"); status != http.StatusUnauthorized {
		t.Fatalf("anonymous delete status = %d, want 401", status)
	}

	for i, want := range []bool{true, false} {
		status, body := doDelete(t, ts.URL, tok, "/me/telegram")
		if status != http.StatusOK {
			t.Fatalf("call %d: status = %d body=%s", i, status, body)
		}
		var env struct {
			Data struct {
				Unlinked *bool `json:"unlinked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil || env.Data.Unlinked == nil {
			t.Fatalf("call %d: body=%s err=%v", i, body, err)
		}
		if *env.Data.Unlinked != want {
			t.Fatalf("call %d: unlinked=%v want %v", i, *env.Data.Unlinked, want)
		}
	}
	if _, err := link.Q.GetTelegramAccountByProfileID(ctx, claims.ProfileID); err == nil {
		t.Fatal("own link still present after DELETE /me/telegram")
	}
	if acc, err := link.Q.GetTelegramAccountByProfileID(ctx, other); err != nil || acc.TgUserID != 7202 {
		t.Fatalf("other profile's link touched: %+v %v", acc, err)
	}
}

// GET /me/telegram tells the Mini App whether the link is phone-verified: a
// linked-but-unverified learner must still be asked to share the phone once,
// or auto-login answers need_phone forever.
func TestGetTelegramStatusReportsPhoneVerified(t *testing.T) {
	ts, tok, link := setupLinkHandlerServer(t)
	ctx := context.Background()
	claims, err := auth.ParseAccess([]byte(handlerTestSecret), tok)
	if err != nil {
		t.Fatal(err)
	}
	get := func() TelegramStatus {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/me/telegram", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var env struct {
			Data TelegramStatus `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		return env.Data
	}
	for _, verified := range []bool{false, true} {
		if err := link.Q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: claims.ProfileID, TgUserID: 7301, PhoneVerified: verified}); err != nil {
			t.Fatal(err)
		}
		if st := get(); !st.Linked || st.PhoneVerified != verified {
			t.Fatalf("status=%+v want phone_verified=%v", st, verified)
		}
	}
}

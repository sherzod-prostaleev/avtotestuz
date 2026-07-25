package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

const handlerTestSecret = "test-secret"

func setupLinkHandlerServer(t *testing.T) (*httptest.Server, string, *LinkService) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901130001"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerTestSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	link := NewLinkService(pool, q)
	r := chi.NewRouter()
	h := &Handler{Link: link, BotUsername: "AvtoTestBot"}
	h.AuthedRoutes(r.With(auth.Required([]byte(handlerTestSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, link
}

func doPost(t *testing.T, ts *httptest.Server, token, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf
}

func TestCreateLinkTokenRequiresAuth(t *testing.T) {
	ts, _, _ := setupLinkHandlerServer(t)
	status, _ := doPost(t, ts, "", "/me/telegram/link-token")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
}

func TestCreateLinkTokenReturnsRedeemableToken(t *testing.T) {
	ts, tok, link := setupLinkHandlerServer(t)
	status, body := doPost(t, ts, tok, "/me/telegram/link-token")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", status, body)
	}

	var resp struct {
		Data linkTokenResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v; body: %s", err, body)
	}
	if resp.Data.Token == "" {
		t.Fatal("want non-empty token")
	}
	wantPrefix := "https://t.me/AvtoTestBot?start="
	if !strings.HasPrefix(resp.Data.DeepLink, wantPrefix) || !strings.HasSuffix(resp.Data.DeepLink, resp.Data.Token) {
		t.Errorf("deep_link = %q, want prefix %q and suffix = token", resp.Data.DeepLink, wantPrefix)
	}
	if resp.Data.ExpiresAt == "" {
		t.Error("want non-empty expires_at")
	}

	// The returned token must actually redeem — proves it's the raw value,
	// not e.g. its hash or an empty placeholder.
	claims, err := auth.ParseAccess([]byte(handlerTestSecret), tok)
	if err != nil {
		t.Fatalf("parse test token: %v", err)
	}
	res, err := link.RedeemLinkToken(context.Background(), resp.Data.Token, 9999, "handlertest")
	if err != nil {
		t.Fatalf("RedeemLinkToken: %v", err)
	}
	if res.ProfileID != claims.ProfileID {
		t.Fatalf("ProfileID = %v, want %v", res.ProfileID, claims.ProfileID)
	}
}

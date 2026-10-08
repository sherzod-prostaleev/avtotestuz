package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

type fakeAvatars struct {
	urls  map[uuid.UUID]string
	kinds []string
}

func (f *fakeAvatars) AvatarURL(_ context.Context, id uuid.UUID, kind string) string {
	f.kinds = append(f.kinds, kind)
	return f.urls[id]
}

func avatarServer(t *testing.T, avatars account.AvatarURLs) (*httptest.Server, sqlc.Profile, string) {
	t.Helper()
	_, profile, pool := setup(t)
	q := sqlc.New(pool)
	r := chi.NewRouter()
	h := &account.Handler{Q: q, Billing: billing.Service{Q: q}, Avatars: avatars}
	h.Routes(r.With(auth.Required([]byte(testSecret))))
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return ts, profile, tok
}

func profileField(t *testing.T, raw json.RawMessage, nested bool) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if nested {
		m, _ = m["profile"].(map[string]any)
	}
	return m
}

func TestMeCarriesAvatarURL(t *testing.T) {
	avatars := &fakeAvatars{urls: map[uuid.UUID]string{}}
	ts, profile, tok := avatarServer(t, avatars)
	want := "https://drivergo.test/media/images/avatars/abc.jpg"
	avatars.urls[profile.ID] = want

	status, env := doReq(t, ts, http.MethodGet, "/me", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if got := profileField(t, env.Data, true)["avatar_url"]; got != want {
		t.Fatalf("GET /me avatar_url = %v, want %q", got, want)
	}
	if len(avatars.kinds) != 1 || avatars.kinds[0] != "user" {
		t.Fatalf("kind passed = %v, want the profile's kind", avatars.kinds)
	}

	// PATCH answers with the profile the page then keeps; without the URL
	// the photo would vanish after saving a name.
	status, env = doReq(t, ts, http.MethodPatch, "/me", tok, []byte(`{"name":"Ali"}`))
	if status != http.StatusOK {
		t.Fatalf("PATCH status=%d", status)
	}
	if got := profileField(t, env.Data, false)["avatar_url"]; got != want {
		t.Fatalf("PATCH /me avatar_url = %v, want %q", got, want)
	}
}

func TestMeOmitsAvatarURLWhenNone(t *testing.T) {
	for name, avatars := range map[string]account.AvatarURLs{
		"no photo":         &fakeAvatars{urls: map[uuid.UUID]string{}},
		"avatars disabled": nil,
	} {
		t.Run(name, func(t *testing.T) {
			ts, _, tok := avatarServer(t, avatars)
			status, env := doReq(t, ts, http.MethodGet, "/me", tok, nil)
			if status != http.StatusOK {
				t.Fatalf("status=%d", status)
			}
			if _, ok := profileField(t, env.Data, true)["avatar_url"]; ok {
				t.Fatal("avatar_url present without a photo")
			}
		})
	}
}

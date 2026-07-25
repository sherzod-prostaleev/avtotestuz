package site

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/testdb"
)

func TestSupportBannerGetPut(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}

	empty, err := store.GetSupportBanner(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Enabled || empty.Message != "" {
		t.Fatalf("expected empty banner, got %+v", empty)
	}

	saved, err := store.PutSupportBanner(t.Context(), SupportBanner{
		Enabled: true,
		Message: "  Hello  ",
		Href:    "/uz-Latn/premium",
	}, "admin-test")
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled || saved.Message != "Hello" || saved.Href != "/uz-Latn/premium" {
		t.Fatalf("saved=%+v", saved)
	}

	_, err = store.PutSupportBanner(t.Context(), SupportBanner{
		Enabled: true,
		Message: "",
	}, "admin-test")
	if err == nil {
		t.Fatal("expected message required error")
	}

	_, err = store.PutSupportBanner(t.Context(), SupportBanner{
		Enabled: true,
		Message: "ok",
		Href:    "https://evil.example",
	}, "admin-test")
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetSupportBanner(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Href != "" {
		t.Fatalf("external href should be cleared, got %q", got.Href)
	}

	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/site/banner", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("public status=%d body=%s", w.Code, w.Body.String())
	}
}

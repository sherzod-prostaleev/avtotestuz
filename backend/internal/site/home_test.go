package site

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/testdb"
)

func TestHomeHeroGetPut(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}

	empty, err := store.GetHomeHero(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Headline != "" || empty.CTALabel != "" {
		t.Fatalf("expected empty hero, got %+v", empty)
	}

	saved, err := store.PutHomeHero(t.Context(), HomeHero{
		Headline: "  Prava oson  ",
		Subtitle: "VIP bilan tezroq",
		CTALabel: "Boshlash",
		CTAHref:  "/uz-Latn/login",
	}, "ops-test")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Headline != "Prava oson" {
		t.Fatalf("headline trim = %q", saved.Headline)
	}

	got, err := store.GetHomeHero(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.CTAHref != "/uz-Latn/login" || got.Subtitle != "VIP bilan tezroq" {
		t.Fatalf("get after put = %+v", got)
	}

	_, err = store.PutHomeHero(t.Context(), HomeHero{CTAHref: "https://evil.example"}, "ops-test")
	if err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("want relative href error, got %v", err)
	}

	_, err = store.PutHomeHero(t.Context(), HomeHero{Headline: strings.Repeat("x", maxHeroFieldLen+1)}, "ops-test")
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("want too long, got %v", err)
	}
}

func TestPublicHomeHero(t *testing.T) {
	pool := testdb.New(t)
	_, err := Store{Pool: pool}.PutHomeHero(t.Context(), HomeHero{
		Headline: "CMS headline",
		CTALabel: "Go",
		CTAHref:  "/uz-Latn/login",
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	h.PublicRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/site/home", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "CMS headline") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

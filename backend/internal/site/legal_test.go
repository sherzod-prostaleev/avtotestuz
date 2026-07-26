package site

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/testdb"
)

func TestLegalGetPut(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}

	empty, err := store.GetLegalBundle(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Locales["uz-Latn"].Oferta != "" || empty.Locales["ru"].Privacy != "" {
		t.Fatalf("expected empty legal, got %+v", empty)
	}

	saved, err := store.PutLegalBundle(t.Context(), LegalBundle{
		Locales: map[string]LegalDoc{
			"uz-Latn": {Oferta: "  Oferta matn  ", Privacy: "Maxfiylik", Refund: ""},
			"ru":      {Oferta: "Оферта", Privacy: "Конфиденциальность"},
		},
	}, "ops-test")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Locales["uz-Latn"].Oferta != "Oferta matn" {
		t.Fatalf("trim = %q", saved.Locales["uz-Latn"].Oferta)
	}
	if saved.Locales["uz-Cyrl"].Oferta != "" {
		t.Fatalf("missing locale should stay empty, got %+v", saved.Locales["uz-Cyrl"])
	}

	doc, err := store.GetLegalDoc(t.Context(), "uz-Latn")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Oferta != "Oferta matn" || doc.Privacy != "Maxfiylik" {
		t.Fatalf("GetLegalDoc = %+v", doc)
	}

	_, err = store.PutLegalBundle(t.Context(), LegalBundle{
		Locales: map[string]LegalDoc{
			"uz-Latn": {Oferta: strings.Repeat("x", maxLegalFieldLen+1)},
		},
	}, "ops-test")
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("want field-too-long error, got %v", err)
	}
}

func TestPublicLegalHandler(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}
	if _, err := store.PutLegalBundle(t.Context(), LegalBundle{
		Locales: map[string]LegalDoc{
			"ru": {Oferta: "RU oferta", Privacy: "RU privacy"},
		},
	}, "ops-test"); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	h.PublicRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/site/legal?locale=ru", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Locale  string `json:"locale"`
			Oferta  string `json:"oferta"`
			Privacy string `json:"privacy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Locale != "ru" || env.Data.Oferta != "RU oferta" || env.Data.Privacy != "RU privacy" {
		t.Fatalf("data=%+v", env.Data)
	}

	// Unknown locale falls back to uz-Latn (empty when unset).
	req = httptest.NewRequest(http.MethodGet, "/site/legal?locale=kaa", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("fallback status=%d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Locale != "uz-Latn" {
		t.Fatalf("locale=%q", env.Data.Locale)
	}
}

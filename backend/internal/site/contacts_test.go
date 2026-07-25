package site

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/testdb"
)

func TestContactsGetPut(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}

	empty, err := store.GetContacts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Phone != "" || empty.Email != "" {
		t.Fatalf("expected empty contacts, got %+v", empty)
	}

	saved, err := store.PutContacts(t.Context(), Contacts{
		Phone:       " +998 90 123 45 67 ",
		PhoneTel:    "+998901234567",
		Email:       "hello@example.uz",
		Address:     "Toshkent",
		Hours:       "Du–Sha 09:00–20:00",
		Telegram:    "@DriverGo",
		TelegramURL: "https://t.me/DriverGo",
		Instagram:   "@drivergo.uz",
		InstagramURL: "https://instagram.com/drivergo.uz",
	}, "ops-test")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Phone != "+998 90 123 45 67" {
		t.Fatalf("phone trim = %q", saved.Phone)
	}

	got, err := store.GetContacts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "hello@example.uz" || got.TelegramURL != "https://t.me/DriverGo" {
		t.Fatalf("get after put = %+v", got)
	}

	_, err = store.PutContacts(t.Context(), Contacts{
		Phone: strings.Repeat("x", maxFieldLen+1),
	}, "ops-test")
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("want field-too-long error, got %v", err)
	}
}

func TestPublicAndDecodeHandlers(t *testing.T) {
	pool := testdb.New(t)
	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	h.PublicRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/site/contacts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("public get status = %d body=%s", w.Code, w.Body.String())
	}

	body := bytes.NewReader([]byte(`{"phone":"99","email":"a@b.c"}`))
	req = httptest.NewRequest(http.MethodPut, "/ops/site-contacts", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	DecodeAndPut(w, req, Store{Pool: pool}, "ops")
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data Contacts `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Phone != "99" || env.Data.Email != "a@b.c" {
		t.Fatalf("put response = %+v", env.Data)
	}
}

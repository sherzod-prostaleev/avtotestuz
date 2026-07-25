package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/site"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminCMSContacts(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("get empty then put", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/cms/contacts", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get status=%d body=%s", w.Code, w.Body.String())
		}

		body := `{"phone":"+998 71 200 00 00","phoneTel":"+998712000000","email":"hello@drivergo.uz","address":"Toshkent","hours":"09:00–18:00","telegram":"@drivergo","telegramUrl":"https://t.me/drivergo","instagram":"drivergo","instagramUrl":"https://instagram.com/drivergo"}`
		req = httptest.NewRequest(http.MethodPut, "/admin/v1/cms/contacts", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data site.Contacts `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Phone != "+998 71 200 00 00" || env.Data.Email != "hello@drivergo.uz" {
			t.Fatalf("contacts=%+v", env.Data)
		}

		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM admin_audit_log WHERE action='cms.contacts.put' AND entity_id='contacts'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("audit count=%d", n)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/cms/contacts", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("reload status=%d", w.Code)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Telegram != "@drivergo" {
			t.Fatalf("reload=%+v", env.Data)
		}
	})

	t.Run("editor read ok write forbidden; finance no cms", func(t *testing.T) {
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		editorID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor@example.uz", "password123")

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/cms/contacts", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("editor get status=%d", w.Code)
		}

		req = httptest.NewRequest(http.MethodPut, "/admin/v1/cms/contacts",
			bytes.NewBufferString(`{"phone":"x"}`))
		req.Header.Set("Authorization", "Bearer "+edAccess)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor put status=%d want 403", w.Code)
		}

		financeID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Fin',$3,'active')`, financeID, "finance@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='finance'`, financeID); err != nil {
			t.Fatal(err)
		}
		finAccess := loginAccess(t, r, "finance@example.uz", "password123")
		req = httptest.NewRequest(http.MethodGet, "/admin/v1/cms/contacts", nil)
		req.Header.Set("Authorization", "Bearer "+finAccess)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("finance get status=%d want 403", w.Code)
		}
	})
}

func TestAdminCMSHome(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/cms/home", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d", w.Code)
	}

	body := `{"headline":"Prava oson","subtitle":"Tezroq","ctaLabel":"Boshlash","ctaHref":"/uz-Latn/login"}`
	req = httptest.NewRequest(http.MethodPut, "/admin/v1/cms/home", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data site.HomeHero `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Headline != "Prava oson" {
		t.Fatalf("hero=%+v", env.Data)
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log WHERE action='cms.home.put' AND entity_id='home_hero'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("audit=%d", n)
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/v1/cms/home",
		bytes.NewBufferString(`{"ctaHref":"https://evil.example"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("evil href status=%d want 400", w.Code)
	}
}

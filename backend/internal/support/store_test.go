package support_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/support"
	"avtotest.uz/backend/internal/testdb"
)

func TestSupportTicketCreateAndList(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := support.Store{Pool: pool}

	t.Run("public create requires contact", func(t *testing.T) {
		_, err := store.Create(context.Background(), support.CreateInput{
			Subject: "Help",
			Body:    "Cannot pay",
			Source:  "public",
		})
		if err == nil {
			t.Fatal("expected contact required")
		}
	})

	t.Run("create list get update", func(t *testing.T) {
		created, err := store.Create(context.Background(), support.CreateInput{
			ContactEmail: "learner@example.uz",
			Subject:      "VIP savol",
			Body:         "To‘lov o‘tmadi",
			Locale:       "uz-Latn",
			Source:       "public",
		})
		if err != nil {
			t.Fatal(err)
		}
		if created.Status != "open" || created.Source != "public" {
			t.Fatalf("ticket=%+v", created)
		}

		list, err := store.List(context.Background(), "open", "VIP", 1, 20)
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != 1 || len(list.Items) != 1 {
			t.Fatalf("list=%+v", list)
		}

		got, err := store.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Subject != "VIP savol" {
			t.Fatalf("got=%+v", got)
		}

		note := "checked sandbox"
		updated, err := store.UpdateStatus(context.Background(), created.ID, "in_progress", &note)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != "in_progress" || updated.AdminNote != note {
			t.Fatalf("updated=%+v", updated)
		}
	})

	t.Run("http public + authed", func(t *testing.T) {
		h := &support.Handler{Pool: pool}
		secret := []byte("test-learner-secret-at-least-32-bytes!")
		r := chi.NewRouter()
		r.Route("/api/v1", func(api chi.Router) {
			h.PublicRoutes(api)
			api.Group(func(ar chi.Router) {
				ar.Use(auth.Required(secret))
				h.AuthedRoutes(ar)
			})
		})

		body := `{"contact_phone":"+998901112233","subject":"Public","body":"Hello","locale":"ru"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/support/tickets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("public status=%d body=%s", w.Code, w.Body.String())
		}

		profileID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
			profileID, "+998909998877", "T"); err != nil {
			t.Fatal(err)
		}
		tok, err := auth.IssueAccess(secret, profileID, "user", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		req = httptest.NewRequest(http.MethodPost, "/api/v1/me/support/tickets",
			bytes.NewBufferString(`{"subject":"From profile","body":"Need help"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("authed status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data support.Ticket `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Source != "profile" || env.Data.ProfileID == nil {
			t.Fatalf("authed ticket=%+v", env.Data)
		}
	})
}

func TestPublicSupportAbuseControls(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	redisClient := redisx.NewTest(t)
	h := &support.Handler{
		Pool:      pool,
		Lim:       auth.Limiter{R: redisClient},
		ClientIPs: auth.NewClientIPResolver(nil),
	}
	router := chi.NewRouter()
	h.PublicRoutes(router)

	request := func(payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/support/tickets", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.50:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	honeypot := request(`{"contact_email":"bot@example.test","subject":"Spam","body":"Spam","website":"https://spam.test"}`)
	if honeypot.Code != http.StatusCreated {
		t.Fatalf("honeypot status=%d body=%s", honeypot.Code, honeypot.Body.String())
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM support_ticket`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("honeypot submission persisted %d tickets", count)
	}

	payload := `{"contact_email":"real@example.test","subject":"Help","body":"Please help"}`
	// Honeypot already consumed one IP-window attempt, so four real submissions
	// remain before the fifth real attempt is refused.
	for i := 0; i < 4; i++ {
		if w := request(payload); w.Code != http.StatusCreated {
			t.Fatalf("allowed request %d status=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if w := request(payload); w.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status=%d body=%s", w.Code, w.Body.String())
	}

	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	if w := request(payload); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("redis failure must fail closed: status=%d body=%s", w.Code, w.Body.String())
	}
}

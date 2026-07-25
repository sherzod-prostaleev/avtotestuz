package b2b

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestTeacherPortal(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	secret := []byte("test-learner-secret-at-least-32-bytes!")

	ctx := context.Background()
	owner, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901180001", Name: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901180002", Name: "Student"})
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901180003", Name: "Out"})
	if err != nil {
		t.Fatal(err)
	}

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO b2b_org (name) VALUES ('Test School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_member (org_id, profile_id, role) VALUES
		  ($1, $2, 'owner'), ($1, $3, 'student')`, orgID, owner.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 25, now() - interval '1 day', now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		h.AuthedRoutes(api.With(auth.Required(secret)))
	})

	ownerTok, err := auth.IssueAccess(secret, owner.ID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	outsiderTok, err := auth.IssueAccess(secret, outsider.ID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	studentTok, err := auth.IssueAccess(secret, student.ID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("owner lists and opens org", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs", nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
		}
		var listEnv struct {
			Data []OrgSummary `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listEnv); err != nil {
			t.Fatal(err)
		}
		if len(listEnv.Data) != 1 || listEnv.Data[0].MyRole != "owner" {
			t.Fatalf("list=%+v", listEnv.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs/"+orgID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("detail status=%d body=%s", w.Code, w.Body.String())
		}
		var detEnv struct {
			Data OrgDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &detEnv); err != nil {
			t.Fatal(err)
		}
		if len(detEnv.Data.Members) != 2 || detEnv.Data.Org.ActiveSeats != 25 {
			t.Fatalf("detail=%+v", detEnv.Data)
		}
	})

	t.Run("outsider not found; student not listed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs/"+orgID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("outsider status=%d want 404", w.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs", nil)
		req.Header.Set("Authorization", "Bearer "+studentTok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("student list status=%d", w.Code)
		}
		var listEnv struct {
			Data []OrgSummary `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listEnv); err != nil {
			t.Fatal(err)
		}
		if len(listEnv.Data) != 0 {
			t.Fatalf("student should see no teacher orgs, got %+v", listEnv.Data)
		}
	})
}

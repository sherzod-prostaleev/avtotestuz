package b2b

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	pendingPhone := "+998901180099"
	// invitee profile created later for accept flow
	invitee, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901180004", Name: "Invitee"})
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
	inviteeTok, err := auth.IssueAccess(secret, invitee.ID, "user", time.Hour)
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

	t.Run("invite enroll existing + pending phone", func(t *testing.T) {
		// enroll existing invitee immediately
		req := httptest.NewRequest(http.MethodPost, "/api/v1/me/teacher/orgs/"+orgID.String()+"/invites",
			bytes.NewBufferString(`{"phone":"+998901180004","role":"student"}`))
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("enroll status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data InviteResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Status != "enrolled" || env.Data.Member == nil {
			t.Fatalf("want enrolled, got %+v", env.Data)
		}

		// pending for unknown phone
		req = httptest.NewRequest(http.MethodPost, "/api/v1/me/teacher/orgs/"+orgID.String()+"/invites",
			bytes.NewBufferString(`{"phone":"`+pendingPhone+`","role":"student"}`))
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("pending status=%d body=%s", w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Status != "pending" || env.Data.Invite == nil || env.Data.Invite.Token == "" {
			t.Fatalf("want pending invite, got %+v", env.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs/"+orgID.String()+"/invites", nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list invites status=%d", w.Code)
		}
	})

	t.Run("accept invite by matching phone", func(t *testing.T) {
		// create fresh org invite for invitee phone after removing them
		if _, err := pool.Exec(ctx, `DELETE FROM b2b_org_member WHERE org_id=$1 AND profile_id=$2`, orgID, invitee.ID); err != nil {
			t.Fatal(err)
		}
		token, err := newInviteToken()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO b2b_invite (token, org_id, phone, role, expires_at, created_by)
			VALUES ($1, $2, $3, 'student', now() + interval '7 days', $4)`,
			token, orgID, invitee.Phone, owner.ID); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/invites", nil)
		req.Header.Set("Authorization", "Bearer "+inviteeTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("my invites status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/me/invites/accept",
			bytes.NewBufferString(`{"token":"`+token+`"}`))
		req.Header.Set("Authorization", "Bearer "+inviteeTok)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("accept status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("change role and remove member", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/me/teacher/orgs/"+orgID.String()+"/members/"+student.ID.String(),
			bytes.NewBufferString(`{"role":"teacher"}`))
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("change role status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete,
			"/api/v1/me/teacher/orgs/"+orgID.String()+"/members/"+invitee.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("remove status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("stats and csv", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs/"+orgID.String()+"/stats", nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data OrgStats `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.MembersTotal < 2 || env.Data.ActiveSeats != 25 {
			t.Fatalf("stats=%+v", env.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/me/teacher/orgs/"+orgID.String()+"/export.csv", nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("csv status=%d body=%s", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
			t.Fatalf("content-type=%s", ct)
		}
		if !strings.Contains(w.Body.String(), "profile_id,phone_masked") {
			t.Fatalf("csv body=%s", w.Body.String())
		}
	})

	t.Run("cannot remove last owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/me/teacher/orgs/"+orgID.String()+"/members/"+owner.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+ownerTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("last owner status=%d want 409 body=%s", w.Code, w.Body.String())
		}
	})
}

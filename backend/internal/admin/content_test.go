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
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminContentQuestionsAndExplanations(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	var catID, qID, vID, explID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO category (code, sort_order) VALUES ('signs', 1) RETURNING id`).Scan(&catID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO category_translation VALUES ($1,'uz-Latn','Belgilar','verified')`, catID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO question (source_ext_id, category_id, content_hash, source)
		 VALUES ('ext-q-42',$1,'h1','avtoimtihon') RETURNING id`, catID).Scan(&qID); err != nil {
		t.Fatal(err)
	}
	var ansID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO answer (question_id, position, is_correct) VALUES ($1,1,true) RETURNING id`,
		qID).Scan(&ansID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE question SET correct_answer_id=$2 WHERE id=$1`, qID, ansID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO answer_translation VALUES ($1,'uz-Latn','To''g''ri','verified')`, ansID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO question_translation VALUES ($1,'uz-Latn','Qizil chiroq','verified','')`, qID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO variant (number) VALUES (7) RETURNING id`).Scan(&vID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO variant_question VALUES ($1,$2,3)`, vID, qID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO explanation (question_id) VALUES ($1) RETURNING id`, qID).Scan(&explID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO explanation_translation (explanation_id, locale, blocks, status, source)
		 VALUES ($1,'uz-Latn','[{"type":"intro","text":"Draft text"}]'::jsonb,'draft','ai-stub')`,
		explID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("list search and filters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/admin/v1/content/questions?q=Qizil&category=signs&validation=valid&explanation=draft&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListQuestionsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		row := env.Data.Items[0]
		if row.SourceExtID != "ext-q-42" || row.ExplanationStatus != "draft" {
			t.Fatalf("row=%+v", row)
		}
		if len(row.VariantNumbers) != 1 || row.VariantNumbers[0] != 7 {
			t.Fatalf("variants=%v", row.VariantNumbers)
		}
	})

	t.Run("detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/content/questions/"+qID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data QuestionDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data.Translations) != 1 || len(env.Data.Answers) != 1 || len(env.Data.Variants) != 1 {
			t.Fatalf("detail=%+v", env.Data)
		}
		if env.Data.Explanation == nil || len(env.Data.Explanation.Locales) != 1 {
			t.Fatalf("explanation=%+v", env.Data.Explanation)
		}
		if env.Data.Explanation.Locales[0].Status != "draft" {
			t.Fatalf("status=%s", env.Data.Explanation.Locales[0].Status)
		}
	})

	t.Run("explanations queue draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/content/explanations?status=draft", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListExplanationsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || env.Data.Items[0].QuestionID != qID {
			t.Fatalf("got %+v", env.Data)
		}
	})

	t.Run("verify explanation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost,
			"/admin/v1/content/questions/"+qID.String()+"/explanation/verify", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM explanation_translation WHERE explanation_id=$1 AND locale='uz-Latn'`,
			explID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "verified" {
			t.Fatalf("status=%q", status)
		}
		var auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log WHERE action='content.verify' AND entity_id=$1`,
			qID.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount < 1 {
			t.Fatal("expected audit for verify")
		}
	})

	t.Run("patch validation_status", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"validation_status":"quarantined"}`))
		req := httptest.NewRequest(http.MethodPatch, "/admin/v1/content/questions/"+qID.String(), body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var vs string
		if err := pool.QueryRow(context.Background(),
			`SELECT validation_status FROM question WHERE id=$1`, qID).Scan(&vs); err != nil {
			t.Fatal(err)
		}
		if vs != "quarantined" {
			t.Fatalf("validation_status=%q", vs)
		}
		var auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log WHERE action='content.questions.patch' AND entity_id=$1`,
			qID.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount < 1 {
			t.Fatal("expected audit for patch")
		}
	})

	t.Run("list tickets", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/content/tickets?q=7&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListTicketsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		if env.Data.Items[0].Number != 7 || env.Data.Items[0].QuestionCount != 1 {
			t.Fatalf("row=%+v", env.Data.Items[0])
		}
	})

	t.Run("list signs", func(t *testing.T) {
		var sgID, signID uuid.UUID
		if err := pool.QueryRow(context.Background(),
			`INSERT INTO sign_group (code, sort_order) VALUES ('warning', 1) RETURNING id`).Scan(&sgID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO sign_group_translation VALUES ($1,'uz-Latn','Ogohlantirish','verified')`, sgID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(),
			`INSERT INTO sign (group_id, code, sort_order) VALUES ($1,'1.1',1) RETURNING id`, sgID).Scan(&signID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO sign_translation VALUES ($1,'uz-Latn','Xavfli burilish','','verified')`, signID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO question_sign VALUES ($1,$2)`, qID, signID); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet,
			"/admin/v1/content/signs?q=Xavf&group=warning&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListSignsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		row := env.Data.Items[0]
		if row.Code != "1.1" || row.GroupCode != "warning" || row.QuestionCount != 1 {
			t.Fatalf("row=%+v", row)
		}
	})

	t.Run("support denied content.read", func(t *testing.T) {
		supportID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Sup',$3,'active')`, supportID, "support@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='support'`, supportID); err != nil {
			t.Fatal(err)
		}
		supAccess := loginAccess(t, r, "support@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/content/questions", nil)
		req.Header.Set("Authorization", "Bearer "+supAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", w.Code)
		}
	})
}

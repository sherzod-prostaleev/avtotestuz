package explanation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &explanation.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, qids[0].String()
}

func TestFeedbackWithoutDraftReturns404(t *testing.T) {
	ts, tok, questionID := setupHandlerServer(t)
	body, _ := json.Marshal(map[string]any{"question_id": questionID, "helpful": true})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/explanations/feedback", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want 404", resp.StatusCode)
	}
}

func TestFeedbackRequiresAuth(t *testing.T) {
	ts, _, questionID := setupHandlerServer(t)
	body, _ := json.Marshal(map[string]any{"question_id": questionID, "helpful": true})
	resp, err := ts.Client().Post(ts.URL+"/explanations/feedback", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

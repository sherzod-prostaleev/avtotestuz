package progress_test

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
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
)

func wrongAndCorrectAnswerIDs(t *testing.T, q *sqlc.Queries, questionID uuid.UUID) (wrong, correct uuid.UUID) {
	t.Helper()
	ans, err := q.ListAnswersByQuestionIDs(context.Background(),
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{questionID}, Locale: "uz-Latn"})
	if err != nil || len(ans) < 2 {
		t.Fatalf("answers: %v len=%d", err, len(ans))
	}
	for _, a := range ans {
		full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: a.ID, QuestionID: questionID})
		if err != nil {
			continue
		}
		if full.IsCorrect {
			correct = a.ID
		} else if wrong == uuid.Nil {
			wrong = a.ID
		}
	}
	if wrong == uuid.Nil || correct == uuid.Nil {
		t.Fatal("need both wrong and correct answers")
	}
	return wrong, correct
}

func TestMigrateDemoProgress_IncorrectAgain_CorrectSkipped(t *testing.T) {
	q, svc, profileID, questionID := seed(t)
	svc.Learning = learning.NewService(q)
	wrong, _ := wrongAndCorrectAnswerIDs(t, q, questionID)

	result, err := svc.MigrateDemoProgress(context.Background(), profileID, []progress.DemoMigrateAnswer{
		{QuestionID: questionID, AnswerID: wrong, Correct: false},
		{QuestionID: questionID, AnswerID: wrong, Correct: false}, // duplicate Q → skip
	})
	if err != nil {
		t.Fatalf("MigrateDemoProgress: %v", err)
	}
	if result.Migrated != 1 || result.Skipped != 1 {
		t.Fatalf("result=%+v want migrated=1 skipped=1", result)
	}

	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) < 2 {
		t.Fatalf("need 2 questions: %v", err)
	}
	q2 := qids[1]
	_, correct2 := wrongAndCorrectAnswerIDs(t, q, q2)
	result2, err := svc.MigrateDemoProgress(context.Background(), profileID, []progress.DemoMigrateAnswer{
		{QuestionID: q2, AnswerID: correct2, Correct: true},
	})
	if err != nil {
		t.Fatalf("correct-only: %v", err)
	}
	if result2.Migrated != 0 || result2.Skipped != 1 {
		t.Fatalf("correct-only result=%+v", result2)
	}

	result3, err := svc.MigrateDemoProgress(context.Background(), profileID, []progress.DemoMigrateAnswer{
		{QuestionID: questionID, AnswerID: wrong, Correct: false},
	})
	if err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	if result3.Migrated != 1 {
		t.Fatalf("idempotent migrated=%d", result3.Migrated)
	}
}

func TestMigrateDemoProgress_InvalidAnswerSkipped(t *testing.T) {
	q, svc, profileID, questionID := seed(t)
	svc.Learning = learning.NewService(q)

	result, err := svc.MigrateDemoProgress(context.Background(), profileID, []progress.DemoMigrateAnswer{
		{QuestionID: questionID, AnswerID: uuid.MustParse("00000000-0000-4000-8000-000000000099"), Correct: false},
	})
	if err != nil {
		t.Fatalf("MigrateDemoProgress: %v", err)
	}
	if result.Migrated != 0 || result.Skipped != 1 {
		t.Fatalf("result=%+v", result)
	}
}

func TestMigrateDemoProgressHTTP(t *testing.T) {
	q, svc, profileID, questionID := seed(t)
	svc.Learning = learning.NewService(q)
	wrong, _ := wrongAndCorrectAnswerIDs(t, q, questionID)

	tok, err := auth.IssueAccess([]byte(handlerSecret), profileID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h := &progress.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	body, _ := json.Marshal(map[string]any{
		"answers": []map[string]any{
			{
				"question_id": questionID.String(),
				"answer_id":   wrong.String(),
				"correct":     false,
				"answered_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
	status, respBody := doReq(t, ts, http.MethodPost, "/me/demo-progress/migrate", tok, body)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, respBody)
	}
	var env struct {
		Data struct {
			Migrated int `json:"migrated"`
			Skipped  int `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Migrated != 1 {
		t.Fatalf("data=%+v", env.Data)
	}
}

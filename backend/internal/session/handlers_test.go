package session_test

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
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupServer(t *testing.T) (*httptest.Server, string, *sqlc.Queries) {
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
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	svc := session.NewService(q, pool, billing.Service{Q: q}, learning.NewService(q), progress.NewService(q))
	r := chi.NewRouter()
	h := &session.Handler{Svc: svc, Content: &content.Handler{Q: q, MediaBase: "http://media.test"}}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, q
}

type respEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doReq(t *testing.T, ts *httptest.Server, method, path, token string, body []byte) (int, respEnvelope) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env respEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}

func installVerifiedExplanation(t *testing.T, q *sqlc.Queries, questionID uuid.UUID, locale, sentinel string) {
	t.Helper()
	ctx := context.Background()
	explanationID, err := q.UpsertExplanation(ctx, sqlc.UpsertExplanationParams{
		QuestionID: questionID,
		LegalRefs:  json.RawMessage(`[{"code":"TEST","title":"Test rule"}]`),
	})
	if err != nil {
		t.Fatalf("upsert explanation: %v", err)
	}
	blocks, err := json.Marshal([]map[string]string{{"type": "muhim", "text": sentinel}})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertExplanationTranslation(ctx, sqlc.UpsertExplanationTranslationParams{
		ExplanationID: explanationID,
		Locale:        locale,
		Blocks:        blocks,
		Status:        "verified",
		Source:        "session-test",
	}); err != nil {
		t.Fatalf("upsert explanation translation: %v", err)
	}
}

func assignedQuestionWithExplanation(t *testing.T, q *sqlc.Queries, ids []string) uuid.UUID {
	t.Helper()
	for _, rawID := range ids {
		id := uuid.MustParse(rawID)
		if _, err := q.GetVerifiedExplanation(context.Background(), sqlc.GetVerifiedExplanationParams{
			QuestionID: id, Locale: "uz-Latn",
		}); err == nil {
			return id
		}
	}
	t.Fatal("session has no question with a verified fixture explanation")
	return uuid.Nil
}

func assertNoFeedbackLeak(t *testing.T, raw json.RawMessage, sentinel string) map[string]json.RawMessage {
	t.Helper()
	if strings.Contains(string(raw), sentinel) {
		t.Fatalf("answer-revealing sentinel leaked: %s", raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if explanation, ok := fields["explanation"]; ok && string(explanation) != "null" {
		t.Fatalf("explanation leaked before disclosure: %s", explanation)
	}
	if _, ok := fields["correct"]; ok {
		t.Fatalf("correct leaked before disclosure: %s", raw)
	}
	if _, ok := fields["correct_answer_id"]; ok {
		t.Fatalf("correct_answer_id leaked before disclosure: %s", raw)
	}
	return fields
}

// assertNoExplanationLeak allows per-answer green/red grades while the exam
// is still running, but keeps explanation prose sealed until finish.
func assertNoExplanationLeak(t *testing.T, raw json.RawMessage, sentinel string) map[string]json.RawMessage {
	t.Helper()
	if strings.Contains(string(raw), sentinel) {
		t.Fatalf("explanation sentinel leaked: %s", raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if explanation, ok := fields["explanation"]; ok && string(explanation) != "null" {
		t.Fatalf("explanation leaked before disclosure: %s", explanation)
	}
	return fields
}

func TestFullVariantSessionOverHTTP(t *testing.T) {
	ts, tok, q := setupServer(t)
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": v.ID, "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d body=%s", status, env.Data)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total != 20 || len(created.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions: %+v", created)
	}

	ansBody, _ := json.Marshal(map[string]any{"question_id": created.QuestionIDs[0], "answer_id": "00000000-0000-0000-0000-000000000000"})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, ansBody)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_answer" {
		t.Fatalf("expected 400 invalid_answer for a made-up answer id, got status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/variants", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("me/variants status=%d", status)
	}
	var statuses []struct {
		Number   int32 `json:"number"`
		Unlocked bool  `json:"unlocked"`
	}
	if err := json.Unmarshal(env.Data, &statuses); err != nil {
		t.Fatal(err)
	}
	if !statuses[0].Unlocked || statuses[1].Unlocked {
		t.Fatalf("expected only variant 1 unlocked initially: %+v", statuses)
	}
}

func TestSessionQuestionDetailRequiresAuthAndMembership(t *testing.T) {
	ts, tok, q := setupServer(t)
	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": "1", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d env=%+v", status, env)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	path := "/sessions/" + created.ID + "/questions/" + created.QuestionIDs[0]
	status, _ = doReq(t, ts, http.MethodGet, path, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("session question detail without auth status=%d want 401", status)
	}
	other, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998907770099"})
	if err != nil {
		t.Fatalf("create other profile: %v", err)
	}
	otherToken, err := auth.IssueAccess([]byte(handlerSecret), other.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, env = doReq(t, ts, http.MethodGet, path, otherToken, nil)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("another profile's session detail status=%d env=%+v want 404 not_found", status, env)
	}

	secondVariant, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	outsideIDs, err := q.ListVariantQuestionIDsOrdered(context.Background(), secondVariant.ID)
	if err != nil || len(outsideIDs) == 0 {
		t.Fatalf("outside questions: len=%d err=%v", len(outsideIDs), err)
	}
	outsidePath := "/sessions/" + created.ID + "/questions/" + outsideIDs[0].String()
	status, env = doReq(t, ts, http.MethodGet, outsidePath, tok, nil)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("unassigned question status=%d env=%+v want 404 not_found", status, env)
	}
}

func TestFeedbackSessionQuestionDetailRedactsUntilAnswered(t *testing.T) {
	ts, tok, q := setupServer(t)
	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": "1", "locale": "ru"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d env=%+v", status, env)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	questionID := assignedQuestionWithExplanation(t, q, created.QuestionIDs)
	const sentinel = "SESSION_FEEDBACK_RU_SENTINEL"
	installVerifiedExplanation(t, q, questionID, "ru", sentinel)
	path := "/sessions/" + created.ID + "/questions/" + questionID.String() + "?locale=ru"

	status, env = doReq(t, ts, http.MethodGet, path, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get unanswered detail status=%d env=%+v", status, env)
	}
	fields := assertNoFeedbackLeak(t, env.Data, sentinel)
	if _, ok := fields["answers"]; !ok {
		t.Fatalf("core answers missing before feedback: %s", env.Data)
	}
	if _, ok := fields["signs"]; !ok {
		t.Fatalf("core signs missing before feedback: %s", env.Data)
	}

	answerID := correctAnswerID(t, q, questionID)
	answerBody, _ := json.Marshal(map[string]any{"question_id": questionID, "answer_id": answerID})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, answerBody)
	if status != http.StatusOK {
		t.Fatalf("submit answer status=%d env=%+v", status, env)
	}
	if !strings.Contains(string(env.Data), sentinel) {
		t.Fatalf("feedback submit response lacks localized explanation sentinel: %s", env.Data)
	}

	status, env = doReq(t, ts, http.MethodGet, path, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get answered detail status=%d env=%+v", status, env)
	}
	if !strings.Contains(string(env.Data), sentinel) {
		t.Fatalf("answered feedback detail lacks localized explanation: %s", env.Data)
	}
	var answered map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &answered); err != nil {
		t.Fatal(err)
	}
	var correct bool
	var correctAnswerID string
	var userAnswerID string
	if err := json.Unmarshal(answered["correct"], &correct); err != nil || !correct {
		t.Fatalf("answered feedback detail correct=%s err=%v", answered["correct"], err)
	}
	if err := json.Unmarshal(answered["correct_answer_id"], &correctAnswerID); err != nil || correctAnswerID != answerID.String() {
		t.Fatalf("answered feedback correct_answer_id=%q err=%v want %s", correctAnswerID, err, answerID)
	}
	if err := json.Unmarshal(answered["user_answer_id"], &userAnswerID); err != nil || userAnswerID != answerID.String() {
		t.Fatalf("answered feedback user_answer_id=%q err=%v want %s", userAnswerID, err, answerID)
	}
}

func TestExamSessionQuestionDetailRedactsUntilCompletion(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)
	body, _ := json.Marshal(map[string]any{"mode": "exam", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create exam status=%d env=%+v", status, env)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	questionID := uuid.MustParse(created.QuestionIDs[0])
	const sentinel = "SESSION_EXAM_SECRET_SENTINEL"
	installVerifiedExplanation(t, q, questionID, "uz-Latn", sentinel)
	path := "/sessions/" + created.ID + "/questions/" + questionID.String() + "?locale=uz-Latn"

	status, env = doReq(t, ts, http.MethodGet, path, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get active exam detail status=%d env=%+v", status, env)
	}
	assertNoFeedbackLeak(t, env.Data, sentinel)

	answerID := correctAnswerID(t, q, questionID)
	answerBody, _ := json.Marshal(map[string]any{"question_id": questionID, "answer_id": answerID})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, answerBody)
	if status != http.StatusOK {
		t.Fatalf("submit exam answer status=%d env=%+v", status, env)
	}
	// The exam submit answer response now returns correct/correct_answer_id
	// per-answer (matching the official Avtotest desktop app), but the
	// explanation is still withheld until finish.
	var submitFields map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &submitFields); err != nil {
		t.Fatal(err)
	}
	if _, ok := submitFields["correct"]; !ok {
		t.Fatalf("exam submit should now disclose correctness: %s", env.Data)
	}
	if _, ok := submitFields["correct_answer_id"]; !ok {
		t.Fatalf("exam submit should now disclose correct_answer_id: %s", env.Data)
	}
	if explanation, ok := submitFields["explanation"]; ok && string(explanation) != "null" {
		t.Fatalf("exam submit should NOT disclose explanation before finish: %s", env.Data)
	}

	status, env = doReq(t, ts, http.MethodGet, path, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get answered active exam detail status=%d env=%+v", status, env)
	}
	answeredFields := assertNoExplanationLeak(t, env.Data, sentinel)
	var answeredCorrect bool
	if err := json.Unmarshal(answeredFields["correct"], &answeredCorrect); err != nil || !answeredCorrect {
		t.Fatalf("answered in-progress exam detail must disclose correct=true: %s", env.Data)
	}
	if _, ok := answeredFields["correct_answer_id"]; !ok {
		t.Fatalf("answered in-progress exam detail must disclose correct_answer_id: %s", env.Data)
	}

	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/finish", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("finish exam status=%d env=%+v", status, env)
	}
	status, env = doReq(t, ts, http.MethodGet, path, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get completed exam detail status=%d env=%+v", status, env)
	}
	if !strings.Contains(string(env.Data), sentinel) {
		t.Fatalf("completed exam did not disclose explanation sentinel: %s", env.Data)
	}
	var completed map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &completed); err != nil {
		t.Fatal(err)
	}
	if _, ok := completed["correct"]; !ok {
		t.Fatalf("completed exam did not disclose correctness: %s", env.Data)
	}
	if _, ok := completed["correct_answer_id"]; !ok {
		t.Fatalf("completed exam did not disclose answer key: %s", env.Data)
	}
}

func TestGetNewExamSessionOverHTTPReturnsAllQuestionsAndTimeLimit(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	grantVIP(t, q, profile.ID)

	body, _ := json.Marshal(map[string]any{"mode": "exam", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create exam status=%d env=%+v", status, env)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}

	status, env = doReq(t, ts, http.MethodGet, "/sessions/"+created.ID, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get exam status=%d env=%+v", status, env)
	}
	var detail struct {
		TimeLimitSec *int `json:"time_limit_sec"`
		Answers      []struct {
			QuestionID string `json:"question_id"`
			Position   int    `json:"position"`
			Answered   bool   `json:"answered"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.TimeLimitSec == nil || *detail.TimeLimitSec != session.ExamTimeLimitSec {
		t.Fatalf("time_limit_sec=%v want %d", detail.TimeLimitSec, session.ExamTimeLimitSec)
	}
	if len(detail.Answers) != len(created.QuestionIDs) {
		t.Fatalf("new exam GET returned %d questions, want %d", len(detail.Answers), len(created.QuestionIDs))
	}
	for i, answer := range detail.Answers {
		if answer.QuestionID != created.QuestionIDs[i] || answer.Position != i+1 || answer.Answered {
			t.Fatalf("answer row %d=%+v want question=%s position=%d unanswered", i, answer, created.QuestionIDs[i], i+1)
		}
	}
}

func TestSubmitQuestionOutsideSessionOverHTTPIsRejected(t *testing.T) {
	ts, tok, q := setupServer(t)
	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": "1", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d env=%+v", status, env)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	variant, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	outsideIDs, err := q.ListVariantQuestionIDsOrdered(context.Background(), variant.ID)
	if err != nil || len(outsideIDs) == 0 {
		t.Fatalf("outside questions: len=%d err=%v", len(outsideIDs), err)
	}
	answerID := correctAnswerID(t, q, outsideIDs[0])
	answerBody, _ := json.Marshal(map[string]any{
		"question_id": outsideIDs[0],
		"answer_id":   answerID,
	})

	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, answerBody)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "question_not_assigned" {
		t.Fatalf("outside question must return 400 question_not_assigned; status=%d env=%+v", status, env)
	}
}

func TestExamSessionHTTPRedactsThenDisclosesAnswerDetails(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	grantVIP(t, q, profile.ID)

	body, _ := json.Marshal(map[string]any{"mode": "exam", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create exam status=%d env=%+v", status, env)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	questionID := uuid.MustParse(created.QuestionIDs[0])
	answerID := correctAnswerID(t, q, questionID)
	answerBody, _ := json.Marshal(map[string]any{"question_id": questionID, "answer_id": answerID})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, answerBody)
	if status != http.StatusOK {
		t.Fatalf("submit answer status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/sessions/"+created.ID, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get in-progress exam status=%d env=%+v", status, env)
	}
	var inProgress struct {
		Answers []map[string]json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &inProgress); err != nil {
		t.Fatal(err)
	}
	first := inProgress.Answers[0]
	var userAnswerID string
	if raw, ok := first["user_answer_id"]; !ok || json.Unmarshal(raw, &userAnswerID) != nil || userAnswerID != answerID.String() {
		t.Fatalf("in-progress exam must include the user's answer id: %v", first)
	}
	var correct bool
	var correctAnswerIDVal string
	if err := json.Unmarshal(first["correct"], &correct); err != nil || !correct {
		t.Fatalf("in-progress exam must disclose correct=true for answered question: %v", first)
	}
	if err := json.Unmarshal(first["correct_answer_id"], &correctAnswerIDVal); err != nil || correctAnswerIDVal != answerID.String() {
		t.Fatalf("in-progress exam must disclose correct_answer_id for answered question: %v", first)
	}
	if len(inProgress.Answers) > 1 {
		if _, ok := inProgress.Answers[1]["correct_answer_id"]; ok {
			t.Fatalf("in-progress exam must not leak answer key for unanswered questions: %v", inProgress.Answers[1])
		}
	}

	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/finish", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("finish exam status=%d env=%+v", status, env)
	}
	status, env = doReq(t, ts, http.MethodGet, "/sessions/"+created.ID, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get finished exam status=%d env=%+v", status, env)
	}
	var finished struct {
		Answers []map[string]json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &finished); err != nil {
		t.Fatal(err)
	}
	first = finished.Answers[0]
	correct = false
	correctAnswerIDVal = ""
	if err := json.Unmarshal(first["correct"], &correct); err != nil || !correct {
		t.Fatalf("finished exam must disclose correct=true: %v", first)
	}
	if err := json.Unmarshal(first["correct_answer_id"], &correctAnswerIDVal); err != nil || correctAnswerIDVal != answerID.String() {
		t.Fatalf("finished exam must disclose correct answer id: %v", first)
	}
	if _, ok := finished.Answers[1]["correct_answer_id"]; !ok {
		t.Fatalf("finished exam must disclose the answer key for skipped questions: %v", finished.Answers[1])
	}
}

// TestVariantSessionByNumberOverHTTP proves the same content-contract gap
// TestPracticeSessionByCategoryCodeOverHTTP fixes for category_id/sign_id,
// but for variant_id: GET /variants (content.VariantListItemDTO) never
// exposes a bilet's UUID, only its `number` — matching what
// variants_screen.dart actually sends (`variant.number.toString()`) — so a
// real client can only send `variant_id: "1"`, not a UUID it doesn't have.
func TestVariantSessionByNumberOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": "1", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session by variant number status=%d body=%s err=%+v", status, env.Data, env.Error)
	}
	var created struct {
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total != 20 || len(created.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions: %+v", created)
	}
}

// TestVariantSessionByBogusNumberOverHTTP confirms an unrecognized bilet
// number is reported as not_found — same 404 convention as
// TestPracticeSessionByBogusCategoryCodeOverHTTP — rather than a
// UUID-parse-shaped invalid_body/invalid_request.
func TestVariantSessionByBogusNumberOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": "999999", "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("expected 404 not_found for a bogus variant number, got status=%d env=%+v", status, env)
	}
}

// TestPracticeSessionByCategoryCodeOverHTTP proves the content-contract gap
// this covers is actually fixed end-to-end: the Flutter Category model
// (content.CategoryDTO / GET /categories) only ever exposes `code`, never a
// UUID, so a real client can only send `category_id: "signs"` — not a UUID
// it doesn't have. POST /sessions must accept that code directly.
func TestPracticeSessionByCategoryCodeOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{
		"mode": "practice", "category_id": "signs", "locale": "uz-Latn", "count": 5,
	})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session by category code status=%d body=%s err=%+v", status, env.Data, env.Error)
	}
	var created struct {
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total == 0 || len(created.QuestionIDs) != created.Total {
		t.Fatalf("expected non-empty question set: %+v", created)
	}
}

// TestPracticeSessionByImagePresenceOverHTTP covers the third practice
// selector, which unlike category/sign needs no code resolution: has_image
// filters straight on question.image_id. Both polarities must work, and the
// selector must stay mutually exclusive with the other two.
func TestPracticeSessionByImagePresenceOverHTTP(t *testing.T) {
	for _, hasImage := range []bool{true, false} {
		ts, tok, _ := setupServer(t)

		body, _ := json.Marshal(map[string]any{
			"mode": "practice", "has_image": hasImage, "locale": "uz-Latn", "count": 5,
		})
		status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
		if status != http.StatusCreated {
			t.Fatalf("has_image=%v status=%d body=%s err=%+v", hasImage, status, env.Data, env.Error)
		}
		var created struct {
			QuestionIDs []string `json:"question_ids"`
			Total       int      `json:"total"`
		}
		if err := json.Unmarshal(env.Data, &created); err != nil {
			t.Fatal(err)
		}
		if created.Total == 0 || len(created.QuestionIDs) != created.Total {
			t.Fatalf("has_image=%v expected non-empty question set: %+v", hasImage, created)
		}
	}
}

// TestPracticeSessionByVariantRangeOverHTTP covers the bilet-span selector,
// which lets a learner mix-review a range they have already worked through.
func TestPracticeSessionByVariantRangeOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{
		"mode": "practice", "variant_from": 1, "variant_to": 2,
		"locale": "uz-Latn", "count": 5,
	})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("variant range status=%d body=%s err=%+v", status, env.Data, env.Error)
	}
	var created struct {
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total == 0 || len(created.QuestionIDs) != created.Total {
		t.Fatalf("expected non-empty question set: %+v", created)
	}
}

// TestPracticeSessionRejectsMalformedVariantRange: a half-open or inverted
// span must fail loudly rather than quietly widening or emptying the draw.
func TestPracticeSessionRejectsMalformedVariantRange(t *testing.T) {
	cases := []map[string]any{
		{"variant_from": 5, "variant_to": 0},
		{"variant_from": 0, "variant_to": 5},
		{"variant_from": 9, "variant_to": 3},
	}
	for _, extra := range cases {
		ts, tok, _ := setupServer(t)
		payload := map[string]any{"mode": "practice", "locale": "uz-Latn", "count": 5}
		for k, v := range extra {
			payload[k] = v
		}
		body, _ := json.Marshal(payload)
		status, _ := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
		if status != http.StatusBadRequest {
			t.Fatalf("range %+v: status=%d want 400", extra, status)
		}
	}
}

// TestPracticeAllowanceReportsBudget: the picker needs the real remaining
// budget, otherwise it offers sizes the server then silently clamps.
func TestPracticeAllowanceReportsBudget(t *testing.T) {
	ts, tok, _ := setupServer(t)

	status, env := doReq(t, ts, http.MethodGet, "/me/practice-allowance", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("allowance status=%d err=%+v", status, env.Error)
	}
	var got struct {
		Unlimited bool `json:"unlimited"`
		Limit     int  `json:"limit"`
		Used      int  `json:"used"`
		Remaining int  `json:"remaining"`
	}
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Unlimited {
		return // VIP fixtures report no finite budget, which is also valid
	}
	if got.Limit <= 0 || got.Remaining != got.Limit-got.Used {
		t.Fatalf("inconsistent allowance: %+v", got)
	}
}

type mockEligibilityDTO struct {
	Eligible             bool    `json:"eligible"`
	MasteryPercent       int     `json:"mastery_percent"`
	MinRequiredPercent   int     `json:"min_required_percent"`
	QuestionsStudied     int     `json:"questions_studied"`
	MinRequiredQuestions int     `json:"min_required_questions"`
	IsVIP                bool    `json:"is_vip"`
	Reason               *string `json:"reason"`
}

// TestMockEligibilityHandler exercises GET /me/mock-eligibility end-to-end
// over HTTP for both ineligible reasons and the eligible case, mirroring
// TestPracticeAllowanceReportsBudget's setupServer/doReq pattern.
func TestMockEligibilityHandler(t *testing.T) {
	t.Run("mastery too low", func(t *testing.T) {
		ts, tok, q := setupServer(t)
		profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
		if err != nil {
			t.Fatal(err)
		}
		grantVIP(t, q, profile.ID)
		// Studied plenty, answered everything wrong: past the volume floor,
		// blocked on accuracy.
		studyQuestions(t, q, learning.NewService(q), profile.ID, 0, learning.Again)

		status, env := doReq(t, ts, http.MethodGet, "/me/mock-eligibility", tok, nil)
		if status != http.StatusOK {
			t.Fatalf("status=%d err=%+v", status, env.Error)
		}
		var got mockEligibilityDTO
		if err := json.Unmarshal(env.Data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Eligible || got.Reason == nil || *got.Reason != "mastery_too_low" {
			t.Fatalf("expected ineligible/mastery_too_low, got %+v", got)
		}
		if !got.IsVIP {
			t.Fatalf("expected is_vip=true, got %+v", got)
		}
	})

	t.Run("too few studied", func(t *testing.T) {
		ts, tok, q := setupServer(t)
		profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
		if err != nil {
			t.Fatal(err)
		}
		grantVIP(t, q, profile.ID)
		// One correct answer per category: bank-honest readiness stays low and
		// studied count is far below the volume floor (reported first).
		studied := studyOnePerCategoryCorrectly(t, q, learning.NewService(q), profile.ID)

		status, env := doReq(t, ts, http.MethodGet, "/me/mock-eligibility", tok, nil)
		if status != http.StatusOK {
			t.Fatalf("status=%d err=%+v", status, env.Error)
		}
		var got mockEligibilityDTO
		if err := json.Unmarshal(env.Data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Eligible || got.Reason == nil || *got.Reason != "too_few_studied" {
			t.Fatalf("expected ineligible/too_few_studied, got %+v", got)
		}
		if got.QuestionsStudied != studied || got.MinRequiredQuestions <= studied {
			t.Fatalf("expected studied=%d below a meaningful floor, got %+v", studied, got)
		}
	})

	t.Run("vip required", func(t *testing.T) {
		ts, tok, q := setupServer(t)
		profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
		if err != nil {
			t.Fatal(err)
		}
		grantMastery(t, q, learning.NewService(q), profile.ID)

		status, env := doReq(t, ts, http.MethodGet, "/me/mock-eligibility", tok, nil)
		if status != http.StatusOK {
			t.Fatalf("status=%d err=%+v", status, env.Error)
		}
		var got mockEligibilityDTO
		if err := json.Unmarshal(env.Data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Eligible || got.Reason == nil || *got.Reason != "vip_required" {
			t.Fatalf("expected ineligible/vip_required, got %+v", got)
		}
		if got.IsVIP {
			t.Fatalf("expected is_vip=false, got %+v", got)
		}
	})

	t.Run("eligible", func(t *testing.T) {
		ts, tok, q := setupServer(t)
		profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
		if err != nil {
			t.Fatal(err)
		}
		grantVIP(t, q, profile.ID)
		grantMastery(t, q, learning.NewService(q), profile.ID)

		status, env := doReq(t, ts, http.MethodGet, "/me/mock-eligibility", tok, nil)
		if status != http.StatusOK {
			t.Fatalf("status=%d err=%+v", status, env.Error)
		}
		var got mockEligibilityDTO
		if err := json.Unmarshal(env.Data, &got); err != nil {
			t.Fatal(err)
		}
		if !got.Eligible || got.Reason != nil {
			t.Fatalf("expected eligible with no reason, got %+v", got)
		}
		if !got.IsVIP || got.MasteryPercent < got.MinRequiredPercent {
			t.Fatalf("expected is_vip=true and mastery>=min, got %+v", got)
		}
	})
}

// TestPracticeSessionRejectsCombinedSelectors pins the invariant that practice
// takes exactly one selector — combining them would silently ignore one.
func TestPracticeSessionRejectsCombinedSelectors(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{
		"mode": "practice", "category_id": "signs", "has_image": true,
		"locale": "uz-Latn", "count": 5,
	})
	status, _ := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for combined selectors, got %d", status)
	}
}

// TestPracticeSessionBySignCodeOverHTTP is TestPracticeSessionByCategoryCodeOverHTTP's
// counterpart for sign_id/content.SignDTOs, which likewise never expose a UUID.
func TestPracticeSessionBySignCodeOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{
		"mode": "practice", "sign_id": "3.27", "locale": "uz-Latn", "count": 5,
	})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session by sign code status=%d body=%s err=%+v", status, env.Data, env.Error)
	}
	var created struct {
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total == 0 || len(created.QuestionIDs) != created.Total {
		t.Fatalf("expected non-empty question set: %+v", created)
	}
}

// TestPracticeSessionByBogusCategoryCodeOverHTTP confirms an unrecognized
// code (not a UUID, not a real category.code) is reported as not_found —
// same 404 error-code convention as every other not-found path in this
// package — rather than a UUID-parse-shaped invalid_body/invalid_request.
func TestPracticeSessionByBogusCategoryCodeOverHTTP(t *testing.T) {
	ts, tok, _ := setupServer(t)

	body, _ := json.Marshal(map[string]any{
		"mode": "practice", "category_id": "no-such-category", "locale": "uz-Latn", "count": 5,
	})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("expected 404 not_found for a bogus category code, got status=%d env=%+v", status, env)
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	ts, _, _ := setupServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

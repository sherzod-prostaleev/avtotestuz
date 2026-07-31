package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

type recordedCall struct {
	method  string
	payload map[string]any
}

type recordingTG struct {
	mu    sync.Mutex
	calls []recordedCall
	srv   *httptest.Server
}

func (r *recordingTG) methodCalls(method string) []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []map[string]any{}
	for _, c := range r.calls {
		if c.method == method {
			out = append(out, c.payload)
		}
	}
	return out
}

func newRecordingTelegram(t *testing.T) (*recordingTG, *Client) {
	t.Helper()
	r := &recordingTG{}
	msgID := int64(500)
	pollSeq := 0
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(req.URL.Path, "/")
		method := parts[len(parts)-1]
		var body map[string]any
		_ = json.NewDecoder(req.Body).Decode(&body)
		r.mu.Lock()
		r.calls = append(r.calls, recordedCall{method: method, payload: body})
		r.mu.Unlock()
		msgID++
		if method == "sendPoll" {
			pollSeq++
			_, _ = fmt.Fprintf(w,
				`{"ok":true,"result":{"message_id":%d,"poll":{"id":"poll-%d"}}}`, msgID, pollSeq)
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, msgID)
	}))
	t.Cleanup(r.srv.Close)
	return r, NewClient(r.srv.URL, "T", r.srv.Client())
}

// An illustrated question becomes two messages: the photo, then a poll that
// replies to it. Telegram cannot attach an image to a poll.
func TestSendNextQuestionSendsPhotoThenPoll(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, true, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartOrNext(ctx, -8001, 21); err != nil {
		t.Fatalf("StartOrNext: %v", err)
	}

	photos := rec.methodCalls("sendPhoto")
	polls := rec.methodCalls("sendPoll")
	if len(photos) != 1 {
		t.Fatalf("want 1 sendPhoto, got %d", len(photos))
	}
	if len(polls) != 1 {
		t.Fatalf("want 1 sendPoll, got %d", len(polls))
	}
	if polls[0]["reply_to_message_id"] == nil {
		t.Fatal("poll must reply to the photo so the pair reads as one question")
	}
	if polls[0]["type"] != "quiz" {
		t.Fatalf("poll type = %v", polls[0]["type"])
	}
}

// The poll id Telegram returns must be stored, or the poll_answer that comes
// back cannot be matched to a question.
func TestSendNextQuestionRecordsPollID(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartOrNext(ctx, -8002, 22); err != nil {
		t.Fatal(err)
	}
	stored, err := q.GetQuizPoll(ctx, "poll-1")
	if err != nil {
		t.Fatalf("poll id was not recorded: %v", err)
	}
	if stored.QuestionNo != 1 {
		t.Fatalf("QuestionNo = %d, want 1", stored.QuestionNo)
	}
	if stored.CorrectIdx != 0 {
		t.Fatalf("CorrectIdx = %d, want 0", stored.CorrectIdx)
	}
}

// A group session must be marked as such at creation — the final message
// format branches on it, and chat type is the only signal.
func TestStartOrNextMarksGroupMode(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	// limit_config is seeded by migration and never truncated between tests
	// (testdb.Truncate keeps it on purpose); other tests in this package
	// deliberately overwrite tg_quiz_questions to a non-default value and
	// leave it that way, so pin it back to the default explicitly rather
	// than assume a pristine table.
	if _, err := pool.Exec(ctx,
		`DELETE FROM limit_config WHERE key = 'tg_quiz_questions'`); err != nil {
		t.Fatal(err)
	}

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8003, 23, "supergroup"); err != nil {
		t.Fatal(err)
	}
	session, err := q.GetActiveQuizSessionByChat(ctx, -8003)
	if err != nil {
		t.Fatal(err)
	}
	if session.Mode != "group" {
		t.Fatalf("Mode = %q, want group", session.Mode)
	}
	if session.TotalQuestions != defaultQuizQuestions {
		t.Fatalf("TotalQuestions = %d, want %d", session.TotalQuestions, defaultQuizQuestions)
	}
}

func TestStartGameMarksSoloModeInPrivate(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, 9001, 24, "private"); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, 9001)
	if session.Mode != "solo" {
		t.Fatalf("Mode = %q, want solo", session.Mode)
	}
}

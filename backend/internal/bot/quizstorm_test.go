package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// A quiz send Telegram rejects for good (bot muted in the group, polls not
// allowed, user blocked the bot) must not fail the update: the webhook would
// answer 503 and Telegram would redeliver the same update indefinitely.

func groupQuizUpdate(chatID int64, chatType string) Update {
	return Update{UpdateID: 9, Message: &Message{
		Text: "/quiz@AvtoTestBot", From: &User{ID: 77}, Chat: Chat{ID: chatID, Type: chatType},
	}}
}

func TestQuizInMutedGroupDoesNotRetryStorm(t *testing.T) {
	b, _, fake := newTestBot(t)
	ctx := context.Background()
	seedQuizQuestion(t, b.Quiz.Pool, false)
	fake.mu.Lock()
	fake.failSend = true
	fake.mu.Unlock()
	for i := 0; i < 3; i++ {
		if err := b.HandleUpdate(ctx, groupQuizUpdate(-100777, "supergroup")); err != nil {
			t.Fatalf("delivery %d returned %v: the webhook would 503 and Telegram retry forever", i, err)
		}
	}
	var qno int32
	if err := b.Quiz.Pool.QueryRow(ctx,
		`SELECT coalesce(max(question_no), 0) FROM telegram_quiz_session WHERE chat_id = -100777`).Scan(&qno); err != nil {
		t.Fatal(err)
	}
	if qno != 0 {
		t.Fatalf("question_no=%d: no question was ever delivered", qno)
	}
}

// pollForbiddenBot answers every sendPoll with Telegram's "not enough
// rights" 400 and everything else with success.
func pollForbiddenBot(t *testing.T) (*Bot, *int32) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sendPoll") {
			atomic.AddInt32(&polls, 1)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: not enough rights to send polls to the chat"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":5}}`))
	}))
	t.Cleanup(srv.Close)
	for i := 0; i < 3; i++ {
		seedQuizQuestion(t, pool, false)
	}
	return wireTestBot(pool, q, NewClient(srv.URL, "t", srv.Client())), &polls
}

func TestQuizPollForbiddenNeitherRetriesNorAdvances(t *testing.T) {
	b, polls := pollForbiddenBot(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := b.HandleUpdate(ctx, groupQuizUpdate(-100778, "group")); err != nil {
			t.Fatalf("delivery %d returned %v", i, err)
		}
	}
	if got := atomic.LoadInt32(polls); got != 3 {
		t.Fatalf("sendPoll calls=%d want 3 (one per delivery)", got)
	}
	var sessions int
	var qno int32
	if err := b.Quiz.Pool.QueryRow(ctx,
		`SELECT count(*), coalesce(max(question_no), 0) FROM telegram_quiz_session WHERE chat_id = -100778`).Scan(&sessions, &qno); err != nil {
		t.Fatal(err)
	}
	// Each delivery used to burn a question number before sendPoll failed,
	// so a redelivered /quiz walked the game towards its end unseen.
	if sessions != 1 || qno != 0 {
		t.Fatalf("sessions=%d question_no=%d want 1 session still at question 0", sessions, qno)
	}
}

func TestQuizStopCallbackFromBlockedUserDoesNotRetryStorm(t *testing.T) {
	b, _, fake := newTestBot(t)
	ctx := context.Background()
	seedQuizQuestion(t, b.Quiz.Pool, false)
	const chat = 79
	if err := b.HandleUpdate(ctx, Update{UpdateID: 1, Message: &Message{
		Text: "/quiz", From: &User{ID: chat}, Chat: Chat{ID: chat, Type: "private"},
	}}); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.failSend = true
	fake.mu.Unlock()
	stop := Update{UpdateID: 11, CallbackQuery: &CallbackQuery{ID: "c1", From: User{ID: chat}, Data: cbStop,
		Message: &Message{MessageID: 1, Chat: Chat{ID: chat, Type: "private"}}}}
	// First tap ends the active game (its result message 403s); the second
	// finds no game and its "no active quiz" reply 403s too.
	for i := 0; i < 2; i++ {
		if err := b.HandleUpdate(ctx, stop); err != nil {
			t.Fatalf("stop tap %d returned %v", i, err)
		}
	}
	var active int
	if err := b.Quiz.Pool.QueryRow(ctx,
		`SELECT count(*) FROM telegram_quiz_session WHERE chat_id = $1 AND active`, chat).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active sessions=%d: the stop must commit even though its reply failed", active)
	}
}

// The swallowing is for Telegram's permanent rejections only: a DB failure
// still fails the update so Telegram retries it once the database is back.
func TestQuizAndResetCallbacksStillReturnDBErrors(t *testing.T) {
	ctx := context.Background()
	// A closed pool: every query fails the way an unreachable database does.
	dead, err := pgxpool.New(ctx, "postgres://avtotest@127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	dead.Close()
	q := sqlc.New(dead)
	_, client := newFakeTelegram(t)
	b := wireTestBot(dead, q, client)
	attachAuth(t, b, q)

	cases := map[string]Update{
		"quiz stop callback": {UpdateID: 1, CallbackQuery: &CallbackQuery{ID: "c", From: User{ID: 5}, Data: cbStop,
			Message: &Message{MessageID: 1, Chat: Chat{ID: 5, Type: "private"}}}},
		"reset yes callback": callback(5, cbResetYes+"some-nonce"),
		"/quiz":              groupQuizUpdate(-100779, "group"),
	}
	for name, u := range cases {
		if err := b.HandleUpdate(ctx, u); err == nil {
			t.Errorf("%s: DB failure was swallowed (webhook would answer 200 and the update is lost)", name)
		}
	}
}

func TestQuizErrSwallowsOnlyPermanentTelegramRejections(t *testing.T) {
	b := &Bot{}
	forbidden := &APIError{Method: "sendPoll", Code: 400}
	db := errors.New("db down")
	for _, tc := range []struct {
		name    string
		err     error
		swallow bool
	}{
		{"permanent", forbidden, true},
		{"wrapped permanent", fmt.Errorf("quiz: %w", forbidden), true},
		{"joined permanents", errors.Join(forbidden, &APIError{Method: "sendMessage", Code: 403}), true},
		{"429", &APIError{Method: "sendPoll", Code: 429}, false},
		{"5xx", &APIError{Method: "sendPoll", Code: 502}, false},
		{"db", db, false},
		{"db joined with permanent", errors.Join(db, forbidden), false},
	} {
		got := b.quizErr(tc.err)
		if tc.swallow && got != nil {
			t.Errorf("%s: want swallowed, got %v", tc.name, got)
		}
		if !tc.swallow && got == nil {
			t.Errorf("%s: want returned, got nil", tc.name)
		}
	}
}

package bot

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// countQuizSessionRows counts every telegram_quiz_session row for chatID,
// active or not, so a test can tell "the finished session is still the only
// one" apart from "a fresh one got created alongside it".
func countQuizSessionRows(t *testing.T, pool *pgxpool.Pool, chatID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM telegram_quiz_session WHERE chat_id = $1`, chatID,
	).Scan(&n); err != nil {
		t.Fatalf("countQuizSessionRows: %v", err)
	}
	return n
}

// waitForPollCount polls rec until sendPoll has been called at least want
// times, or fails the test after a bounded wait. This avoids sleeping for
// the production countdown (10s+) while still being deterministic: the
// scheduler in these tests always fires within tens of milliseconds.
func waitForPollCount(t *testing.T, rec *recordingTG, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(rec.methodCalls("sendPoll")) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d sendPoll calls, got %d", want, len(rec.methodCalls("sendPoll")))
}

// NewAdvanceScheduler is the seam that drives a group game forward without
// anyone tapping "Keyingi savol". It must actually call back into the quiz
// service once its timer elapses.
//
// svc.Advance is deliberately left nil throughout this test: production's
// quizSeconds() clamps to a 5-second minimum (Telegram's own open_period
// floor), so wiring the real scheduler into svc.Advance would make every
// sendNextQuestion re-arm another multi-second timer and leave it running
// past the end of the test — exactly the cross-test DB contamination
// testdb's package doc warns about. Calling the returned function directly,
// the way scheduleAdvance would, exercises the identical code with a
// duration the test controls.
func TestNewAdvanceSchedulerFiresStartOrNext(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	prev := quizMinInterval
	quizMinInterval = time.Millisecond
	t.Cleanup(func() { quizMinInterval = prev })

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}
	scheduler := NewAdvanceScheduler(svc, nil)

	if err := svc.StartGame(ctx, -8700, 81, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.methodCalls("sendPoll")); got != 1 {
		t.Fatalf("setup: want 1 poll after StartGame, got %d", got)
	}

	scheduler(-8700, 20*time.Millisecond)
	waitForPollCount(t, rec, 2)
}

// A chat that gets a manual /next before its auto-advance timer fires must
// not also be advanced by that stale timer when it eventually does — that
// would silently skip a question out from under the group. See the comment
// on TestNewAdvanceSchedulerFiresStartOrNext for why svc.Advance stays nil.
func TestNewAdvanceSchedulerStaleTimerDoesNotDoubleAdvance(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	prev := quizMinInterval
	quizMinInterval = time.Millisecond
	t.Cleanup(func() { quizMinInterval = prev })

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}
	scheduler := NewAdvanceScheduler(svc, nil)

	if err := svc.StartGame(ctx, -8701, 82, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.methodCalls("sendPoll")); got != 1 {
		t.Fatalf("setup: want 1 poll after StartGame, got %d", got)
	}

	// Simulate the stale-timer race directly: an old, long-delay timer is
	// still outstanding when a fresh, short-delay one is scheduled for the
	// same chat (as if the room had manually advanced already). The old one
	// must become a no-op instead of asking a second question.
	scheduler(-8701, 200*time.Millisecond)
	scheduler(-8701, 20*time.Millisecond)

	waitForPollCount(t, rec, 2)

	// Give the stale (200ms) timer a chance to fire too, then confirm it
	// did not add a third poll.
	time.Sleep(300 * time.Millisecond)
	if got := len(rec.methodCalls("sendPoll")); got != 2 {
		t.Fatalf("stale timer fired: want 2 polls total, got %d", got)
	}
}

// An advance that errors must be logged, not silently dropped and not
// fatal — a webhook handler or long-poll loop must keep serving other chats.
func TestNewAdvanceSchedulerLogsFailureWithoutPanicking(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	log := zap.New(core)

	// Q is deliberately nil: StartOrNext's own guard clause turns that into
	// a plain error return, with no DB or network involved — a clean way to
	// force the failure path without relying on database timing.
	svc := &QuizService{TG: &Client{}}
	scheduler := NewAdvanceScheduler(svc, log)

	// scheduler runs detached (see NewAdvanceScheduler's own doc comment),
	// so this call returns immediately; the assertions below poll for the
	// background goroutine's log line instead of sleeping for a fixed time.
	scheduler(-8702, 5*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if logs.Len() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 warning logged, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("log level = %v, want Warn", entry.Level)
	}
	ctx := entry.ContextMap()
	if ctx["chat_id"] != int64(-8702) {
		t.Fatalf("chat_id = %v, want -8702", ctx["chat_id"])
	}
	if ctx["error"] == nil {
		t.Fatalf("expected an error field on the log entry, got none")
	}
}

// A pending advance timer must not resurrect a game the room already ended.
// StartOrNext creates a session when none is active — exactly what /quiz and
// /next want from a live user action — but a timer armed before /stop, and
// still outstanding when /stop runs, must not reuse that entry point: /stop
// does not (and should not) touch the scheduler's generation counter, so the
// stale timer is still "current" when it fires ~11s later.
func TestNewAdvanceSchedulerDoesNotResurrectStoppedGame(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	prev := quizMinInterval
	quizMinInterval = time.Millisecond
	t.Cleanup(func() { quizMinInterval = prev })

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}
	scheduler := NewAdvanceScheduler(svc, nil)

	const chatID = int64(-8704)
	if err := svc.StartGame(ctx, chatID, 84, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.methodCalls("sendPoll")); got != 1 {
		t.Fatalf("setup: want 1 poll after StartGame, got %d", got)
	}
	sessionsBefore := countQuizSessionRows(t, pool, chatID)

	if err := svc.Stop(ctx, chatID); err != nil {
		t.Fatal(err)
	}

	// The timer StartGame's sendNextQuestion armed is still "current" as far
	// as the scheduler's generation counter is concerned — /stop never
	// touches it. Firing it is what a real, un-mocked clock would do ~11s
	// after the question that /stop interrupted.
	scheduler(chatID, 20*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	if got := len(rec.methodCalls("sendPoll")); got != 1 {
		t.Fatalf("stale timer resurrected the game: want 1 poll total (none after /stop), got %d", got)
	}
	if got := countQuizSessionRows(t, pool, chatID); got != sessionsBefore {
		t.Fatalf("stale timer created a new session row: had %d before Stop, now %d", sessionsBefore, got)
	}
}

// A nil Advance must not panic — long-poll dev runs wire it, tests may not.
func TestSendNextQuestionWithoutSchedulerDoesNotPanic(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8703, 83, "supergroup"); err != nil {
		t.Fatalf("nil Advance must be tolerated: %v", err)
	}
}

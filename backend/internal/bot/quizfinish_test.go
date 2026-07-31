package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestRankingTextListsEveryParticipant(t *testing.T) {
	rows := []sqlc.ListQuizRankingRow{
		{DisplayName: "Aziz", CorrectCount: 9, AnsweredCount: 10, TotalMs: 42000},
		{DisplayName: "Malika", CorrectCount: 8, AnsweredCount: 10, TotalMs: 61000},
		{DisplayName: "Bekzod", CorrectCount: 8, AnsweredCount: 10, TotalMs: 97000},
		{DisplayName: "Nodira", CorrectCount: 6, AnsweredCount: 10, TotalMs: 74000},
	}
	out := rankingText(rows, "group", 10)
	for _, name := range []string{"Aziz", "Malika", "Bekzod", "Nodira"} {
		if !strings.Contains(out, name) {
			t.Fatalf("ranking is missing %s:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "9/10") {
		t.Fatalf("ranking lost the winner's score:\n%s", out)
	}
	// The winner must be named, not just listed first.
	if !strings.Contains(out, "Tabriklaymiz") {
		t.Fatalf("ranking does not congratulate anyone:\n%s", out)
	}
	if strings.Index(out, "Aziz") > strings.Index(out, "Malika") {
		t.Fatalf("winner is not first:\n%s", out)
	}
}

// Solo games get a plain result, not a one-line leaderboard.
func TestRankingTextSoloFormat(t *testing.T) {
	rows := []sqlc.ListQuizRankingRow{
		{DisplayName: "Aziz", CorrectCount: 8, AnsweredCount: 10, TotalMs: 54000},
	}
	out := rankingText(rows, "solo", 10)
	if !strings.Contains(out, "8/10") {
		t.Fatalf("solo result missing score:\n%s", out)
	}
	if strings.Contains(out, "🥇") {
		t.Fatalf("solo result should not render a podium:\n%s", out)
	}
}

func TestRankingTextHandlesNobodyPlaying(t *testing.T) {
	out := rankingText(nil, "group", 10)
	if !strings.Contains(out, "Hech kim") {
		t.Fatalf("empty ranking must say nobody played:\n%s", out)
	}
	if strings.Contains(out, "Tabriklaymiz") {
		t.Fatalf("must not congratulate nobody:\n%s", out)
	}
}

// The whole point of the timer: after the configured number of questions the
// game ends and the ranking is sent.
func TestFinishGameDeactivatesAndSendsRanking(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8200, 41, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 701, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8200)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	if _, err := q.GetActiveQuizSessionByChat(ctx, -8200); err == nil {
		t.Fatal("session is still active after finishGame")
	}
	var found bool
	for _, p := range rec.methodCalls("sendMessage") {
		if text, _ := p["text"].(string); strings.Contains(text, "Aziz") {
			found = true
		}
	}
	if !found {
		t.Fatal("final message did not include the ranking")
	}
}

// /stop mid-game must still report what people scored so far.
func TestStopSendsRankingSoFar(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8201, 42, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 801, FirstName: "Malika"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, -8201); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, p := range rec.methodCalls("sendMessage") {
		if text, _ := p["text"].(string); strings.Contains(text, "Malika") {
			found = true
		}
	}
	if !found {
		t.Fatal("/stop did not report the scores collected so far")
	}
}

// --- Addendum A -------------------------------------------------------
//
// awaiting_answer used to get cleared by the old inline-callback answer
// path. Task 5 replaced that path with polls (HandlePollAnswer correctly
// does not clear it, since a group poll has no single "the answer" moment),
// but left StartOrNext's post-throttle re-read guard checking a flag that
// can now never go false. A /next sent inside the throttle window dead-ends:
// it acks "Biroz kuting" and then sends nothing.
func TestStartOrNextDeliversNextQuestionAfterThrottleWindow(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	prev := quizMinInterval
	quizMinInterval = 40 * time.Millisecond
	t.Cleanup(func() { quizMinInterval = prev })

	if err := svc.StartGame(ctx, -8400, 51, "supergroup"); err != nil {
		t.Fatal(err)
	}
	// Someone answers via the poll — this must NOT be what unblocks the
	// guard; the group model has no single "the answer" moment.
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 901, FirstName: "Sardor"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}

	// This call lands inside the throttle window (quizMinInterval), so
	// StartOrNext acks "Biroz kuting", waits it out, re-reads the session,
	// and must still deliver question 2.
	if err := svc.StartOrNext(ctx, -8400, 51); err != nil {
		t.Fatal(err)
	}

	polls := rec.methodCalls("sendPoll")
	if len(polls) != 2 {
		t.Fatalf("want 2 polls sent (question 1 + question 2 after the throttle wait), got %d", len(polls))
	}
}

// --- Addendum B -------------------------------------------------------
//
// StartOrNext refuses to continue a session whose question_no has already
// reached total_questions. StartGame had no equivalent guard: because
// question_no != 0 it skips the "new game" intro branch and falls straight
// into sendNextQuestion, so /quiz on a finished-but-still-active session
// (e.g. after a crash that never ran finishGame) would ask question 11,
// 12, ... forever.
func TestStartGameOnFinishedSessionStartsFreshRatherThanOverrunning(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	one := 1
	setQuizLimit(t, pool, limitKeyQuizQuestions, &one)

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	// First game: one question, sent and never finished (simulates a lost
	// timer / crashed process — finishGame never runs).
	if err := svc.StartGame(ctx, -8500, 61, "supergroup"); err != nil {
		t.Fatal(err)
	}
	firstSession, err := q.GetActiveQuizSessionByChat(ctx, -8500)
	if err != nil {
		t.Fatal(err)
	}
	if firstSession.QuestionNo != firstSession.TotalQuestions {
		t.Fatalf("setup: session should already be at its last question, got %+v", firstSession)
	}

	// /quiz again on the same, still-active, finished session.
	if err := svc.StartGame(ctx, -8500, 61, "supergroup"); err != nil {
		t.Fatal(err)
	}

	polls := rec.methodCalls("sendPoll")
	if len(polls) != 2 {
		t.Fatalf("want exactly 2 polls (game 1's only question + a fresh game's first question), got %d", len(polls))
	}

	session, err := q.GetActiveQuizSessionByChat(ctx, -8500)
	if err != nil {
		t.Fatal(err)
	}
	if session.QuestionNo > session.TotalQuestions {
		t.Fatalf("session overran total_questions: %+v", session)
	}
	if session.ID == firstSession.ID {
		t.Fatalf("expected a fresh session to replace the finished one, got the same id")
	}
}

// scheduleAdvance is what lets the game move on without the room having to
// tap "Keyingi savol" — it must fire once per question, for the right chat,
// after the poll's own open_period plus the 1s grace the plan specifies.
func TestSendNextQuestionSchedulesAdvance(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	ten := 10
	setQuizLimit(t, pool, limitKeyQuizSeconds, &ten)

	_, client := newRecordingTelegram(t)
	var calls int
	var gotChat int64
	var gotAfter time.Duration
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test",
		Advance: func(chatID int64, after time.Duration) {
			calls++
			gotChat = chatID
			gotAfter = after
		},
	}

	if err := svc.StartGame(ctx, -8600, 71, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("want Advance scheduled exactly once, got %d", calls)
	}
	if gotChat != -8600 {
		t.Fatalf("Advance chatID = %d, want -8600", gotChat)
	}
	if gotAfter != 11*time.Second {
		t.Fatalf("Advance delay = %v, want 11s (quizSeconds=10 + 1s grace)", gotAfter)
	}
}

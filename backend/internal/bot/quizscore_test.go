package bot

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// answerN records n answers for one participant, correct of which were right,
// each taking msEach milliseconds.
func answerN(t *testing.T, q *sqlc.Queries, sessionID uuid.UUID, tgUserID int64, name string, n, correct int, msEach int64) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		delta := int32(0)
		if i < correct {
			delta = 1
		}
		if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
			SessionID: sessionID, TgUserID: tgUserID, DisplayName: name,
			CorrectDelta: delta, ElapsedMs: msEach,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// Ranking must order by correct_count first and break ties by the faster
// average — not by insertion order, which is what an unordered query returns.
func TestListQuizRankingOrdersByCorrectThenSpeed(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7001, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// "slow" is inserted first so a query that forgets ORDER BY returns it
	// first and fails loudly.
	answerN(t, q, session.ID, 101, "slow", 3, 3, 10000) // 3/3, 10.0s avg
	answerN(t, q, session.ID, 102, "fast", 3, 3, 3000)  // 3/3,  3.0s avg
	answerN(t, q, session.ID, 103, "weak", 3, 1, 1000)  // 1/3,  1.0s avg

	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 participants, got %d", len(rows))
	}
	if rows[0].DisplayName != "fast" {
		t.Fatalf("want fast first (same score, quicker average), got %q", rows[0].DisplayName)
	}
	if rows[1].DisplayName != "slow" {
		t.Fatalf("want slow second, got %q", rows[1].DisplayName)
	}
	// Being quickest must not outrank being right.
	if rows[2].DisplayName != "weak" {
		t.Fatalf("want weak last despite the fastest average, got %q", rows[2].DisplayName)
	}
	if rows[0].AnsweredCount != 3 || rows[0].CorrectCount != 3 || rows[0].TotalMs != 9000 {
		t.Fatalf("upsert did not accumulate: %+v", rows[0])
	}
}

// A second poll_answer for the same (session, user) must accumulate, never
// insert a duplicate row.
func TestUpsertQuizParticipantAccumulates(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7002, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
			SessionID: session.ID, TgUserID: 555, DisplayName: "Aziz",
			CorrectDelta: 1, ElapsedMs: 2000,
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].AnsweredCount != 2 || rows[0].CorrectCount != 2 || rows[0].TotalMs != 4000 {
		t.Fatalf("accumulation wrong: %+v", rows[0])
	}
}

// A later vote arriving without a name must not blank out the name we
// already have — the ranking would lose the label mid-game.
func TestUpsertQuizParticipantKeepsExistingName(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7004, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
		SessionID: session.ID, TgUserID: 777, DisplayName: "Nodira",
		CorrectDelta: 1, ElapsedMs: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
		SessionID: session.ID, TgUserID: 777, DisplayName: "",
		CorrectDelta: 0, ElapsedMs: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	rows, _ := q.ListQuizRanking(ctx, session.ID)
	if len(rows) != 1 || rows[0].DisplayName != "Nodira" {
		t.Fatalf("name was overwritten by an empty one: %+v", rows)
	}
}

// The poll registry is what maps an inbound poll_answer back to a question.
func TestQuizPollRoundTrip(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7003, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateQuizPoll(ctx, sqlc.CreateQuizPollParams{
		PollID: "poll-abc", SessionID: session.ID,
		QuestionID: uuid.NullUUID{}, QuestionNo: 1, CorrectIdx: 2,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetQuizPoll(ctx, "poll-abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.CorrectIdx != 2 || got.QuestionNo != 1 || got.SessionID != session.ID {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if got.Closed {
		t.Fatal("new poll should not be closed")
	}
	if err := q.CloseQuizPoll(ctx, "poll-abc"); err != nil {
		t.Fatal(err)
	}
	got, _ = q.GetQuizPoll(ctx, "poll-abc")
	if !got.Closed {
		t.Fatal("CloseQuizPoll did not stick")
	}
}

// New sessions must carry the defaults the game loop reads; a zero
// total_questions would end every game before the first question.
func TestCreateQuizSessionHasMultiplayerDefaults(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7005, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.TotalQuestions != 10 || session.QuestionNo != 0 || session.Mode != "solo" {
		t.Fatalf("defaults wrong: total=%d no=%d mode=%q",
			session.TotalQuestions, session.QuestionNo, session.Mode)
	}

	// The read path must surface them too — it selects columns by name.
	got, err := q.GetActiveQuizSessionByChat(ctx, -7005)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalQuestions != 10 || got.Mode != "solo" {
		t.Fatalf("read path lost the new columns: %+v", got)
	}
}

// Deleting a session must take its poll and participant rows with it.
func TestSessionDeleteCascades(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7006, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateQuizPoll(ctx, sqlc.CreateQuizPollParams{
		PollID: "poll-cascade", SessionID: session.ID,
		QuestionID: uuid.NullUUID{}, QuestionNo: 1, CorrectIdx: 0,
	}); err != nil {
		t.Fatal(err)
	}
	answerN(t, q, session.ID, 909, "Aziz", 1, 1, 1000)

	if _, err := pool.Exec(ctx,
		`DELETE FROM telegram_quiz_session WHERE id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}

	var polls, participants int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM telegram_quiz_poll WHERE session_id = $1`,
		session.ID).Scan(&polls); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM telegram_quiz_participant WHERE session_id = $1`,
		session.ID).Scan(&participants); err != nil {
		t.Fatal(err)
	}
	if polls != 0 || participants != 0 {
		t.Fatalf("cascade left orphans: %d polls, %d participants", polls, participants)
	}
}

// Two different Telegram users answering the same poll must produce two
// separate rows. Before polls, the first tap answered for the whole chat.
func TestHandlePollAnswerScoresEachUserSeparately(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8100, 31, "supergroup"); err != nil {
		t.Fatal(err)
	}

	// correct index is 0 (seed marks the first answer correct)
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 401, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 402, FirstName: "Malika"}, OptionIDs: []int{1},
	}); err != nil {
		t.Fatal(err)
	}

	session, _ := q.GetActiveQuizSessionByChat(ctx, -8100)
	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 participants, got %d", len(rows))
	}
	if rows[0].DisplayName != "Aziz" || rows[0].CorrectCount != 1 {
		t.Fatalf("winner row = %+v", rows[0])
	}
	if rows[1].DisplayName != "Malika" || rows[1].CorrectCount != 0 {
		t.Fatalf("second row = %+v", rows[1])
	}
}

// A poll_answer for a poll we never recorded (an old game, another bot run)
// must be ignored rather than error the webhook.
func TestHandlePollAnswerIgnoresUnknownPoll(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client}

	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "does-not-exist", User: User{ID: 1}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatalf("unknown poll must be ignored, got %v", err)
	}
}

// Retracting a vote sends option_ids: [] — it must not count as an answer.
func TestHandlePollAnswerIgnoresEmptyVote(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8101, 32, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 501, FirstName: "Bekzod"}, OptionIDs: []int{},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8101)
	rows, _ := q.ListQuizRanking(ctx, session.ID)
	if len(rows) != 0 {
		t.Fatalf("retracted vote created a participant: %+v", rows)
	}
}

// A user with no first name still needs a label in the ranking.
func TestHandlePollAnswerFallsBackToUsername(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8102, 33, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 601, Username: "nodira_u"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8102)
	rows, _ := q.ListQuizRanking(ctx, session.ID)
	if len(rows) != 1 || rows[0].DisplayName != "nodira_u" {
		t.Fatalf("display name fallback failed: %+v", rows)
	}
}

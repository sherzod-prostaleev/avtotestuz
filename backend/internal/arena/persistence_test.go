package arena

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func persistenceFixture(t *testing.T) (*Service, *sqlc.Queries, *Match) {
	return persistenceFixtureMode(t, ModeRanked)
}

func persistenceFixtureMode(t *testing.T, mode string) (*Service, *sqlc.Queries, *Match) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(ctx, pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	a, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901111111"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998902222222"})
	if err != nil {
		t.Fatal(err)
	}
	qids, err := q.RandomQuestionIDs(ctx, 2)
	if err != nil || len(qids) != 2 {
		t.Fatalf("questions=%d err=%v", len(qids), err)
	}
	correct := make(map[uuid.UUID]uuid.UUID, len(qids))
	for _, qid := range qids {
		correct[qid], err = q.GetCorrectAnswerID(ctx, qid)
		if err != nil {
			t.Fatal(err)
		}
	}
	row, err := q.InsertArenaMatch(ctx, sqlc.InsertArenaMatchParams{
		QuestionIds: qids, QuestionTimeSec: 20, Mode: mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	rdb := redisx.NewTest(t)
	l := learning.NewService(q)
	p := progress.NewService(q)
	p.Learning = l
	svc := &Service{
		Q: q, Pool: pool, R: rdb, Learning: l, Progress: p,
		Rating: FixedRating{Value: 1000}, Hub: NewHub(), Log: zap.NewNop(),
		Now: time.Now, matches: make(map[uuid.UUID]*Match),
	}
	specB := SeatSpec{ID: b.ID, Locale: "uz-Latn", Card: PlayerCard{Name: "B", Rating: 1000}}
	if mode == ModeBot {
		// The bot has no profile: a random id that must never reach a FK.
		specB = SeatSpec{ID: uuid.New(), Locale: "uz-Latn", Bot: true, Card: PlayerCard{Name: BotName, Rating: 1000, Bot: true}}
	}
	m := NewMatch(svc, row.ID, mode,
		SeatSpec{ID: a.ID, Locale: "uz-Latn", Card: PlayerCard{Name: "A", Rating: 1000}}, specB,
		qids, nil, correct)
	m.endReason = "completed"
	m.seats[0].score = 100
	m.seats[0].correctN = 1
	m.seats[0].state = seatDone
	m.seats[1].state = seatDone
	m.seats[0].answers[0] = playerAnswer{
		answered: true, answerID: correct[qids[0]], correct: true,
		responseMs: 500, points: 100, at: time.Now(),
	}
	svc.matches[m.id] = m
	svc.Hub.SetMatch(a.ID, m.id)
	svc.Hub.SetMatch(m.b, m.id)
	return svc, q, m
}

func TestFinishPersistRollsBackEveryArenaWriteOnFailure(t *testing.T) {
	svc, q, m := persistenceFixture(t)
	ctx := context.Background()
	_, err := svc.Pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION arena_test_fail_answer() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected arena answer failure'; END $$;
		CREATE TRIGGER arena_test_fail_answer_trigger
		BEFORE INSERT ON arena_answer
		FOR EACH ROW EXECUTE FUNCTION arena_test_fail_answer()`)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.FinishPersist(ctx, m, 0, 0); err == nil {
		t.Fatal("FinishPersist succeeded despite injected answer failure")
	}
	row, err := q.GetArenaMatchPlayer(ctx, sqlc.GetArenaMatchPlayerParams{MatchID: m.id, ProfileID: m.a})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("player write survived rollback: row=%+v err=%v", row, err)
	}
	var status string
	if err := svc.Pool.QueryRow(ctx, `SELECT status FROM arena_match WHERE id=$1`, m.id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "in_progress" {
		t.Fatalf("match status=%q survived rollback", status)
	}

	if _, err := svc.Pool.Exec(ctx, `
		DROP TRIGGER arena_test_fail_answer_trigger ON arena_answer;
		DROP FUNCTION arena_test_fail_answer()`); err != nil {
		t.Fatal(err)
	}
	if err := svc.FinishPersist(ctx, m, 0, 0); err != nil {
		t.Fatalf("retry after transient failure: %v", err)
	}
	if _, err := q.GetArenaMatchPlayer(ctx, sqlc.GetArenaMatchPlayerParams{MatchID: m.id, ProfileID: m.a}); err != nil {
		t.Fatalf("player missing after successful retry: %v", err)
	}
	if err := svc.FinishPersist(ctx, m, 0, 0); err != nil {
		t.Fatalf("committed persistence must be idempotent: %v", err)
	}
}

func TestFinishPersistBotDuelWritesOnlyTheHuman(t *testing.T) {
	svc, q, m := persistenceFixtureMode(t, ModeBot)
	ctx := context.Background()
	if err := svc.FinishPersist(ctx, m, 0, 0); err != nil {
		t.Fatalf("bot duel persistence: %v", err)
	}
	var players, answers int
	if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM arena_match_player WHERE match_id=$1`, m.id).Scan(&players); err != nil {
		t.Fatal(err)
	}
	if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM arena_answer WHERE match_id=$1`, m.id).Scan(&answers); err != nil {
		t.Fatal(err)
	}
	if players != 1 || answers != len(m.questions) {
		t.Fatalf("players=%d answers=%d, want 1 and %d", players, answers, len(m.questions))
	}
	hist, err := q.ListArenaMatchesForProfile(ctx, sqlc.ListArenaMatchesForProfileParams{ProfileID: m.a, Limit: 5})
	if err != nil || len(hist) != 1 {
		t.Fatalf("history=%v err=%v", hist, err)
	}
	if hist[0].Mode != ModeBot || hist[0].OpponentID.Valid || hist[0].Outcome.String != "won" {
		t.Fatalf("bot history row %+v", hist[0])
	}
	if hist[0].RatingDelta.Int32 != 0 {
		t.Fatalf("bot duel moved rating: %+v", hist[0])
	}
	if svc.Hub.InMatch(m.a) {
		t.Fatal("human still marked in match after persistence")
	}
}

func TestHistoryNamesTheHumanOpponent(t *testing.T) {
	svc, q, m := persistenceFixture(t)
	ctx := context.Background()
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET name='Dilnoza' WHERE id=$1`, m.b); err != nil {
		t.Fatal(err)
	}
	if err := svc.FinishPersist(ctx, m, 12, -12); err != nil {
		t.Fatal(err)
	}
	hist, err := q.ListArenaMatchesForProfile(ctx, sqlc.ListArenaMatchesForProfileParams{ProfileID: m.a, Limit: 5})
	if err != nil || len(hist) != 1 {
		t.Fatalf("history=%v err=%v", hist, err)
	}
	if !hist[0].OpponentID.Valid || hist[0].OpponentID.UUID != m.b || hist[0].OpponentName.String != "Dilnoza" {
		t.Fatalf("opponent not named: %+v", hist[0])
	}
	if hist[0].Mode != ModeRanked || hist[0].RatingDelta.Int32 != 12 {
		t.Fatalf("ranked row %+v", hist[0])
	}
}

// TestBotDuelRunsToTheEndThroughTheRealLoop drives Run's goroutine with real
// timers (shortened): the human answers every question as it arrives, the bot
// plays on its own clock, and the match must end with one match.end and a
// persisted result — no stalled timer, no double finish.
func TestBotDuelRunsToTheEndThroughTheRealLoop(t *testing.T) {
	svc, _, m := persistenceFixtureMode(t, ModeBot)
	for _, st := range m.seats {
		st.state, st.score, st.correctN = "", 0, 0
		st.answers = make([]playerAnswer, len(m.questions))
	}
	m.endReason = ""
	m.countdown, m.qTime, m.revealFor, m.grace = 20*time.Millisecond, 400*time.Millisecond, 20*time.Millisecond, 50*time.Millisecond
	conn := &Conn{ProfileID: m.a, out: make(chan []byte, 256)}
	svc.Hub.conns[m.a] = conn
	m.Start()
	go m.Run()

	deadline := time.After(10 * time.Second)
	var questions int
	for {
		select {
		case payload := <-conn.out:
			env, err := Decode(payload)
			if err != nil {
				t.Fatal(err)
			}
			switch env.T {
			case "question":
				questions++
				var qd QuestionData
				_ = json.Unmarshal(env.D, &qd)
				if !m.SubmitAnswer(m.a, qd.Index, m.correct[m.questions[qd.Index]]) {
					t.Fatal("answer not queued")
				}
			case "match.end":
				var end MatchEndData
				_ = json.Unmarshal(env.D, &end)
				if questions != len(m.questions) || end.Correct.You != len(m.questions) || end.Rated {
					t.Fatalf("questions=%d end=%+v", questions, end)
				}
				if len(end.Marks.You) != len(m.questions) || len(end.Marks.Opponent) != len(m.questions) {
					t.Fatalf("marks %+v", end.Marks)
				}
				<-m.done
				var status string
				if err := svc.Pool.QueryRow(context.Background(), `SELECT status FROM arena_match WHERE id=$1`, m.id).Scan(&status); err != nil || status != "finished" {
					t.Fatalf("status=%q err=%v", status, err)
				}
				return
			}
		case <-deadline:
			t.Fatalf("duel stalled after %d questions", questions)
		}
	}
}

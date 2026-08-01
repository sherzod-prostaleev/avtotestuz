package arena

import (
	"context"
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
		QuestionIds: qids, QuestionTimeSec: 20,
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
	m := NewMatch(svc, row.ID, a.ID, b.ID, "uz-Latn", "uz-Latn", qids, correct)
	m.endReason = "completed"
	m.score[a.ID] = 100
	m.correctN[a.ID] = 1
	m.answers[a.ID][0] = playerAnswer{
		answered: true, answerID: correct[qids[0]], correct: true,
		responseMs: 500, points: 100, at: time.Now(),
	}
	svc.matches[m.id] = m
	svc.Hub.SetMatch(a.ID, m.id)
	svc.Hub.SetMatch(b.ID, m.id)
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

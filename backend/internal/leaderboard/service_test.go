package leaderboard_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func newTestService(t *testing.T) (*leaderboard.Service, *sqlc.Queries) {
	t.Helper()
	pool := testdb.New(t)
	rdb := redisx.NewTest(t)
	q := sqlc.New(pool)
	return leaderboard.NewService(rdb, q, billing.Service{Q: q}), q
}

func createProfile(t *testing.T, q *sqlc.Queries, phone string) uuid.UUID {
	t.Helper()
	p, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: phone})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p.ID
}

func TestRecordPointCreditsAllFourPeriods(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111101")

	if err := svc.RecordPoint(ctx, profileID); err != nil {
		t.Fatalf("RecordPoint: %v", err)
	}

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard(daily): %v", err)
	}
	if res.YouScore != 1 {
		t.Errorf("daily YouScore = %d, want 1", res.YouScore)
	}
	for _, p := range []leaderboard.Period{leaderboard.PeriodWeekly, leaderboard.PeriodMonthly, leaderboard.PeriodAllTime} {
		res, err := svc.GetLeaderboard(ctx, profileID, p)
		if err != nil {
			t.Fatalf("GetLeaderboard(%s): %v", p, err)
		}
		if res.YouScore != 1 {
			t.Errorf("%s YouScore = %d, want 1", p, res.YouScore)
		}
	}
}

func TestRecordPointAccumulates(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111102")

	for i := 0; i < 5; i++ {
		if err := svc.RecordPoint(ctx, profileID); err != nil {
			t.Fatalf("RecordPoint #%d: %v", i, err)
		}
	}

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodAllTime)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouScore != 5 {
		t.Errorf("YouScore = %d, want 5", res.YouScore)
	}
}

func TestRecordPointStopsAtFreeDailyCap(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111103")

	// Free daily cap is 30 (migration 0017). Record 35 correct answers;
	// only 30 should count.
	for i := 0; i < 35; i++ {
		if err := svc.RecordPoint(ctx, profileID); err != nil {
			t.Fatalf("RecordPoint #%d: %v", i, err)
		}
	}

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouScore != 30 {
		t.Errorf("YouScore = %d, want 30 (capped)", res.YouScore)
	}
}

func TestRecordPointVIPGetsHigherDailyCap(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111104")
	billingSvc := billing.Service{Q: q}
	if _, err := billingSvc.GrantDays(ctx, profileID, 7, "admin", "test", uuid.NullUUID{}); err != nil {
		t.Fatalf("grant vip: %v", err)
	}

	for i := 0; i < 35; i++ {
		if err := svc.RecordPoint(ctx, profileID); err != nil {
			t.Fatalf("RecordPoint #%d: %v", i, err)
		}
	}

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	// VIP cap is 100 (migration 0017); 35 answers is under it, so all count.
	if res.YouScore != 35 {
		t.Errorf("YouScore = %d, want 35 (VIP, under cap)", res.YouScore)
	}
}

func TestGetLeaderboardTopRanksHighestFirst(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	low := createProfile(t, q, "+998901111105")
	high := createProfile(t, q, "+998901111106")

	for i := 0; i < 2; i++ {
		_ = svc.RecordPoint(ctx, low)
	}
	for i := 0; i < 5; i++ {
		_ = svc.RecordPoint(ctx, high)
	}

	res, err := svc.GetLeaderboard(ctx, low, leaderboard.PeriodAllTime)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(res.Top) != 2 {
		t.Fatalf("len(Top) = %d, want 2", len(res.Top))
	}
	if res.Top[0].Score != 5 || res.Top[0].Rank != 1 {
		t.Errorf("Top[0] = %+v, want Score=5 Rank=1", res.Top[0])
	}
	if res.Top[1].Score != 2 || res.Top[1].Rank != 2 {
		t.Errorf("Top[1] = %+v, want Score=2 Rank=2", res.Top[1])
	}
}

func TestGetLeaderboardYouRankNilWhenNoScore(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111107")

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouRank != nil {
		t.Errorf("YouRank = %v, want nil", *res.YouRank)
	}
	if res.YouScore != 0 {
		t.Errorf("YouScore = %d, want 0", res.YouScore)
	}
}

func TestGetLeaderboardAroundYouOmittedWhenInTop(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111108")
	_ = svc.RecordPoint(ctx, profileID)

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(res.AroundYou) != 0 {
		t.Errorf("AroundYou = %+v, want empty (profile is in Top)", res.AroundYou)
	}
}

func TestGetLeaderboardAroundYouPopulatedWhenOutsideTop(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	// 12 profiles, each with a distinct score 12..1, so the 11th- and
	// 12th-highest scorers fall outside TopN (10).
	var ids []uuid.UUID
	for i := 0; i < 12; i++ {
		id := createProfile(t, q, fmt.Sprintf("+998901112%03d", i))
		ids = append(ids, id)
		for j := 0; j < 12-i; j++ {
			_ = svc.RecordPoint(ctx, id)
		}
	}
	last := ids[11] // lowest score (1 point), rank 12

	res, err := svc.GetLeaderboard(ctx, last, leaderboard.PeriodAllTime)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouRank == nil || *res.YouRank != 12 {
		t.Fatalf("YouRank = %v, want 12", res.YouRank)
	}
	if len(res.AroundYou) == 0 {
		t.Fatal("AroundYou is empty, want the rank-10..12 neighborhood")
	}
	foundSelf := false
	for _, e := range res.AroundYou {
		if e.Rank == 12 {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Errorf("AroundYou = %+v, missing the caller's own rank-12 entry", res.AroundYou)
	}
}

func TestGetLeaderboardResolvesProfileName(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111109")
	// UpdateProfileMe (backend/internal/db/queries/auth.sql) is the only
	// existing write path for profile.name and requires every column —
	// pass the same defaults CreateProfile leaves in place for the ones
	// this test doesn't care about.
	if _, err := q.UpdateProfileMe(ctx, sqlc.UpdateProfileMeParams{
		ID: profileID, Name: "Aziz Karimov", Region: "", District: "",
		LocalePref: "uz-Latn", ThemePref: "dark",
	}); err != nil {
		t.Fatalf("set name: %v", err)
	}
	_ = svc.RecordPoint(ctx, profileID)

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouName != "Aziz Karimov" {
		t.Errorf("YouName = %q, want %q", res.YouName, "Aziz Karimov")
	}
	if res.Top[0].Name != "Aziz Karimov" {
		t.Errorf("Top[0].Name = %q, want %q", res.Top[0].Name, "Aziz Karimov")
	}
}

func TestRebuildPeriodReconstructsFromPostgres(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111110")

	for i := 0; i < 4; i++ {
		if err := svc.RecordPoint(ctx, profileID); err != nil {
			t.Fatalf("RecordPoint #%d: %v", i, err)
		}
	}
	// This test only verifies RebuildPeriod's *query shape* compiles and
	// runs without error against real data; it does not assert score
	// equality against session_answer directly, because RecordPoint in
	// this package increments Redis only — session_answer rows are written
	// by internal/session.Service.SubmitAnswer (Task 6), a different
	// package this test does not depend on. The full
	// SubmitAnswer -> session_answer -> RebuildPeriod round trip is
	// covered by Task 6's integration test instead.
	now := time.Now().UTC()
	if err := svc.RebuildPeriod(ctx, leaderboard.PeriodDaily, now); err != nil {
		t.Fatalf("RebuildPeriod: %v", err)
	}
}

func TestRebuildPeriodAppliesDailyCap(t *testing.T) {
	svc, q := newTestService(t)
	// testdb.New truncates all app tables unconditionally on every call (see
	// internal/testdb/testdb.go — no per-test caching or idempotency), so
	// the pool must be captured here, before any data is inserted below,
	// and reused rather than re-fetched later in the test (a second call
	// after createProfile/CreateExamSession would truncate the very rows
	// this test just created and break the FK inserts that follow).
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901111111")

	// Seed a category/question/correct-answer directly via raw SQL: unlike
	// internal/session and internal/learning tests, this package's
	// newTestService does not load the fixture dataset, so there is no
	// pre-existing "signs" category to look up via GetCategoryIDByCode.
	var categoryID, questionID, correctAnswerID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO category (code) VALUES ('leaderboard-cap-test') RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash)
		VALUES ('leaderboard-cap-test-q', $1, 'leaderboard-cap-test-h') RETURNING id`, categoryID).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO answer (question_id, position, is_correct) VALUES ($1, 1, true) RETURNING id`,
		questionID,
	).Scan(&correctAnswerID); err != nil {
		t.Fatalf("insert answer: %v", err)
	}

	// Seed 35 correct answers "today" via 35 separate single-question
	// sessions — more than the free daily cap (30) — bypassing RecordPoint
	// entirely so Postgres has uncapped history, exactly like a real
	// profile that exceeded its cap on the live board before Redis was
	// lost. 35 separate sessions (not 35 session_answer rows in one
	// session) because session_answer's PRIMARY KEY is
	// (session_id, question_id) — one row per question per session, so a
	// single session can't hold 35 answers to the same question.
	for i := 0; i < 35; i++ {
		session, err := q.CreateExamSession(ctx, sqlc.CreateExamSessionParams{
			ProfileID: profileID, Mode: "practice", Locale: "uz-Latn",
			QuestionIds: []uuid.UUID{questionID},
		})
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position, answered_at)
			 VALUES ($1, $2, $3, true, 0, now())`,
			session.ID, questionID, correctAnswerID,
		); err != nil {
			t.Fatalf("seed answer %d: %v", i, err)
		}
	}

	now := time.Now().UTC()
	if err := svc.RebuildPeriod(ctx, leaderboard.PeriodDaily, now); err != nil {
		t.Fatalf("RebuildPeriod: %v", err)
	}

	res, err := svc.GetLeaderboard(ctx, profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouScore != 30 {
		t.Errorf("YouScore after rebuild = %d, want 30 (capped, not 35 uncapped)", res.YouScore)
	}
}

// TestRebuildPeriodPurgesStaleRedisMembers reproduces the reconciliation gap
// found in code review: RebuildPeriod must fully reconstruct the Redis key
// from Postgres, which means any member present in Redis but absent from
// the fresh Postgres result (stale data, drift, corruption) must be removed
// — not merely left with its old, now-untrustworthy score.
func TestRebuildPeriodPurgesStaleRedisMembers(t *testing.T) {
	svc, q := newTestService(t)
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now().UTC()

	profileID := createProfile(t, q, "+998901111111")

	// Seed session_answer directly via raw SQL (RecordPoint only writes to
	// Redis, never to session_answer — see internal/session.Service.
	// SubmitAnswer for the real write path) so RebuildPeriod's Postgres
	// query has real rows to reconstruct from. Two correct answers within
	// today's window for profileID.
	var categoryID, questionID, answerID, sessionID string
	if err := pool.QueryRow(ctx, `INSERT INTO category (code) VALUES ('rebuild-stale') RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash)
		VALUES ('rebuild-stale-q', $1, 'rebuild-stale-h') RETURNING id`, categoryID).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO answer (question_id, position) VALUES ($1, 1) RETURNING id`, questionID).Scan(&answerID); err != nil {
		t.Fatalf("insert answer: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO exam_session (profile_id, mode, locale, total)
		VALUES ($1, 'practice', 'uz-Latn', 1) RETURNING id`, profileID).Scan(&sessionID); err != nil {
		t.Fatalf("insert exam_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_question (session_id, question_id, position)
		VALUES ($1, $2, 1)`, sessionID, questionID); err != nil {
		t.Fatalf("insert session_question: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position, answered_at)
		VALUES ($1, $2, $3, true, 1, $4)`, sessionID, questionID, answerID, now); err != nil {
		t.Fatalf("insert session_answer: %v", err)
	}

	key := leaderboard.RedisKey(leaderboard.PeriodDaily, now)
	staleMember := "00000000-0000-4000-8000-000000000abc" // syntactically valid UUID, not in Postgres
	if err := svc.Redis.ZAdd(ctx, key, redis.Z{Score: 999, Member: staleMember}).Err(); err != nil {
		t.Fatalf("seed stale redis member: %v", err)
	}

	if err := svc.RebuildPeriod(ctx, leaderboard.PeriodDaily, now); err != nil {
		t.Fatalf("RebuildPeriod: %v", err)
	}

	if _, err := svc.Redis.ZScore(ctx, key, staleMember).Result(); !errors.Is(err, redis.Nil) {
		t.Errorf("stale member ZScore error = %v, want redis.Nil (member should be purged)", err)
	}

	gotScore, err := svc.Redis.ZScore(ctx, key, profileID.String()).Result()
	if err != nil {
		t.Fatalf("ZScore(profileID): %v", err)
	}
	if got := leaderboard.DecodePoints(gotScore); got != 1 {
		t.Errorf("profile correct count = %d, want 1", got)
	}
}

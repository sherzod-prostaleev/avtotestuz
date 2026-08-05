package leaderboard_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// A classroom PC racks up correct answers all day. It must never appear on a
// learner leaderboard.
func TestStationProfilesAreNotRanked(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	ctx := context.Background()

	var stationProfile uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), 'PC-1', 'station') RETURNING id`).Scan(&stationProfile); err != nil {
		t.Fatal(err)
	}
	seedCorrectAnswers(t, pool, stationProfile, 5)

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(time.Hour)
	rows, err := q.CountCorrectAnswersByProfileInRange(ctx, sqlc.CountCorrectAnswersByProfileInRangeParams{
		FromTs: pgTimestamptz(from), ToTs: pgTimestamptz(to),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ProfileID == stationProfile {
			t.Fatal("station profile leaked into the leaderboard aggregate")
		}
	}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// seedCorrectAnswers gives profileID n correct answers in a finished practice
// session, building the minimum content rows the FKs demand.
func seedCorrectAnswers(t *testing.T, pool *pgxpool.Pool, profileID uuid.UUID, n int) {
	t.Helper()
	ctx := context.Background()

	var categoryID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code) VALUES ('cat-' || gen_random_uuid()) RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}

	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO exam_session (profile_id, mode, locale, total, status)
		VALUES ($1, 'practice', 'uz-Latn', $2, 'passed') RETURNING id`,
		profileID, n).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < n; i++ {
		var questionID, answerID uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO question (source_ext_id, category_id, content_hash)
			VALUES ('q-' || gen_random_uuid(), $1, 'hash') RETURNING id`,
			categoryID).Scan(&questionID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO answer (question_id, position, is_correct)
			VALUES ($1, 1, true) RETURNING id`, questionID).Scan(&answerID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			UPDATE question SET correct_answer_id = $2 WHERE id = $1`, questionID, answerID); err != nil {
			t.Fatal(err)
		}
		// session_answer carries a composite FK to session_question (added in
		// migration 0007), so the join row must exist before the answer row.
		// position must be distinct per session (UNIQUE(session_id, position)).
		if _, err := pool.Exec(ctx, `
			INSERT INTO session_question (session_id, question_id, position)
			VALUES ($1, $2, $3)`, sessionID, questionID, i+1); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
			VALUES ($1, $2, $3, true, 1)`, sessionID, questionID, answerID); err != nil {
			t.Fatal(err)
		}
	}
}

package session_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

// seedOrdered is seed() with the pool handed back, because these tests need to
// insert questions with production-shaped source ids and to read the cursor
// table directly.
func seedOrdered(t *testing.T) (*pgxpool.Pool, *sqlc.Queries, *session.Service, uuid.UUID) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	svc := session.NewService(q, pool, billing.Service{Q: q}, learning.NewService(q), progress.NewService(q))
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return pool, q, svc, profile.ID
}

// categoryIDByCode looks a fixture category up by its code.
func categoryIDByCode(t *testing.T, q *sqlc.Queries, code string) uuid.UUID {
	t.Helper()
	id, err := q.GetCategoryIDByCode(context.Background(), code)
	if err != nil {
		t.Fatalf("category %q: %v", code, err)
	}
	return id
}

// cursorOf reads a profile's stored position in a topic. Absent means zero.
func cursorOf(t *testing.T, pool *pgxpool.Pool, profileID, categoryID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT COALESCE((SELECT next_index FROM practice_cursor
		                   WHERE profile_id = $1 AND category_id = $2), 0)`,
		profileID, categoryID).Scan(&n)
	if err != nil {
		t.Fatalf("read cursor: %v", err)
	}
	return n
}

// answerFirst answers the first n questions of a session, in the order the
// session serves them.
func answerFirst(t *testing.T, q *sqlc.Queries, svc *session.Service, profileID uuid.UUID, view session.SessionView, n int) {
	t.Helper()
	for i := 0; i < n && i < len(view.QuestionIDs); i++ {
		answerOne(t, q, svc, profileID, view.ID, view.QuestionIDs[i])
	}
}

func answerOne(t *testing.T, q *sqlc.Queries, svc *session.Service, profileID, sessionID, questionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	answerID, err := q.GetCorrectAnswerID(ctx, questionID)
	if err != nil {
		t.Fatalf("correct answer for %s: %v", questionID, err)
	}
	if _, err := svc.SubmitAnswer(ctx, profileID, sessionID, questionID, answerID,
		session.SubmitAnswerOpts{SkipFSRS: true}); err != nil {
		t.Fatalf("submit answer: %v", err)
	}
}

func startOrdered(t *testing.T, svc *session.Service, profileID, categoryID uuid.UUID, count int) session.SessionView {
	t.Helper()
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: categoryID, Count: count, Ordered: true,
	})
	if err != nil {
		t.Fatalf("start ordered session: %v", err)
	}
	return view
}

// TestOrderedDrawSortsByNumberNotByText is the trap this feature would
// otherwise fall into, and the fixture cannot catch it.
//
// Production ids are 'avtoimtihon-<N>' with no zero padding, so text order and
// number order disagree the moment N reaches 10: as text, 'avtoimtihon-100'
// comes before 'avtoimtihon-9'. The test fixture uses 'nmn-%04d', which is
// padded, so text and number order agree there and a plain ORDER BY
// source_ext_id would pass every fixture-based test and then serve a
// classroom its questions in the order 1, 10, 100, 11, 12.
//
// These rows are shaped like production's on purpose.
func TestOrderedDrawSortsByNumberNotByText(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()

	categoryID := categoryIDByCode(t, q, "signs")
	// Wipe the fixture's questions from this topic so only the numbers below
	// are in play.
	if _, err := pool.Exec(ctx, `DELETE FROM question WHERE category_id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	// Deliberately inserted out of order, and chosen so text sorting produces a
	// different sequence: as text this is 1, 10, 100, 2, 21, 3.
	numbers := []int{100, 3, 21, 1, 10, 2}
	for _, n := range numbers {
		if _, err := pool.Exec(ctx, `
			INSERT INTO question (source_ext_id, category_id, content_hash, source, validation_status)
			VALUES ($1, $2, $3, 'test', 'valid')`,
			fmt.Sprintf("avtoimtihon-%d", n), categoryID, fmt.Sprintf("hash-%d", n)); err != nil {
			t.Fatal(err)
		}
	}

	view := startOrdered(t, svc, profileID, categoryID, 100)
	got := extIDsOf(t, pool, view.QuestionIDs)
	want := []string{
		"avtoimtihon-1", "avtoimtihon-2", "avtoimtihon-3",
		"avtoimtihon-10", "avtoimtihon-21", "avtoimtihon-100",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d questions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d = %s, want %s\nfull order: %v", i+1, got[i], want[i], got)
		}
	}
}

// extIDsOf resolves question ids to their source ids, keeping the input order.
func extIDsOf(t *testing.T, pool *pgxpool.Pool, ids []uuid.UUID) []string {
	t.Helper()
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		var ext string
		if err := pool.QueryRow(context.Background(),
			`SELECT source_ext_id FROM question WHERE id = $1`, id).Scan(&ext); err != nil {
			t.Fatalf("ext id of %s: %v", id, err)
		}
		out = append(out, ext)
	}
	return out
}

// TestOrderedDrawIsTheSameSequenceEveryTime is the teacher's actual
// requirement: pick the topic, press "Hammasi", and get the same walk. The
// random path returns a different set on almost every call, so a passing run
// here means the ordered path really is ordered and not merely lucky.
func TestOrderedDrawIsTheSameSequenceEveryTime(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	categoryID := categoryIDByCode(t, q, "signs")

	first := startOrdered(t, svc, profileID, categoryID, 100)
	// No answers in between, so the cursor stays at 0 and the second draw must
	// repeat the first exactly.
	second := startOrdered(t, svc, profileID, categoryID, 100)

	if len(first.QuestionIDs) == 0 {
		t.Fatal("the topic drew no questions")
	}
	if len(first.QuestionIDs) != len(second.QuestionIDs) {
		t.Fatalf("draws differ in length: %d vs %d", len(first.QuestionIDs), len(second.QuestionIDs))
	}
	for i := range first.QuestionIDs {
		if first.QuestionIDs[i] != second.QuestionIDs[i] {
			t.Fatalf("position %d differs between draws", i+1)
		}
	}
	_ = pool
}

// TestOrderedPracticeResumesWhereTheClassStopped is the "continue from 123"
// requirement, at the scale the test bank allows.
func TestOrderedPracticeResumesWhereTheClassStopped(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID) // the daily free allowance would clamp the draw
	categoryID := categoryIDByCode(t, q, "signs")

	full := startOrdered(t, svc, profileID, categoryID, 100)
	total := len(full.QuestionIDs)
	if total < 6 {
		t.Fatalf("fixture topic has only %d questions; this test needs at least 6", total)
	}
	const done = 4
	answerFirst(t, q, svc, profileID, full, done)

	if got := cursorOf(t, pool, profileID, categoryID); got != done {
		t.Fatalf("cursor=%d after answering %d, want %d", got, done, done)
	}

	next := startOrdered(t, svc, profileID, categoryID, 100)
	if len(next.QuestionIDs) != total-done {
		t.Fatalf("resumed draw has %d questions, want the remaining %d", len(next.QuestionIDs), total-done)
	}
	// It must continue at the next question, not repeat one already done.
	if next.QuestionIDs[0] != full.QuestionIDs[done] {
		t.Fatalf("resumed at the wrong question: got %s, want %s", next.QuestionIDs[0], full.QuestionIDs[done])
	}
	for i, id := range next.QuestionIDs {
		if id != full.QuestionIDs[done+i] {
			t.Fatalf("resumed order diverges at position %d", i+1)
		}
	}
}

// TestOrderedCursorNeverRewinds: a student may answer questions out of order,
// and the class's position must only ever move forward -- and only over
// material actually covered. Going back to fill a gap in is what completes the
// run and lets the position move.
func TestOrderedCursorNeverRewinds(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	categoryID := categoryIDByCode(t, q, "signs")

	view := startOrdered(t, svc, profileID, categoryID, 100)
	if len(view.QuestionIDs) < 5 {
		t.Fatalf("fixture topic has only %d questions; this test needs at least 5", len(view.QuestionIDs))
	}
	answerFirst(t, q, svc, profileID, view, 3)
	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d after the first three, want 3", got)
	}

	// Skip the 4th and answer the 5th: nothing new is covered contiguously, so
	// the position holds -- it must not jump to 5 and it must not rewind.
	answerOne(t, q, svc, profileID, view.ID, view.QuestionIDs[4])
	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d after skipping ahead to the 5th, want it held at 3", got)
	}

	// Filling the 4th in closes the gap, and the run now reaches the 5th.
	answerOne(t, q, svc, profileID, view.ID, view.QuestionIDs[3])
	if got := cursorOf(t, pool, profileID, categoryID); got != 5 {
		t.Fatalf("cursor=%d after filling in the 4th, want 5", got)
	}
}

// TestAbandonedOrderedSessionLeavesThePositionAlone: the cursor moves on
// answers, never on session creation. A teacher who opens the topic and closes
// it again has not taught anything, and tomorrow must not skip a lesson's
// worth of questions.
func TestAbandonedOrderedSessionLeavesThePositionAlone(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	categoryID := categoryIDByCode(t, q, "signs")

	first := startOrdered(t, svc, profileID, categoryID, 100)
	if got := cursorOf(t, pool, profileID, categoryID); got != 0 {
		t.Fatalf("cursor=%d after merely starting, want 0", got)
	}
	second := startOrdered(t, svc, profileID, categoryID, 100)
	if len(second.QuestionIDs) != len(first.QuestionIDs) {
		t.Fatalf("second draw has %d questions, want the same %d", len(second.QuestionIDs), len(first.QuestionIDs))
	}
	if second.QuestionIDs[0] != first.QuestionIDs[0] {
		t.Fatal("an abandoned session moved the class forward")
	}
}

// TestOrderedPracticeWrapsAtTheEndOfTheTopic: finishing the topic starts it
// again, so the next group begins at question 1 with nobody resetting anything.
func TestOrderedPracticeWrapsAtTheEndOfTheTopic(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	categoryID := categoryIDByCode(t, q, "signs")

	full := startOrdered(t, svc, profileID, categoryID, 100)
	total := len(full.QuestionIDs)
	answerFirst(t, q, svc, profileID, full, total)
	if got := cursorOf(t, pool, profileID, categoryID); got != total {
		t.Fatalf("cursor=%d after finishing the topic, want %d", got, total)
	}

	// The progress report shows the wrap rather than a position past the end.
	items, err := svc.PracticeProgress(context.Background(), profileID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range items {
		if it.CategoryID == categoryID {
			found = true
			if it.NextIndex != 0 {
				t.Fatalf("progress next_index=%d at the end of the topic, want 0", it.NextIndex)
			}
			if it.Total != total {
				t.Fatalf("progress total=%d, want %d", it.Total, total)
			}
		}
	}
	if !found {
		t.Fatal("the finished topic is missing from the progress report")
	}

	again := startOrdered(t, svc, profileID, categoryID, 100)
	if len(again.QuestionIDs) != total {
		t.Fatalf("draw after the wrap has %d questions, want the whole topic (%d)", len(again.QuestionIDs), total)
	}
	if again.QuestionIDs[0] != full.QuestionIDs[0] {
		t.Fatal("the wrap did not start at the topic's first question")
	}
}

// TestForwardJumpCountsOnlyWhatWasActuallyDone is the difference between "how
// far down the list someone clicked" and "how much of the topic the class has
// worked through".
//
// The session screen renders one clickable chip per question with no gating
// (session/[id]/page.tsx), so a student can scroll to the end of a 337-question
// walk and answer the last one. Counting the furthest position answered would
// read that single click as "topic complete": the next draw would wrap, the
// stored position would be discarded, and the class's real place -- 123, say --
// would be gone with nothing on screen to recover it. The everyday version is
// quieter and worse: any forward jump silently buries everything skipped over.
//
// This is not theoretical. On production today 30 of 504 answered practice
// sessions already have gaps between the highest position answered and the
// number of answers.
//
// So the position is the contiguous run answered from the start of the
// session, not the maximum.
func TestForwardJumpCountsOnlyWhatWasActuallyDone(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	categoryID := categoryIDByCode(t, q, "signs")

	view := startOrdered(t, svc, profileID, categoryID, 100)
	total := len(view.QuestionIDs)
	if total < 5 {
		t.Fatalf("fixture topic has only %d questions; this test needs at least 5", total)
	}

	// Three from the start, then a jump to the very last question.
	answerFirst(t, q, svc, profileID, view, 3)
	answerOne(t, q, svc, profileID, view.ID, view.QuestionIDs[total-1])

	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d, want 3: answering the last chip is one question done, not a finished topic", got)
	}

	// The topic must not be treated as finished: the next draw continues at 4,
	// it does not wrap back to the beginning.
	next := startOrdered(t, svc, profileID, categoryID, 100)
	if next.QuestionIDs[0] != view.QuestionIDs[3] {
		t.Fatal("the walk restarted instead of continuing where the class actually got to")
	}
	if len(next.QuestionIDs) != total-3 {
		t.Fatalf("next draw has %d questions, want the remaining %d", len(next.QuestionIDs), total-3)
	}
}

// TestAnswersFromAnEarlierLapDoNotJumpTheClassForward guards the other end of
// the same idea: a session that belongs to a walk the class has already left
// behind must not drag the cursor back up to where that walk had reached.
//
// Practice sessions are left open routinely -- production holds 251 of them,
// the oldest from a month ago -- and the session history makes them reopenable.
// Without a guard, one answer in a stale session would write the old walk's
// position over the current one and skip everything in between.
func TestAnswersFromAnEarlierLapDoNotJumpTheClassForward(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	ctx := context.Background()
	categoryID := categoryIDByCode(t, q, "signs")

	// Walk most of the topic, leaving an open session positioned well down it.
	first := startOrdered(t, svc, profileID, categoryID, 100)
	total := len(first.QuestionIDs)
	if total < 6 {
		t.Fatalf("fixture topic has only %d questions; this test needs at least 6", total)
	}
	answerFirst(t, q, svc, profileID, first, total-2)

	stale := startOrdered(t, svc, profileID, categoryID, 100) // ordered_from = total-2
	staleFrom := total - 2

	// The teacher starts the topic over for a new group.
	if err := svc.ResetPracticeProgress(ctx, profileID, categoryID); err != nil {
		t.Fatal(err)
	}
	fresh := startOrdered(t, svc, profileID, categoryID, 100)
	answerFirst(t, q, svc, profileID, fresh, 2)
	if got := cursorOf(t, pool, profileID, categoryID); got != 2 {
		t.Fatalf("cursor=%d after the new group did two questions, want 2", got)
	}

	// Now somebody answers in the abandoned session from before the reset.
	answerOne(t, q, svc, profileID, stale.ID, stale.QuestionIDs[0])

	if got := cursorOf(t, pool, profileID, categoryID); got != 2 {
		t.Fatalf("cursor=%d, want it held at 2: an answer from the walk before the reset (ordered_from=%d) must not move the new group",
			got, staleFrom)
	}
}

// TestSecondLapAdvancesLikeTheFirst is the bug the wrap test above does not
// reach: it proves the topic can be started again, but not that the second lap
// makes progress.
//
// The cursor advances with GREATEST, so it can never go backwards -- which is
// what keeps an out-of-order answer from rewinding a class. That same GREATEST
// makes a wrap that is only computed at draw time useless: the stored cursor is
// still at 337, the draw starts at 0, and answering question 1 writes
// GREATEST(337, 1) = 337. The cursor never moves again, every later draw wraps
// to 0, and the class repeats questions 1..N for the rest of the topic's life
// without ever advancing.
//
// So the wrap has to be written down, not just applied in memory.
func TestSecondLapAdvancesLikeTheFirst(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	categoryID := categoryIDByCode(t, q, "signs")

	// Lap one, to the very end.
	first := startOrdered(t, svc, profileID, categoryID, 100)
	total := len(first.QuestionIDs)
	if total < 3 {
		t.Fatalf("fixture topic has only %d questions; this test needs at least 3", total)
	}
	answerFirst(t, q, svc, profileID, first, total)

	// Lap two: the wrap puts us back at question 1.
	second := startOrdered(t, svc, profileID, categoryID, 100)
	if len(second.QuestionIDs) != total {
		t.Fatalf("second lap drew %d questions, want the whole topic (%d)", len(second.QuestionIDs), total)
	}
	answerFirst(t, q, svc, profileID, second, 2)

	if got := cursorOf(t, pool, profileID, categoryID); got != 2 {
		t.Fatalf("cursor=%d after two answers on the second lap, want 2 -- the class is stuck repeating the topic", got)
	}

	// And the third draw must continue from there, not start over again.
	third := startOrdered(t, svc, profileID, categoryID, 100)
	if len(third.QuestionIDs) != total-2 {
		t.Fatalf("third draw has %d questions, want the remaining %d", len(third.QuestionIDs), total-2)
	}
	if third.QuestionIDs[0] != second.QuestionIDs[2] {
		t.Fatal("the second lap did not resume where it stopped")
	}
}

// TestResetPracticeProgressReturnsTheTopicToTheStart covers the "Boshidan
// boshlash" control, including on a topic nobody has started -- an operator
// should not have to know which case they are in.
func TestResetPracticeProgressReturnsTheTopicToTheStart(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	ctx := context.Background()
	categoryID := categoryIDByCode(t, q, "signs")

	view := startOrdered(t, svc, profileID, categoryID, 100)
	answerFirst(t, q, svc, profileID, view, 3)
	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d, want 3", got)
	}

	if err := svc.ResetPracticeProgress(ctx, profileID, categoryID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := cursorOf(t, pool, profileID, categoryID); got != 0 {
		t.Fatalf("cursor=%d after reset, want 0", got)
	}
	after := startOrdered(t, svc, profileID, categoryID, 100)
	if after.QuestionIDs[0] != view.QuestionIDs[0] {
		t.Fatal("the topic did not restart at its first question")
	}

	// A topic that was never started resets without complaint.
	untouched := categoryIDByCode(t, q, "rules")
	if err := svc.ResetPracticeProgress(ctx, profileID, untouched); err != nil {
		t.Fatalf("reset of an untouched topic: %v", err)
	}
}

// TestOrderedIsIgnoredOutsideCategoryPractice guards the blast radius. Ordered
// is a property of "all questions of one topic"; every other draw must stay
// random and must not touch or read the cursor.
func TestOrderedIsIgnoredOutsideCategoryPractice(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	ctx := context.Background()
	categoryID := categoryIDByCode(t, q, "signs")

	// Walk the topic to the end so the cursor is non-zero and would visibly
	// change any draw that wrongly consulted it.
	full := startOrdered(t, svc, profileID, categoryID, 100)
	answerFirst(t, q, svc, profileID, full, 3)
	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d, want 3", got)
	}

	signID, err := q.GetSignIDByCode(ctx, "3.27")
	if err != nil {
		t.Fatal(err)
	}
	hasImage := true
	cases := []struct {
		name string
		req  session.StartRequest
	}{
		{"sign selector with ordered set", session.StartRequest{
			Mode: "practice", SignID: signID, Count: 5, Ordered: true}},
		{"bilet range with ordered set", session.StartRequest{
			Mode: "practice", VariantFrom: 1, VariantTo: 2, Count: 5, Ordered: true}},
		{"image selector with ordered set", session.StartRequest{
			Mode: "practice", HasImage: &hasImage, Count: 5, Ordered: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.StartSession(ctx, profileID, tc.req); err != nil {
				t.Fatalf("start: %v", err)
			}
			if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
				t.Fatalf("cursor=%d, want it untouched at 3", got)
			}
		})
	}

	// A category draw WITHOUT ordered stays random: it starts from the whole
	// topic, not from the cursor, so it may serve questions the class already
	// did. Assert it does not silently become the ordered walk.
	randomDraw, err := svc.StartSession(ctx, profileID, session.StartRequest{
		Mode: "practice", CategoryID: categoryID, Count: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(randomDraw.QuestionIDs) != len(full.QuestionIDs) {
		t.Fatalf("random draw returned %d questions, want the whole topic (%d) -- it must not resume from the cursor",
			len(randomDraw.QuestionIDs), len(full.QuestionIDs))
	}
	if got := cursorOf(t, pool, profileID, categoryID); got != 3 {
		t.Fatalf("cursor=%d after a random draw, want it untouched at 3", got)
	}
}

// TestEveryTopicWalksItsWholeBankExactlyOnce runs the walk over all of the
// fixture's topics rather than one, because the feature ships for all 13 in
// production and a bug that only bites an odd-sized topic would otherwise wait
// to be found by a classroom.
func TestEveryTopicWalksItsWholeBankExactlyOnce(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	grantVIP(t, q, profileID)
	ctx := context.Background()

	rows, err := q.CountValidQuestionsByCategory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no categories in the fixture")
	}
	for _, row := range rows {
		total := int(row.QuestionCount)
		seen := make(map[uuid.UUID]bool, total)
		var walked []uuid.UUID
		// Two questions at a time, so the walk crosses many session boundaries
		// -- that is where an off-by-one in the cursor would show up.
		for len(walked) < total {
			view := startOrdered(t, svc, profileID, row.CategoryID, 2)
			if len(view.QuestionIDs) == 0 {
				t.Fatalf("category %s: draw returned nothing at %d/%d", row.CategoryID, len(walked), total)
			}
			for _, id := range view.QuestionIDs {
				if seen[id] {
					t.Fatalf("category %s: question %s served twice within one walk", row.CategoryID, id)
				}
				seen[id] = true
				walked = append(walked, id)
			}
			answerFirst(t, q, svc, profileID, view, len(view.QuestionIDs))
		}
		if len(walked) != total {
			t.Fatalf("category %s: walked %d questions, want exactly %d", row.CategoryID, len(walked), total)
		}
		if got := cursorOf(t, pool, profileID, row.CategoryID); got != total {
			t.Fatalf("category %s: cursor=%d at the end, want %d", row.CategoryID, got, total)
		}
	}
}

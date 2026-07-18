package learning_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *learning.Service, uuid.UUID, []uuid.UUID) {
	t.Helper()
	q, svc, profileID, qids, _ := seedWithPool(t)
	return q, svc, profileID, qids
}

func seedWithPool(t *testing.T) (*sqlc.Queries, *learning.Service, uuid.UUID, []uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v %d", err, len(qids))
	}
	svc := learning.NewService(q)
	return q, svc, profile.ID, qids, pool
}

// backdateDueAt forces question_memory.due_at to an explicit timestamp for
// the given profile/question pair, so the row's "due now" status (and its
// relative due-urgency ordering vs. other rows) is fully deterministic
// without depending on FSRS's real interval math (which always schedules
// >= 1 day out, even for an Again rating).
func backdateDueAt(t *testing.T, pool *pgxpool.Pool, profileID, questionID uuid.UUID, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE question_memory SET due_at = $3 WHERE profile_id = $1 AND question_id = $2`,
		profileID, questionID, at); err != nil {
		t.Fatalf("backdate due_at: %v", err)
	}
}

func TestRecordReviewFirstTimeCreatesRow(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	card, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("RecordReview: %v", err)
	}
	if card.Reps != 1 || card.Lapses != 0 {
		t.Fatalf("unexpected card: %+v", card)
	}

	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 1 {
		t.Fatalf("stored reps = %d, want 1", row.Reps)
	}
}

func TestRecordReviewUpdatesCategoryMastery(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	catID, err := q.GetQuestionCategoryID(context.Background(), qids[0])
	if err != nil {
		t.Fatalf("category: %v", err)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good); err != nil {
		t.Fatalf("review 1: %v", err)
	}
	m, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
	if err != nil {
		t.Fatalf("mastery: %v", err)
	}
	if m.Seen != 1 || m.Correct != 1 {
		t.Fatalf("mastery after 1 correct = %+v", m)
	}

	// qids is variant 1's ordered question list (position 1..20 == fixture
	// questions nmn-0001..nmn-0020, i.e. qids[i] is fixture question n=i+1).
	// fixture.Sample assigns Category: cats[n%len(cats)] with len(cats)==4,
	// so questions n and n+4 always share a category (same n%4). qids[0] is
	// n=1 (cats[1%4]) and qids[4] is n=5 (cats[5%4]==cats[1%4]): guaranteed
	// same category, unlike qids[0]/qids[1] (n=1 vs n=2, different n%4).
	catID2, err := q.GetQuestionCategoryID(context.Background(), qids[4])
	if err != nil {
		t.Fatalf("category 2: %v", err)
	}
	if catID2 != catID {
		t.Fatalf("test fixture assumption broken: qids[0] and qids[4] must share a category (got %v vs %v)", catID, catID2)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[4], learning.Again); err != nil {
		t.Fatalf("review 2: %v", err)
	}
	m2, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
	if err != nil {
		t.Fatalf("mastery: %v", err)
	}
	if m2.Seen != 2 || m2.Correct != 1 {
		t.Fatalf("mastery after 1 correct + 1 wrong (same category) = %+v", m2)
	}
}

func TestRecordReviewInvalidRating(t *testing.T) {
	_, svc, profileID, qids := seed(t)
	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Rating(99)); err != learning.ErrInvalidRating {
		t.Fatalf("err=%v want ErrInvalidRating", err)
	}
}

func TestRecordReviewSecondTimeUsesStoredState(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	first, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Stability <= first.Stability {
		t.Fatalf("a 2nd successful review must increase stability further: first=%v second=%v", first.Stability, second.Stability)
	}
	if second.Reps != 2 {
		t.Fatalf("Reps = %d, want 2", second.Reps)
	}
	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 2 {
		t.Fatalf("stored reps = %d, want 2", row.Reps)
	}
}

// TestNextDueInterleavesCategories verifies NextDue round-robins across
// categories (by ascending due-urgency) instead of returning one category's
// due items as a contiguous block.
//
// Determinism: qids is variant 1's ordered question list, where qids[i] is
// fixture question n=i+1 and fixture.Sample assigns
// Category: cats[n%len(cats)] with the 4-category cycle
// {rules, priority, safety, signs} (n%4 == 1,2,3,0 respectively — see
// TestRecordReviewUpdatesCategoryMastery for the same fixture assumption).
// So among qids[:8] (n=1..8), each of the 4 categories appears exactly
// twice, in first-appearance order n=1,2,3,4 (categories rules, priority,
// safety, signs) and again at n=5,6,7,8 in the same order.
//
// We record a review for each of qids[:8] (creating question_memory rows)
// then directly backdate due_at to explicit, strictly increasing
// timestamps in n-order — all in the past, so ListDueQuestions' due_at ASC
// ordering exactly reproduces n=1..8 order without depending on wall-clock
// timing or DB tie-break behavior for equal timestamps. With that ordering,
// a correct round-robin interleave produces the category sequence
// rules, priority, safety, signs, rules, priority, safety, signs — i.e.
// never two consecutive same-category entries, and all 4 categories
// represented in the first 4 results.
func TestNextDueInterleavesCategories(t *testing.T) {
	q, svc, profileID, qids, pool := seedWithPool(t)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	for i, qid := range qids[:8] {
		if _, err := svc.RecordReview(ctx, profileID, qid, learning.Again); err != nil {
			t.Fatalf("record review: %v", err)
		}
		// Strictly increasing due_at in n-order, all safely in the past.
		backdateDueAt(t, pool, profileID, qid, base.Add(time.Duration(i)*time.Minute))
	}

	got, err := svc.NextDue(ctx, profileID, 8)
	if err != nil {
		t.Fatalf("NextDue: %v", err)
	}
	if len(got) != 8 {
		t.Fatalf("NextDue returned %d ids, want 8", len(got))
	}

	cats := make([]uuid.UUID, len(got))
	for i, qid := range got {
		catID, err := q.GetQuestionCategoryID(ctx, qid)
		if err != nil {
			t.Fatalf("category for result %d: %v", i, err)
		}
		cats[i] = catID
	}

	distinct := map[uuid.UUID]bool{}
	for _, c := range cats {
		distinct[c] = true
	}
	if len(distinct) != 4 {
		t.Fatalf("expected 4 distinct categories among due items, got %d", len(distinct))
	}

	// No two consecutive results share a category — proves genuine
	// round-robin interleaving, not block-grouping by category.
	for i := 1; i < len(cats); i++ {
		if cats[i] == cats[i-1] {
			t.Fatalf("consecutive same-category entries at positions %d,%d (category %v): %v", i-1, i, cats[i-1], cats)
		}
	}

	// Every represented category appears at least once among the first
	// 2*numCategoriesWithDueItems (= 8, i.e. all) results.
	firstFour := map[uuid.UUID]bool{}
	for _, c := range cats[:4] {
		firstFour[c] = true
	}
	if len(firstFour) != 4 {
		t.Fatalf("expected all 4 categories represented in the first 4 results, got %d: %v", len(firstFour), cats[:4])
	}
}

func TestStatsReadinessWeightedByQuestionCount(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	ctx := context.Background()

	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats (fresh profile): %v", err)
	}
	if stats.ReadinessPct != 0 {
		t.Fatalf("fresh profile readiness = %d, want 0", stats.ReadinessPct)
	}
	if len(stats.Categories) != 4 {
		t.Fatalf("expected 4 categories (fixture), got %d", len(stats.Categories))
	}

	// answer all 10 questions in the "signs" category correctly (fixture:
	// 40 questions / 4 categories = 10 each, round-robin assignment)
	signsCatID, err := q.GetCategoryIDByCode(ctx, "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	answered := 0
	for _, qid := range qids {
		catID, err := q.GetQuestionCategoryID(ctx, qid)
		if err != nil {
			t.Fatalf("question category: %v", err)
		}
		if catID != signsCatID {
			continue
		}
		if _, err := svc.RecordReview(ctx, profileID, qid, learning.Good); err != nil {
			t.Fatalf("review: %v", err)
		}
		answered++
	}
	if answered == 0 {
		t.Fatal("fixture must contain at least one 'signs' question reachable from variant 1")
	}

	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats (after reviews): %v", err)
	}
	if stats.ReadinessPct <= 0 {
		t.Fatalf("readiness should be > 0 after mastering part of one category, got %d", stats.ReadinessPct)
	}
	if stats.ReadinessPct >= 100 {
		t.Fatalf("readiness should be < 100 since only 1 of 4 categories was touched, got %d", stats.ReadinessPct)
	}
}

func TestStatsDueCount(t *testing.T) {
	_, svc, profileID, qids, pool := seedWithPool(t)
	ctx := context.Background()
	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.DueCount != 0 {
		t.Fatalf("fresh profile due count = %d, want 0", stats.DueCount)
	}
	if _, err := svc.RecordReview(ctx, profileID, qids[0], learning.Again); err != nil {
		t.Fatalf("review: %v", err)
	}
	// an Again review schedules due_at >=1 day in the future (interval
	// floor), so it is NOT immediately due yet — verify that directly.
	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats after review: %v", err)
	}
	if stats.DueCount != 0 {
		t.Fatalf("due count immediately after a real (non-backdated) review = %d, want 0", stats.DueCount)
	}

	// Positive case: backdate due_at into the past (same deterministic
	// technique as TestNextDueInterleavesCategories) and confirm DueCount
	// increments.
	backdateDueAt(t, pool, profileID, qids[0], time.Now().Add(-time.Hour))
	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats after backdate: %v", err)
	}
	if stats.DueCount != 1 {
		t.Fatalf("due count after backdating one review's due_at into the past = %d, want 1", stats.DueCount)
	}
}

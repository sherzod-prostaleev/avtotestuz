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

func TestMistakeBankSummaryTracksTotalDueAndNextDate(t *testing.T) {
	_, svc, profileID, qids, pool := seedWithPool(t)

	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Again); err != nil {
		t.Fatalf("record future mistake: %v", err)
	}
	if _, err := svc.RecordReview(context.Background(), profileID, qids[1], learning.Again); err != nil {
		t.Fatalf("record due mistake: %v", err)
	}
	dueAt := time.Now().UTC().Add(-time.Hour)
	backdateDueAt(t, pool, profileID, qids[1], dueAt)

	summary, err := svc.MistakeBankSummary(context.Background(), profileID)
	if err != nil {
		t.Fatalf("MistakeBankSummary: %v", err)
	}
	if summary.TotalBankCount != 2 || summary.DueCount != 1 {
		t.Fatalf("summary=%+v, want total=2 due=1", summary)
	}
	if summary.NextDueAt == nil || !summary.NextDueAt.After(time.Now()) {
		t.Fatalf("next_due_at=%v, want future timestamp", summary.NextDueAt)
	}
}

func TestMistakeBankSummaryEmpty(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	summary, err := svc.MistakeBankSummary(context.Background(), profileID)
	if err != nil {
		t.Fatalf("MistakeBankSummary: %v", err)
	}
	if summary.DueCount != 0 || summary.TotalBankCount != 0 || summary.NextDueAt != nil {
		t.Fatalf("unexpected empty summary: %+v", summary)
	}
}

func TestMistakeBankSummaryIsProfileScopedAndIgnoresInvalidQuestions(t *testing.T) {
	q, svc, profileAID, qids, pool := seedWithPool(t)
	profileB, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998907770088"})
	if err != nil {
		t.Fatalf("create profile B: %v", err)
	}

	for _, review := range []struct {
		profileID  uuid.UUID
		questionID uuid.UUID
	}{
		{profileAID, qids[0]},
		{profileAID, qids[1]}, // quarantined below; must disappear from the bank
		{profileB.ID, qids[2]},
	} {
		if _, err := svc.RecordReview(context.Background(), review.profileID, review.questionID, learning.Again); err != nil {
			t.Fatalf("record review for %s/%s: %v", review.profileID, review.questionID, err)
		}
		backdateDueAt(t, pool, review.profileID, review.questionID, time.Now().UTC().Add(-time.Hour))
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE question SET validation_status = 'quarantined' WHERE id = $1`, qids[1]); err != nil {
		t.Fatalf("quarantine question: %v", err)
	}

	summaryA, err := svc.MistakeBankSummary(context.Background(), profileAID)
	if err != nil {
		t.Fatalf("profile A summary: %v", err)
	}
	if summaryA.DueCount != 1 || summaryA.TotalBankCount != 1 || summaryA.NextDueAt != nil {
		t.Fatalf("profile A summary includes another tenant or invalid content: %+v", summaryA)
	}
	summaryB, err := svc.MistakeBankSummary(context.Background(), profileB.ID)
	if err != nil {
		t.Fatalf("profile B summary: %v", err)
	}
	if summaryB.DueCount != 1 || summaryB.TotalBankCount != 1 || summaryB.NextDueAt != nil {
		t.Fatalf("profile B summary includes profile A data: %+v", summaryB)
	}
}

func TestMistakeBankQuestionIDsIgnoreInvalidQuestions(t *testing.T) {
	q, svc, profileID, qids, pool := seedWithPool(t)
	ctx := context.Background()

	for _, questionID := range qids[:2] {
		if _, err := svc.RecordReview(ctx, profileID, questionID, learning.Again); err != nil {
			t.Fatalf("record mistake for %s: %v", questionID, err)
		}
		backdateDueAt(t, pool, profileID, questionID, time.Now().UTC().Add(-time.Hour))
	}
	if _, err := pool.Exec(ctx,
		`UPDATE question SET validation_status = 'quarantined' WHERE id = $1`, qids[1]); err != nil {
		t.Fatalf("quarantine question: %v", err)
	}

	ids, err := q.ListMistakeBankQuestionIDs(ctx, sqlc.ListMistakeBankQuestionIDsParams{
		ProfileID: profileID, LimitCount: 10,
	})
	if err != nil {
		t.Fatalf("ListMistakeBankQuestionIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != qids[0] {
		t.Fatalf("mistake bank ids = %v, want only valid question %s", ids, qids[0])
	}
}

func TestMistakeBankSummaryUsesEarliestFutureDueAndNullWhenAllDue(t *testing.T) {
	_, svc, profileID, qids, pool := seedWithPool(t)
	for _, questionID := range qids[:3] {
		if _, err := svc.RecordReview(context.Background(), profileID, questionID, learning.Again); err != nil {
			t.Fatalf("record mistake: %v", err)
		}
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	past := now.Add(-time.Hour)
	firstFuture := now.Add(2 * time.Hour)
	secondFuture := now.Add(4 * time.Hour)
	backdateDueAt(t, pool, profileID, qids[0], past)
	backdateDueAt(t, pool, profileID, qids[1], secondFuture)
	backdateDueAt(t, pool, profileID, qids[2], firstFuture)

	summary, err := svc.MistakeBankSummary(context.Background(), profileID)
	if err != nil {
		t.Fatalf("MistakeBankSummary: %v", err)
	}
	if summary.DueCount != 1 || summary.TotalBankCount != 3 {
		t.Fatalf("mixed schedule summary=%+v, want due=1 total=3", summary)
	}
	if summary.NextDueAt == nil || !summary.NextDueAt.Equal(firstFuture) {
		t.Fatalf("next_due_at=%v want earliest future %v", summary.NextDueAt, firstFuture)
	}
	if summary.NextDueAt.Location() != time.UTC {
		t.Fatalf("next_due_at location=%v want UTC", summary.NextDueAt.Location())
	}

	backdateDueAt(t, pool, profileID, qids[1], past.Add(-time.Hour))
	backdateDueAt(t, pool, profileID, qids[2], past.Add(-2*time.Hour))
	summary, err = svc.MistakeBankSummary(context.Background(), profileID)
	if err != nil {
		t.Fatalf("all-due summary: %v", err)
	}
	if summary.DueCount != 3 || summary.TotalBankCount != 3 || summary.NextDueAt != nil {
		t.Fatalf("all-due summary=%+v want due=total=3 and next_due_at=nil", summary)
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

	var signsStat learning.CategoryStat
	for _, c := range stats.Categories {
		if c.CategoryCode == "signs" {
			signsStat = c
			break
		}
	}
	if signsStat.Total <= 0 || signsStat.Studied != answered {
		t.Fatalf("signs studied=%d total=%d answered=%d", signsStat.Studied, signsStat.Total, answered)
	}
	// Bank-honest: perfect accuracy on the studied subset → mastery = studied/total.
	wantMastery := float64(answered) / float64(signsStat.Total)
	if signsStat.Mastery < wantMastery-0.01 || signsStat.Mastery > wantMastery+0.01 {
		t.Fatalf("signs mastery=%.3f want ≈%.3f (studied/total with 100%% accuracy)", signsStat.Mastery, wantMastery)
	}
}

func TestStatsMasteryDoesNotInflateOnTinySubset(t *testing.T) {
	_, svc, profileID, qids := seed(t)
	ctx := context.Background()

	if _, err := svc.RecordReview(ctx, profileID, qids[0], learning.Good); err != nil {
		t.Fatalf("review: %v", err)
	}
	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	var touched learning.CategoryStat
	for _, c := range stats.Categories {
		if c.Studied > 0 {
			touched = c
			break
		}
	}
	if touched.Total < 2 {
		t.Fatalf("fixture category too small to prove coverage penalty: total=%d", touched.Total)
	}
	// One correct question in a multi-question category must stay well below 100%.
	if touched.Mastery >= 0.5 {
		t.Fatalf("mastery=%.3f after 1/%d questions — coverage inflation bug", touched.Mastery, touched.Total)
	}
	if stats.ReadinessPct >= 25 {
		t.Fatalf("readiness=%d after a single answer — too high for bank-honest formula", stats.ReadinessPct)
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

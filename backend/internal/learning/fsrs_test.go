package learning

import (
	"math"
	"testing"
	"time"
)

func approxEqual(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (tol %v)", name, got, want, tol)
	}
}

func TestReviewFirstTimeInitialValues(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		rating Rating
		wantS  float64
		wantD  float64
	}{
		{Again, 0.40255, 7.1949},
		{Hard, 1.18385, 6.4883},
		{Good, 3.173, 5.2824},
		{Easy, 15.69105, 3.2245},
	}
	for _, c := range cases {
		card := Review(Card{}, c.rating, now, DefaultDesiredRetention)
		approxEqual(t, "stability", card.Stability, c.wantS, 1e-4)
		approxEqual(t, "difficulty", card.Difficulty, c.wantD, 5e-4)
		if card.Reps != 1 {
			t.Errorf("Reps = %d, want 1", card.Reps)
		}
		if c.rating == Again && card.Lapses != 1 {
			t.Errorf("Lapses = %d, want 1 for Again", card.Lapses)
		}
		if c.rating != Again && card.Lapses != 0 {
			t.Errorf("Lapses = %d, want 0 for non-Again", card.Lapses)
		}
		wantState := int16(1)
		if c.rating == Again {
			wantState = 0
		}
		if card.State != wantState {
			t.Errorf("State = %d, want %d", card.State, wantState)
		}
		if !card.DueAt.After(now) {
			t.Errorf("DueAt %v must be after review time %v", card.DueAt, now)
		}
	}
}

func TestIntervalAtDesiredRetentionEqualsStability(t *testing.T) {
	// I(r, S) == S exactly when r == the target retention FACTOR was derived
	// from (0.9) — a structural identity, not a coincidence.
	for _, s := range []float64{0.4, 3.173, 15.69, 50.0} {
		got := interval(0.9, s)
		approxEqual(t, "interval", got, s, 1e-9)
	}
}

func TestReviewSecondTimeGoodIncreaseStability(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := Review(Card{}, Good, now, DefaultDesiredRetention)
	second := Review(first, Good, now.AddDate(0, 0, 3), DefaultDesiredRetention)
	approxEqual(t, "stability after 2nd Good", second.Stability, 10.73893, 1e-3)
	approxEqual(t, "difficulty after 2nd Good", second.Difficulty, 5.27297, 1e-3)
	if second.Reps != 2 {
		t.Errorf("Reps = %d, want 2", second.Reps)
	}
	if second.Lapses != 0 {
		t.Errorf("Lapses = %d, want 0", second.Lapses)
	}
	if second.State != 1 {
		t.Errorf("State = %d, want 1", second.State)
	}

	third := Review(second, Again, now.AddDate(0, 0, 3+5), DefaultDesiredRetention)
	approxEqual(t, "stability after fail", third.Stability, 1.94435, 1e-3)
	approxEqual(t, "difficulty after fail", third.Difficulty, 6.79057, 1e-3)
	if third.Reps != 3 {
		t.Errorf("Reps = %d, want 3", third.Reps)
	}
	if third.Lapses != 1 {
		t.Errorf("Lapses = %d, want 1", third.Lapses)
	}
	if third.State != 0 {
		t.Errorf("State = %d, want 0", third.State)
	}
	if third.Stability >= second.Stability {
		t.Errorf("a forgotten review must not increase stability: got %v, was %v", third.Stability, second.Stability)
	}
}

func TestReviewStabilityOrderingByGrade(t *testing.T) {
	// Easy > Good > Hard > Again for a first review's initial stability —
	// higher confidence grades must produce longer initial intervals.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	again := Review(Card{}, Again, now, DefaultDesiredRetention)
	hard := Review(Card{}, Hard, now, DefaultDesiredRetention)
	good := Review(Card{}, Good, now, DefaultDesiredRetention)
	easy := Review(Card{}, Easy, now, DefaultDesiredRetention)
	if !(again.Stability < hard.Stability && hard.Stability < good.Stability && good.Stability < easy.Stability) {
		t.Fatalf("expected strictly increasing stability Again<Hard<Good<Easy, got %v %v %v %v",
			again.Stability, hard.Stability, good.Stability, easy.Stability)
	}
}

func TestCardIsNew(t *testing.T) {
	var c Card
	if !c.IsNew() {
		t.Fatal("zero-value Card must be new")
	}
	c.LastReviewedAt = time.Now()
	if c.IsNew() {
		t.Fatal("Card with LastReviewedAt set must not be new")
	}
}

// TestReviewHandlesZeroStabilityAsNew is a regression test for a real corrupted
// row found in the dev DB: stability=NaN, difficulty=1, reps=1. That signature
// proves the row entered Review() with Stability=0, Difficulty=0 (schema
// defaults) despite LastReviewedAt being set (so NOT IsNew()), because
// math.Pow(0, -w[9]) == +Inf, and 0 * (1+Inf) == NaN in stabilitySuccess.
// Review() must treat such a degenerate card the same as a brand-new one.
func TestReviewHandlesZeroStabilityAsNew(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -3)
	corrupt := Card{
		Stability:      0,
		Difficulty:     0,
		LastReviewedAt: past,
		Reps:           1,
	}
	if corrupt.IsNew() {
		t.Fatal("test setup invalid: corrupt card must not be IsNew() (LastReviewedAt is set)")
	}

	got := Review(corrupt, Good, now, DefaultDesiredRetention)

	if math.IsNaN(got.Stability) || math.IsInf(got.Stability, 0) || got.Stability <= 0 {
		t.Fatalf("Stability = %v, want a valid positive value", got.Stability)
	}

	wantS := s0(Good)
	approxEqual(t, "stability (reset path)", got.Stability, wantS, 1e-9)

	wantD := d0(Good)
	approxEqual(t, "difficulty (reset path)", got.Difficulty, wantD, 1e-9)
}

// TestReviewNeverProducesNaNOrInf checks, across every rating and both a
// genuinely new card and a corrupted (Stability=0, previously-reviewed) card,
// that Review() always yields a finite, positive Stability.
func TestReviewNeverProducesNaNOrInf(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -3)

	cards := map[string]Card{
		"new":       {},
		"corrupted": {Stability: 0, Difficulty: 0, LastReviewedAt: past, Reps: 1},
	}
	ratings := []Rating{Again, Hard, Good, Easy}

	for name, card := range cards {
		for _, rating := range ratings {
			got := Review(card, rating, now, DefaultDesiredRetention)
			if math.IsNaN(got.Stability) {
				t.Errorf("card=%s rating=%v: Stability is NaN", name, rating)
			}
			if math.IsInf(got.Stability, 0) {
				t.Errorf("card=%s rating=%v: Stability is Inf", name, rating)
			}
			if got.Stability <= 0 {
				t.Errorf("card=%s rating=%v: Stability = %v, want > 0", name, rating, got.Stability)
			}
		}
	}
}

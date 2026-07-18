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

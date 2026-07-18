package progress

import (
	"testing"
	"time"
)

func day(offset int) time.Time {
	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, offset)
}

func TestBumpStreakBrandNew(t *testing.T) {
	got := BumpStreak(StreakState{}, day(0))
	if got.Current != 1 || got.Best != 1 || got.TodayDone != 1 {
		t.Fatalf("brand new bump = %+v", got)
	}
	if got.LastActiveDate == nil || !got.LastActiveDate.Equal(day(0)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(0))
	}
}

func TestBumpStreakSameDayOnlyIncrementsTodayDone(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 3, Best: 5, TodayDone: 2, LastActiveDate: &d}
	got := BumpStreak(s, day(0))
	if got.Current != 3 || got.Best != 5 || got.TodayDone != 3 {
		t.Fatalf("same-day bump = %+v", got)
	}
}

func TestBumpStreakConsecutiveDayIncrementsCurrent(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 3, Best: 5, TodayDone: 10, LastActiveDate: &d}
	got := BumpStreak(s, day(1))
	if got.Current != 4 || got.Best != 5 || got.TodayDone != 1 {
		t.Fatalf("consecutive-day bump = %+v", got)
	}
	if !got.LastActiveDate.Equal(day(1)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(1))
	}
}

func TestBumpStreakConsecutiveDayCanExceedPriorBest(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 5, Best: 5, TodayDone: 1, LastActiveDate: &d}
	got := BumpStreak(s, day(1))
	if got.Current != 6 || got.Best != 6 {
		t.Fatalf("new-best bump = %+v", got)
	}
}

func TestBumpStreakGapResetsCurrent(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 7, Best: 7, TodayDone: 1, LastActiveDate: &d}
	got := BumpStreak(s, day(3))
	if got.Current != 1 || got.Best != 7 || got.TodayDone != 1 {
		t.Fatalf("gap bump = %+v", got)
	}
	if !got.LastActiveDate.Equal(day(3)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(3))
	}
}

package leaderboard_test

import (
	"testing"
	"time"

	"avtotest.uz/backend/internal/leaderboard"
)

func TestRedisKeyDaily(t *testing.T) {
	tm := time.Date(2026, 7, 25, 14, 30, 0, 0, time.UTC)
	got := leaderboard.RedisKey(leaderboard.PeriodDaily, tm)
	want := "lb:daily:2026-07-25"
	if got != want {
		t.Errorf("RedisKey(daily) = %q, want %q", got, want)
	}
}

func TestRedisKeyWeekly(t *testing.T) {
	// 2026-07-25 is a Saturday; ISO week 30 of 2026.
	tm := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	got := leaderboard.RedisKey(leaderboard.PeriodWeekly, tm)
	want := "lb:weekly:2026-W30"
	if got != want {
		t.Errorf("RedisKey(weekly) = %q, want %q", got, want)
	}
}

func TestRedisKeyMonthly(t *testing.T) {
	tm := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	got := leaderboard.RedisKey(leaderboard.PeriodMonthly, tm)
	want := "lb:monthly:2026-07"
	if got != want {
		t.Errorf("RedisKey(monthly) = %q, want %q", got, want)
	}
}

func TestRedisKeyAllTimeHasNoDateComponent(t *testing.T) {
	tm := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	got := leaderboard.RedisKey(leaderboard.PeriodAllTime, tm)
	want := "lb:alltime"
	if got != want {
		t.Errorf("RedisKey(alltime) = %q, want %q", got, want)
	}
}

func TestRedisKeyUsesUTCNotLocalTime(t *testing.T) {
	// 23:30 in UTC+5 (Tashkent) on 2026-07-25 is 18:30 UTC on the same day —
	// but 01:30 in UTC+5 on 2026-07-26 is 20:30 UTC on 2026-07-25, the
	// PREVIOUS day. Pass a time.Time already in a non-UTC location and
	// confirm RedisKey converts to UTC before formatting, matching this
	// codebase's existing UTC-day-boundary convention (todayUTC in
	// internal/progress/service.go).
	loc := time.FixedZone("UZT", 5*60*60)
	tm := time.Date(2026, 7, 26, 1, 30, 0, 0, loc) // 2026-07-25 20:30 UTC
	got := leaderboard.RedisKey(leaderboard.PeriodDaily, tm)
	want := "lb:daily:2026-07-25"
	if got != want {
		t.Errorf("RedisKey(daily) with non-UTC input = %q, want %q", got, want)
	}
}

func TestPeriodStartEndDaily(t *testing.T) {
	tm := time.Date(2026, 7, 25, 14, 30, 0, 0, time.UTC)
	start := leaderboard.PeriodStart(leaderboard.PeriodDaily, tm)
	end := leaderboard.PeriodEnd(leaderboard.PeriodDaily, tm)
	wantStart := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("PeriodStart(daily) = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("PeriodEnd(daily) = %v, want %v", end, wantEnd)
	}
}

func TestPeriodStartEndWeeklyIsISOMondayToMonday(t *testing.T) {
	// Saturday 2026-07-25 -> week starts Monday 2026-07-20, ends Monday 2026-07-27.
	tm := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	start := leaderboard.PeriodStart(leaderboard.PeriodWeekly, tm)
	end := leaderboard.PeriodEnd(leaderboard.PeriodWeekly, tm)
	wantStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("PeriodStart(weekly) = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("PeriodEnd(weekly) = %v, want %v", end, wantEnd)
	}
}

func TestPeriodStartEndMonthly(t *testing.T) {
	tm := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	start := leaderboard.PeriodStart(leaderboard.PeriodMonthly, tm)
	end := leaderboard.PeriodEnd(leaderboard.PeriodMonthly, tm)
	wantStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("PeriodStart(monthly) = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("PeriodEnd(monthly) = %v, want %v", end, wantEnd)
	}
}

func TestPeriodStartEndAllTimeCoversEverythingUpToNow(t *testing.T) {
	tm := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	start := leaderboard.PeriodStart(leaderboard.PeriodAllTime, tm)
	end := leaderboard.PeriodEnd(leaderboard.PeriodAllTime, tm)
	if !start.Before(time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("PeriodStart(alltime) = %v, want at/near Unix epoch", start)
	}
	if !end.After(tm) {
		t.Errorf("PeriodEnd(alltime) = %v, want strictly after %v", end, tm)
	}
}

func TestTTLZeroForAllTime(t *testing.T) {
	if got := leaderboard.TTL(leaderboard.PeriodAllTime); got != 0 {
		t.Errorf("TTL(alltime) = %v, want 0", got)
	}
}

func TestTTLPositiveForBoundedPeriods(t *testing.T) {
	for _, p := range []leaderboard.Period{leaderboard.PeriodDaily, leaderboard.PeriodWeekly, leaderboard.PeriodMonthly} {
		if got := leaderboard.TTL(p); got <= 0 {
			t.Errorf("TTL(%s) = %v, want > 0", p, got)
		}
	}
}

func TestEncodeScorePreservesIntegerPart(t *testing.T) {
	now := time.Now()
	score := leaderboard.EncodeScore(42, now)
	if got := leaderboard.DecodePoints(score); got != 42 {
		t.Errorf("DecodePoints(EncodeScore(42, now)) = %d, want 42", got)
	}
}

func TestEncodeScoreBreaksTiesInFavorOfEarlierAchiever(t *testing.T) {
	earlier := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	later := earlier.Add(1 * time.Hour)
	scoreEarlier := leaderboard.EncodeScore(10, earlier)
	scoreLater := leaderboard.EncodeScore(10, later)
	// ZREVRANGE is descending: the earlier achiever must have the LARGER
	// score so they rank first among equal point totals.
	if !(scoreEarlier > scoreLater) {
		t.Errorf("EncodeScore(10, earlier)=%v should be > EncodeScore(10, later)=%v", scoreEarlier, scoreLater)
	}
	// Both must still decode to the same integer point total.
	if leaderboard.DecodePoints(scoreEarlier) != 10 || leaderboard.DecodePoints(scoreLater) != 10 {
		t.Errorf("tie-break fraction leaked into the integer part: earlier=%v later=%v",
			leaderboard.DecodePoints(scoreEarlier), leaderboard.DecodePoints(scoreLater))
	}
}

func TestEncodeScoreHigherPointsAlwaysOutranksLowerRegardlessOfTime(t *testing.T) {
	veryLate := time.Now().Add(24 * time.Hour)
	veryEarly := time.Now().Add(-24 * time.Hour)
	lowPointsLate := leaderboard.EncodeScore(5, veryEarly)  // fewer points, but earliest timestamp
	highPointsEarly := leaderboard.EncodeScore(6, veryLate) // more points, latest timestamp
	if !(highPointsEarly > lowPointsLate) {
		t.Errorf("6 points (score=%v) should always outrank 5 points (score=%v) regardless of timing", highPointsEarly, lowPointsLate)
	}
}

func TestDisplayNameUsesNameWhenPresent(t *testing.T) {
	got := leaderboard.DisplayName("Aziz Karimov", "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	if got != "Aziz Karimov" {
		t.Errorf("DisplayName = %q, want %q", got, "Aziz Karimov")
	}
}

func TestDisplayNameFallsBackToShortID(t *testing.T) {
	got := leaderboard.DisplayName("", "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	want := "Foydalanuvchi #3fa8"
	if got != want {
		t.Errorf("DisplayName = %q, want %q", got, want)
	}
}

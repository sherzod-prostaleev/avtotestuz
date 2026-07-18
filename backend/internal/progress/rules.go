// Package progress owns saved (bookmarked) questions and daily streak
// tracking — two simple per-profile engagement features grouped together
// since neither warrants the complexity of its own package the way FSRS
// (internal/learning) or scoring (internal/session) did. rules.go is pure;
// service.go integrates it with saved_question/streak.
package progress

import "time"

type StreakState struct {
	Current, Best, TodayDone int
	LastActiveDate           *time.Time
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// BumpStreak applies one day's worth of activity to s, evaluated as of
// today (already truncated to a calendar day by the caller).
func BumpStreak(s StreakState, today time.Time) StreakState {
	if s.LastActiveDate == nil {
		best := s.Best
		if best < 1 {
			best = 1
		}
		return StreakState{Current: 1, Best: best, TodayDone: 1, LastActiveDate: &today}
	}
	if sameDay(*s.LastActiveDate, today) {
		s.TodayDone++
		return s
	}
	if sameDay(*s.LastActiveDate, today.AddDate(0, 0, -1)) {
		s.Current++
		if s.Current > s.Best {
			s.Best = s.Current
		}
		s.TodayDone = 1
		s.LastActiveDate = &today
		return s
	}
	best := s.Best
	if best < 1 {
		best = 1
	}
	return StreakState{Current: 1, Best: best, TodayDone: 1, LastActiveDate: &today}
}

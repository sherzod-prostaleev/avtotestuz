// Package leaderboard ranks profiles by correct-answer counts across four
// rolling windows (daily/weekly/monthly/all-time). Redis sorted sets hold
// the live ranking; session_answer (see internal/session) stays the sole
// durable source of truth, so a lost/evicted Redis key is always fully
// recoverable via Service.RebuildPeriod. See
// docs/superpowers/specs/2026-07-25-m4-01-leaderboard-design.md.
package leaderboard

import (
	"fmt"
	"math"
	"time"
)

// Period is one of the four ranking windows this package tracks.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
	PeriodAllTime Period = "alltime"
)

// AllPeriods lists every period RecordPoint updates on each correct answer.
var AllPeriods = []Period{PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodAllTime}

// TopN is how many entries GetLeaderboard's "top" list returns.
const TopN = 10

// AroundRadius is how many neighbors on each side of the caller's own rank
// GetLeaderboard's "around_you" list returns.
const AroundRadius = 2

// RedisKey returns the sorted-set key for period p covering the window
// that contains t. Day/week/month boundaries are always UTC, matching this
// codebase's existing convention (see internal/progress/service.go's
// todayUTC) — t is converted to UTC internally regardless of its original
// location, so callers never need to convert first.
func RedisKey(p Period, t time.Time) string {
	if p == PeriodAllTime {
		return "lb:alltime"
	}
	return "lb:" + string(p) + ":" + periodSuffix(p, t)
}

func periodSuffix(p Period, t time.Time) string {
	t = t.UTC()
	switch p {
	case PeriodDaily:
		return t.Format("2006-01-02")
	case PeriodWeekly:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	case PeriodMonthly:
		return t.Format("2006-01")
	default:
		return ""
	}
}

// TTL returns how long a bounded period's Redis key should live past its
// own window (a short grace period so e.g. yesterday's leaderboard is
// still briefly readable), or 0 for no expiry (all-time). Every key is
// rebuildable from Postgres via Service.RebuildPeriod if it expires or is
// evicted early, so this is purely a memory-management knob, not a data
// durability concern.
func TTL(p Period) time.Duration {
	switch p {
	case PeriodDaily:
		return 3 * 24 * time.Hour
	case PeriodWeekly:
		return 3 * 7 * 24 * time.Hour
	case PeriodMonthly:
		return 3 * 31 * 24 * time.Hour
	default:
		return 0
	}
}

// PeriodStart returns the inclusive UTC start of the window containing t
// for period p. Weeks are ISO weeks (Monday start).
func PeriodStart(p Period, t time.Time) time.Time {
	t = t.UTC()
	switch p {
	case PeriodDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case PeriodWeekly:
		isoWeekday := int(t.Weekday())
		if isoWeekday == 0 { // time.Sunday == 0; ISO treats Sunday as day 7
			isoWeekday = 7
		}
		monday := t.AddDate(0, 0, -(isoWeekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	case PeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default: // all-time
		return time.Unix(0, 0).UTC()
	}
}

// PeriodEnd returns the exclusive UTC end of the window containing t for
// period p. For all-time, "end" is simply just past t itself (there is no
// natural end — this only exists so RebuildPeriod can use one uniform
// [start, end) range query for every period).
func PeriodEnd(p Period, t time.Time) time.Time {
	start := PeriodStart(p, t)
	switch p {
	case PeriodDaily:
		return start.AddDate(0, 0, 1)
	case PeriodWeekly:
		return start.AddDate(0, 0, 7)
	case PeriodMonthly:
		return start.AddDate(0, 1, 0)
	default: // all-time
		return t.UTC().Add(24 * time.Hour)
	}
}

// tieBreakDivisor scales a Unix-nanosecond timestamp down into a fraction
// in (0, 1) that (a) never changes a score's integer point total when
// added to it — floor(points + fraction) == points as long as fraction
// stays below 1, which holds for roughly two and a half centuries in the
// future —
// and (b) remains distinguishable from adjacent timestamps seconds-to-days
// apart at typical point totals, given float64's ~15-17 significant
// decimal digits. It stops distinguishing events within roughly tens to
// hundreds of milliseconds of each other once point totals reach the tens of
// thousands (~18ms at 10,000 points, ~145ms at 100,000 points) — an accepted,
// documented limit (see spec section 3), not a bug: RecordPoint calls are one
// per correct answer, so two DIFFERENT profiles would need to submit an
// answer within milliseconds of each other AND already be tied on points for
// this to matter, and even then the worst case is a coin-flip on ONE ranking
// position, self-correcting on the next RebuildPeriod run.
const tieBreakDivisor = 1e19

// EncodeScore combines an integer point total with a tiebreak derived from
// lastAt so that, under ZREVRANGE (descending) order, two equal point
// totals rank the EARLIER achiever higher.
//
// The tiebreak fraction is (1 - lastAt.UnixNano()/tieBreakDivisor): a
// LATER lastAt has a LARGER lastAt.UnixNano()/tieBreakDivisor term, so
// (1 - that term) is SMALLER — meaning a later timestamp contributes a
// smaller fraction and therefore a smaller score, so the earlier achiever
// ends up with the larger score and ranks first. The fraction must be
// ADDED, not subtracted: DecodePoints recovers the integer part via
// math.Floor, and floor(points - fraction) for any 0 < fraction < 1 always
// equals points-1, never points — only floor(points + fraction) recovers
// points exactly (proved wrong the naive subtractive version of this
// formula during Task 3's TDD cycle; kept here as the documented reason
// addition is required, not an arbitrary choice).
func EncodeScore(points int, lastAt time.Time) float64 {
	fraction := 1 - float64(lastAt.UnixNano())/tieBreakDivisor
	return float64(points) + fraction
}

// DecodePoints extracts the integer point total from a score produced by
// EncodeScore (or from any score read back from Redis, since every score
// this package writes went through EncodeScore).
func DecodePoints(score float64) int {
	return int(math.Floor(score))
}

// DisplayName returns name if non-empty, otherwise a stable fallback built
// from the profile's UUID (its first 4 hex characters) so the same profile
// always renders identically without ever exposing a phone number.
func DisplayName(name string, profileIDString string) string {
	if name != "" {
		return name
	}
	short := profileIDString
	if len(short) > 4 {
		short = short[:4]
	}
	return "Foydalanuvchi #" + short
}

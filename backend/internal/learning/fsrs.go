// Package learning implements FSRS-4.5 (Free Spaced Repetition Scheduler) —
// the memory model behind due-question scheduling, weak-area detection, and
// exam-readiness prediction. fsrs.go is pure (no DB); service.go integrates
// it with question_memory/category_mastery.
package learning

import (
	"math"
	"time"
)

type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// DefaultDesiredRetention is the target recall probability FSRS schedules
// reviews for (90%) — not configurable in M1.
const DefaultDesiredRetention = 0.9

// w holds the 19 FSRS-4.5 default weights (see plan Global Constraints for
// provenance).
var w = [19]float64{
	0.40255, 1.18385, 3.173, 15.69105, 7.1949, 0.5345, 1.4604, 0.0046, 1.54575,
	0.1192, 1.01925, 1.9395, 0.11, 0.29605, 2.2698, 0.2315, 2.9898, 0.51655, 0.6621,
}

const (
	factor = 19.0 / 81.0
	decay  = -0.5
)

type Card struct {
	Stability      float64
	Difficulty     float64
	DueAt          time.Time
	LastReviewedAt time.Time
	Reps           int
	Lapses         int
	State          int16 // 0 = last rating was Again, 1 = otherwise
}

// IsNew reports whether this card has never been reviewed.
func (c Card) IsNew() bool { return c.LastReviewedAt.IsZero() }

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func s0(g Rating) float64 { return w[g-1] }

func d0(g Rating) float64 {
	return clamp(w[4]-math.Exp(w[5]*float64(g-1))+1, 1, 10)
}

func retrievability(t, s float64) float64 {
	return math.Pow(1+factor*t/s, decay)
}

func interval(r, s float64) float64 {
	return (s / factor) * (math.Pow(r, 1/decay) - 1)
}

func stabilitySuccess(s, d, r float64, g Rating) float64 {
	td := 11 - d
	ts := math.Pow(s, -w[9])
	tr := math.Exp(w[10]*(1-r)) - 1
	h := 1.0
	if g == Hard {
		h = w[15]
	}
	b := 1.0
	if g == Easy {
		b = w[16]
	}
	return s * (1 + td*ts*tr*h*b*math.Exp(w[8]))
}

func stabilityFail(s, d, r float64) float64 {
	df := math.Pow(d, -w[12])
	sf := math.Pow(s+1, w[13]) - 1
	rf := math.Exp(w[14] * (1 - r))
	forgotten := w[11] * df * sf * rf
	return math.Min(forgotten, s)
}

func difficultyUpdate(d float64, g Rating) float64 {
	deltaD := -w[6] * float64(g-3)
	dPrime := d + deltaD*((10-d)/9)
	return clamp(w[7]*d0(Easy)+(1-w[7])*dPrime, 1, 10)
}

// Review computes the next Card state after grading a review at time now,
// targeting desiredRetention (e.g. DefaultDesiredRetention).
func Review(c Card, rating Rating, now time.Time, desiredRetention float64) Card {
	var newStability, newDifficulty float64
	if c.IsNew() {
		newStability = s0(rating)
		newDifficulty = d0(rating)
	} else {
		t := now.Sub(c.LastReviewedAt).Hours() / 24
		r := retrievability(t, c.Stability)
		if rating == Again {
			newStability = stabilityFail(c.Stability, c.Difficulty, r)
		} else {
			newStability = stabilitySuccess(c.Stability, c.Difficulty, r, rating)
		}
		newDifficulty = difficultyUpdate(c.Difficulty, rating)
	}

	days := interval(desiredRetention, newStability)
	if days < 1 {
		days = 1
	}
	state := int16(1)
	lapses := c.Lapses
	if rating == Again {
		state = 0
		lapses++
	}

	return Card{
		Stability:      newStability,
		Difficulty:     newDifficulty,
		DueAt:          now.Add(time.Duration(math.Round(days)) * 24 * time.Hour),
		LastReviewedAt: now,
		Reps:           c.Reps + 1,
		Lapses:         lapses,
		State:          state,
	}
}

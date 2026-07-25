package arena

import (
	"math"
	"time"
)

// Bucket maps an ELO-like rating into a matchmaking bucket index.
func Bucket(rating int) int {
	if rating < 0 {
		rating = 0
	}
	return rating / 100
}

// SearchBuckets returns bucket indices to scan at the given wait duration
// (own bucket first, then widening ±1, ±2, …).
func SearchBuckets(own int, waited time.Duration) []int {
	steps := int(waited / (5 * time.Second))
	if steps < 0 {
		steps = 0
	}
	if steps > 8 {
		steps = 8
	}
	out := make([]int, 0, 1+2*steps)
	out = append(out, own)
	for i := 1; i <= steps; i++ {
		out = append(out, own-i, own+i)
	}
	return out
}

// AnswerPoints awards speed-weighted points for a correct answer.
// Max 100 at instant response; 0 at/after the window.
func AnswerPoints(correct bool, responseMs, windowMs int64) int {
	if !correct || responseMs < 0 || windowMs <= 0 {
		return 0
	}
	if responseMs >= windowMs {
		return 0
	}
	frac := 1 - float64(responseMs)/float64(windowMs)
	return int(math.Round(100 * frac))
}

// OutcomeFromScores returns won/lost/draw for player A relative to B.
func OutcomeFromScores(scoreA, scoreB int) (outA, outB string) {
	switch {
	case scoreA > scoreB:
		return "won", "lost"
	case scoreA < scoreB:
		return "lost", "won"
	default:
		return "draw", "draw"
	}
}

// ExpectedScore is classic ELO expected score for ratingA vs ratingB.
func ExpectedScore(ratingA, ratingB int) float64 {
	return 1 / (1 + math.Pow(10, float64(ratingB-ratingA)/400))
}

// EloDelta returns rating change for A given outcome (1=win, 0.5=draw, 0=loss).
func EloDelta(ratingA, ratingB int, score float64, k float64) int {
	if k <= 0 {
		k = 32
	}
	exp := ExpectedScore(ratingA, ratingB)
	return int(math.Round(k * (score - exp)))
}

// MedalForRating maps rating to a display medal tier (M4-04).
func MedalForRating(rating int) string {
	switch {
	case rating >= 2000:
		return "brilliant"
	case rating >= 1800:
		return "diamond"
	case rating >= 1600:
		return "platinum"
	case rating >= 1400:
		return "gold"
	case rating >= 1200:
		return "silver"
	default:
		return "bronze"
	}
}

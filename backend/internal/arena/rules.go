package arena

import (
	"crypto/rand"
	"math"
	"strings"
	"time"
)

// Bucket maps an ELO-like rating into a matchmaking bucket index.
func Bucket(rating int) int {
	if rating < 0 {
		rating = 0
	}
	return rating / 100
}

// MaxSearchSteps is how far (in ±100-rating buckets) a joining player looks
// for someone already waiting. Wide on purpose: the arena is small, and a
// duel against a player 500 points away beats no duel. The closest bucket
// still wins whenever it has anyone in it.
const MaxSearchSteps = 8

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

// BotAccuracy is how often the practice bot answers correctly against a
// player of the given rating: a beatable opponent for a beginner, a real one
// for a strong player, never a wall.
func BotAccuracy(rating int) float64 {
	acc := 0.6 + float64(rating-1000)/1000*0.3
	return math.Max(0.45, math.Min(0.85, acc))
}

// BotDelay maps a uniform r in [0,1) to the bot's thinking time: somewhere
// between a quick and a slow human, always well inside the window.
func BotDelay(r float64, window time.Duration) time.Duration {
	lo, hi := 0.2, 0.7
	if r < 0 {
		r = 0
	}
	if r >= 1 {
		r = 0.999
	}
	return time.Duration(float64(window) * (lo + (hi-lo)*r))
}

// inviteAlphabet has no 0/O, 1/I/L: the code is read aloud and typed by hand.
const inviteAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// InviteCodeLen keeps a code short enough to dictate; 31^6 ≈ 887M codes over
// a 10-minute lifetime is far beyond what the rate limit lets anyone probe.
const InviteCodeLen = 6

func NewInviteCode() (string, error) {
	buf := make([]byte, InviteCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, InviteCodeLen)
	for i, b := range buf {
		// 256 % 31 != 0, so this is very slightly biased; irrelevant for a
		// short-lived code that is not a secret credential.
		out[i] = inviteAlphabet[int(b)%len(inviteAlphabet)]
	}
	return string(out), nil
}

// NormalizeInviteCode accepts what a person actually types or pastes:
// lower case, spaces, dashes. It returns "" for anything that cannot be a code.
func NormalizeInviteCode(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		switch {
		case r == ' ' || r == '-' || r == '\t':
			continue
		case strings.ContainsRune(inviteAlphabet, r):
			b.WriteRune(r)
		default:
			return ""
		}
	}
	if b.Len() != InviteCodeLen {
		return ""
	}
	return b.String()
}

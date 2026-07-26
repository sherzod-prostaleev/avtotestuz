package session

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
)

// shuffleSessionAnswers permutes answer options with a seed derived from
// sessionID+questionID. The order is stable across resume/reload so the
// learner cannot memorize fixed slots, but also cannot get a new layout mid-session.
func shuffleSessionAnswers(answers []content.AnswerDTO, sessionID, questionID uuid.UUID) []content.AnswerDTO {
	n := len(answers)
	if n <= 1 {
		return answers
	}
	seedBytes := make([]byte, 0, 32)
	seedBytes = append(seedBytes, sessionID[:]...)
	seedBytes = append(seedBytes, questionID[:]...)
	sum := sha256.Sum256(seedBytes)
	seed := int64(binary.BigEndian.Uint64(sum[:8]))
	r := rand.New(rand.NewSource(seed))

	out := make([]content.AnswerDTO, n)
	copy(out, answers)
	r.Shuffle(n, func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	// Keep each answer's DB position; UI labels (F1/F2…) use array index.
	// Scoring always uses answer IDs, so order is cosmetic only.
	return out
}

package session

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"regexp"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
)

// rePositional matches answers that rely on option order or ordinal position:
// - F-key references: F1, F2, F3, F4, F5
// - Numbered references: "1 va 2", "2 va 3", "1 va 3", "1, 2", "1- va 2-", "1 yoki 2", "1 и 2", "2 и 3", "1 и 3", "в ответах"
// - Ordinal words: "birinchi va ikkinchi", "ikkinchi va uchinchi", "birinchi va uchinchi"
// - Collective options: "barcha javoblar", "barcha keltirilgan", "ko'rsatilgan barcha", "barcha hollarda", "barchasi", "hammasi", "yuqoridagi barcha", "все ответы", "во всех перечисленных", "все вышеперечисленные"
var rePositional = regexp.MustCompile(`(?i)(` +
	`\bF[1-5]\b|` +
	`1\s*(va|va\s*yoki|yoki|и|или)\s*2|` +
	`2\s*(va|va\s*yoki|yoki|и|или)\s*3|` +
	`1\s*(va|va\s*yoki|yoki|и|или)\s*3|` +
	`1,\s*2|1,\s*3|2,\s*3|1,\s*2,\s*3|` +
	`1-?\s*va\s*2|2-?\s*va\s*3|1-?\s*va\s*3|` +
	`birinchi\s*va\s*ikkinchi|ikkinchi\s*va\s*uchinchi|birinchi\s*va\s*uchinchi|` +
	`в\s*ответах|во\s*2\s*и\s*3|в\s*1\s*и\s*2|в\s*1\s*и\s*3|` +
	`barcha\s*javob|barcha\s*keltirilgan|barcha\s*ko['‘]rsatilgan|barcha\s*hollarda|` +
	`barchasi|hammasi|yuqoridagi\s*barcha|ko['‘]rsatilgan\s*barcha|` +
	`все\s*ответы|во\s*всех\s*перечисленных|все\s*вышеперечисленные` +
	`)`)

// isOrderSensitive checks if any answer in the list refers to slot positions or is collective.
func isOrderSensitive(answers []content.AnswerDTO) bool {
	for _, a := range answers {
		if rePositional.MatchString(a.Text) {
			return true
		}
	}
	return false
}

// shuffleSessionAnswers permutes answer options with a seed derived from
// sessionID+questionID for independent questions, while preserving original DB order
// (position 1..5) for questions whose options reference specific slots ("F1 va F2",
// "1 va 2 javoblar", "Barcha javoblar to'g'ri", etc.).
func shuffleSessionAnswers(answers []content.AnswerDTO, sessionID, questionID uuid.UUID) []content.AnswerDTO {
	n := len(answers)
	if n <= 1 || isOrderSensitive(answers) {
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

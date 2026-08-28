package session

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"regexp"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
)

// rePositional matches an option that only makes sense in its authored slot:
//
//   - a slot reference: "F1 va F2 javoblar to'g'ri", "F2 va F3 javoblarda"
//   - a numeric or ordinal cross-reference: "1 va 2", "2 va 4", "1, 2",
//     "2- va 3-chiziq", "Birinchi va ikkinchi rasmda"
//   - a collective option — the "all of the above" that must stay last:
//     "Barcha javoblar to'g'ri", "Sanab o'tilgan barcha hollarda", "Hammasi"
//
// It is matched against AnswerDTO.TextUzLatn, never the localized text. The
// three translations are worded independently, so a per-locale decision
// disagreed with itself: of the 121 questions this protected in at least one
// language, only 10 were protected in all three — uz-Cyrl "1 ва 2" and ru
// "во втором и третьем ответах" both slipped past a pattern list built from
// uz-Latn phrasing, and those questions shuffled for exactly the readers the
// rule exists to protect. Deciding on one locale makes the verdict a property
// of the question, so it cannot vary by reader.
//
// Two deliberate exclusions, both verified against the live bank: the comma
// form stays an explicit list ("1,2", "1,3", "2,3") because a general
// [1-5],[1-5] also swallows decimals like "3,5 tonna" and "2,5 mm"; and the
// va/yoki form requires the digits to stand alone, because "5.15.1 yoki
// 5.15.2" is a pair of sign codes, not a pair of options.
var rePositional = regexp.MustCompile(`(?i)(` +
	`\bF\s*[1-5]\b` +
	`|(?:^|[\s(])[1-5]\s*-?\s*(?:va|yoki)\s*-?\s*[1-5](?:[-\s,.;)]|$)` +
	`|1,\s*2|1,\s*3|2,\s*3|1,\s*2,\s*3` +
	`|\b(?:birinchi|ikkinchi|uchinchi|to['‘]rtinchi|beshinchi)\s+va\s+` +
	`(?:birinchi|ikkinchi|uchinchi|to['‘]rtinchi|beshinchi)\b` +
	`|\b(?:barchasi|hammasi)` +
	`|\b(?:barcha|hamma)\s+(?:javob|hol|aytilgan|keltirilgan|ko['‘]rsatilgan|sanab|yuqorida)` +
	`|sanab\s+o['‘]tilgan` +
	`|yuqoridagi\s+(?:barcha|hamma)` +
	`|ko['‘]rsatilgan\s+(?:barcha|hamma)|keltirilgan\s+(?:barcha|hamma)` +
	`)`)

// isOrderSensitive reports whether any option pins the question to its
// authored order. It reads TextUzLatn so the answer is the same for every
// reader; TextUzLatn falls back to the localized text only when a question has
// no uz-Latn translation at all, which the importer does not allow.
func isOrderSensitive(answers []content.AnswerDTO) bool {
	for _, a := range answers {
		text := a.TextUzLatn
		if text == "" {
			text = a.Text
		}
		if rePositional.MatchString(text) {
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

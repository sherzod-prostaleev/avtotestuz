package session

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"avtotest.uz/backend/internal/content"
)

// seedPath is the shipped question bank. It is the same file the importer
// reads, so this test measures the rule against the content that actually
// reaches learners rather than a handful of invented strings.
const seedPath = "../../seed/avtoimtihon/data.json"

type seedAnswer struct {
	Correct bool              `json:"correct"`
	Texts   map[string]string `json:"texts"`
}

type seedQuestion struct {
	ExtID   string       `json:"ext_id"`
	Answers []seedAnswer `json:"answers"`
}

type seedBank struct {
	Questions []seedQuestion `json:"questions"`
}

func loadSeedBank(t *testing.T) seedBank {
	t.Helper()
	raw, err := os.ReadFile(seedPath)
	if err != nil {
		t.Skipf("seed bank not readable at %s: %v", seedPath, err)
	}
	var bank seedBank
	if err := json.Unmarshal(raw, &bank); err != nil {
		t.Fatalf("parse %s: %v", seedPath, err)
	}
	if len(bank.Questions) == 0 {
		t.Fatalf("%s has no questions", seedPath)
	}
	return bank
}

// answersIn renders a seed question the way the API would for one locale:
// Text carries that locale, TextUzLatn always carries uz-Latn.
func answersIn(q seedQuestion, locale string) []content.AnswerDTO {
	out := make([]content.AnswerDTO, 0, len(q.Answers))
	for i, a := range q.Answers {
		out = append(out, content.AnswerDTO{
			ID:         q.ExtID + "-" + string(rune('a'+i)),
			Position:   int16(i + 1),
			Text:       a.Texts[locale],
			TextUzLatn: a.Texts["uz-Latn"],
		})
	}
	return out
}

// Whether a question keeps its option order must be a property of the question.
// This is the regression that shipped: the rule was evaluated on the localized
// text, so of the 121 questions it protected in at least one language only 10
// were protected in all three — uz-Cyrl protected 11 and ru 60.
func TestOrderLockDoesNotVaryByLocale(t *testing.T) {
	bank := loadSeedBank(t)
	locales := []string{"uz-Latn", "uz-Cyrl", "ru"}

	var disagreed []string
	for _, q := range bank.Questions {
		first := isOrderSensitive(answersIn(q, locales[0]))
		for _, locale := range locales[1:] {
			if isOrderSensitive(answersIn(q, locale)) != first {
				disagreed = append(disagreed, q.ExtID)
				break
			}
		}
	}
	if len(disagreed) > 0 {
		sort.Strings(disagreed)
		shown := disagreed
		if len(shown) > 10 {
			shown = shown[:10]
		}
		t.Fatalf("%d questions lock in some locales and shuffle in others: %v",
			len(disagreed), shown)
	}
}

// Named questions that must never reorder, each standing for one shape of the
// rule. They are pinned by ext_id because each was found shuffling in at least
// one locale before the fix.
func TestOrderLockCoversKnownSlotReferencingQuestions(t *testing.T) {
	bank := loadSeedBank(t)
	byExt := make(map[string]seedQuestion, len(bank.Questions))
	for _, q := range bank.Questions {
		byExt[q.ExtID] = q
	}

	cases := map[string]string{
		"avtoimtihon-322":  `answer cites other answers ("F2 va F3 javoblarda")`,
		"avtoimtihon-1025": `answer cites other answers ("F1 va F2 javoblar to'g'ri")`,
		"avtoimtihon-1113": `numeric cross-reference ("1 va 2")`,
		"avtoimtihon-1121": `numeric cross-reference ("Faqat 1 va 2")`,
		"avtoimtihon-1048": `ordinal cross-reference ("Ikkinchi va uchinchi rasmlarda")`,
		"avtoimtihon-1047": `numeric pair the old list missed ("2 va 4")`,
		"avtoimtihon-192":  `ordinal pair the old list missed ("Birinchi va beshinchi")`,
		"avtoimtihon-80":   `collective option ("Barcha sanab o'tilgan hollarda")`,
		"avtoimtihon-328":  `collective option without "barcha" ("Sanab o'tilgan hollarda")`,
		"avtoimtihon-140":  `collective option ("Hammasi")`,
	}

	for ext, why := range cases {
		q, ok := byExt[ext]
		if !ok {
			t.Errorf("%s missing from the bank (%s)", ext, why)
			continue
		}
		for _, locale := range []string{"uz-Latn", "uz-Cyrl", "ru"} {
			if !isOrderSensitive(answersIn(q, locale)) {
				t.Errorf("%s must keep its order in %s: %s", ext, locale, why)
			}
		}
	}
}

// The rule must stay a narrow exception. Freezing the bank would silently undo
// shuffling for everyone, so this pins the share of locked questions: it caught
// a draft where a bare "barcha" also matched plain options like "Barcha
// yo'nalishlarga" and locked 275 questions instead of 155.
func TestOrderLockStaysAnException(t *testing.T) {
	bank := loadSeedBank(t)
	locked := 0
	for _, q := range bank.Questions {
		if isOrderSensitive(answersIn(q, "uz-Latn")) {
			locked++
		}
	}
	total := len(bank.Questions)
	if locked == 0 {
		t.Fatalf("no question is order-locked; the rule stopped matching anything")
	}
	if locked*100/total > 20 {
		t.Fatalf("order-locked %d of %d questions (%d%%): the rule is too broad, "+
			"shuffling is effectively off", locked, total, locked*100/total)
	}
}

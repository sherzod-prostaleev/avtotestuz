package session

import (
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
)

func TestShuffleSessionAnswersIsDeterministicAndPermutes(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	questionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	input := []content.AnswerDTO{
		{ID: "a", Position: 1, Text: "A"},
		{ID: "b", Position: 2, Text: "B"},
		{ID: "c", Position: 3, Text: "C"},
		{ID: "d", Position: 4, Text: "D"},
	}

	first := shuffleSessionAnswers(input, sessionID, questionID)
	second := shuffleSessionAnswers(input, sessionID, questionID)
	if len(first) != len(input) {
		t.Fatalf("len=%d want %d", len(first), len(input))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("not deterministic: %v vs %v", ids(first), ids(second))
		}
	}
	byID := map[string]int16{"a": 1, "b": 2, "c": 3, "d": 4}
	for _, a := range first {
		if a.Position != byID[a.ID] {
			t.Fatalf("answer %s position mutated to %d", a.ID, a.Position)
		}
	}

	sameOrder := true
	for i := range first {
		if first[i].ID != input[i].ID {
			sameOrder = false
			break
		}
	}
	if sameOrder {
		t.Fatalf("expected a permutation for this seed, got original order %v", ids(first))
	}

	// Different session → different layout (almost always; assert not identical).
	otherSession := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	other := shuffleSessionAnswers(input, otherSession, questionID)
	identical := true
	for i := range first {
		if first[i].ID != other[i].ID {
			identical = false
			break
		}
	}
	if identical {
		t.Fatalf("different session should change order; both=%v", ids(first))
	}

	// Original slice untouched.
	if input[0].ID != "a" || input[0].Position != 1 {
		t.Fatalf("input mutated: %+v", input)
	}
}

func TestShufflePreservesOrderForPositionalAndCollectiveAnswers(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	questionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	testCases := []struct {
		name    string
		answers []content.AnswerDTO
	}{
		{
			name: "F-key reference (avtoimtihon-322)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "Yo'nalishli taksilarga"},
				{ID: "2", Position: 2, Text: "Taksometri ishlab turgan taksilarga"},
				{ID: "3", Position: 3, Text: "«Nogiron» taniqlik belgisi o'rnatilgan nogiron boshqarayotgan avtomobilga"},
				{ID: "4", Position: 4, Text: "Belgilangan yo'nalishdagi transport vositalariga"},
				{ID: "5", Position: 5, Text: "F2 va F3 javoblarda ko'rsatilgan transport vositalariga"},
			},
		},
		{
			name: "F1 va F2 (avtoimtihon-1025)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "12 yoshgacha"},
				{ID: "2", Position: 2, Text: "Homilador ayollarga"},
				{ID: "3", Position: 3, Text: "Orqaga harakatlanayotgan"},
				{ID: "4", Position: 4, Text: "F1 va F2 javoblar to'g'ri"},
				{ID: "5", Position: 5, Text: "Barcha hollarda"},
			},
		},
		{
			name: "Numbered reference (1 va 2)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "Faqat birinchi rasmda"},
				{ID: "2", Position: 2, Text: "Faqat ikkinchi rasmda"},
				{ID: "3", Position: 3, Text: "1 va 2 javoblarda"},
			},
		},
		{
			name: "Collective option (Barcha javoblar to'g'ri)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "Tunnellarda"},
				{ID: "2", Position: 2, Text: "Chorrahalarda"},
				{ID: "3", Position: 3, Text: "Piyodalarning o'tish joyida"},
				{ID: "4", Position: 4, Text: "Barcha javoblar to'g'ri"},
			},
		},
		{
			// Rendered in Russian, decided on uz-Latn: that is the production
			// shape, and the reason the rule no longer needs Russian patterns.
			name: "Russian collective option (Все ответы правильные)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "Вариант 1", TextUzLatn: "Birinchi variant"},
				{ID: "2", Position: 2, Text: "Вариант 2", TextUzLatn: "Ikkinchi variant"},
				{ID: "3", Position: 3, Text: "Все ответы правильные", TextUzLatn: "Barcha javoblar to'g'ri"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := shuffleSessionAnswers(tc.answers, sessionID, questionID)
			for i := range tc.answers {
				if res[i].ID != tc.answers[i].ID {
					t.Fatalf("expected order preserved for %q; got %v want %v", tc.name, ids(res), ids(tc.answers))
				}
			}
		})
	}
}

// The reader's language must not decide whether a question keeps its option
// order: "F2 va F3 javoblarda" is a slot reference in Uzbek and reads as "во
// втором и третьем ответах" in Russian, which no uz-Latn pattern list can
// match. Before the fix this question shuffled for Russian readers only.
func TestShuffleDecidesOnUzLatnNotReaderLocale(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	questionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	// avtoimtihon-322, rendered in each of the three locales.
	byLocale := map[string][]string{
		"uz-Latn": {
			"Yo'nalishli taksilarga",
			"Taksometri ishlab turgan taksilarga",
			"«Nogiron» taniqlik belgisi o'rnatilgan avtomobilga",
			"Belgilangan yo'nalishdagi transport vositalariga",
			"F2 va F3 javoblarda ko'rsatilgan transport vositalariga",
		},
		"uz-Cyrl": {
			"Йўналишли таксиларга",
			"Таксометри ишлаб турган таксиларга",
			"«Ногирон» таниқлик белгиси ўрнатилган автомобилга",
			"Белгиланган йўналишдаги транспорт воситаларига",
			"F2 ва F3 жавобларда кўрсатилган транспорт воситаларига",
		},
		"ru": {
			"Маршрутное такси",
			"Такси с включенным таксометром",
			"На автомобиль, управляемый инвалидом",
			"Маршрутные транспортные средства",
			"Транспортные средства, указанные во втором и третьем ответах",
		},
	}
	uzLatn := byLocale["uz-Latn"]

	for locale, texts := range byLocale {
		answers := make([]content.AnswerDTO, len(texts))
		for i, text := range texts {
			answers[i] = content.AnswerDTO{
				ID:         string(rune('a' + i)),
				Position:   int16(i + 1),
				Text:       text,
				TextUzLatn: uzLatn[i],
			}
		}
		got := shuffleSessionAnswers(answers, sessionID, questionID)
		for i := range answers {
			if got[i].ID != answers[i].ID {
				t.Fatalf("%s: order changed on a slot-referencing question; got %v", locale, ids(got))
			}
		}
	}
}

// A question with no slot reference must still shuffle, in every locale, or the
// fix would have simply frozen the whole bank.
func TestShuffleStillPermutesPlainQuestionsInEveryLocale(t *testing.T) {
	sessionID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	questionID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	uzLatn := []string{"Chapga", "O'ngga", "To'g'riga", "Orqaga"}
	for _, texts := range [][]string{
		uzLatn,
		{"Чапга", "Ўнгга", "Тўғрига", "Орқага"},
		{"Налево", "Направо", "Прямо", "Назад"},
	} {
		answers := make([]content.AnswerDTO, len(texts))
		for i, text := range texts {
			answers[i] = content.AnswerDTO{
				ID:         string(rune('a' + i)),
				Position:   int16(i + 1),
				Text:       text,
				TextUzLatn: uzLatn[i],
			}
		}
		got := shuffleSessionAnswers(answers, sessionID, questionID)
		same := true
		for i := range answers {
			if got[i].ID != answers[i].ID {
				same = false
				break
			}
		}
		if same {
			t.Fatalf("plain question kept its order for this seed; got %v", ids(got))
		}
	}
}

func ids(answers []content.AnswerDTO) []string {
	out := make([]string, len(answers))
	for i, a := range answers {
		out[i] = a.ID
	}
	return out
}

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
			name: "Russian collective option (Все ответы правильные)",
			answers: []content.AnswerDTO{
				{ID: "1", Position: 1, Text: "Вариант 1"},
				{ID: "2", Position: 2, Text: "Вариант 2"},
				{ID: "3", Position: 3, Text: "Все ответы правильные"},
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

func ids(answers []content.AnswerDTO) []string {
	out := make([]string, len(answers))
	for i, a := range answers {
		out[i] = a.ID
	}
	return out
}

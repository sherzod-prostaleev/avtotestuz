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

func ids(answers []content.AnswerDTO) []string {
	out := make([]string, len(answers))
	for i, a := range answers {
		out[i] = a.ID
	}
	return out
}

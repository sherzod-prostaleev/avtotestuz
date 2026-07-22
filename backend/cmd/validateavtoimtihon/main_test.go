package main

import "testing"

func q(id int, answers int, correct int, image, ticket string) srcQuestion {
	list := make([]string, answers)
	for i := range list {
		list[i] = "a"
	}
	return srcQuestion{ID: id, Answers: list, CorrectAnswer: correct, Image: image, Ticket: ticket}
}

func kinds(problems []Problem) map[string]int {
	out := map[string]int{}
	for _, p := range problems {
		out[p.Kind]++
	}
	return out
}

func TestValidateAcceptsAlignedLocales(t *testing.T) {
	data := map[string]map[int]srcQuestion{}
	for _, loc := range locales {
		data[loc] = map[int]srcQuestion{1: q(1, 3, 2, "img.webp", "7")}
	}

	if got := validate(data); len(got) != 0 {
		t.Fatalf("expected no problems, got %+v", got)
	}
}

// The failure that actually shipped: same id, different question per locale.
// It surfaces as a differing answer count and a differing image.
func TestValidateDetectsMisalignedLocales(t *testing.T) {
	data := map[string]map[int]srcQuestion{
		"uz-Latn": {1: q(1, 3, 1, "a.webp", "1")},
		"uz-Cyrl": {1: q(1, 3, 1, "a.webp", "1")},
		"ru":      {1: q(1, 4, 4, "b.webp", "1")},
	}

	got := kinds(validate(data))
	if got["answer-count-mismatch"] != 1 {
		t.Fatalf("answer-count-mismatch = %d, want 1 (%+v)", got["answer-count-mismatch"], got)
	}
	if got["image-mismatch"] != 1 {
		t.Fatalf("image-mismatch = %d, want 1 (%+v)", got["image-mismatch"], got)
	}
}

func TestValidateDetectsMissingAndOutOfRange(t *testing.T) {
	data := map[string]map[int]srcQuestion{
		"uz-Latn": {1: q(1, 3, 9, "a.webp", "1"), 2: q(2, 3, 1, "b.webp", "1")},
		"uz-Cyrl": {1: q(1, 3, 1, "a.webp", "1")},
		"ru":      {1: q(1, 3, 1, "a.webp", "1")},
	}

	got := kinds(validate(data))
	if got["missing-in-locale"] != 1 {
		t.Fatalf("missing-in-locale = %d, want 1 (%+v)", got["missing-in-locale"], got)
	}
	if got["correct-answer-out-of-range"] != 1 {
		t.Fatalf("correct-answer-out-of-range = %d, want 1 (%+v)", got["correct-answer-out-of-range"], got)
	}
}

func TestValidateDetectsTicketMismatch(t *testing.T) {
	data := map[string]map[int]srcQuestion{
		"uz-Latn": {1: q(1, 3, 1, "a.webp", "1")},
		"uz-Cyrl": {1: q(1, 3, 1, "a.webp", "1")},
		"ru":      {1: q(1, 3, 1, "a.webp", "9")},
	}

	if got := kinds(validate(data)); got["ticket-mismatch"] != 1 {
		t.Fatalf("ticket-mismatch = %d, want 1 (%+v)", got["ticket-mismatch"], got)
	}
}

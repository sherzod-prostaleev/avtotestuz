package importer

import "testing"

func TestContentHashStableAndSensitive(t *testing.T) {
	q1 := validQuestion("q-1")
	q2 := validQuestion("q-1")
	if ContentHash(q1) != ContentHash(q2) {
		t.Fatal("same content must hash equal")
	}
	q2.Texts = map[string]string{"uz-Latn": "S", "uz-Cyrl": "С", "ru": "boshqa"}
	if ContentHash(q1) == ContentHash(q2) {
		t.Fatal("text change must change hash")
	}
	q3 := validQuestion("q-1")
	q3.Answers[1].Correct = false
	q3.Answers[2].Correct = true
	if ContentHash(q1) == ContentHash(q3) {
		t.Fatal("correct-answer change must change hash")
	}
}

package arena_test

import (
	"testing"
	"time"

	"avtotest.uz/backend/internal/arena"
)

func TestBucket(t *testing.T) {
	if got := arena.Bucket(1050); got != 10 {
		t.Fatalf("Bucket(1050)=%d want 10", got)
	}
	if got := arena.Bucket(-5); got != 0 {
		t.Fatalf("Bucket(-5)=%d want 0", got)
	}
}

func TestSearchBucketsWidens(t *testing.T) {
	got := arena.SearchBuckets(10, 0)
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("wait0 = %v", got)
	}
	got = arena.SearchBuckets(10, 5*time.Second)
	if len(got) != 3 || got[0] != 10 || got[1] != 9 || got[2] != 11 {
		t.Fatalf("wait5s = %v", got)
	}
}

func TestAnswerPoints(t *testing.T) {
	if p := arena.AnswerPoints(false, 100, 15000); p != 0 {
		t.Fatalf("wrong=%d", p)
	}
	if p := arena.AnswerPoints(true, 0, 15000); p != 100 {
		t.Fatalf("instant=%d", p)
	}
	if p := arena.AnswerPoints(true, 15000, 15000); p != 0 {
		t.Fatalf("at_window=%d", p)
	}
}

func TestOutcomeFromScores(t *testing.T) {
	a, b := arena.OutcomeFromScores(10, 5)
	if a != "won" || b != "lost" {
		t.Fatalf("%s/%s", a, b)
	}
	a, b = arena.OutcomeFromScores(3, 3)
	if a != "draw" || b != "draw" {
		t.Fatalf("%s/%s", a, b)
	}
}

func TestEncodeDecode(t *testing.T) {
	b, err := arena.Encode("hello", arena.HelloData{Protocol: 1})
	if err != nil {
		t.Fatal(err)
	}
	env, err := arena.Decode(b)
	if err != nil || env.T != "hello" || env.V != 1 {
		t.Fatalf("%+v %v", env, err)
	}
}

func TestDecodeRejectsBadVersion(t *testing.T) {
	_, err := arena.Decode([]byte(`{"v":99,"t":"hello","d":{}}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEloAndMedal(t *testing.T) {
	d := arena.EloDelta(1000, 1000, 1, 32)
	if d <= 0 {
		t.Fatalf("delta=%d", d)
	}
	if m := arena.MedalForRating(1000); m != "bronze" {
		t.Fatalf("medal=%s", m)
	}
	if m := arena.MedalForRating(2100); m != "brilliant" {
		t.Fatalf("medal=%s", m)
	}
}

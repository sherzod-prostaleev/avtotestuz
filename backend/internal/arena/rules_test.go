package arena_test

import (
	"strings"
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

func TestInviteCodeRoundTrip(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := arena.NewInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != arena.InviteCodeLen {
			t.Fatalf("code %q has length %d", code, len(code))
		}
		if strings.ContainsAny(code, "01OIL") {
			t.Fatalf("code %q has an ambiguous character", code)
		}
		if arena.NormalizeInviteCode(code) != code {
			t.Fatalf("code %q does not normalize to itself", code)
		}
		seen[code] = true
	}
	if len(seen) < 190 {
		t.Fatalf("codes collide too often: %d unique of 200", len(seen))
	}
}

func TestNormalizeInviteCodeAcceptsWhatPeopleType(t *testing.T) {
	cases := map[string]string{
		"abc234":    "ABC234",
		" AB-C2 34": "ABC234",
		"ABC23":     "",
		"ABC2345":   "",
		"ABC0O1":    "",
		"ABC23<":    "",
	}
	for in, want := range cases {
		if got := arena.NormalizeInviteCode(in); got != want {
			t.Errorf("NormalizeInviteCode(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBotStaysBeatableAndInsideTheWindow(t *testing.T) {
	for _, r := range []int{0, 600, 1000, 1400, 3000} {
		acc := arena.BotAccuracy(r)
		if acc < 0.45 || acc > 0.85 {
			t.Fatalf("BotAccuracy(%d)=%v out of [0.45,0.85]", r, acc)
		}
	}
	if arena.BotAccuracy(1400) <= arena.BotAccuracy(900) {
		t.Fatal("bot must play stronger against a stronger player")
	}
	window := 15 * time.Second
	for _, r := range []float64{-1, 0, 0.5, 0.999, 1, 2} {
		d := arena.BotDelay(r, window)
		if d <= 0 || d >= window {
			t.Fatalf("BotDelay(%v)=%v outside (0,%v)", r, d, window)
		}
	}
}

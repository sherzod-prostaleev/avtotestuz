package demo

import (
	"testing"

	"github.com/google/uuid"
)

func TestWhitelistTakesFirstDemoQuestionCount(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	got := Whitelist(ids)
	if len(got) != demoQuestionCount {
		t.Fatalf("len=%d want %d", len(got), demoQuestionCount)
	}
	for i, id := range got {
		if id != ids[i] {
			t.Fatalf("whitelist[%d]=%s want %s (must be the ordered prefix, not arbitrary picks)", i, id, ids[i])
		}
	}
}

func TestWhitelistShorterThanCount(t *testing.T) {
	ids := []uuid.UUID{uuid.New()}
	got := Whitelist(ids)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1 (fewer questions than demoQuestionCount must not panic or pad)", len(got))
	}
}

func TestDiagnosticWhitelistTakesTenOrderedQuestions(t *testing.T) {
	ids := make([]uuid.UUID, 12)
	for i := range ids {
		ids[i] = uuid.New()
	}
	got := DiagnosticWhitelist(ids)
	if len(got) != diagnosticQuestionCount {
		t.Fatalf("len=%d want %d", len(got), diagnosticQuestionCount)
	}
	for i, id := range got {
		if id != ids[i] {
			t.Fatalf("diagnostic[%d]=%s want %s", i, id, ids[i])
		}
	}
}

func TestWhitelistEmpty(t *testing.T) {
	got := Whitelist(nil)
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
}

func TestIsWhitelisted(t *testing.T) {
	in, out := uuid.New(), uuid.New()
	wl := []uuid.UUID{in}
	if !IsWhitelisted(wl, in) {
		t.Fatal("expected in to be whitelisted")
	}
	if IsWhitelisted(wl, out) {
		t.Fatal("expected out to NOT be whitelisted")
	}
}

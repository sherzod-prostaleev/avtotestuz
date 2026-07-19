package main

import (
	"os"
	"testing"

	"avtotest.uz/backend/internal/importer"
)

// srcRoot is the user-owned avtoimtihon dataset root. The test is skipped when
// it is not present (e.g. CI without the local content).
const srcRoot = "/home/sher/Рабочий стол/aaa"

func loadOrSkip(t *testing.T) Result {
	t.Helper()
	if _, err := os.Stat(srcRoot + "/src/data/questions.uz-Latn.json"); err != nil {
		t.Skipf("source dataset not present at %s: %v", srcRoot, err)
	}
	res, err := Convert(srcRoot)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return res
}

func TestConvertValidates(t *testing.T) {
	res := loadOrSkip(t)

	if got := len(res.Dataset.Questions); got != 1235 {
		t.Errorf("questions = %d, want 1235", got)
	}
	if got := len(res.Dataset.Variants); got != 61 {
		t.Errorf("variants = %d, want 61", got)
	}
	for _, v := range res.Dataset.Variants {
		if len(v.Questions) != 20 {
			t.Errorf("variant %d has %d questions, want 20", v.Number, len(v.Questions))
		}
	}
	if got := len(res.LeftoverExtI); got != 15 {
		t.Errorf("leftover = %d, want 15", got)
	}
	if got := len(res.NoComment); got != 16 {
		t.Errorf("no-comment questions = %d, want 16", got)
	}

	issues := importer.Validate(res.Dataset)
	quarantined := importer.QuarantinedIDs(issues)
	if len(quarantined) != 0 {
		t.Errorf("quarantined = %d, want 0", len(quarantined))
	}
	// The only tolerable issue category would be informational; assert there are
	// none of the data-quality codes on any question.
	for _, is := range issues {
		switch is.Code {
		case "answers_count", "no_correct", "multiple_correct", "missing_locale", "unknown_category", "duplicate_ext_id":
			t.Errorf("unexpected data-quality issue: %+v", is)
		default:
			t.Errorf("unexpected issue: %+v", is)
		}
	}
}

func TestConvertQuestionFidelity(t *testing.T) {
	res := loadOrSkip(t)
	byExt := map[string]importer.CanonQuestion{}
	for _, q := range res.Dataset.Questions {
		byExt[q.ExtID] = q
	}

	// id 1: 4 answers, correct_answer=4, image i1_1.
	q1 := byExt["avtoimtihon-1"]
	if len(q1.Answers) != 4 {
		t.Errorf("q1 answers = %d, want 4", len(q1.Answers))
	}
	if q1.Answers[3].Position != 4 || !q1.Answers[3].Correct {
		t.Errorf("q1 correct answer should be position 4: %+v", q1.Answers[3])
	}
	for i, a := range q1.Answers {
		if a.Correct != (i == 3) {
			t.Errorf("q1 answer %d correct=%v, want %v", a.Position, a.Correct, i == 3)
		}
	}
	if q1.Image != "images/i1_1.webp" {
		t.Errorf("q1 image = %q, want images/i1_1.webp", q1.Image)
	}
	if q1.Category != "umumiy" || q1.Source != "avtoimtihon" {
		t.Errorf("q1 category/source = %q/%q", q1.Category, q1.Source)
	}

	// id 614: reconciled cross-locale mismatch, must be valid with exactly one correct.
	q614 := byExt["avtoimtihon-614"]
	if len(q614.Answers) != 3 {
		t.Errorf("q614 answers = %d, want 3 (min reconciliation)", len(q614.Answers))
	}
	correct := 0
	for _, a := range q614.Answers {
		if a.Correct {
			correct++
		}
		for _, loc := range importer.RequiredLocales {
			if a.Texts[loc] == "" {
				t.Errorf("q614 answer %d missing locale %s", a.Position, loc)
			}
		}
	}
	if correct != 1 {
		t.Errorf("q614 correct count = %d, want 1", correct)
	}
}

func TestExplanationSchema(t *testing.T) {
	res := loadOrSkip(t)
	if len(res.Dataset.Explanations) == 0 {
		t.Fatal("no explanations produced")
	}
	e := res.Dataset.Explanations[0]
	for loc, blocks := range e.Blocks {
		if len(blocks) == 0 {
			t.Errorf("explanation %s locale %s has no blocks", e.Question, loc)
		}
		b := blocks[0]
		if b["type"] != "muhim" {
			t.Errorf("explanation block type = %v, want muhim (canonical schema)", b["type"])
		}
		if _, ok := b["text"]; !ok {
			t.Errorf("explanation block missing canonical 'text' field: %v", b)
		}
		if _, bad := b["md"]; bad {
			t.Errorf("explanation block uses stale 'md' field: %v", b)
		}
	}
}

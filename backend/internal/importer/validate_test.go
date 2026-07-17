package importer

import (
	"fmt"
	"testing"
)

func validQuestion(ext string) CanonQuestion {
	texts := map[string]string{"uz-Latn": "S", "uz-Cyrl": "С", "ru": "В"}
	mk := func(pos int, correct bool) CanonAnswer {
		return CanonAnswer{Position: pos, Correct: correct, Texts: texts}
	}
	return CanonQuestion{
		ExtID: ext, Category: "signs", Texts: texts,
		Answers: []CanonAnswer{mk(1, false), mk(2, true), mk(3, false), mk(4, false)},
	}
}

func baseDataset(n int) Dataset {
	ds := Dataset{Categories: []CanonCategory{{Code: "signs", Names: map[string]string{"uz-Latn": "B"}}}}
	for i := 0; i < n; i++ {
		ds.Questions = append(ds.Questions, validQuestion(fmtID(i)))
	}
	return ds
}

func fmtID(i int) string { return fmt.Sprintf("q-%03d", i) }

func has(issues []Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestValidDatasetNoIssues(t *testing.T) {
	ds := baseDataset(20)
	var ids []string
	for _, q := range ds.Questions {
		ids = append(ids, q.ExtID)
	}
	ds.Variants = []CanonVariant{{Number: 1, Questions: ids}}
	if issues := Validate(ds); len(issues) != 0 {
		t.Fatalf("want clean, got %+v", issues)
	}
}

func TestAnswerInvariants(t *testing.T) {
	ds := baseDataset(1)
	ds.Questions[0].Answers = ds.Questions[0].Answers[:3]
	if !has(Validate(ds), "answers_count") {
		t.Fatal("want answers_count")
	}

	ds = baseDataset(1)
	ds.Questions[0].Answers[0].Correct = true // two corrects
	if !has(Validate(ds), "multiple_correct") {
		t.Fatal("want multiple_correct")
	}

	ds = baseDataset(1)
	for i := range ds.Questions[0].Answers {
		ds.Questions[0].Answers[i].Correct = false
	}
	if !has(Validate(ds), "no_correct") {
		t.Fatal("want no_correct")
	}
}

func TestLocaleAndRefInvariants(t *testing.T) {
	ds := baseDataset(1)
	ds.Questions[0].Texts = map[string]string{"uz-Latn": "S", "uz-Cyrl": "С"} // ru yo'q
	if !has(Validate(ds), "missing_locale") {
		t.Fatal("want missing_locale (question text)")
	}

	ds = baseDataset(1)
	ds.Questions[0].Category = "nope"
	if !has(Validate(ds), "unknown_category") {
		t.Fatal("want unknown_category")
	}

	ds = baseDataset(2)
	ds.Questions[1].ExtID = ds.Questions[0].ExtID
	if !has(Validate(ds), "duplicate_ext_id") {
		t.Fatal("want duplicate_ext_id")
	}

	ds = baseDataset(1)
	ds.Questions[0].Signs = []string{"9.99"}
	if !has(Validate(ds), "unknown_sign") {
		t.Fatal("want unknown_sign")
	}
}

func TestVariantInvariants(t *testing.T) {
	ds := baseDataset(19)
	var ids []string
	for _, q := range ds.Questions {
		ids = append(ids, q.ExtID)
	}
	ds.Variants = []CanonVariant{{Number: 1, Questions: ids}}
	if !has(Validate(ds), "variant_size") {
		t.Fatal("want variant_size for 19 questions")
	}

	ds = baseDataset(20)
	ids = ids[:0]
	for _, q := range ds.Questions {
		ids = append(ids, q.ExtID)
	}
	ids[1] = ids[0]
	ds.Variants = []CanonVariant{{Number: 1, Questions: ids}}
	if !has(Validate(ds), "variant_duplicate_question") {
		t.Fatal("want variant_duplicate_question")
	}

	// quarantined question poisons its variant
	ds = baseDataset(20)
	ds.Questions[0].Answers = ds.Questions[0].Answers[:2]
	ids = ids[:0]
	for _, q := range ds.Questions {
		ids = append(ids, q.ExtID)
	}
	ds.Variants = []CanonVariant{{Number: 1, Questions: ids}}
	if !has(Validate(ds), "variant_quarantined_question") {
		t.Fatal("want variant_quarantined_question")
	}

	// unknown question ref
	ds = baseDataset(20)
	ids = ids[:0]
	for _, q := range ds.Questions {
		ids = append(ids, q.ExtID)
	}
	ids[19] = "yo-q"
	ds.Variants = []CanonVariant{{Number: 1, Questions: ids}}
	if !has(Validate(ds), "variant_unknown_question") {
		t.Fatal("want variant_unknown_question")
	}
}

func TestQuarantineAndBrokenSets(t *testing.T) {
	ds := baseDataset(2)
	ds.Questions[0].Answers = ds.Questions[0].Answers[:1]
	issues := Validate(ds)
	q := QuarantinedIDs(issues)
	if !q[ds.Questions[0].ExtID] || q[ds.Questions[1].ExtID] {
		t.Fatalf("quarantine set wrong: %+v", q)
	}
}

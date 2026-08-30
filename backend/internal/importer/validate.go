package importer

import "fmt"

// Validate checks every invariant from the spec (§7.1). It never mutates the
// dataset; callers use QuarantinedIDs to exclude bad questions and their variants.
func Validate(ds Dataset) []Issue {
	var issues []Issue
	add := func(entity, id, code, detail string) {
		issues = append(issues, Issue{Entity: entity, ID: id, Code: code, Detail: detail})
	}

	categories := map[string]bool{}
	for _, c := range ds.Categories {
		categories[c.Code] = true
	}
	signs := map[string]bool{}
	for _, s := range ds.Signs {
		signs[s.Code] = true
	}

	seen := map[string]bool{}
	badQuestions := map[string]bool{}
	markBad := func(id string) { badQuestions[id] = true }

	for _, q := range ds.Questions {
		if seen[q.ExtID] {
			add("question", q.ExtID, "duplicate_ext_id", "ext_id takrorlangan")
			markBad(q.ExtID)
			continue
		}
		seen[q.ExtID] = true

		if !categories[q.Category] {
			add("question", q.ExtID, "unknown_category", q.Category)
			markBad(q.ExtID)
		}
		validCount := len(q.Answers) >= 2 && len(q.Answers) <= 5
		if !validCount {
			add("question", q.ExtID, "answers_count", fmt.Sprintf("%d ta javob", len(q.Answers)))
			markBad(q.ExtID)
		}
		correct := 0
		for _, a := range q.Answers {
			if a.Correct {
				correct++
			}
			for _, loc := range RequiredLocales {
				if a.Texts[loc] == "" {
					add("question", q.ExtID, "missing_locale",
						fmt.Sprintf("answer %d: %s", a.Position, loc))
					markBad(q.ExtID)
				}
			}
		}
		if validCount {
			if correct == 0 {
				add("question", q.ExtID, "no_correct", "")
				markBad(q.ExtID)
			} else if correct > 1 {
				add("question", q.ExtID, "multiple_correct", fmt.Sprintf("%d ta", correct))
				markBad(q.ExtID)
			}
		}
		for _, loc := range RequiredLocales {
			if q.Texts[loc] == "" {
				add("question", q.ExtID, "missing_locale", "question: "+loc)
				markBad(q.ExtID)
			}
		}
		for _, sc := range q.Signs {
			if !signs[sc] {
				add("question", q.ExtID, "unknown_sign", sc)
				markBad(q.ExtID)
			}
		}
	}

	// Only the highest-numbered ticket may be partial: newly imported questions
	// accumulate there until it reaches 20. A short ticket anywhere else means a
	// question was dropped, so that stays an issue.
	lastVariant := 0
	for _, v := range ds.Variants {
		if v.Number > lastVariant {
			lastVariant = v.Number
		}
	}

	for _, v := range ds.Variants {
		vid := fmt.Sprintf("%d", v.Number)
		filling := v.Number == lastVariant && len(v.Questions) >= 1 && len(v.Questions) < 20
		if len(v.Questions) != 20 && !filling {
			add("variant", vid, "variant_size", fmt.Sprintf("%d ta savol", len(v.Questions)))
			continue
		}
		inVariant := map[string]bool{}
		for _, qid := range v.Questions {
			if !seen[qid] {
				add("variant", vid, "variant_unknown_question", qid)
				continue
			}
			if inVariant[qid] {
				add("variant", vid, "variant_duplicate_question", qid)
			}
			inVariant[qid] = true
			if badQuestions[qid] {
				add("variant", vid, "variant_quarantined_question", qid)
			}
		}
	}

	return issues
}

// QuarantinedIDs returns the set of question ext_ids that must be stored as
// quarantined (every question that produced a question-level issue).
func QuarantinedIDs(issues []Issue) map[string]bool {
	out := map[string]bool{}
	for _, i := range issues {
		if i.Entity == "question" {
			out[i.ID] = true
		}
	}
	return out
}

// BrokenVariants returns variant numbers (as strings) that may not be stored at all.
func BrokenVariants(issues []Issue) map[string]bool {
	out := map[string]bool{}
	for _, i := range issues {
		if i.Entity == "variant" {
			out[i.ID] = true
		}
	}
	return out
}

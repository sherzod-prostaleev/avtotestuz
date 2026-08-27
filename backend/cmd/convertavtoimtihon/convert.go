package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"avtotest.uz/backend/internal/importer"
)

// locales in canonical order. All three source files are required.
var locales = []string{"uz-Latn", "uz-Cyrl", "ru"}

// sourceFile maps a locale to its source JSON filename under <src>/src/data/.
var sourceFile = map[string]string{
	"uz-Latn": "questions.uz-Latn.json",
	"uz-Cyrl": "questions.uz-Cyrl.json",
	"ru":      "questions.ru.json",
}

const (
	extIDPrefix     = "avtoimtihon-"
	sourceLabel     = "avtoimtihon"
	imageExt        = ".webp"
	imageDestPrefix = "images/"
)

// srcQuestion mirrors one object in the source questions.<locale>.json array.
type srcQuestion struct {
	ID            int      `json:"id"`
	Question      string   `json:"question"`
	Image         string   `json:"image"`
	Comment       string   `json:"comment"`
	Answers       []string `json:"answers"`
	CorrectAnswer int      `json:"correct_answer"`
	Ticket        string   `json:"ticket"`
}

// imageCopy records one image file that must be copied into the output layout.
type imageCopy struct {
	SrcAbs string // absolute source path
	Rel    string // relative dest inside dataset dir, e.g. "images/i1_1.webp"
}

// UnresolvedQuestion is a question no deterministic rule could categorize —
// input for the LLM classification pass (assignments.json).
type UnresolvedQuestion struct {
	ExtID         string   `json:"ext_id"`
	TextUzLatn    string   `json:"text_uz_latn"`
	TextRu        string   `json:"text_ru"`
	AnswersUzLatn []string `json:"answers_uz_latn"`
	CommentRu     string   `json:"comment_ru"`
}

// Result bundles the converted dataset with the side data main() needs.
type Result struct {
	Dataset      importer.Dataset
	ImageCopies  []imageCopy
	MissingImg   []string             // image refs with no file on disk (question ext_ids affected reported separately)
	MissingImgQ  []string             // ext_ids that lost their image because the file was missing
	LeftoverExtI []string             // ext_ids not assigned to any variant
	NoComment    []string             // ext_ids with no comment in any locale (no explanation emitted)
	PartialComnt []string             // ext_ids with a comment in some-but-not-all locales
	AnswerFixups []string             // human-readable notes about per-question answer reconciliation
	Warnings     []string             // prominent warnings for the operator
	Unresolved   []UnresolvedQuestion // questions with no explicit assignment and no citation match
}

// Convert reads the three source locale files under <srcDir>/src/data and the
// image directory under <srcDir>/public/quiz-images, and builds the canonical
// dataset. It cross-validates the three files and fails loudly on any structural
// inconsistency it cannot deterministically and safely reconcile.
//
// assignments maps ext_id -> category code and wins over citation
// classification; every code in it must be a known category code.
func Convert(srcDir string, assignments map[string]string) (Result, error) {
	var res Result

	// 0. Validate assignments up front against the known category codes.
	knownCodes := map[string]bool{}
	for _, d := range categoryDefs {
		knownCodes[d.code] = true
	}
	for extID, code := range assignments {
		if !knownCodes[code] {
			return res, fmt.Errorf("assignments: unknown category code %q for %s", code, extID)
		}
	}

	// 1. Read + parse all three locale files.
	byLocale := map[string][]srcQuestion{}
	for _, loc := range locales {
		p := filepath.Join(srcDir, "src", "data", sourceFile[loc])
		raw, err := os.ReadFile(p)
		if err != nil {
			return res, fmt.Errorf("read %s: %w", p, err)
		}
		var qs []srcQuestion
		if err := json.Unmarshal(raw, &qs); err != nil {
			return res, fmt.Errorf("parse %s: %w", p, err)
		}
		byLocale[loc] = qs
	}

	// 1b. Assert aligned id sequences across all three files.
	base := byLocale[locales[0]]
	n := len(base)
	for _, loc := range locales[1:] {
		if len(byLocale[loc]) != n {
			return res, fmt.Errorf("locale %s has %d questions, expected %d", loc, len(byLocale[loc]), n)
		}
	}
	for i := 0; i < n; i++ {
		id := base[i].ID
		for _, loc := range locales[1:] {
			if byLocale[loc][i].ID != id {
				return res, fmt.Errorf("id misalignment at index %d: %s=%d %s=%d",
					i, locales[0], id, loc, byLocale[loc][i].ID)
			}
		}
	}

	imgDir := filepath.Join(srcDir, "public", "quiz-images")
	seenImgRefMissing := map[string]bool{}

	// ticketOrder preserves first-seen ticket order and question order within.
	ticketQuestions := map[int][]string{}
	var ticketOrder []int

	// 2. Build one CanonQuestion per source id.
	for i := 0; i < n; i++ {
		b := base[i]
		extID := extIDPrefix + strconv.Itoa(b.ID)

		// texts per locale
		texts := map[string]string{}
		for _, loc := range locales {
			texts[loc] = byLocale[loc][i].Question
		}

		// answer count: normally identical across locales. Reconcile a mismatch
		// deterministically by using the minimum common count (drop trailing
		// extras present in only some locales), which keeps the question valid
		// and the correct answer intact. Fail loudly only if that would drop the
		// correct answer for some locale.
		count := len(b.Answers)
		mismatch := false
		for _, loc := range locales {
			if l := len(byLocale[loc][i].Answers); l != count {
				mismatch = true
				if l < count {
					count = l
				}
			}
		}
		if mismatch {
			// correct_answer agrees across locales (asserted below); ensure it
			// still fits within the reconciled count for every locale.
			for _, loc := range locales {
				ca := byLocale[loc][i].CorrectAnswer
				if ca > count {
					return res, fmt.Errorf("id %d: answer-count mismatch cannot be safely reconciled: correct_answer=%d > min count %d (locale %s)",
						b.ID, ca, count, loc)
				}
			}
			lens := map[string]int{}
			for _, loc := range locales {
				lens[loc] = len(byLocale[loc][i].Answers)
			}
			note := fmt.Sprintf("id %d (%s): cross-locale answer count differs %v — reconciled to min=%d, dropped trailing extras",
				b.ID, extID, lens, count)
			res.AnswerFixups = append(res.AnswerFixups, note)
			res.Warnings = append(res.Warnings, "WARNING: "+note)
		}

		// correct_answer must agree across locales (1-based).
		correct := b.CorrectAnswer
		for _, loc := range locales[1:] {
			if byLocale[loc][i].CorrectAnswer != correct {
				return res, fmt.Errorf("id %d: correct_answer disagrees across locales (%s=%d %s=%d)",
					b.ID, locales[0], correct, loc, byLocale[loc][i].CorrectAnswer)
			}
		}
		if correct < 1 || correct > count {
			return res, fmt.Errorf("id %d: correct_answer %d out of range 1..%d", b.ID, correct, count)
		}

		answers := make([]importer.CanonAnswer, 0, count)
		for p := 0; p < count; p++ {
			at := map[string]string{}
			for _, loc := range locales {
				at[loc] = byLocale[loc][i].Answers[p]
			}
			answers = append(answers, importer.CanonAnswer{
				Position: p + 1,          // 1-based canonical position
				Correct:  p+1 == correct, // source correct_answer is 1-based
				Texts:    at,
			})
		}

		ruComment := byLocale["ru"][i].Comment
		category := ""
		if code, ok := assignments[extID]; ok {
			category = code // explicit assignment wins
		} else {
			category = "general_rules" // provisional fallback; recorded below
			ansUz := make([]string, 0, count)
			for p := 0; p < count; p++ {
				ansUz = append(ansUz, byLocale["uz-Latn"][i].Answers[p])
			}
			res.Unresolved = append(res.Unresolved, UnresolvedQuestion{
				ExtID:         extID,
				TextUzLatn:    texts["uz-Latn"],
				TextRu:        texts["ru"],
				AnswersUzLatn: ansUz,
				CommentRu:     ruComment,
			})
		}

		q := importer.CanonQuestion{
			ExtID:    extID,
			Category: category,
			Texts:    texts,
			Answers:  answers,
			Source:   sourceLabel,
		}

		// 4. Image handling: only set Image if the file actually exists.
		if b.Image != "" {
			rel := imageDestPrefix + b.Image + imageExt
			abs := filepath.Join(imgDir, b.Image+imageExt)
			if _, err := os.Stat(abs); err == nil {
				q.Image = rel
				res.ImageCopies = append(res.ImageCopies, imageCopy{SrcAbs: abs, Rel: rel})
			} else {
				// Missing asset: emit the question without an image (still usable
				// via its text) and record the gap.
				if !seenImgRefMissing[b.Image] {
					seenImgRefMissing[b.Image] = true
					res.MissingImg = append(res.MissingImg, b.Image)
				}
				res.MissingImgQ = append(res.MissingImgQ, extID)
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("WARNING: image %q referenced by %s not found at %s — question emitted without image",
						b.Image, extID, abs))
			}
		}

		res.Dataset.Questions = append(res.Dataset.Questions, q)

		// 5. Explanation: one block per locale that has a non-empty comment.
		// Comment coverage is all-or-nothing in this source, but handle a partial
		// case deliberately: omit the block for a locale lacking a comment rather
		// than fabricating or duplicating one.
		blocks := map[string][]map[string]any{}
		anyComment := false
		allComment := true
		for _, loc := range locales {
			c := byLocale[loc][i].Comment
			if c == "" {
				allComment = false
				continue
			}
			anyComment = true
			blocks[loc] = []map[string]any{
				{"type": "muhim", "text": c},
			}
		}
		if anyComment {
			res.Dataset.Explanations = append(res.Dataset.Explanations, importer.CanonExplanation{
				Question: extID,
				// Structured refs filled post-convert by scripts/seed/extract_legal_refs.py
				// (wired into `make seed-import` / `make extract-legal-refs`).
				LegalRefs: nil,
				Blocks:    blocks,
			})
			if !allComment {
				res.PartialComnt = append(res.PartialComnt, extID)
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("WARNING: %s has a comment in some but not all locales — emitted explanation only for present locales", extID))
			}
		} else {
			res.NoComment = append(res.NoComment, extID)
		}

		// accumulate ticket grouping
		tn, err := strconv.Atoi(b.Ticket)
		if err != nil {
			return res, fmt.Errorf("id %d: unparsable ticket %q: %w", b.ID, b.Ticket, err)
		}
		if _, ok := ticketQuestions[tn]; !ok {
			ticketOrder = append(ticketOrder, tn)
		}
		ticketQuestions[tn] = append(ticketQuestions[tn], extID)
	}

	// 3. Pair consecutive tickets into 20-question variants.
	sort.Ints(ticketOrder)
	assigned := map[string]bool{}
	variantNum := 0
	for k := 0; k+1 < len(ticketOrder); k += 2 {
		a, bb := ticketOrder[k], ticketOrder[k+1]
		qa, qb := ticketQuestions[a], ticketQuestions[bb]
		if len(qa)+len(qb) != 20 {
			// leftover: not a clean 20-question pair
			continue
		}
		variantNum++
		qs := make([]string, 0, 20)
		qs = append(qs, qa...)
		qs = append(qs, qb...)
		for _, e := range qs {
			assigned[e] = true
		}
		res.Dataset.Variants = append(res.Dataset.Variants, importer.CanonVariant{
			Number:    variantNum,
			Questions: qs,
		})
	}
	// Everything not assigned to a clean 20-pair is leftover (includes any final
	// unpaired odd ticket and any pair whose combined size != 20).
	for _, tn := range ticketOrder {
		for _, e := range ticketQuestions[tn] {
			if !assigned[e] {
				res.LeftoverExtI = append(res.LeftoverExtI, e)
			}
		}
	}

	// 6. Category catalog: 13 approved categories; keep the umumiy fallback
	// only while unresolved questions still reference it.
	res.Dataset.Categories = categoriesForDataset(len(res.Unresolved) > 0)
	// 7. No sign groups / signs.

	return res, nil
}

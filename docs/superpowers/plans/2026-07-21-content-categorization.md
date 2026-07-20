# Content Categorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Assign each of the 1235 real questions one of the 13 user-approved categories so mastery-map, weak-area focus, readiness % and category practice become real features instead of a single `umumiy` bucket.

**Architecture:** The converter (`cmd/convertavtoimtihon`) gains a deterministic rule-based classifier (regex over each question's ru explanation citation → static chapter/appendix→category lookup) plus an overrides file for the ~32% of questions with no usable citation. Those overrides are produced by an LLM-agent classification pass (Task 2) and committed as `assignments.json`. Final strict conversion + re-import diversifies `question.category_id`; no DB schema or importer code changes are needed (`importer.Store` already upserts categories and re-resolves per-question category codes).

**Tech Stack:** Go (converter + tests), jq (batch extraction/merge/validation), existing importer pipeline, psql for verification.

## Global Constraints

- The 13 category codes, sort orders and 3-locale names are FIXED (user-approved 2026-07-21, source: `docs/superpowers/research/2026-07-21-category-taxonomy-proposal.md` §3). Codes, verbatim, in sort order 1..13: `road_signs_markings`, `priority_intersections`, `maneuvering_lane_position`, `vehicle_equipment_lighting`, `stopping_parking`, `overtaking_speed`, `pedestrians_public_transport`, `special_road_zones`, `traffic_signals_gestures`, `towing_special_vehicles`, `accidents_first_aid_dynamics`, `cargo_passenger_carriage`, `general_provisions_admin`. Locale names appear verbatim in Task 1's code block — they are draft copy; the user (native speaker) reviews them at the Task 3 checkpoint, and any edit is a one-line change in `categories.go` + re-run.
- Chapter→category mapping (user-approved via taxonomy §3; chapter numbers are the DATASET's own internal numbering, not the current 2022 law — do not "correct" them): 1,2,25,29→`general_provisions_admin`; 3→`accidents_first_aid_dynamics`; 4,5,17,22,28→`pedestrians_public_transport`; 6,24→`towing_special_vehicles`; 7→`traffic_signals_gestures`; 8,23→`vehicle_equipment_lighting`; 9,10→`maneuvering_lane_position`; 11,12→`overtaking_speed`; 13→`stopping_parking`; 14,15,16→`priority_intersections`; 18,19,20,21→`special_road_zones`; 26,27→`cargo_passenger_carriage`. Appendix→category: 1,2→`road_signs_markings`; 3→`vehicle_equipment_lighting`; 4→`cargo_passenger_carriage`. Precedence: first chapter citation wins; else first appendix citation; else unresolved. An unknown chapter (>29) or appendix (>4) number is UNRESOLVED, never guessed.
- Explanation TEXT is untouched. The old-PDD "глава N ПДД" citations stay as-is in user-visible explanations (user decision 2026-07-21: re-numbering is a future expert-verify work item). This plan only reads them as classification signals.
- No new DB migrations, no `internal/importer` code changes, no API changes.
- LLM classification (Task 2) must choose from the fixed 13 codes only — a 14th code or an invented code is a validation failure. "Genuinely unsure" routes to `general_provisions_admin` and is listed in the task report for spot-check.
- Environment: `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`; repo path contains Cyrillic + space — always double-quote. Backend DB tests: `go test ./... -p 1`, never concurrently with another backend agent. `make check` green before every commit. Commits on `main` (project convention), conventional-commit style.
- Source dataset: `/home/sher/Рабочий стол/aaa` (converter `-src` default). Canonical output `backend/seed/avtoimtihon/data.json` is COMMITTED; regenerate via converter, never hand-edit.

---

### Task 1: Rule-based classifier in the converter

**Files:**
- Create: `backend/cmd/convertavtoimtihon/categories.go`
- Create: `backend/cmd/convertavtoimtihon/categories_test.go`
- Modify: `backend/cmd/convertavtoimtihon/convert.go` (lines ~26, ~180-186, ~288-297; `Result` struct)
- Modify: `backend/cmd/convertavtoimtihon/main.go` (flags + call site + report lines)
- Modify: `backend/cmd/convertavtoimtihon/convert_test.go` (call-site signature + category expectations)

**Interfaces:**
- Consumes: `importer.CanonCategory{Code string, Sort int, Names map[string]string}`, `importer.CanonQuestion.Category string` (existing).
- Produces (Task 2/3 rely on these):
  - `classifyByCitation(ruComment string) (code string, ok bool)`
  - `categoriesForDataset(includeFallback bool) []importer.CanonCategory`
  - `Convert(src string, assignments map[string]string) (Result, error)` — signature change; `assignments` maps ext_id→category code and wins over citation classification.
  - `Result.Unresolved []UnresolvedQuestion` where `type UnresolvedQuestion struct { ExtID string `json:"ext_id"`; TextUzLatn string `json:"text_uz_latn"`; TextRu string `json:"text_ru"`; AnswersUzLatn []string `json:"answers_uz_latn"`; CommentRu string `json:"comment_ru"` }`
  - main.go flags: `-assignments <path>` (JSON `{"ext_id":"code"}`), `-unresolved <path>` (writes `[]UnresolvedQuestion`), `-strict` (exit 1 if any unresolved).

- [ ] **Step 1: Write the failing classifier tests**

`backend/cmd/convertavtoimtihon/categories_test.go`:

```go
package main

import "testing"

func TestClassifyByCitation(t *testing.T) {
	cases := []struct {
		name string
		in   string
		code string
		ok   bool
	}{
		{"chapter cite", "Пункта 2 главы 16 ПДД, гласит: на нерегулируемом перекрестке...", "priority_intersections", true},
		{"chapter lowercase form", "согласно пункту 5 главы 13 ПДД остановка запрещена", "stopping_parking", true},
		{"appendix only", "Приложение №1 к ПДД пункт 3.27: Остановка запрещена...", "road_signs_markings", true},
		{"appendix no numero sign", "Приложение 3 к ПДД: неисправности...", "vehicle_equipment_lighting", true},
		{"chapter beats appendix", "Пункта 1 главы 9 ПДД... см. также Приложение №2", "maneuvering_lane_position", true},
		{"unknown chapter number", "глава 30 ПДД", "", false},
		{"unknown appendix number", "Приложение №7 к ПДД", "", false},
		{"no citation", "При торможении на скользкой дороге возможен занос.", "", false},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ok := classifyByCitation(c.in)
			if code != c.code || ok != c.ok {
				t.Fatalf("classifyByCitation(%q) = (%q,%v), want (%q,%v)", c.in, code, ok, c.code, c.ok)
			}
		})
	}
}

func TestCategoriesForDataset(t *testing.T) {
	cats := categoriesForDataset(false)
	if len(cats) != 13 {
		t.Fatalf("want 13 categories, got %d", len(cats))
	}
	seen := map[string]bool{}
	for i, c := range cats {
		if c.Sort != i+1 {
			t.Fatalf("category %s: sort %d, want %d (sorted, 1-based)", c.Code, c.Sort, i+1)
		}
		if seen[c.Code] {
			t.Fatalf("duplicate code %s", c.Code)
		}
		seen[c.Code] = true
		for _, loc := range []string{"uz-Latn", "uz-Cyrl", "ru"} {
			if c.Names[loc] == "" {
				t.Fatalf("category %s: missing %s name", c.Code, loc)
			}
		}
	}
	withFallback := categoriesForDataset(true)
	if len(withFallback) != 14 || withFallback[13].Code != "umumiy" {
		t.Fatalf("includeFallback: want umumiy appended as 14th, got %d entries", len(withFallback))
	}
	// every chapter/appendix mapping target must be a real category code
	for ch, code := range chapterToCategory {
		if !seen[code] {
			t.Fatalf("chapterToCategory[%d] -> unknown code %q", ch, code)
		}
	}
	for ap, code := range appendixToCategory {
		if !seen[code] {
			t.Fatalf("appendixToCategory[%d] -> unknown code %q", ap, code)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd "/home/sher/Рабочий стол/avtotest/backend" && go test ./cmd/convertavtoimtihon/ -run 'TestClassifyByCitation|TestCategoriesForDataset' -v`
Expected: FAIL — `undefined: classifyByCitation`, `undefined: categoriesForDataset`.

- [ ] **Step 3: Implement `categories.go`**

```go
// Category taxonomy for the real avtoimtihon dataset — 13 user-approved
// categories (docs/superpowers/research/2026-07-21-category-taxonomy-proposal.md §3,
// approved 2026-07-21) plus a deterministic citation-based classifier.
//
// The chapter numbers below are the DATASET's own internal PDD numbering
// (older than the 2022 revision) — they are classification signals, not
// legal citations, and must not be "corrected" here.
package main

import (
	"regexp"
	"strconv"

	"avtotest.uz/backend/internal/importer"
)

type categoryDef struct {
	code   string
	uzLatn string
	uzCyrl string
	ru     string
}

// Order = sort order (1-based), largest estimated first (taxonomy §3).
var categoryDefs = []categoryDef{
	{"road_signs_markings", "Yo'l belgilari va chizig'i", "Йўл белгилари ва чизиғи", "Дорожные знаки и разметка"},
	{"priority_intersections", "Chorrahalar va yo'l ustunligi", "Чорраҳалар ва йўл устунлиги", "Перекрёстки и преимущество проезда"},
	{"maneuvering_lane_position", "Manyovr va yo'lda joylashuv", "Манёвр ва йўлда жойлашув", "Манёврирование и расположение на проезжей части"},
	{"vehicle_equipment_lighting", "Transport vositasi jihozi va yorug'lik", "Транспорт воситаси жиҳози ва ёруғлик", "Техническое состояние ТС и освещение"},
	{"stopping_parking", "To'xtash va to'xtab turish", "Тўхташ ва тўхтаб туриш", "Остановка и стоянка"},
	{"overtaking_speed", "Quvib o'tish va tezlik", "Қувиб ўтиш ва тезлик", "Обгон и скорость движения"},
	{"pedestrians_public_transport", "Piyodalar, yo'lovchilar va yo'nalishli transport", "Пиёдалар, йўловчилар ва йўналишли транспорт", "Пешеходы, пассажиры и маршрутный транспорт"},
	{"special_road_zones", "Maxsus yo'l uchastkalari", "Махсус йўл участкалари", "Особые участки дорог"},
	{"traffic_signals_gestures", "Svetofor va tartibga soluvchi ishoralari", "Светофор ва тартибга солувчи ишоралари", "Сигналы светофора и регулировщика"},
	{"towing_special_vehicles", "Shatakka olish va maxsus transport", "Шатакка олиш ва махсус транспорт", "Буксировка и спецтранспорт"},
	{"accidents_first_aid_dynamics", "YHH, tez tibbiy yordam va tormozlash", "ЙТҲ, тез тиббий ёрдам ва тормозлаш", "ДТП, первая помощь и динамика торможения"},
	{"cargo_passenger_carriage", "Yuk va odam tashish", "Юк ва одам ташиш", "Перевозка людей и грузов"},
	{"general_provisions_admin", "Umumiy qoidalar va majburiyatlar", "Умумий қоидалар ва мажбуриятлар", "Общие положения и обязанности"},
}

var chapterToCategory = map[int]string{
	1: "general_provisions_admin", 2: "general_provisions_admin",
	3: "accidents_first_aid_dynamics",
	4: "pedestrians_public_transport", 5: "pedestrians_public_transport",
	6: "towing_special_vehicles",
	7: "traffic_signals_gestures",
	8: "vehicle_equipment_lighting",
	9: "maneuvering_lane_position", 10: "maneuvering_lane_position",
	11: "overtaking_speed", 12: "overtaking_speed",
	13: "stopping_parking",
	14: "priority_intersections", 15: "priority_intersections", 16: "priority_intersections",
	17: "pedestrians_public_transport",
	18: "special_road_zones", 19: "special_road_zones", 20: "special_road_zones", 21: "special_road_zones",
	22: "pedestrians_public_transport",
	23: "vehicle_equipment_lighting",
	24: "towing_special_vehicles",
	25: "general_provisions_admin",
	26: "cargo_passenger_carriage", 27: "cargo_passenger_carriage",
	28: "pedestrians_public_transport",
	29: "general_provisions_admin",
}

var appendixToCategory = map[int]string{
	1: "road_signs_markings", 2: "road_signs_markings",
	3: "vehicle_equipment_lighting",
	4: "cargo_passenger_carriage",
}

var (
	chapterRe  = regexp.MustCompile(`(?i)глав[а-яё]*\s+(\d+)\s+ПДД`)
	appendixRe = regexp.MustCompile(`(?i)приложени[а-яё]*\s*№?\s*(\d+)`)
)

// classifyByCitation resolves a category from the FIRST chapter citation in
// the ru explanation text, else the FIRST appendix citation. Unknown numbers
// are unresolved (never guessed).
func classifyByCitation(ruComment string) (string, bool) {
	if m := chapterRe.FindStringSubmatch(ruComment); m != nil {
		n, _ := strconv.Atoi(m[1])
		code, ok := chapterToCategory[n]
		return code, ok
	}
	if m := appendixRe.FindStringSubmatch(ruComment); m != nil {
		n, _ := strconv.Atoi(m[1])
		code, ok := appendixToCategory[n]
		return code, ok
	}
	return "", false
}

// categoriesForDataset returns the 13 canonical categories in sort order.
// includeFallback appends the legacy umumiy category — only needed while
// unresolved questions still fall back to it (non-strict conversions).
func categoriesForDataset(includeFallback bool) []importer.CanonCategory {
	out := make([]importer.CanonCategory, 0, len(categoryDefs)+1)
	for i, d := range categoryDefs {
		out = append(out, importer.CanonCategory{
			Code: d.code,
			Sort: i + 1,
			Names: map[string]string{
				"uz-Latn": d.uzLatn,
				"uz-Cyrl": d.uzCyrl,
				"ru":      d.ru,
			},
		})
	}
	if includeFallback {
		out = append(out, importer.CanonCategory{
			Code: "umumiy",
			Sort: len(categoryDefs) + 1,
			Names: map[string]string{
				"uz-Latn": "Umumiy savollar",
				"uz-Cyrl": "Умумий саволлар",
				"ru":      "Общие вопросы",
			},
		})
	}
	return out
}
```

- [ ] **Step 4: Run classifier tests to verify they pass**

Run: `go test ./cmd/convertavtoimtihon/ -run 'TestClassifyByCitation|TestCategoriesForDataset' -v`
Expected: PASS.

- [ ] **Step 5: Wire classification into `Convert`**

In `convert.go`:
1. Delete the `categoryCode = "umumiy"` constant (line ~26).
2. Change the signature: `func Convert(src string, assignments map[string]string) (Result, error)`.
3. Add to `Result`: `Unresolved []UnresolvedQuestion` and define:

```go
// UnresolvedQuestion is a question no deterministic rule could categorize —
// input for the LLM classification pass (assignments.json).
type UnresolvedQuestion struct {
	ExtID         string   `json:"ext_id"`
	TextUzLatn    string   `json:"text_uz_latn"`
	TextRu        string   `json:"text_ru"`
	AnswersUzLatn []string `json:"answers_uz_latn"`
	CommentRu     string   `json:"comment_ru"`
}
```

4. In the question loop (where `q := importer.CanonQuestion{...}` is built, line ~180), replace `Category: categoryCode` with resolution logic placed just before constructing `q` (the ru comment is `byLocale["ru"][i].Comment`, texts map is already built):

```go
		ruComment := byLocale["ru"][i].Comment
		category := ""
		if code, ok := assignments[extID]; ok {
			category = code // explicit assignment wins over citation
		} else if code, ok := classifyByCitation(ruComment); ok {
			category = code
		} else {
			category = "umumiy" // provisional fallback; recorded below
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
```

and use `Category: category` in the `CanonQuestion`.
5. Validate assignments up front (top of `Convert`, before the loop): every assignment code must be a known category code, else `return res, fmt.Errorf("assignments: unknown category code %q for %s", code, extID)`. Build the known-code set from `categoryDefs`.
6. Replace the single-category emission (line ~288) with:

```go
	// 6. Category catalog: 13 approved categories; keep the umumiy fallback
	// only while unresolved questions still reference it.
	res.Dataset.Categories = categoriesForDataset(len(res.Unresolved) > 0)
```

- [ ] **Step 6: Wire flags into `main.go`**

After the existing `src`/`out` flags:

```go
	assignmentsPath := flag.String("assignments", "", "optional JSON file mapping ext_id -> category code (wins over citation classification)")
	unresolvedPath := flag.String("unresolved", "", "optional path to write the unresolved-questions JSON report")
	strict := flag.Bool("strict", false, "fail (exit 1) if any question remains uncategorized")
```

After `flag.Parse()`, load assignments (nil map when flag empty):

```go
	assignments := map[string]string{}
	if *assignmentsPath != "" {
		raw, err := os.ReadFile(*assignmentsPath)
		fatal(err)
		fatal(json.Unmarshal(raw, &assignments))
	}

	res, err := Convert(*src, assignments)
	fatal(err)

	if *unresolvedPath != "" {
		u, err := json.MarshalIndent(res.Unresolved, "", "  ")
		fatal(err)
		fatal(os.WriteFile(*unresolvedPath, u, 0o644))
	}
	if *strict && len(res.Unresolved) > 0 {
		fatal(fmt.Errorf("strict: %d questions uncategorized (run with -unresolved to list them)", len(res.Unresolved)))
	}
```

Add to the report section: `fmt.Printf("  uncategorized (fallback umumiy): %d\n", len(res.Unresolved))` and a per-category count line:

```go
	catCounts := map[string]int{}
	for _, q := range res.Dataset.Questions {
		catCounts[q.Category]++
	}
	fmt.Printf("  category distribution: %v\n", catCounts)
```

- [ ] **Step 7: Fix `convert_test.go` call sites and category expectations**

Every `Convert(dir)` becomes `Convert(dir, nil)`. The assertion `q1.Category != "umumiy"` (line ~87): keep the fixture's comment citation-free so the expectation stays `umumiy` AND extend the test fixture with one question whose ru comment contains `"Пункта 2 главы 16 ПДД"` asserting `Category == "priority_intersections"`, plus one call passing `assignments = map[string]string{"<that ext_id>": "stopping_parking"}` asserting the override wins. Assert `res.Unresolved` contains exactly the citation-free questions.

- [ ] **Step 8: Run the package tests, then the full suite**

Run: `go test ./cmd/convertavtoimtihon/ -v` → PASS, then `cd "/home/sher/Рабочий стол/avtotest" && make check` → green (0 lint issues, all packages pass; DB tests run with `-p 1` via Makefile).

- [ ] **Step 9: Real-data smoke + unresolved report**

Run:

```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && go run ./cmd/convertavtoimtihon \
  -src "/home/sher/Рабочий стол/aaa" -out seed/avtoimtihon \
  -unresolved ../.superpowers/sdd/unresolved-questions.json
```

Expected: `questions=1235`, `categories=14` (13 + umumiy fallback), `uncategorized` ≈ 390 ± 30 (taxonomy §1.3 predicted 391; a large deviation means the regexes disagree with the research pass — STOP and report DONE_WITH_CONCERNS with the actual number). Validate: `issues=0 quarantined=0`. Record the actual uncategorized count and category distribution in your report. Do NOT commit the regenerated `data.json` in this task (it still contains umumiy fallbacks) — `git checkout -- seed/avtoimtihon/data.json` after recording the numbers.

- [ ] **Step 10: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add backend/cmd/convertavtoimtihon/ && git commit -m "feat(content): rule-based category classifier in converter (13 approved categories)"
```

---

### Task 2: LLM classification of unresolved questions → assignments.json

This task is agent work, not code. Input: `.superpowers/sdd/unresolved-questions.json` (from Task 1 Step 9, ~390 entries). Output: `backend/seed/avtoimtihon/assignments.json` — a single JSON object `{"<ext_id>": "<category code>", ...}` covering EVERY unresolved ext_id exactly once.

**Files:**
- Create: `backend/seed/avtoimtihon/assignments.json` (committed — it is a curated content artifact, tracked like data.json)

**Interfaces:**
- Consumes: `UnresolvedQuestion` JSON array (fields: `ext_id`, `text_uz_latn`, `text_ru`, `answers_uz_latn`, `comment_ru`).
- Produces: `assignments.json` consumed by `Convert` via `-assignments` (Task 1 Step 6). Codes limited to the 13 in Global Constraints.

- [ ] **Step 1: Split the input into 3 batches** (for parallel classification by 3 agents, each writing its own part file — never the same file):

```bash
cd "/home/sher/Рабочий стол/avtotest" && jq '.[0:130]' .superpowers/sdd/unresolved-questions.json > .superpowers/sdd/unresolved-part1.json
jq '.[130:260]' .superpowers/sdd/unresolved-questions.json > .superpowers/sdd/unresolved-part2.json
jq '.[260:]'    .superpowers/sdd/unresolved-questions.json > .superpowers/sdd/unresolved-part3.json
```

- [ ] **Step 2: Classify each batch** (one agent per part; controller dispatches 3 in parallel). Each classifier agent MUST: read its part file; for every entry choose exactly one of the 13 codes based on question text + answers + comment content (the comment, when present, is uncited but topically rich — e.g. braking-physics prose → `accidents_first_aid_dynamics`, sign descriptions → `road_signs_markings`); when genuinely unsure, use `general_provisions_admin` and list that ext_id under `"unsure"` in its report; write `.superpowers/sdd/assignments-partN.json` as `{"<ext_id>":"<code>", ...}` covering every entry of its part exactly once. Category scope one-liners the classifier prompts must include, verbatim from taxonomy §3's table: signs & road markings; intersections & right-of-way; maneuvering & lane position; vehicle equipment/technical condition & lighting; stopping & parking; overtaking & speed; pedestrians, passengers & route transport (incl. bicycles/carts); special road sections (rail crossings, motorways, residential zones, grades); traffic-light & controller signals; towing & special vehicles; accidents, first aid & braking dynamics; cargo & passenger carriage; general provisions & duties.

- [ ] **Step 3: Merge and validate**

```bash
cd "/home/sher/Рабочий стол/avtotest" && jq -s 'add' .superpowers/sdd/assignments-part*.json > backend/seed/avtoimtihon/assignments.json
# coverage: every unresolved ext_id assigned exactly once, nothing extra
jq -r '.[].ext_id' .superpowers/sdd/unresolved-questions.json | sort > /tmp/claude-1000/want.txt
jq -r 'keys[]' backend/seed/avtoimtihon/assignments.json | sort > /tmp/claude-1000/got.txt
diff /tmp/claude-1000/want.txt /tmp/claude-1000/got.txt && echo COVERAGE-OK
# code validity: every value is one of the 13 approved codes
jq -r '.[]' backend/seed/avtoimtihon/assignments.json | sort -u
```

Expected: `COVERAGE-OK` and the last command lists ONLY approved codes. Any diff output or unknown code = fix before proceeding.

- [ ] **Step 4: Adversarial spot-check** (independent reviewer agent, NOT one of the classifiers): re-classify a random sample of 40 assignments blind (given the same 13-category scopes but not the chosen answers), then compare. Also fully re-review every "unsure"-flagged ext_id. Disagreement >15% on the random sample → controller escalates (re-brief classifiers with the disagreement patterns, re-run affected batches); ≤15% → adjudicate individual disagreements (reviewer's argument vs classifier's) and patch `assignments.json` where the reviewer is right.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add backend/seed/avtoimtihon/assignments.json && git commit -m "feat(content): LLM category assignments for citation-less questions"
```

---

### Task 3: Strict conversion, re-import, verification, docs

**Files:**
- Modify: `backend/seed/avtoimtihon/data.json` (regenerated — categories diversified)
- Modify: `README.md` (Kontent section: replace the single-`umumiy` caveat)
- Modify: `Makefile` (only if `seed-real` needs the `-assignments` flag added — check first)

**Interfaces:**
- Consumes: Task 1's flags, Task 2's `assignments.json`.
- Produces: dev DB + committed data.json where every question has one of the 13 categories and zero questions are `umumiy`.

- [ ] **Step 1: Strict conversion**

```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && go run ./cmd/convertavtoimtihon \
  -src "/home/sher/Рабочий стол/aaa" -out seed/avtoimtihon \
  -assignments seed/avtoimtihon/assignments.json -strict
```

Expected: exits 0, `categories=13` (no umumiy — strict passed means zero fallback), `questions=1235`, `Validate: issues=0 quarantined=0`, and the printed `category distribution` has 13 nonzero buckets. Record the distribution in your report and compare against taxonomy §3's estimate ranges — categories far outside their range are worth a sentence of explanation (not automatically wrong; the ranges were estimates).

- [ ] **Step 2: Update `make seed-real` if needed**

Check the Makefile's `seed-real` target: if it invokes the converter, add `-assignments seed/avtoimtihon/assignments.json -strict` so regeneration stays reproducible. If it only runs the importer on the committed output, no change.

- [ ] **Step 3: Re-import into the dev DB**

The dev DB may hold the old single-category content; the documented clean path (README "Kontent" section) is truncate + reseed. Follow it: truncate content tables the same way the original real-content import did (check README/compose for the psql connection — `make up` stack), then `make seed-real` (or `go run ./cmd/importer -data seed/avtoimtihon -verified` from `backend/`). Expected import report: `categories=13 ... questions valid=1235 quarantined=0 · variants stored=61`.

- [ ] **Step 4: SQL + API verification**

Against the dev DB (same connection as Step 3):

```sql
SELECT c.code, count(q.id) AS n
FROM category c LEFT JOIN question q ON q.category_id = c.id
GROUP BY c.code ORDER BY n DESC;
```

Expected: exactly 13 rows, every `n > 0`, sum = 1235, no `umumiy` row (fresh import) — if an old `umumiy` row survived with `n = 0`, delete it (`DELETE FROM category WHERE code = 'umumiy';` — safe: no questions reference it, verified by the same query). Then API spot-checks with the running server (`PORT=8090 go run ./cmd/api` if not already up):

```bash
curl -s "localhost:8090/api/v1/categories?locale=uz-Latn" | jq '.data | length'   # → 13
curl -s "localhost:8090/api/v1/categories?locale=ru" | jq '.data[0]'              # names in ru
```

- [ ] **Step 5: Update README + commit**

README "Kontent" section: replace the `umumiy` fallback paragraph with: 13 categories (list codes), assigned via citation rules + committed `assignments.json`, regeneration one-liner now includes `-assignments ... -strict`. Then:

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add backend/seed/avtoimtihon/data.json README.md Makefile && git commit -m "feat(content): categorize all 1235 questions into 13 approved categories"
```

- [ ] **Step 6: USER CHECKPOINT — native copy review**

Present the final per-category question counts AND the 13×3 locale-name table to the user (native speaker) for copy review. Any name edits: one line each in `categories.go` `categoryDefs`, then re-run Task 3 Steps 1+3+5 (cheap). Do not mark this plan complete until the user has seen the name table.

---

## Verification (whole plan)

- `make check` green at every commit.
- Final state: `SELECT count(*) FROM question q JOIN category c ON q.category_id=c.id WHERE c.code='umumiy'` → 0; 13 categories each with >0 questions summing to 1235; `GET /api/v1/categories` returns 13 in all 3 locales; explanation text byte-identical to before (only `category` fields and the `categories` array changed in data.json — spot-check with `git diff` stats on the commit).

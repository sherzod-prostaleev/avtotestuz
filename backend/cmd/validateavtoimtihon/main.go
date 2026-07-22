// Command validateavtoimtihon checks that the three locale files of the
// avtoimtihon dataset still describe the same questions.
//
// The converter joins the locales by question id. Nothing in the export
// enforces that the id means the same thing in each file, so a regenerated or
// reordered export can silently pair an Uzbek stem with a Russian answer set —
// which is how a "correct" answer ends up depending on the reader's language.
// The converter refuses such data, but only reports the first offending id;
// this reports all of them so the export can actually be fixed.
//
//	go run ./cmd/validateavtoimtihon -src "/path/to/aaa"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

var locales = []string{"uz-Latn", "uz-Cyrl", "ru"}

type srcQuestion struct {
	ID            int      `json:"id"`
	Question      string   `json:"question"`
	Answers       []string `json:"answers"`
	CorrectAnswer int      `json:"correct_answer"`
	Image         string   `json:"image"`
	Ticket        string   `json:"ticket"`
}

// Problem is one inconsistency, named so the report reads as a work list.
type Problem struct {
	ID      int
	Kind    string
	Details string
}

func main() {
	src := flag.String("src", "/home/sher/Рабочий стол/aaa", "source dataset root (contains src/data)")
	limit := flag.Int("limit", 20, "how many problems of each kind to print (0 = all)")
	flag.Parse()

	byLocale := map[string]map[int]srcQuestion{}
	for _, loc := range locales {
		path := filepath.Join(*src, "src", "data", fmt.Sprintf("questions.%s.json", loc))
		items, err := load(path)
		if err != nil {
			fatal(err)
		}
		byLocale[loc] = items
		fmt.Printf("%-10s %4d questions\n", loc, len(items))
	}

	problems := validate(byLocale)
	if len(problems) == 0 {
		fmt.Println("\nOK: all locales agree on every question.")
		return
	}

	byKind := map[string][]Problem{}
	for _, p := range problems {
		byKind[p.Kind] = append(byKind[p.Kind], p)
	}
	kinds := make([]string, 0, len(byKind))
	for kind := range byKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	fmt.Printf("\n%d problem(s) across %d kind(s):\n", len(problems), len(kinds))
	for _, kind := range kinds {
		list := byKind[kind]
		fmt.Printf("\n%s — %d question(s)\n", kind, len(list))
		shown := len(list)
		if *limit > 0 && shown > *limit {
			shown = *limit
		}
		for _, p := range list[:shown] {
			fmt.Printf("  id %-5d %s\n", p.ID, p.Details)
		}
		if shown < len(list) {
			fmt.Printf("  ... and %d more\n", len(list)-shown)
		}
	}
	os.Exit(1)
}

func validate(byLocale map[string]map[int]srcQuestion) []Problem {
	ids := map[int]bool{}
	for _, items := range byLocale {
		for id := range items {
			ids[id] = true
		}
	}
	ordered := make([]int, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)

	var problems []Problem
	for _, id := range ordered {
		var missing []string
		for _, loc := range locales {
			if _, ok := byLocale[loc][id]; !ok {
				missing = append(missing, loc)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, Problem{id, "missing-in-locale", fmt.Sprint(missing)})
			continue
		}

		base := byLocale[locales[0]][id]
		for _, loc := range locales[1:] {
			other := byLocale[loc][id]
			if len(base.Answers) != len(other.Answers) {
				problems = append(problems, Problem{id, "answer-count-mismatch",
					fmt.Sprintf("%s=%d vs %s=%d", locales[0], len(base.Answers), loc, len(other.Answers))})
			}
			if base.Image != other.Image {
				problems = append(problems, Problem{id, "image-mismatch",
					fmt.Sprintf("%s=%q vs %s=%q", locales[0], base.Image, loc, other.Image)})
			}
			if base.Ticket != other.Ticket {
				problems = append(problems, Problem{id, "ticket-mismatch",
					fmt.Sprintf("%s=%q vs %s=%q", locales[0], base.Ticket, loc, other.Ticket)})
			}
		}
		for _, loc := range locales {
			q := byLocale[loc][id]
			if q.CorrectAnswer < 1 || q.CorrectAnswer > len(q.Answers) {
				problems = append(problems, Problem{id, "correct-answer-out-of-range",
					fmt.Sprintf("%s: correct_answer=%d, answers=%d", loc, q.CorrectAnswer, len(q.Answers))})
			}
		}
	}
	return problems
}

func load(path string) (map[int]srcQuestion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list []srcQuestion
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make(map[int]srcQuestion, len(list))
	for _, q := range list {
		out[q.ID] = q
	}
	return out, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}

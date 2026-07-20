// Command convertavtoimtihon converts the user-owned avtoimtihon-format dataset
// (three per-locale questions.*.json files + a quiz-images directory) into the
// canonical import format (data.json + images/) consumable by cmd/importer.
//
// This is real, user-licensed content (spec D5), not the [NAMUNA] fixture.
//
// Regenerate:
//
//	cd backend && go run ./cmd/convertavtoimtihon \
//	  -src "/home/sher/Рабочий стол/aaa" -out seed/avtoimtihon
//
// then import:
//
//	go run ./cmd/importer -data seed/avtoimtihon -verified
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"avtotest.uz/backend/internal/importer"
)

func main() {
	src := flag.String("src", "/home/sher/Рабочий стол/aaa", "source dataset root (contains src/data + public/quiz-images)")
	out := flag.String("out", "seed/avtoimtihon", "output dataset directory (data.json + images/)")
	flag.Parse()

	res, err := Convert(*src)
	fatal(err)

	// Write data.json.
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}
	data, err := json.MarshalIndent(res.Dataset, "", "  ")
	fatal(err)
	fatal(os.WriteFile(filepath.Join(*out, "data.json"), data, 0o644))

	// Copy referenced images into the output layout.
	copied := 0
	for _, ic := range res.ImageCopies {
		dst := filepath.Join(*out, filepath.FromSlash(ic.Rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fatal(err)
		}
		if err := copyFile(ic.SrcAbs, dst); err != nil {
			fatal(fmt.Errorf("copy %s: %w", ic.Rel, err))
		}
		copied++
	}

	// Validate our own output before anyone attempts a DB import.
	issues := importer.Validate(res.Dataset)
	quarantined := importer.QuarantinedIDs(issues)

	// Report.
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	fmt.Printf("wrote %s\n", *out)
	fmt.Printf("  categories=%d questions=%d variants=%d explanations=%d\n",
		len(res.Dataset.Categories), len(res.Dataset.Questions),
		len(res.Dataset.Variants), len(res.Dataset.Explanations))
	fmt.Printf("  images: copied=%d missing-refs=%d %v\n", copied, len(res.MissingImg), res.MissingImg)
	fmt.Printf("  leftover (unassigned to any variant): %d %v\n", len(res.LeftoverExtI), res.LeftoverExtI)
	fmt.Printf("  no-comment questions (no explanation): %d\n", len(res.NoComment))
	fmt.Printf("  partial-comment questions: %d\n", len(res.PartialComnt))
	fmt.Printf("  answer reconciliations: %d\n", len(res.AnswerFixups))
	for _, f := range res.AnswerFixups {
		fmt.Printf("    - %s\n", f)
	}

	// Issue breakdown by code.
	byCode := map[string]int{}
	for _, is := range issues {
		byCode[is.Code]++
	}
	fmt.Printf("Validate: issues=%d quarantined=%d\n", len(issues), len(quarantined))
	for code, c := range byCode {
		fmt.Printf("  issue %s: %d\n", code, c)
	}
	for _, is := range issues {
		fmt.Printf("  ISSUE %s[%s] %s %s\n", is.Entity, is.ID, is.Code, is.Detail)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Command verifyexplanation marks the uz-Latn draft explanation for a
// question as verified by an expert reviewer — admin side, no HTTP endpoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
)

func main() {
	question := flag.String("question", "", "question UUID")
	by := flag.String("by", "", "reviewing profile UUID")
	flag.Parse()
	if *question == "" || *by == "" {
		fmt.Fprintln(os.Stderr, "usage: verifyexplanation -question <uuid> -by <profile-uuid>")
		os.Exit(2)
	}

	questionID, err := uuid.Parse(*question)
	fatal(err)
	verifiedBy, err := uuid.Parse(*by)
	fatal(err)

	cfg, err := config.Load()
	fatal(err)

	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	q := sqlc.New(pool)
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	err = svc.Verify(context.Background(), questionID, verifiedBy)
	if errors.Is(err, explanation.ErrNotFound) {
		fmt.Fprintln(os.Stderr, "error: no draft exists — run gendraft first")
		os.Exit(1)
	}
	fatal(err)

	fmt.Printf("verified explanation for question %s\n", questionID)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Command gendraft generates (or regenerates) an AI draft explanation for a
// question — admin side, no HTTP endpoint for this.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
)

func main() {
	question := flag.String("question", "", "question UUID")
	flag.Parse()
	if *question == "" {
		fmt.Fprintln(os.Stderr, "usage: gendraft -question <uuid>")
		os.Exit(2)
	}

	questionID, err := uuid.Parse(*question)
	fatal(err)

	cfg, err := config.Load()
	fatal(err)

	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	q := sqlc.New(pool)
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	err = svc.CreateDraft(context.Background(), questionID)
	if errors.Is(err, pgx.ErrNoRows) {
		fmt.Fprintln(os.Stderr, "error: question not found — check the -question UUID")
		os.Exit(1)
	}
	fatal(err)

	fmt.Printf("draft created for question %s\n", questionID)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Command linkquestionsigns applies a committed ext_id → sign-code map onto
// question_sign after questions and the sign catalog are already imported.
//
// The avtoimtihon question dump and the signs catalog are separate datasets;
// Validate() cannot resolve cross-dataset sign codes during question import.
// This command is the durable join step (see make seed-link-signs / seed-dev).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
)

func main() {
	linksPath := flag.String("links", "seed/avtoimtihon/question_signs.json", "JSON map of source_ext_id → []sign_code")
	flag.Parse()

	raw, err := os.ReadFile(*linksPath)
	fatal(err)

	var links map[string][]string
	fatal(json.Unmarshal(raw, &links))
	if len(links) == 0 {
		fmt.Fprintln(os.Stderr, "error: links file is empty")
		os.Exit(1)
	}

	cfg, err := config.Load()
	fatal(err)
	fatal(db.Migrate(cfg.DatabaseURL))

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	rep, err := applyLinks(ctx, pool, links)
	fatal(err)
	fmt.Printf("linkquestionsigns: questions=%d linked_rows=%d missing_questions=%d unknown_signs=%d empty=%d\n",
		rep.Questions, rep.Rows, len(rep.MissingQuestions), len(rep.UnknownSigns), rep.Empty)
	if len(rep.MissingQuestions) > 0 {
		fmt.Fprintf(os.Stderr, "missing questions (%d): %v\n", len(rep.MissingQuestions), trimList(rep.MissingQuestions, 20))
	}
	if len(rep.UnknownSigns) > 0 {
		fmt.Fprintf(os.Stderr, "unknown signs (%d): %v\n", len(rep.UnknownSigns), trimList(rep.UnknownSigns, 20))
		os.Exit(1)
	}
}

type report struct {
	Questions        int
	Rows             int
	Empty            int
	MissingQuestions []string
	UnknownSigns     []string
}

func applyLinks(ctx context.Context, pool *pgxpool.Pool, links map[string][]string) (report, error) {
	var rep report

	signRows, err := pool.Query(ctx, `SELECT id, code FROM sign`)
	if err != nil {
		return rep, err
	}
	signIDs := map[string]uuid.UUID{}
	for signRows.Next() {
		var id uuid.UUID
		var code string
		if err := signRows.Scan(&id, &code); err != nil {
			signRows.Close()
			return rep, err
		}
		signIDs[code] = id
	}
	signRows.Close()
	if err := signRows.Err(); err != nil {
		return rep, err
	}
	if len(signIDs) == 0 {
		return rep, fmt.Errorf("no signs in database — run seed-signs first")
	}

	extIDs := make([]string, 0, len(links))
	for extID := range links {
		extIDs = append(extIDs, extID)
	}
	sort.Strings(extIDs)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return rep, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	// Full replace: drop every existing link first so removals in the JSON
	// actually shrink per-sign question_count (otherwise stale rows linger).
	if _, err := tx.Exec(ctx, `DELETE FROM question_sign`); err != nil {
		return rep, fmt.Errorf("clear question_sign: %w", err)
	}

	unknown := map[string]bool{}
	for _, extID := range extIDs {
		codes := uniqueCodes(links[extID])
		rep.Questions++

		var qid uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM question WHERE source_ext_id = $1`, extID).Scan(&qid)
		if err != nil {
			rep.MissingQuestions = append(rep.MissingQuestions, extID)
			continue
		}

		if len(codes) == 0 {
			rep.Empty++
			continue
		}
		for _, code := range codes {
			sid, ok := signIDs[code]
			if !ok {
				if !unknown[code] {
					unknown[code] = true
					rep.UnknownSigns = append(rep.UnknownSigns, code)
				}
				continue
			}
			if err := q.InsertQuestionSign(ctx, sqlc.InsertQuestionSignParams{
				QuestionID: qid, SignID: sid,
			}); err != nil {
				return rep, fmt.Errorf("insert %s → %s: %w", extID, code, err)
			}
			rep.Rows++
		}
	}
	sort.Strings(rep.UnknownSigns)
	sort.Strings(rep.MissingQuestions)

	if len(rep.UnknownSigns) > 0 {
		return rep, nil // caller exits 1 after printing; no commit
	}
	if err := tx.Commit(ctx); err != nil {
		return rep, err
	}
	return rep, nil
}

func uniqueCodes(codes []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func trimList(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(append([]string{}, items[:n]...), "…")
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

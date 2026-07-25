// Command grantvip grants (or extends) a profile's VIP entitlement from the
// admin side — no payment involved.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
)

func main() {
	phone := flag.String("phone", "", "profile phone number (any UZ format)")
	days := flag.Int("days", 0, "number of days to grant")
	note := flag.String("note", "", "audit note")
	flag.Parse()
	if *phone == "" || *days <= 0 {
		fmt.Fprintln(os.Stderr, "usage: grantvip -phone <phone> -days <n> [-note <text>]")
		os.Exit(2)
	}

	cfg, err := config.Load()
	fatal(err)
	normalized, err := auth.NormalizePhone(*phone)
	fatal(err)

	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	q := sqlc.New(pool)
	profile, err := q.GetProfileByPhone(context.Background(), normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		fmt.Fprintln(os.Stderr, "error: profile not found — user must sign in once first")
		os.Exit(1)
	}
	fatal(err)

	// In a transaction so GrantDays' FOR UPDATE on the profile row actually
	// holds: outside one, each statement auto-commits and the lock is released
	// before the stacking insert, which would let this admin grant race a
	// webhook completing at the same moment and lose one of the two.
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	fatal(err)
	defer func() { _ = tx.Rollback(ctx) }()

	svc := billing.Service{Q: sqlc.New(tx)}
	until, err := svc.GrantDays(ctx, profile.ID, *days, "admin", *note, uuid.NullUUID{})
	fatal(err)
	fatal(tx.Commit(ctx))
	fmt.Printf("VIP until: %s\n", until.Format("2006-01-02T15:04:05Z07:00"))
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

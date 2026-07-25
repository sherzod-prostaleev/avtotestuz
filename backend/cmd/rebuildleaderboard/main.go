// Command rebuildleaderboard recomputes one or all leaderboard periods'
// Redis sorted sets from session_answer (the durable source of truth) —
// the recovery path for a lost/flushed/evicted Redis leaderboard key.
//
// Cap note (U-22): RebuildPeriod reapplies the daily points cap using each
// profile's CURRENT VIP status and CURRENT limit_config values — not perfect
// historical fidelity. See docs/superpowers/specs/2026-07-26-u22-leaderboard-rebuild-cap.md
// and m4-01 design §5.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/redisx"
)

func main() {
	periodFlag := flag.String("period", "all", "daily, weekly, monthly, alltime, or all")
	flag.Parse()

	var periods []leaderboard.Period
	switch *periodFlag {
	case "all":
		periods = leaderboard.AllPeriods
	case string(leaderboard.PeriodDaily), string(leaderboard.PeriodWeekly), string(leaderboard.PeriodMonthly), string(leaderboard.PeriodAllTime):
		periods = []leaderboard.Period{leaderboard.Period(*periodFlag)}
	default:
		fmt.Fprintln(os.Stderr, "usage: rebuildleaderboard [-period daily|weekly|monthly|alltime|all]")
		fmt.Fprintln(os.Stderr, "note: daily cap uses current VIP/limit_config (see U-22 rebuild-cap doc)")
		os.Exit(2)
	}

	cfg, err := config.Load()
	fatal(err)

	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	rdb, err := redisx.New(cfg.RedisURL)
	fatal(err)
	defer func() { _ = rdb.Close() }()

	q := sqlc.New(pool)
	svc := leaderboard.NewService(rdb, q, billing.Service{Q: q})

	now := time.Now().UTC()
	for _, p := range periods {
		if err := svc.RebuildPeriod(context.Background(), p, now); err != nil {
			fmt.Fprintf(os.Stderr, "error rebuilding %s: %v\n", p, err)
			os.Exit(1)
		}
		fmt.Printf("rebuilt %s\n", p)
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

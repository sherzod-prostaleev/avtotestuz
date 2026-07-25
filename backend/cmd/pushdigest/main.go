// Command pushdigest is the documented ops stub for future web-push retention
// digests (FSRS due reminders). Delivery itself already lives in
// internal/push.Service.Notify — this binary only defines the cron contract
// until product copy + due-queue selection land.
//
// Usage:
//
//	go run ./cmd/pushdigest            # dry-run (default): count subscribers
//	go run ./cmd/pushdigest -send      # exits 2 — send path not implemented yet
//
// Intended schedule (ops): daily ~09:00 Asia/Tashkent once -send is real.
// See docs/superpowers/specs/2026-07-26-m4-08-web-push-design.md §6–7.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
)

func main() {
	send := flag.Bool("send", false, "attempt digest delivery (not implemented yet)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var n int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM push_subscription`).Scan(&n); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("pushdigest dry-run: %d push subscription(s)\n", n)
	fmt.Println("contract: select profiles with FSRS due cards + ≥1 push_subscription;")
	fmt.Println("          call push.Service.Notify(kind=fsrs_due) with locale-safe URL;")
	fmt.Println("          skip if VAPID unconfigured; prune gone endpoints via sender.")
	fmt.Println("schedule (when implemented): daily ~09:00 Asia/Tashkent via host cron.")

	if *send {
		fmt.Fprintln(os.Stderr, "error: -send not implemented yet (U-11 retention digest deferred)")
		os.Exit(2)
	}
}

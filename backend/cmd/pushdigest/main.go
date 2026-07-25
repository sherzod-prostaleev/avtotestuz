// Command pushdigest runs the FSRS due retention web-push digest.
//
// Usage:
//
//	go run ./cmd/pushdigest                 # dry-run: count eligible profiles
//	go run ./cmd/pushdigest -send           # deliver via push.Service.Notify
//	go run ./cmd/pushdigest -send -limit 100
//
// Schedule (ops): daily ~09:00 Asia/Tashkent. Requires VAPID_* on the runner.
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
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
)

func main() {
	send := flag.Bool("send", false, "deliver FSRS due digests (default: dry-run)")
	limit := flag.Int("limit", 500, "max profiles to consider")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var subCount int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM push_subscription`).Scan(&subCount); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	svc := push.NewService(pool, sqlc.New(pool), push.Config{
		PublicKey:  cfg.VAPIDPublicKey,
		PrivateKey: cfg.VAPIDPrivateKey,
		Subject:    cfg.VAPIDSubject,
	}, nil)

	res, err := svc.RunFSRSDueDigest(ctx, push.DigestOpts{
		Limit:  *limit,
		DryRun: !*send,
	})
	if err != nil {
		if err == push.ErrUnconfigured {
			fmt.Fprintln(os.Stderr, "error: VAPID keys not configured — set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	mode := "dry-run"
	if *send {
		mode = "send"
	}
	fmt.Printf("pushdigest %s: subscriptions=%d candidates=%d notified=%d deliveries=%d skipped=%d errors=%d limit=%d\n",
		mode, subCount, res.Candidates, res.Notified, res.Deliveries, res.Skipped, res.Errors, *limit)
	if !*send {
		fmt.Println("hint: pass -send to deliver (requires VAPID_*); gone endpoints are pruned automatically")
	}
}

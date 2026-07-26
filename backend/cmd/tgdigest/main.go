// Command tgdigest sends soft FSRS-due reminders to linked Telegram accounts.
//
// Usage:
//
//	go run ./cmd/tgdigest                 # dry-run: count eligible profiles
//	go run ./cmd/tgdigest -send           # deliver DMs
//	go run ./cmd/tgdigest -send -limit 100
//
// Schedule (ops): daily ~09:00 Asia/Tashkent. Requires TELEGRAM_BOT_TOKEN.
// Groups are NOT messaged — on-demand /quiz is the group growth surface.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
)

func main() {
	send := flag.Bool("send", false, "deliver Telegram DM digests (default: dry-run)")
	limit := flag.Int("limit", 500, "max linked profiles to consider")
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

	var tg *bot.Client
	if *send {
		if strings.TrimSpace(cfg.TelegramBotToken) == "" {
			fmt.Fprintln(os.Stderr, "error: TELEGRAM_BOT_TOKEN required for -send")
			os.Exit(1)
		}
		tg = bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
	}

	svc := &bot.DigestService{
		Q:             sqlc.New(pool),
		Pool:          pool,
		TG:            tg,
		PublicBaseURL: cfg.PublicBaseURL,
	}

	res, err := svc.RunDueDigest(ctx, bot.DigestOpts{
		Limit:  *limit,
		DryRun: !*send,
	})
	if err != nil {
		if errors.Is(err, bot.ErrDigestDisabled) {
			fmt.Fprintln(os.Stderr, "error: feature_flag telegram_dm_digest is off")
			os.Exit(1)
		}
		if errors.Is(err, bot.ErrDigestUnconfigured) {
			fmt.Fprintln(os.Stderr, "error: TELEGRAM_BOT_TOKEN not configured")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	mode := "dry-run"
	if *send {
		mode = "send"
	}
	fmt.Printf("tgdigest %s: candidates=%d notified=%d errors=%d skipped=%d limit=%d\n",
		mode, res.Candidates, res.Notified, res.Errors, res.Skipped, *limit)
}

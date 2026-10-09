// Command tgdigest previews the daily «Kun savoli» Telegram reminder.
//
// It only ever reads: the reminder itself is sent by the api process at
// 19:00 Asia/Tashkent (bot.RunDailyReminderScheduler, flag
// telegram_daily_reminder). This tool prints today's audience by personal
// line segment and the question a run would send, and sends nothing.
//
// Usage:
//
//	go run ./cmd/tgdigest --dry-run
//	docker exec <api container> /tgdigest --dry-run     # prod (env from the container)
//
// It replaces the old linked-only due digest (`tgdigest -send`), which is
// gone so that exactly one sender exists.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print today's reminder audience and question; sends nothing")
	flag.Parse()
	if !*dryRun {
		fmt.Fprintln(os.Stderr, "tgdigest only previews (--dry-run). The api process sends the daily reminder at 19:00 Tashkent when feature flag telegram_daily_reminder is on.")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()

	r := &bot.DailyReminder{
		Q:             sqlc.New(pool),
		Pool:          pool,
		TG:            bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil),
		MediaBaseURL:  cfg.MediaBaseURL,
		PublicBaseURL: cfg.PublicBaseURL,
		WebAppURL:     cfg.TelegramWebAppURL,
	}
	rep, err := r.DryRun(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("daily reminder dry-run (nothing sent)\n")
	fmt.Printf("  day (Tashkent)   %s, now %s, in 19:00-21:00 window: %v\n", rep.Day, rep.Now.Format("15:04"), rep.InWindow)
	fmt.Printf("  flag on          %v (telegram_daily_reminder)\n", rep.FlagOn)
	fmt.Printf("  bot token set    %v\n", rep.TokenSet)
	fmt.Printf("  audience         total=%d eligible=%d pending_today=%d opted_out=%d blocked=%d\n",
		rep.Total, rep.Eligible, rep.Pending, rep.OptedOut, rep.Blocked)
	keys := make([]string, 0, len(rep.Segments))
	for k := range rep.Segments {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  by segment      ")
	for _, k := range keys {
		fmt.Printf(" %s=%d", k, rep.Segments[k])
	}
	fmt.Println()
	if rep.QuestionErr != "" {
		fmt.Printf("  question         NONE: %s\n", rep.QuestionErr)
		os.Exit(1)
	}
	source := "would be chosen"
	if rep.QuestionStored {
		source = "already recorded for today"
	}
	fmt.Printf("  question         %s (%s)\n", rep.QuestionID, source)
	fmt.Printf("  fits poll limits %v (q<=300, options<=100, <=10 options; uz-Latn and ru)\n", rep.QuestionFits)
	fmt.Printf("  image            %v; explanation uz=%v ru=%v (<=200 or omitted)\n",
		rep.HasImage, rep.ExplanationUz, rep.ExplanationRu)
}

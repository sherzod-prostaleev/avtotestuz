// Command payrecon runs a dry-run payment↔provider consistency check (U-27).
//
// Usage:
//
//	go run ./cmd/payrecon                 # last 24h
//	go run ./cmd/payrecon -hours 48
//
// Does not call live Payme/Click APIs (no merchant keys required). Compares
// local payment rows to payme_transaction / click_transaction — the same
// window GetStatement would serve. Exit 1 if any error-severity findings.
//
// Intended schedule: daily after settlement; future M3 admin queue can persist findings.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"avtotest.uz/backend/internal/billing/recon"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
)

func main() {
	hours := flag.Int("hours", 24, "lookback window in hours")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()

	to := time.Now().UTC()
	from := to.Add(-time.Duration(*hours) * time.Hour)
	res, err := recon.Run(ctx, pool, recon.Options{From: from, To: to})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("payrecon dry-run: from=%s to=%s payments=%d provider_txns=%d findings=%d\n",
		res.From.Format(time.RFC3339), res.To.Format(time.RFC3339),
		res.ScannedPay, res.ScannedTxn, len(res.Findings))
	errCount := 0
	for _, f := range res.Findings {
		fmt.Printf("  [%s] %s payment=%s provider=%s — %s\n",
			f.Severity, f.Code, f.PaymentID, f.Provider, f.Detail)
		if f.Severity == recon.SeverityError {
			errCount++
		}
	}
	if errCount > 0 {
		os.Exit(1)
	}
}

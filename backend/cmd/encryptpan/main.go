package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	secret := os.Getenv("JWT_SECRET")
	if databaseURL == "" || len(secret) < 32 {
		log.Fatal("DATABASE_URL and JWT_SECRET (at least 32 bytes) are required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	result, err := billing.EncryptStoredPANs(ctx, pool, []byte(secret))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("manual_cards_encrypted=%d referral_payouts_encrypted=%d\n", result.ManualCards, result.ReferralPayouts)
}

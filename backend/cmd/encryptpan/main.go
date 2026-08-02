package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/config"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	// Same resolution as the API (config.Config.DataKey): DATA_ENCRYPTION_KEY
	// when set, else JWT_SECRET. Encrypting under a key the server would not
	// derive is silent, unrecoverable data loss — the rows still exist and
	// nothing can read them.
	secret := config.ResolveDataKey(os.Getenv("DATA_ENCRYPTION_KEY"), os.Getenv("JWT_SECRET"))
	if databaseURL == "" || len(secret) < 32 {
		log.Fatal("DATABASE_URL and DATA_ENCRYPTION_KEY (or JWT_SECRET, when the former is unset), at least 32 bytes, are required")
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

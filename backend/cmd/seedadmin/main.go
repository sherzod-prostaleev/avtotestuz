// Command seedadmin creates or updates a local superadmin from env.
//
// Required:
//
//	ADMIN_SEED_EMAIL
//	ADMIN_SEED_PASSWORD  (min 8 chars; never commit a real value)
//
// Optional:
//
//	ADMIN_SEED_NAME  (default Superadmin)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"avtotest.uz/backend/internal/admin"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
)

func main() {
	email := strings.TrimSpace(os.Getenv("ADMIN_SEED_EMAIL"))
	password := os.Getenv("ADMIN_SEED_PASSWORD")
	name := strings.TrimSpace(os.Getenv("ADMIN_SEED_NAME"))
	if email == "" || password == "" {
		fmt.Fprintln(os.Stderr, "usage: ADMIN_SEED_EMAIL=... ADMIN_SEED_PASSWORD=... [ADMIN_SEED_NAME=...] go run ./cmd/seedadmin")
		os.Exit(2)
	}

	cfg, err := config.Load()
	fatal(err)
	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	id, err := admin.Store{Pool: pool}.EnsureSuperadmin(context.Background(), email, password, name)
	fatal(err)
	fmt.Printf("superadmin ready id=%s email=%s\n", id, strings.ToLower(email))
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

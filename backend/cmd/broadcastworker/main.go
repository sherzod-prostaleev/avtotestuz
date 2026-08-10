// Command broadcastworker drains queued broadcast campaigns (outbox).
//
// Usage:
//
//	go run ./cmd/broadcastworker           # run until interrupted
//	go run ./cmd/broadcastworker -once     # single expand+claim cycle
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/broadcast"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/redisx"
)

func main() {
	once := flag.Bool("once", false, "run a single process cycle and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var lim auth.Limiter
	if rdb, err := redisx.New(cfg.RedisURL); err == nil {
		lim = auth.Limiter{R: rdb}
		defer func() { _ = rdb.Close() }()
	}

	q := sqlc.New(pool)
	pushSvc := push.NewService(pool, q, push.Config{
		PublicKey:  cfg.VAPIDPublicKey,
		PrivateKey: cfg.VAPIDPrivateKey,
		Subject:    cfg.VAPIDSubject,
	}, nil)
	svc := &broadcast.Service{
		Pool: pool,
		Q:    q,
		Push: pushSvc,
		Cfg: broadcast.Config{
			MaxRecipients: cfg.BroadcastMaxRecipients,
			ImageHosts:    cfg.BroadcastImageHosts,
		},
		Lim: lim,
	}

	if *once {
		runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := svc.ProcessOnce(runCtx); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("broadcastworker: once ok")
		return
	}

	broadcast.RunWorker(ctx, svc, nil)
}

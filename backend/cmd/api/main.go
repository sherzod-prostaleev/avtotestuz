package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/sentryx"
	"avtotest.uz/backend/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger, _ := zap.NewDevelopment()
	if cfg.Env == "prod" {
		logger, _ = zap.NewProduction()
	}
	defer func() { _ = logger.Sync() }()

	flushSentry, err := sentryx.Init(cfg.SentryDSN, cfg.Env)
	if err != nil {
		logger.Fatal("sentry", zap.Error(err))
	}
	defer flushSentry()
	if cfg.SentryDSN != "" {
		logger.Info("sentry: enabled")
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Fatal("migrate", zap.Error(err))
	}
	pool, err := db.NewPoolConfigured(context.Background(), cfg.DatabaseURL, db.PoolConfig{
		MaxConns: cfg.DBPoolMaxConns, MinConns: cfg.DBPoolMinConns,
		MaxConnLifetime: cfg.DBPoolMaxLifetime, MaxConnIdleTime: cfg.DBPoolMaxIdleTime,
		HealthCheckPeriod: cfg.DBPoolHealthCheck,
	})
	if err != nil {
		logger.Fatal("db", zap.Error(err))
	}
	defer pool.Close()
	if created, err := db.MaintainEventPartitions(context.Background(), pool, 18); err != nil {
		logger.Error("event partition maintenance", zap.Error(err))
	} else if created > 0 {
		logger.Info("event partitions prepared", zap.Int("created", created))
	}

	redisClient, err := redisx.New(cfg.RedisURL)
	if err != nil {
		logger.Fatal("redis", zap.Error(err))
	}
	defer func() { _ = redisClient.Close() }()

	h, arenaSvc := server.New(cfg, server.Deps{
		Queries: sqlc.New(pool),
		Pool:    pool,
		Redis:   redisClient,
		Log:     logger,
	})
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go maintainEventPartitions(ctx, pool, logger)

	// Long-poll is the dev-only alternative to the webhook route server.New
	// registers — see docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md
	// §5.1. config.validate() already rejects this mode when ENV=prod.
	if cfg.TelegramBotMode == "longpoll" {
		q := sqlc.New(pool)
		tgClient := bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
		linkSvc := bot.NewLinkService(pool, q)
		quizSvc := &bot.QuizService{
			Q:             q,
			Pool:          pool,
			TG:            tgClient,
			MediaBaseURL:  cfg.MediaBaseURL,
			PublicBaseURL: cfg.PublicBaseURL,
			WinnerSticker: cfg.TelegramQuizWinnerSticker,
			Log:           logger,
		}
		quizSvc.Advance = bot.NewAdvanceScheduler(quizSvc, logger)
		progressSvc := progress.NewService(q)
		progressSvc.Billing = billing.Service{Q: q}
		botSvc := &bot.Bot{
			Link:          linkSvc,
			Quiz:          quizSvc,
			Billing:       billing.Service{Q: q},
			Progress:      progressSvc,
			TG:            tgClient,
			PublicBaseURL: cfg.PublicBaseURL,
			Log:           logger,
		}
		go bot.RunLongPoll(ctx, tgClient, botSvc, logger)
		logger.Info("telegram bot: long-poll started")
	}

	go func() {
		logger.Info("listening", zap.Int("port", cfg.Port), zap.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if arenaSvc != nil {
		arenaSvc.Drain(shutdownCtx)
	}
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("stopped")
}

func maintainEventPartitions(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			created, err := db.MaintainEventPartitions(ctx, pool, 18)
			if err != nil {
				logger.Error("event partition maintenance", zap.Error(err))
			} else if created > 0 {
				logger.Info("event partitions prepared", zap.Int("created", created))
			}
		}
	}
}

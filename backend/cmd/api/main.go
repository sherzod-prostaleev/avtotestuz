package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/broadcast"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/sentryx"
	"avtotest.uz/backend/internal/server"
	"avtotest.uz/backend/internal/session"
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

	// One avatar service for the process (the dev long-poll bot shares it),
	// so shutdown can drain it.
	avatars := server.NewAvatarService(cfg, sqlc.New(pool), logger)
	h, arenaSvc, broadcastSvc := server.New(cfg, server.Deps{
		Queries: sqlc.New(pool),
		Pool:    pool,
		Redis:   redisClient,
		Log:     logger,
		Avatars: avatars,
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
	// event_batch holds one row per accepted telemetry batch purely to make a
	// client retry idempotent, and nothing ever removed them -- the table grew
	// for the life of the database remembering keys no client will present
	// again. The partition maintainer above is deliberately separate: it never
	// drops an event, and this never touches one.
	go events.RunRetentionWorker(ctx, events.NewService(sqlc.New(pool), pool), logger)
	go billing.RunManualExpireWorker(ctx, billing.Service{Q: sqlc.New(pool)}, logger)
	// An exam whose clock ran out cannot be resumed, but nothing closed it
	// either: it stayed 'in_progress' for good, showing in the learner's
	// history as still running. Untimed sessions -- practice, variant, review,
	// mistakes -- are outside this sweep and stay open, because reopening them
	// where the class stopped is the feature. See ExpireTimedOutSessions.
	go session.RunExpiryWorker(ctx, newExpirySessionService(pool), logger)
	if broadcastSvc != nil {
		go broadcast.RunWorker(ctx, broadcastSvc, logger)
	}

	// Menu button and command menu sync is best-effort and off the startup
	// path: a slow or unreachable Telegram API must never delay or fail boot.
	if menuButtonSyncWanted(cfg) {
		go func() {
			syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			tg := bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
			if err := bot.SyncMenuButton(syncCtx, tg, cfg.TelegramWebAppURL); err != nil {
				logger.Warn("telegram bot: menu button sync failed", zap.Error(err))
			}
			if err := bot.SyncCommands(syncCtx, tg); err != nil {
				logger.Warn("telegram bot: command menu sync failed", zap.Error(err))
			}
		}()
	}

	// The daily «Kun savoli» reminder runs on its own goroutine so a slow
	// Telegram never touches request handling. It also needs the flag
	// telegram_daily_reminder (default off), checked on every tick.
	if dailyReminderWanted(cfg) {
		go bot.RunDailyReminderScheduler(ctx, &bot.DailyReminder{
			Q:             sqlc.New(pool),
			Pool:          pool,
			TG:            bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil),
			MediaBaseURL:  cfg.MediaBaseURL,
			PublicBaseURL: cfg.PublicBaseURL,
			WebAppURL:     cfg.TelegramWebAppURL,
			Log:           logger,
		})
	}

	// Long-poll is the dev-only alternative to the webhook route server.New
	// registers — see docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md
	// §5.1. config.validate() already rejects this mode when ENV=prod.
	if cfg.TelegramBotMode == "longpoll" {
		q := sqlc.New(pool)
		tgClient := bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
		linkSvc := bot.NewLinkService(pool, q)
		if avatars != nil {
			linkSvc.Avatars = avatars
		}
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
		sender, err := auth.SenderFor(cfg, logger)
		if err != nil {
			logger.Fatal("otp sender", zap.Error(err))
		}
		authSvc := auth.NewService(q, pool, auth.Limiter{R: redisClient}, sender, []byte(cfg.JWTSecret), cfg.Env)
		authSvc.Log = logger
		if avatars != nil {
			authSvc.Avatars = avatars
		}
		botSvc := &bot.Bot{
			WebAppURL:     cfg.TelegramWebAppURL,
			BotUsername:   cfg.TelegramBotUsername,
			Link:          linkSvc,
			Quiz:          quizSvc,
			Billing:       billing.Service{Q: q},
			Progress:      progressSvc,
			TG:            tgClient,
			Auth:          authSvc,
			PublicBaseURL: cfg.PublicBaseURL,
			Log:           logger,
			BotUsers:      q,
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
	// After the HTTP server: no handler can enqueue more work. In-flight
	// photo jobs get a few seconds, then are cancelled and clean up.
	avatarCtx, avatarCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer avatarCancel()
	avatars.Shutdown(avatarCtx)
	logger.Info("stopped")
}

// menuButtonSyncWanted ignores TELEGRAM_BOT_MODE on purpose: the menu button
// lives on Telegram's side and outlives our process, so with the bot switched
// off (mode=off) a stale web_app button would keep opening the Mini App. Any
// token is enough to reset it to match TELEGRAM_WEBAPP_URL (the kill switch).
func menuButtonSyncWanted(cfg config.Config) bool {
	return strings.TrimSpace(cfg.TelegramBotToken) != ""
}

// dailyReminderWanted needs a live bot, not just a token: the reminder's
// opt-out button and /eslatma are answered by the webhook or long-poll
// dispatcher, and sending buttons nobody handles would strand them.
func dailyReminderWanted(cfg config.Config) bool {
	return strings.TrimSpace(cfg.TelegramBotToken) != "" &&
		(cfg.TelegramBotMode == "webhook" || cfg.TelegramBotMode == "longpoll")
}

// newExpirySessionService builds the session service the expiry worker runs
// on. It is its own instance, like every other worker's, rather than the one
// the router holds.
//
// The pass-rate cache is not optional here: finishing an exam takes a
// readiness snapshot, and one sweep finishes up to expirySweepLimit of them.
// Without the cache that would be one full scan of exam_session per session
// closed, which is the very cost this sweep exists to stop growing.
func newExpirySessionService(pool *pgxpool.Pool) *session.Service {
	q := sqlc.New(pool)
	learningSvc := learning.NewService(q)
	learningSvc.PassRates = learning.NewPassRateCache(learning.DefaultPassRateTTL)
	progressSvc := progress.NewService(q)
	progressSvc.Learning = learningSvc
	return session.NewService(q, pool, billing.Service{Q: q}, learningSvc, progressSvc)
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

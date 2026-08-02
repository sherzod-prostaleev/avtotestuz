// Package server wires HTTP routes and middleware.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/admin"
	"avtotest.uz/backend/internal/arena"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/billing/click"
	"avtotest.uz/backend/internal/billing/payme"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/demo"
	"avtotest.uz/backend/internal/devicefp"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/flags"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/ops"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/site"
	"avtotest.uz/backend/internal/support"
)

type Deps struct {
	Queries *sqlc.Queries
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Log     *zap.Logger
}

// New wires HTTP routes. The optional *arena.Service is non-nil when Redis+DB
// are configured; callers should Drain it before http.Server.Shutdown.
func New(cfg config.Config, deps Deps) (http.Handler, *arena.Service) {
	r := chi.NewRouter()
	// NOTE: no middleware.RealIP — it trusts spoofable headers (GHSA-3fxj-6jh8-hvhx).
	// Real client IP extraction will be added with trusted-proxy config in Plan 02.
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(limitRequestBody(defaultMaxRequestBodyBytes))
	if cfg.Env == "dev" {
		r.Use(middleware.Logger)
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Ops-Token", devicefp.Header},
	}))

	// U-41: process-local request counters (not Prometheus). Must wrap before
	// routes so status classes are visible; Hijacker-safe for Arena WS.
	metrics := NewRequestMetrics()
	r.Use(metrics.Middleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// /readyz = process can serve traffic (Postgres + Redis when wired).
	// /healthz stays a cheap liveness probe that does not touch dependencies.
	r.Get("/readyz", readinessHandler(deps.Pool, deps.Redis))
	r.Get("/metrics", metrics.Handler())

	// dataKey seals data at rest (admin TOTP secrets, stored card PANs, the
	// manual-pay Telegram credentials). It is DATA_ENCRYPTION_KEY when the
	// deployment sets one and the JWT secret otherwise — the fallback is what
	// keeps ciphertext written before the two keys were split readable, so it
	// must be threaded into every Service that touches those columns.
	dataKey := []byte(cfg.DataKey())

	// M3 Super Admin — separate mount from learner /api/v1 (blast-radius isolation).
	var pushSvc *push.Service
	if deps.Pool != nil && deps.Queries != nil {
		pushSvc = push.NewService(deps.Pool, deps.Queries, push.Config{
			PublicKey:  cfg.VAPIDPublicKey,
			PrivateKey: cfg.VAPIDPrivateKey,
			Subject:    cfg.VAPIDSubject,
		}, nil)
	}
	if deps.Pool != nil {
		adminStore := admin.Store{Pool: deps.Pool}
		adminH := &admin.Handler{
			Svc:             admin.Service{Store: adminStore, Secret: []byte(cfg.JWTSecret), DataKey: dataKey, Lim: auth.Limiter{R: deps.Redis}},
			Pool:            deps.Pool,
			Redis:           deps.Redis,
			Secret:          []byte(cfg.JWTSecret),
			Billing:         billing.Service{Q: deps.Queries, Pool: deps.Pool, PublicBaseURL: cfg.PublicBaseURL, Secret: []byte(cfg.JWTSecret), DataSecret: dataKey},
			MetricsSnapshot: metrics.Snapshot,
			Push:            pushSvc,
		}
		r.Route("/admin/v1", adminH.Routes)
	}

	var arenaSvc *arena.Service
	if deps.Queries != nil {
		r.Route("/api/v1", func(api chi.Router) {
			api.Use(devicefp.Middleware)

			var stationVIP billing.StationVIPChecker
			if deps.Pool != nil {
				stationVIP = b2b.Store{Pool: deps.Pool}
			}
			learnerBilling := billing.Service{
				Q:             deps.Queries,
				Pool:          deps.Pool,
				StationVIP:    stationVIP,
				PublicBaseURL: cfg.PublicBaseURL,
				Secret:        []byte(cfg.JWTSecret),
				DataSecret:    dataKey,
			}

			ch := &content.Handler{Q: deps.Queries, MediaBase: cfg.MediaBaseURL}
			ch.Routes(api)

			bh := &billing.Handler{
				// PublicBaseURL only matters on this Service — it serves
				// GET /me/referral, the one endpoint that builds a shareable link.
				Svc:               learnerBilling,
				PaymeMerchantID:   cfg.PaymeMerchantID,
				PaymeCheckoutHost: cfg.PaymeCheckoutHost(),
				ClickServiceID:    cfg.ClickServiceID,
				ClickMerchantID:   cfg.ClickMerchantID,
				ManualIngestToken: cfg.ManualPayIngestToken,
			}
			bh.Routes(api)
			bh.InternalRoutes(api)

			sh := &site.Handler{Pool: deps.Pool}
			sh.PublicRoutes(api)

			sup := &support.Handler{
				Pool:      deps.Pool,
				Lim:       auth.Limiter{R: deps.Redis},
				ClientIPs: auth.NewClientIPResolver([]byte(cfg.ClientIPAssertionSecret)),
			}
			sup.PublicRoutes(api)

			fh := &flags.Handler{Pool: deps.Pool}
			fh.PublicRoutes(api)

			if cfg.OpsAdminToken != "" {
				oh := &ops.Handler{
					Billing: billing.Service{Q: deps.Queries, Pool: deps.Pool, PublicBaseURL: cfg.PublicBaseURL},
					Pool:    deps.Pool,
					Token:   cfg.OpsAdminToken,
				}
				oh.Routes(api)
			}

			if deps.Pool != nil && deps.Redis != nil {
				log := deps.Log
				if log == nil {
					log = zap.NewNop()
				}
				sender, err := auth.SenderFor(cfg, log)
				if err != nil {
					log.Fatal("otp sender", zap.Error(err))
				}
				svc := auth.NewService(deps.Queries, deps.Pool, auth.Limiter{R: deps.Redis},
					sender, []byte(cfg.JWTSecret), cfg.Env)
				svc.DebugEcho = cfg.OTPDebugEcho
				ah := &auth.Handler{
					Svc:       svc,
					ClientIPs: auth.NewClientIPResolver([]byte(cfg.ClientIPAssertionSecret)),
				}
				ah.Routes(api)

				dh := &demo.Handler{Svc: demo.NewService(deps.Queries, ch, auth.Limiter{R: deps.Redis})}
				dh.Routes(api)

				learnerAuth := api.With(auth.Required([]byte(cfg.JWTSecret)), auth.RejectBanned(deps.Queries))

				acc := &account.Handler{Q: deps.Queries, Billing: learnerBilling}
				acc.Routes(learnerAuth)

				tb2b := &b2b.Handler{Pool: deps.Pool}
				tb2b.AuthedRoutes(learnerAuth)

				bh.AuthedRoutes(learnerAuth)

				pmh := &payme.Handler{Q: deps.Queries, Svc: billing.Service{Q: deps.Queries}, Key: cfg.PaymeKey(), Pool: deps.Pool}
				api.Post("/billing/payme", pmh.ServeHTTP)

				cmh := &click.Handler{Q: deps.Queries, Svc: billing.Service{Q: deps.Queries}, ServiceID: cfg.ClickServiceID, SecretKey: cfg.ClickSecretKey, Pool: deps.Pool}
				api.Post("/billing/click", cmh.ServeHTTP)

				learningSvc := learning.NewService(deps.Queries)
				progressSvc := progress.NewService(deps.Queries)
				progressSvc.Learning = learningSvc
				progressSvc.Billing = learnerBilling
				lbSvc := leaderboard.NewService(deps.Redis, deps.Queries, learnerBilling)
				sessSvc := session.NewService(deps.Queries, deps.Pool, learnerBilling, learningSvc, progressSvc)
				sessSvc.Leaderboard = lbSvc
				sess := &session.Handler{
					Svc:     sessSvc,
					Content: ch,
				}
				sess.PublicRoutes(api)
				sess.Routes(learnerAuth)

				lbh := &leaderboard.Handler{Svc: lbSvc}
				lbh.Routes(learnerAuth)

				lh := &learning.Handler{Svc: learningSvc}
				lh.Routes(learnerAuth)

				eh := &explanation.Handler{Svc: explanation.NewService(deps.Queries, explanation.TemplateDraftGenerator{})}
				eh.Routes(learnerAuth)

				ph := &progress.Handler{Svc: progressSvc}
				ph.Routes(learnerAuth)

				evh := &events.Handler{Svc: events.NewService(deps.Queries, deps.Pool)}
				evh.Routes(learnerAuth)

				arenaSvc = arena.NewService(deps.Queries, deps.Pool, deps.Redis,
					learnerBilling, ch, learningSvc, progressSvc, log)
				ahArena := &arena.Handler{Svc: arenaSvc, PublicURL: cfg.PublicBaseURL}
				ahArena.PublicRoutes(api)
				ahArena.AuthedRoutes(learnerAuth)

				// Link-token + status are web-facing and do not need an active
				// bot consumer — mount whenever the API has DB+Redis. Webhook /
				// longpoll still gate on TelegramBotMode.
				linkSvc := bot.NewLinkService(deps.Pool, deps.Queries)
				tbh := &bot.Handler{Link: linkSvc, BotUsername: cfg.TelegramBotUsername}
				tbh.AuthedRoutes(learnerAuth)

				if pushSvc == nil {
					pushSvc = push.NewService(deps.Pool, deps.Queries, push.Config{
						PublicKey:  cfg.VAPIDPublicKey,
						PrivateKey: cfg.VAPIDPrivateKey,
						Subject:    cfg.VAPIDSubject,
					}, nil)
				}
				phPush := &push.Handler{Svc: pushSvc}
				phPush.AuthedRoutes(learnerAuth)

				sup.AuthedRoutes(learnerAuth)

				if cfg.TelegramBotMode == "webhook" {
					tgClient := bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
					quizSvc := &bot.QuizService{
						Q:             deps.Queries,
						Pool:          deps.Pool,
						TG:            tgClient,
						MediaBaseURL:  cfg.MediaBaseURL,
						PublicBaseURL: cfg.PublicBaseURL,
						WinnerSticker: cfg.TelegramQuizWinnerSticker,
						Log:           log,
					}
					quizSvc.Advance = bot.NewAdvanceScheduler(quizSvc, log)
					botSvc := &bot.Bot{
						Link:          linkSvc,
						Quiz:          quizSvc,
						Billing:       learnerBilling,
						Progress:      progressSvc,
						TG:            tgClient,
						PublicBaseURL: cfg.PublicBaseURL,
						Log:           log,
					}
					wh := &bot.WebhookHandler{Bot: botSvc, Secret: cfg.TelegramWebhookSecret, Log: log}
					api.Post("/telegram/webhook", wh.ServeHTTP)
				}
				// longpoll mode registers no HTTP route — cmd/api starts
				// bot.RunLongPoll instead (design §5.1).
			}
		})
	}

	return r, arenaSvc
}

const readinessProbeTimeout = 2 * time.Second

func readinessHandler(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{}
		ready := true

		ctx, cancel := context.WithTimeout(r.Context(), readinessProbeTimeout)
		defer cancel()

		if pool == nil {
			checks["postgres"] = "skipped"
		} else if err := pool.Ping(ctx); err != nil {
			checks["postgres"] = "fail"
			ready = false
		} else {
			checks["postgres"] = "ok"
		}

		if rdb == nil {
			checks["redis"] = "skipped"
		} else if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "fail"
			ready = false
		} else {
			checks["redis"] = "ok"
		}

		status := "ok"
		code := http.StatusOK
		if !ready {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}
		httpx.Data(w, code, map[string]any{"status": status, "checks": checks})
	}
}

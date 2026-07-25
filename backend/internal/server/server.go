// Package server wires HTTP routes and middleware.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/billing/click"
	"avtotest.uz/backend/internal/billing/payme"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/demo"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/session"
)

type Deps struct {
	Queries *sqlc.Queries
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Log     *zap.Logger
}

func New(cfg config.Config, deps Deps) http.Handler {
	r := chi.NewRouter()
	// NOTE: no middleware.RealIP — it trusts spoofable headers (GHSA-3fxj-6jh8-hvhx).
	// Real client IP extraction will be added with trusted-proxy config in Plan 02.
	r.Use(middleware.RequestID, middleware.Recoverer)
	if cfg.Env == "dev" {
		r.Use(middleware.Logger)
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if deps.Queries != nil {
		r.Route("/api/v1", func(api chi.Router) {
			ch := &content.Handler{Q: deps.Queries, MediaBase: cfg.MediaBaseURL}
			ch.Routes(api)

			bh := &billing.Handler{
				// PublicBaseURL only matters on this Service — it serves
				// GET /me/referral, the one endpoint that builds a shareable link.
				Svc:               billing.Service{Q: deps.Queries, Pool: deps.Pool, PublicBaseURL: cfg.PublicBaseURL},
				PaymeMerchantID:   cfg.PaymeMerchantID,
				PaymeCheckoutHost: cfg.PaymeCheckoutHost(),
				ClickServiceID:    cfg.ClickServiceID,
				ClickMerchantID:   cfg.ClickMerchantID,
			}
			bh.Routes(api)

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
				ah := &auth.Handler{
					Svc:       svc,
					ClientIPs: auth.NewClientIPResolver([]byte(cfg.ClientIPAssertionSecret)),
				}
				ah.Routes(api)

				dh := &demo.Handler{Svc: demo.NewService(deps.Queries, ch, auth.Limiter{R: deps.Redis})}
				dh.Routes(api)

				acc := &account.Handler{Q: deps.Queries, Billing: billing.Service{Q: deps.Queries}}
				acc.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				bh.AuthedRoutes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				pmh := &payme.Handler{Q: deps.Queries, Svc: billing.Service{Q: deps.Queries}, Key: cfg.PaymeKey(), Pool: deps.Pool}
				api.Post("/billing/payme", pmh.ServeHTTP)

				cmh := &click.Handler{Q: deps.Queries, Svc: billing.Service{Q: deps.Queries}, ServiceID: cfg.ClickServiceID, SecretKey: cfg.ClickSecretKey, Pool: deps.Pool}
				api.Post("/billing/click", cmh.ServeHTTP)

				learningSvc := learning.NewService(deps.Queries)
				progressSvc := progress.NewService(deps.Queries)
				lbSvc := leaderboard.NewService(deps.Redis, deps.Queries, billing.Service{Q: deps.Queries})
				sessSvc := session.NewService(deps.Queries, billing.Service{Q: deps.Queries}, learningSvc, progressSvc)
				sessSvc.Leaderboard = lbSvc
				sess := &session.Handler{
					Svc:     sessSvc,
					Content: ch,
				}
				sess.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				lbh := &leaderboard.Handler{Svc: lbSvc}
				lbh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				lh := &learning.Handler{Svc: learningSvc}
				lh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				eh := &explanation.Handler{Svc: explanation.NewService(deps.Queries, explanation.TemplateDraftGenerator{})}
				eh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				ph := &progress.Handler{Svc: progressSvc}
				ph.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				evh := &events.Handler{Svc: events.NewService(deps.Queries)}
				evh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				if cfg.TelegramBotMode != "off" {
					linkSvc := bot.NewLinkService(deps.Pool, deps.Queries)
					tbh := &bot.Handler{Link: linkSvc, BotUsername: cfg.TelegramBotUsername}
					tbh.AuthedRoutes(api.With(auth.Required([]byte(cfg.JWTSecret))))

					if cfg.TelegramBotMode == "webhook" {
						tgClient := bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil)
						botSvc := &bot.Bot{
							Link:     linkSvc,
							Billing:  billing.Service{Q: deps.Queries},
							Progress: progressSvc,
							TG:       tgClient,
							Log:      log,
						}
						wh := &bot.WebhookHandler{Bot: botSvc, Secret: cfg.TelegramWebhookSecret, Log: log}
						api.Post("/telegram/webhook", wh.ServeHTTP)
					}
					// longpoll mode registers no HTTP route — cmd/api starts
					// bot.RunLongPoll instead (design §5.1).
				}
			}
		})
	}

	return r
}

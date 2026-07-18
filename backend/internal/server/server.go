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
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/httpx"
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
				ah := &auth.Handler{Svc: svc}
				ah.Routes(api)

				acc := &account.Handler{Q: deps.Queries, Billing: billing.Service{Q: deps.Queries}}
				acc.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				learningSvc := learning.NewService(deps.Queries)
				sess := &session.Handler{Svc: session.NewService(deps.Queries, billing.Service{Q: deps.Queries}, learningSvc)}
				sess.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				lh := &learning.Handler{Svc: learningSvc}
				lh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				eh := &explanation.Handler{Svc: explanation.NewService(deps.Queries, explanation.TemplateDraftGenerator{})}
				eh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))

				ph := &progress.Handler{Svc: progress.NewService(deps.Queries)}
				ph.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))
			}
		})
	}

	return r
}

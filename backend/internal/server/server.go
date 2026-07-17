// Package server wires HTTP routes and middleware.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

type Deps struct {
	Queries *sqlc.Queries
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
		ch := &content.Handler{Q: deps.Queries, MediaBase: cfg.MediaBaseURL}
		r.Route("/api/v1", func(api chi.Router) {
			ch.Routes(api)
		})
	}

	return r
}

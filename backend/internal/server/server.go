// Package server wires HTTP routes and middleware.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

type Deps struct {
	Queries *sqlc.Queries
}

func New(cfg config.Config, deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
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

	// Content routes are mounted in Plan01/T10 when deps.Queries != nil.
	_ = deps

	return r
}

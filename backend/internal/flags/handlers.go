package flags

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes public feature-flag reads.
type Handler struct {
	Pool *pgxpool.Pool
}

// PublicRoutes mounts unauthenticated flag endpoints.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/flags", h.getPublic)
}

func (h *Handler) getPublic(w http.ResponseWriter, r *http.Request) {
	out, err := Public(r.Context(), h.Pool)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "flags query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

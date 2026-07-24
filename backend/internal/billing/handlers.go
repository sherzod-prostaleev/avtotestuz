package billing

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

// Handler serves the public billing read endpoints (tariffs today; payment
// flows land in later M2 plans).
type Handler struct {
	Svc Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/tariffs", h.listTariffs)
}

func (h *Handler) listTariffs(w http.ResponseWriter, r *http.Request) {
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	out, err := h.Svc.ListTariffs(r.Context(), loc)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "tariffs query failed")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.Data(w, http.StatusOK, out)
}

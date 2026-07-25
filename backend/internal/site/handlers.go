package site

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes public site chrome reads.
type Handler struct {
	Pool *pgxpool.Pool
}

// PublicRoutes mounts unauthenticated site endpoints.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/site/contacts", h.getContacts)
	r.Get("/site/banner", h.getBanner)
	r.Get("/site/home", h.getHome)
}

func (h *Handler) store() Store {
	return Store{Pool: h.Pool}
}

func (h *Handler) getContacts(w http.ResponseWriter, r *http.Request) {
	out, err := h.store().GetContacts(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "contacts query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getBanner(w http.ResponseWriter, r *http.Request) {
	out, err := h.store().GetSupportBanner(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "banner query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getHome(w http.ResponseWriter, r *http.Request) {
	out, err := h.store().GetHomeHero(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "home hero query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// PutContactsBody is the ops write payload (same shape as public Contacts).
type PutContactsBody = Contacts

// DecodeAndPut validates JSON and persists contacts for ops handlers.
func DecodeAndPut(w http.ResponseWriter, r *http.Request, store Store, updatedBy string) {
	var body PutContactsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := store.PutContacts(r.Context(), body, updatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "too long") {
			httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to update contacts")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

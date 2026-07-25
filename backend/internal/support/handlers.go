package support

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes public + authenticated ticket create endpoints.
type Handler struct {
	Pool *pgxpool.Pool
}

func (h *Handler) store() Store {
	return Store{Pool: h.Pool}
}

// PublicRoutes mounts unauthenticated ticket create.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/support/tickets", h.createPublic)
}

// AuthedRoutes mounts profile ticket create.
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Post("/me/support/tickets", h.createAuthed)
}

type createBody struct {
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	Subject      string `json:"subject"`
	Body         string `json:"body"`
	Locale       string `json:"locale"`
}

func (h *Handler) createPublic(w http.ResponseWriter, r *http.Request) {
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	t, err := h.store().Create(r.Context(), CreateInput{
		ContactEmail: body.ContactEmail,
		ContactPhone: body.ContactPhone,
		Subject:      body.Subject,
		Body:         body.Body,
		Locale:       body.Locale,
		Source:       "public",
	})
	if err != nil {
		writeCreateErr(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, t)
}

func (h *Handler) createAuthed(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	pid := claims.ProfileID
	t, err := h.store().Create(r.Context(), CreateInput{
		ProfileID:    &pid,
		ContactEmail: body.ContactEmail,
		ContactPhone: body.ContactPhone,
		Subject:      body.Subject,
		Body:         body.Body,
		Locale:       body.Locale,
		Source:       "profile",
	})
	if err != nil {
		writeCreateErr(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, t)
}

func writeCreateErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "required") || strings.Contains(msg, "too long") ||
		strings.Contains(msg, "invalid source") {
		httpx.Error(w, http.StatusBadRequest, "invalid_field", msg)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal", "ticket create failed")
}

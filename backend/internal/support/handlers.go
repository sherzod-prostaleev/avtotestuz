package support

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes public + authenticated ticket create endpoints.
type Handler struct {
	Pool      *pgxpool.Pool
	Lim       auth.Limiter
	ClientIPs auth.ClientIPResolver
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
	Website      string `json:"website"` // honeypot; real clients leave empty
}

func (h *Handler) createPublic(w http.ResponseWriter, r *http.Request) {
	if h.Lim.R != nil {
		ip := h.ClientIPs.Resolve(r)
		allowed, err := h.Lim.Allow(r.Context(), "support:public:ip:"+ip, 5, time.Hour)
		if err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "support is temporarily unavailable")
			return
		}
		if !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many support requests")
			return
		}
	}
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if strings.TrimSpace(body.Website) != "" {
		// Deliberately return a generic accepted response without persisting
		// obvious form-bot submissions.
		httpx.Data(w, http.StatusCreated, map[string]bool{"ok": true})
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
	if h.Lim.R != nil {
		allowed, err := h.Lim.Allow(r.Context(), "support:profile:"+claims.ProfileID.String(), 10, time.Hour)
		if err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "support is temporarily unavailable")
			return
		}
		if !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many support requests")
			return
		}
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

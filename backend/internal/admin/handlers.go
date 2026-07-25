package admin

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/httpx"
)

// Handler mounts /admin/v1 routes.
type Handler struct {
	Svc    Service
	Pool   *pgxpool.Pool
	Secret []byte
}

// Routes mounts public auth + protected admin routes under the given router
// (caller should mount at /admin/v1).
func (h *Handler) Routes(r chi.Router) {
	store := Store{Pool: h.Pool}
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)

	r.Group(func(pr chi.Router) {
		pr.Use(Required(h.Secret, store))
		pr.Get("/me", h.me)
		pr.Get("/ping", h.ping)
	})
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	ip := clientIP(r)
	pair, me, err := h.Svc.Login(r.Context(), body.Email, body.Password, r.UserAgent(), ip)
	if err != nil {
		switch err {
		case ErrInvalidCreds:
			httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		case ErrDisabled:
			httpx.Error(w, http.StatusForbidden, "disabled", "admin account disabled")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "login failed")
		}
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &me.ID, "admin.login", "admin_user", me.ID.String(), nil, map[string]any{
		"email": me.Email,
	}, ip, r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{
		"tokens": pair,
		"admin":  me,
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	pair, err := h.Svc.Refresh(r.Context(), body.RefreshToken, r.UserAgent(), clientIP(r))
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid refresh token")
		return
	}
	httpx.Data(w, http.StatusOK, pair)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = h.Svc.Logout(r.Context(), body.RefreshToken)
	httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	me, err := h.Svc.Me(r.Context(), claims.AdminUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "me failed")
		return
	}
	httpx.Data(w, http.StatusOK, me)
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	httpx.Data(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"permissions": PermsFromContext(r.Context()),
	})
}

func clientIP(r *http.Request) *net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return nil
	}
	return &ip
}

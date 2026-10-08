package bot

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes the one web-facing route in this package: an
// authenticated user minting a link token for themselves. There is
// deliberately no corresponding "redeem" HTTP route — see design §3.2.
type Handler struct {
	Link        *LinkService
	BotUsername string
}

func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Get("/me/telegram", h.getStatus)
	r.Post("/me/telegram/link-token", h.createLinkToken)
	r.Delete("/me/telegram", h.unlink)
}

type telegramStatusResponse struct {
	Linked   bool   `json:"linked"`
	Username string `json:"username,omitempty"`
	LinkedAt string `json:"linked_at,omitempty"`
	TgUserID int64  `json:"tg_user_id,omitempty"`
	// Mirrors TelegramStatus.PhoneVerified.
	PhoneVerified bool `json:"phone_verified"`
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	status, err := h.Link.Status(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load telegram status")
		return
	}
	httpx.Data(w, http.StatusOK, status)
}

type linkTokenResponse struct {
	Token     string `json:"token"`
	DeepLink  string `json:"deep_link"`
	ExpiresAt string `json:"expires_at"`
}

func (h *Handler) createLinkToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	if strings.TrimSpace(h.BotUsername) == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "telegram_bot_unconfigured",
			"telegram bot username is not configured")
		return
	}

	tok, err := h.Link.GenerateLinkToken(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to generate link token")
		return
	}

	httpx.Data(w, http.StatusOK, linkTokenResponse{
		Token:     tok.Token,
		DeepLink:  deepLink(h.BotUsername, tok.Token),
		ExpiresAt: tok.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

type unlinkResponse struct {
	Unlinked bool `json:"unlinked"`
}

// unlink is the website's way to take Telegram off the learner's own profile
// (the bot has /unlink; before this a link planted by someone with brief
// session access could only be removed from that Telegram account). Always
// the caller's own profile, never an id from the request.
func (h *Handler) unlink(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	removed, err := h.Link.UnlinkProfile(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to unlink telegram")
		return
	}
	httpx.Data(w, http.StatusOK, unlinkResponse{Unlinked: removed})
}

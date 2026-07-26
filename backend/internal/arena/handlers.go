package arena

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes Arena HTTP endpoints.
type Handler struct {
	Svc       *Service
	PublicURL string
}

func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Post("/arena/ws-ticket", h.mintTicket)
	r.Get("/me/arena/matches", h.listMatches)
	r.Get("/me/arena/rating", h.getRating)
}

func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/arena/ws", h.serveWS)
}

func (h *Handler) mintTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	tok, exp, err := h.Svc.MintTicket(r.Context(), claims.ProfileID)
	if err != nil {
		if errors.Is(err, ErrFeatureDisabled) {
			httpx.Error(w, http.StatusForbidden, "feature_disabled", "arena is temporarily disabled")
			return
		}
		if err.Error() == "rate_limited" {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many ticket requests")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket mint failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"ticket":         tok,
		"expires_in_sec": exp,
		"ws_url":         h.wsURL(r),
	})
}

func (h *Handler) wsURL(r *http.Request) string {
	// Ticket mint normally arrives via the Next.js BFF, so r.Host is the
	// compose-internal API hostname (e.g. "api:8080") which browsers cannot
	// reach. Prefer the public site origin (same host nginx uses for /api/v1).
	if host, scheme, ok := publicWSBase(h.PublicURL); ok && internalAPIHost(r.Host) {
		return scheme + "://" + host + "/api/v1/arena/ws"
	}
	scheme := "ws"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "wss"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + "/api/v1/arena/ws"
}

// publicWSBase maps PUBLIC_BASE_URL → (host, ws|wss).
func publicWSBase(publicURL string) (host, scheme string, ok bool) {
	if publicURL == "" {
		return "", "", false
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	scheme = "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return u.Host, scheme, true
}

// internalAPIHost is true for Docker/compose service hosts that must never be
// returned to the browser as a WebSocket URL.
func internalAPIHost(host string) bool {
	name := strings.ToLower(host)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	switch name {
	case "", "api", "backend", "avtotest-api", "drivergo-api":
		return true
	}
	// Bare compose DNS labels have no dot (unlike localhost / public FQDNs).
	return name != "localhost" && name != "127.0.0.1" && !strings.Contains(name, ".")
}

func (h *Handler) serveWS(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		httpx.Error(w, http.StatusUnauthorized, "bad_ticket", "missing ticket")
		return
	}
	profileID, err := h.Svc.RedeemTicket(r.Context(), ticket)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "bad_ticket", "invalid or expired ticket")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: originPatterns(h.PublicURL),
	})
	if err != nil {
		return
	}
	c := NewConn(profileID, conn, h.Svc, func() {
		h.Svc.OnDisconnect(profileID)
	})
	h.Svc.Hub.Register(profileID, c)
	_ = h.Svc.R.Set(r.Context(), "arena:conn:"+profileID.String(), h.Svc.Instance, 30*time.Second).Err()
	hello, _ := Encode("hello", HelloData{
		ProfileID:    profileID,
		ServerTimeMs: time.Now().UTC().UnixMilli(),
		Protocol:     ProtocolVersion,
	})
	_ = c.Enqueue(hello)
	c.Run(r.Context())
	h.Svc.Hub.Unregister(profileID, c)
}

func (h *Handler) listMatches(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	rows, err := h.Svc.ListHistory(r.Context(), claims.ProfileID, 20)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "history failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		ratingAfter := 1000
		if row.RatingAfter.Valid {
			ratingAfter = int(row.RatingAfter.Int32)
		}
		item := map[string]any{
			"match_id":      row.ID,
			"status":        row.Status,
			"end_reason":    textPtr(row.EndReason),
			"created_at":    row.CreatedAt,
			"finished_at":   tsPtr(row.FinishedAt),
			"slot":          row.Slot,
			"score":         row.Score,
			"correct_count": row.CorrectCount,
			"outcome":       textPtr(row.Outcome),
			"rating_before": intPtr(row.RatingBefore),
			"rating_after":  intPtr(row.RatingAfter),
			"rating_delta":  intPtr(row.RatingDelta),
			"medal":         MedalForRating(ratingAfter),
		}
		out = append(out, item)
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getRating(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	rating, err := h.Svc.Rating.Rating(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "rating failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"rating": rating,
		"medal":  MedalForRating(rating),
	})
}

func originPatterns(publicURL string) []string {
	if publicURL == "" {
		return []string{"localhost:*", "127.0.0.1:*"}
	}
	u, err := url.Parse(publicURL)
	if err != nil {
		return []string{"*"}
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return []string{"localhost:*", "127.0.0.1:*"}
	}
	out := []string{host}
	if strings.HasPrefix(host, "www.") {
		out = append(out, strings.TrimPrefix(host, "www."))
	} else {
		out = append(out, "www."+host)
	}
	return out
}

func textPtr(t pgtype.Text) any {
	if !t.Valid {
		return nil
	}
	return t.String
}

func tsPtr(t pgtype.Timestamptz) any {
	if !t.Valid {
		return nil
	}
	return t.Time
}

func intPtr(t pgtype.Int4) any {
	if !t.Valid {
		return nil
	}
	return t.Int32
}

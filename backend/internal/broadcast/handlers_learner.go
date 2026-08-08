package broadcast

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

// LearnerHandler exposes /me/notifications* routes.
type LearnerHandler struct {
	Q *sqlc.Queries
}

func (h *LearnerHandler) AuthedRoutes(r chi.Router) {
	r.Get("/me/notifications/unread-count", h.unreadCount)
	r.Get("/me/notifications", h.list)
	r.Post("/me/notifications/read-all", h.readAll)
	r.Post("/me/notifications/{id}/read", h.readOne)
}

type notificationDTO struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	ImageURL   string  `json:"image_url,omitempty"`
	URL        string  `json:"url,omitempty"`
	ReadAt     *string `json:"read_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
	CampaignID string  `json:"campaign_id,omitempty"`
}

func (h *LearnerHandler) unreadCount(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	n, err := h.Q.CountUnreadInappNotifications(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unread count failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"unread": n})
}

func (h *LearnerHandler) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	var before pgtype.Timestamptz
	if raw := r.URL.Query().Get("before"); raw != "" {
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			t, err = time.Parse(time.RFC3339, raw)
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_before", "before must be RFC3339")
			return
		}
		before = pgtype.Timestamptz{Time: t.UTC(), Valid: true}
	}
	rows, err := h.Q.ListInappNotifications(r.Context(), sqlc.ListInappNotificationsParams{
		ProfileID: claims.ProfileID,
		Before:    before,
		Limit:     int32(limit),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list failed")
		return
	}
	items := make([]notificationDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDTO(row))
	}
	httpx.Data(w, http.StatusOK, map[string]any{"items": items})
}

func (h *LearnerHandler) readOne(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid notification id")
		return
	}
	row, err := h.Q.MarkInappNotificationRead(r.Context(), sqlc.MarkInappNotificationReadParams{
		ID:        id,
		ProfileID: claims.ProfileID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "notification not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "mark read failed")
		return
	}
	httpx.Data(w, http.StatusOK, toDTO(row))
}

func (h *LearnerHandler) readAll(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	n, err := h.Q.MarkAllInappNotificationsRead(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "mark all failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"marked": n})
}

func toDTO(row sqlc.Notification) notificationDTO {
	var payload struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		ImageURL string `json:"image_url"`
		URL      string `json:"url"`
	}
	_ = json.Unmarshal(row.Payload, &payload)
	if payload.Title == "" {
		payload.Title = "Driver Go"
	}
	out := notificationDTO{
		ID:        row.ID.String(),
		Title:     payload.Title,
		Body:      payload.Body,
		ImageURL:  payload.ImageURL,
		URL:       payload.URL,
		CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
	if row.ReadAt.Valid {
		s := row.ReadAt.Time.UTC().Format(time.RFC3339Nano)
		out.ReadAt = &s
	}
	if row.CampaignID.Valid {
		out.CampaignID = row.CampaignID.UUID.String()
	}
	return out
}

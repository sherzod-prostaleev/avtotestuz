package ops

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/httpx"
)

// AuditRow is an append-only audit_log preview for ops (M3 precursor).
type AuditRow struct {
	ID        uuid.UUID  `json:"id"`
	ActorID   *uuid.UUID `json:"actor_id,omitempty"`
	Action    string     `json:"action"`
	Entity    string     `json:"entity"`
	EntityID  *uuid.UUID `json:"entity_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ListAudit returns recent audit_log rows newest-first.
func (h *Handler) ListAudit(ctx context.Context, limit int) ([]AuditRow, error) {
	if h.Pool == nil {
		return nil, fmt.Errorf("ops audit requires pool")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := h.Pool.Query(ctx, `
		SELECT id, actor_id, action, entity, entity_id, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var (
			r       AuditRow
			actorID *uuid.UUID
			entityID *uuid.UUID
		)
		if err := rows.Scan(&r.ID, &actorID, &r.Action, &r.Entity, &entityID, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.ActorID = actorID
		r.EntityID = entityID
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.ListAudit(r.Context(), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "audit query failed")
		return
	}
	if out == nil {
		out = []AuditRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}

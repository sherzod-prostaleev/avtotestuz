package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

// OpsFeedItem is one row in the monitoring ops feed (audit + payment fails).
type OpsFeedItem struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// OpsFeedResult is a merged newest-first feed.
type OpsFeedResult struct {
	Items []OpsFeedItem `json:"items"`
	Note  string        `json:"note"`
}

// AlertEval is one alert_rule row with live evaluation.
type AlertEval struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	Status      string `json:"status"` // ok | warn | fail | skipped
	Detail      string `json:"detail,omitempty"`
}

func (h *Handler) getMonitoringFeed(w http.ResponseWriter, r *http.Request) {
	out, err := h.buildOpsFeed(r.Context(), 40)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "ops feed query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getMonitoringAlerts(w http.ResponseWriter, r *http.Request) {
	out, err := h.evaluateAlertRules(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "alerts query failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"items":      out,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"note":       "Static alert_rule rows evaluated live — no pager/Sentry/webhook",
	})
}

func (h *Handler) buildOpsFeed(ctx context.Context, limit int) (OpsFeedResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	items := make([]OpsFeedItem, 0, limit)

	if h.Pool != nil {
		rows, err := h.Pool.Query(ctx, `
			SELECT a.id::text, a.action, a.entity_type, COALESCE(a.entity_id, ''), a.created_at
			FROM admin_audit_log a
			ORDER BY a.created_at DESC
			LIMIT $1`, limit)
		if err != nil {
			return OpsFeedResult{}, fmt.Errorf("audit feed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, action, entityType, entityID string
			var created time.Time
			if err := rows.Scan(&id, &action, &entityType, &entityID, &created); err != nil {
				return OpsFeedResult{}, err
			}
			detail := entityType
			if entityID != "" {
				detail = entityType + ":" + entityID
			}
			items = append(items, OpsFeedItem{
				Kind:      "admin_audit",
				ID:        id,
				Title:     action,
				Detail:    detail,
				CreatedAt: created.UTC(),
			})
		}
		if err := rows.Err(); err != nil {
			return OpsFeedResult{}, err
		}

		payRows, err := h.Pool.Query(ctx, `
			SELECT id::text, provider, COALESCE(provider_txn_id, ''), created_at
			FROM payment
			WHERE status = 'failed'
			ORDER BY created_at DESC
			LIMIT $1`, 20)
		if err != nil {
			return OpsFeedResult{}, fmt.Errorf("payment fail feed: %w", err)
		}
		defer payRows.Close()
		for payRows.Next() {
			var id, provider, txn string
			var created time.Time
			if err := payRows.Scan(&id, &provider, &txn, &created); err != nil {
				return OpsFeedResult{}, err
			}
			detail := provider
			if txn != "" {
				detail = provider + " · " + txn
			}
			items = append(items, OpsFeedItem{
				Kind:      "payment_failed",
				ID:        id,
				Title:     "payment.failed",
				Detail:    detail,
				CreatedAt: created.UTC(),
			})
		}
		if err := payRows.Err(); err != nil {
			return OpsFeedResult{}, err
		}
	}

	// Newest first across kinds (simple insertion sort / bubble for small N).
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt.After(items[i].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return OpsFeedResult{
		Items: items,
		Note:  "Merged admin_audit_log + payment status=failed — not a zap log tail",
	}, nil
}

func (h *Handler) evaluateAlertRules(ctx context.Context) ([]AlertEval, error) {
	if h.Pool == nil {
		return []AlertEval{}, nil
	}
	rows, err := h.Pool.Query(ctx, `
		SELECT id, name, kind, enabled, description
		FROM alert_rule
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list alert_rule: %w", err)
	}
	defer rows.Close()

	out := make([]AlertEval, 0)
	for rows.Next() {
		var a AlertEval
		if err := rows.Scan(&a.ID, &a.Name, &a.Kind, &a.Enabled, &a.Description); err != nil {
			return nil, err
		}
		if !a.Enabled {
			a.Status = "skipped"
			a.Detail = "disabled"
			out = append(out, a)
			continue
		}
		switch a.Kind {
		case "postgres_ready":
			pingCtx, cancel := context.WithTimeout(ctx, monitoringProbeTimeout)
			err := h.Pool.Ping(pingCtx)
			cancel()
			if err != nil {
				a.Status = "fail"
				a.Detail = err.Error()
			} else {
				a.Status = "ok"
				a.Detail = "postgres ping ok"
			}
		case "payment_fails_24h":
			var n int
			err := h.Pool.QueryRow(ctx, `
				SELECT COUNT(*)::int FROM payment
				WHERE status = 'failed' AND created_at > now() - interval '24 hours'`).Scan(&n)
			if err != nil {
				a.Status = "fail"
				a.Detail = err.Error()
			} else if n > 0 {
				a.Status = "warn"
				a.Detail = fmt.Sprintf("%d failed payment(s) in 24h", n)
			} else {
				a.Status = "ok"
				a.Detail = "0 failed payments in 24h"
			}
		default:
			a.Status = "skipped"
			a.Detail = "unknown kind"
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
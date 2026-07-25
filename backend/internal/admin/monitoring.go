package admin

import (
	"context"
	"net/http"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

const monitoringProbeTimeout = 2 * time.Second

// MetricsSnapshot returns process-local request counters (U-41). Optional.
type MetricsSnapshot func() map[string]any

func (h *Handler) getMonitoringHealth(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	ready := true

	ctx, cancel := context.WithTimeout(r.Context(), monitoringProbeTimeout)
	defer cancel()

	if h.Pool == nil {
		checks["postgres"] = "skipped"
	} else if err := h.Pool.Ping(ctx); err != nil {
		checks["postgres"] = "fail"
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if h.Redis == nil {
		checks["redis"] = "skipped"
	} else if err := h.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "fail"
		ready = false
	} else {
		checks["redis"] = "ok"
	}

	status := "ok"
	code := http.StatusOK
	if !ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}
	httpx.Data(w, code, map[string]any{
		"status":     status,
		"live":       "ok",
		"checks":     checks,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) getMonitoringMetrics(w http.ResponseWriter, r *http.Request) {
	if h.MetricsSnapshot == nil {
		httpx.Data(w, http.StatusOK, map[string]any{
			"uptime_seconds":           0,
			"requests_total":           0,
			"requests_by_status_class": map[string]uint64{},
			"note":                     "metrics collector not wired in this process",
		})
		return
	}
	httpx.Data(w, http.StatusOK, h.MetricsSnapshot())
}

// JobCatalogRow is an honest inventory of known background/CLI jobs.
// None are in-process pauseable workers yet — status is always "manual".
type JobCatalogRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Invoke      string `json:"invoke"`
}

func monitoringJobCatalog() []JobCatalogRow {
	return []JobCatalogRow{
		{
			ID:          "payrecon",
			Name:        "Payment reconciliation",
			Kind:        "cli",
			Status:      "manual",
			Description: "Dry-run payment↔provider consistency (U-27). No live merchant API without keys.",
			Invoke:      "go run ./cmd/payrecon [-hours N]",
		},
		{
			ID:          "pushdigest",
			Name:        "FSRS due push digest",
			Kind:        "cli",
			Status:      "manual",
			Description: "Web push digest for due cards (U-11). Needs VAPID on host for -send.",
			Invoke:      "go run ./cmd/pushdigest [-send] [-limit N]",
		},
		{
			ID:          "rebuildleaderboard",
			Name:        "Leaderboard rebuild",
			Kind:        "cli",
			Status:      "manual",
			Description: "Rebuild VIP/cap approximation (U-22). Not historical fidelity.",
			Invoke:      "go run ./cmd/rebuildleaderboard",
		},
		{
			ID:          "seedadmin",
			Name:        "Seed superadmin",
			Kind:        "cli",
			Status:      "manual",
			Description: "Create/update local admin from ADMIN_SEED_* env (M3-0).",
			Invoke:      "make seed-admin",
		},
	}
}

func (h *Handler) listMonitoringJobs(w http.ResponseWriter, _ *http.Request) {
	httpx.Data(w, http.StatusOK, map[string]any{
		"items": monitoringJobCatalog(),
		"note":  "CLI/manual only — no in-process worker control or fake running state",
	})
}

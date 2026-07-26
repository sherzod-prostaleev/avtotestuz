package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const monitoringStreamInterval = 5 * time.Second

func (h *Handler) streamMonitoring(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()
	ticker := time.NewTicker(monitoringStreamInterval)
	defer ticker.Stop()

	writeHealthSSE(w, flusher, h, ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			writeHealthSSE(w, flusher, h, ctx)
		}
	}
}

func writeHealthSSE(w http.ResponseWriter, flusher http.Flusher, h *Handler, ctx context.Context) {
	snap, _ := buildMonitoringHealthSnapshot(h, ctx)
	payload, err := json.Marshal(snap)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "event: health\ndata: %s\n\n", payload); err != nil {
		return
	}
	flusher.Flush()
}

func buildMonitoringHealthSnapshot(h *Handler, ctx context.Context) (map[string]any, int) {
	checks := map[string]string{}
	ready := true

	probeCtx, cancel := context.WithTimeout(ctx, monitoringProbeTimeout)
	defer cancel()

	if h.Pool == nil {
		checks["postgres"] = "skipped"
	} else if err := h.Pool.Ping(probeCtx); err != nil {
		checks["postgres"] = "fail"
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if h.Redis == nil {
		checks["redis"] = "skipped"
	} else if err := h.Redis.Ping(probeCtx).Err(); err != nil {
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
	return map[string]any{
		"status":     status,
		"live":       "ok",
		"checks":     checks,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"alerts":     alertSummaries(h, ctx),
	}, code
}

func hostMetricsSnapshot() map[string]any {
	return map[string]any{
		"available": false,
		"status":    "unavailable",
		"cpu_pct":   nil,
		"ram_pct":   nil,
		"disk_pct":  nil,
		"note":      "no host metrics agent wired — CPU/RAM/disk not reported",
	}
}

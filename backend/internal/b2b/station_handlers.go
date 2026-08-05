package b2b

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// PublicRoutes are the three unauthenticated station endpoints. They are
// public by necessity — a classroom PC has no session until it has proved it
// holds a station key — so each one is rate limited by client IP.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/b2b/stations/enroll", h.enrollStation)
	r.Post("/b2b/stations/challenge", h.stationChallenge)
	r.Post("/b2b/stations/token", h.stationToken)
}

func (h *Handler) stationAuth() StationAuth {
	return StationAuth{Pool: h.Pool, Redis: h.Redis, Secret: h.Secret}
}

// clientIP returns the bare host from r.RemoteAddr, never host:port: the
// result is written to b2b_station.last_ip through a ::inet cast, and a
// malformed value there fails that cast, which silently drops the rest of
// the telemetry UPDATE (last_seen_at, agent_version) along with it. An
// unparseable RemoteAddr yields an empty string, not a mangled one.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

// allow applies a fixed-window IP limit; a missing limiter (tests) allows.
func (h *Handler) allow(r *http.Request, action string, limit int, window time.Duration) bool {
	if h.Lim.R == nil {
		return true
	}
	ok, err := h.Lim.Allow(r.Context(), "station:"+action+":"+clientIP(r), limit, window)
	return err == nil && ok
}

type enrollStationBody struct {
	Code         string `json:"code"`
	PublicKey    string `json:"public_key"`
	HWIDHash     string `json:"hwid_hash"`
	Label        string `json:"label"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) enrollStation(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "enroll", 60, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many enrollment attempts")
		return
	}
	var body enrollStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(body.PublicKey)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "public_key must be base64")
		return
	}
	out, err := h.store().EnrollStation(r.Context(), EnrollInput{
		Code:         body.Code,
		PublicKey:    pub,
		HWIDHash:     body.HWIDHash,
		Label:        body.Label,
		AgentVersion: body.AgentVersion,
	})
	if err != nil {
		writeStoreErr(w, err, "enrollment failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationChallengeBody struct {
	StationID string `json:"station_id"`
}

func (h *Handler) stationChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "challenge", 600, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many challenge requests")
		return
	}
	var body stationChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	out, err := h.stationAuth().Challenge(r.Context(), stationID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "challenge failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationTokenBody struct {
	StationID    string `json:"station_id"`
	Nonce        string `json:"nonce"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
	HWIDHash     string `json:"hwid_hash"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) stationToken(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "token", 600, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many token requests")
		return
	}
	var body stationTokenBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	sig, err := base64.StdEncoding.DecodeString(body.Sig)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "sig must be base64")
		return
	}
	out, err := h.stationAuth().Token(r.Context(), TokenInput{
		StationID:    stationID,
		Nonce:        body.Nonce,
		TS:           body.TS,
		Sig:          sig,
		HWIDHash:     body.HWIDHash,
		AgentVersion: body.AgentVersion,
		IP:           clientIP(r),
	})
	if err != nil {
		if errors.Is(err, ErrStationAuth) {
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "token failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type enrollWindowBody struct {
	TTLMinutes int `json:"ttl_minutes"`
}

func (h *Handler) openEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	var body enrollWindowBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	ttl := time.Duration(body.TTLMinutes) * time.Minute
	out, err := h.store().OpenEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, ttl)
	if err != nil {
		writeStoreErr(w, err, "open enroll window failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	out, err := h.store().ActiveEnrollCodeAsTeacher(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		writeStoreErr(w, err, "enroll window query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) closeEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	codeID, err := uuid.Parse(chi.URLParam(r, "codeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid code id")
		return
	}
	if err := h.store().CloseEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, codeID); err != nil {
		writeStoreErr(w, err, "close enroll window failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

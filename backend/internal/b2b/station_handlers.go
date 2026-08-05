package b2b

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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
// holds a station key — so each one is rate limited two ways: by caller
// identity (station id, or enrollment code for the one endpoint that has no
// station id yet) and, secondarily, by client IP.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/b2b/stations/enroll", h.enrollStation)
	r.Post("/b2b/stations/challenge", h.stationChallenge)
	r.Post("/b2b/stations/token", h.stationToken)
}

func (h *Handler) stationAuth() StationAuth {
	return StationAuth{Pool: h.Pool, Redis: h.Redis, Secret: h.Secret}
}

// Per-action limits. The identity dimension is the real abuse control: nginx
// proxies to this service over loopback and middleware.RealIP is
// deliberately absent, so every request's TCP peer is the proxy itself, not
// the caller. h.ClientIPs can recover a real client IP from a signed
// trusted-proxy header when nginx is configured to send one, but until then
// the IP dimension collapses to one shared bucket for the whole platform —
// identity is what actually bounds a single bad actor.
//
// A live station needs about 4 challenge+token pairs per hour (its access
// token has a 15-minute TTL); the identity limits below give an order of
// magnitude of headroom over that for clock drift, retries and reconnects
// without letting one leaked station id or enrollment code hammer the
// service.
const (
	enrollIdentityLimit = 20
	enrollIPLimit       = 60

	challengeIdentityLimit = 40
	challengeIPLimit       = 600

	tokenIdentityLimit = 40
	tokenIPLimit       = 600
)

// clientIP resolves the caller's address through h.ClientIPs — a signed
// trusted-proxy assertion when nginx is configured to send one, otherwise
// the TCP peer address — and returns bare host only, never host:port: the
// result is written to b2b_station.last_ip through a ::inet cast, and a
// malformed value there fails that cast, which silently drops the rest of
// the telemetry UPDATE (last_seen_at, agent_version) along with it. Any
// value that is not a valid IP (including an unparseable RemoteAddr) yields
// an empty string, not a mangled one.
func (h *Handler) clientIP(r *http.Request) string {
	ip := h.ClientIPs.Resolve(r)
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

// allow applies a two-dimension fixed-window limit: identity first, IP
// second. Either dimension is skipped when its key would be empty (no
// identity yet, or an unresolvable IP) rather than letting every such
// request share one bucket keyed on an empty string. A missing limiter
// (tests) allows. Fails closed: a limiter error denies the request.
func (h *Handler) allow(r *http.Request, action, identity string, identityLimit, ipLimit int, window time.Duration) bool {
	if h.Lim.R == nil {
		return true
	}
	if identity != "" {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":id:"+identity, identityLimit, window)
		if err != nil || !ok {
			return false
		}
	}
	if ip := h.clientIP(r); ip != "" {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":ip:"+ip, ipLimit, window)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

type enrollStationBody struct {
	Code         string `json:"code"`
	PublicKey    string `json:"public_key"`
	HWIDHash     string `json:"hwid_hash"`
	Label        string `json:"label"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) enrollStation(w http.ResponseWriter, r *http.Request) {
	var body enrollStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	// Decoding the body is pure CPU work, no DB or Redis touched yet, so the
	// limiter still runs before any I/O. The enrollment code is the identity
	// dimension here — there is no station id until enrollment succeeds.
	if !h.allow(r, "enroll", body.Code, enrollIdentityLimit, enrollIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many enrollment attempts")
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
	// Decoding the body and parsing the UUID are both pure CPU work, no DB
	// or Redis touched yet, so the limiter still runs before any I/O. The
	// station id is the identity dimension.
	if !h.allow(r, "challenge", stationID.String(), challengeIdentityLimit, challengeIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many challenge requests")
		return
	}
	out, err := h.stationAuth().Challenge(r.Context(), stationID)
	if err != nil {
		if errors.Is(err, ErrStationAuth) {
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
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
	// Decoding the body and parsing the UUID are both pure CPU work, no DB
	// or Redis touched yet, so the limiter still runs before any I/O. The
	// station id is the identity dimension.
	if !h.allow(r, "token", stationID.String(), tokenIdentityLimit, tokenIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many token requests")
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
		IP:           h.clientIP(r),
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		// An empty body keeps meaning "use the default": Decode returns EOF
		// when there is nothing to read. Any other error (malformed JSON, a
		// string ttl_minutes) must not silently fall back to the 2-hour
		// default with no signal to the teacher who asked for something else.
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
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

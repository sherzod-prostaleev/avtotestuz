package b2b

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

// Per-action limits. identity is the real abuse control: nginx proxies to
// this service over loopback and does not yet send the signed trusted-proxy
// assertion h.ClientIPs needs to recover the real caller, so until that is
// configured `allow` skips the IP dimension entirely rather than collapsing
// every caller on the platform into one shared bucket (see allow's doc
// comment for why). It activates automatically, with no code change, the
// moment nginx starts sending the assertion. Each constant below carries its
// own sizing rationale; the one thing they don't repeat per-constant is what
// the identity dimension *is* for each action, so it's worth stating once
// here: enroll's identity is the enrollment code (shared by every PC in a
// school), while challenge's and token's identity is the station id (unique
// per PC) — that difference is why raising one for fleet size does not
// automatically mean raising the others the same way.
//
// challenge issues a bearer-less nonce that proves nothing about the
// caller — every syntactically valid station id "succeeds" whether or not
// that station exists — so there is no failed/successful split to key on;
// its identity ceiling only has to bound raw resource use, not act as a
// per-station lockout.
//
// token verifies a real signature, so unlike challenge a wrong guess is
// distinguishable from the station's own traffic. tokenIdentityLimit is a
// coarse pre-work volume cap only — sized the same way as challenge's, and
// it still counts every attempt including a station's own successful ones,
// so it must never be the sole gate or the same lockout challenge had would
// just move here. tokenFailedIdentityLimit is the control that actually
// matters: it only grows on a failed verification, so a legitimate
// station's own successful traffic can never push it over the edge, while
// an attacker who cannot produce a valid signature for a station id it does
// not own (a station id is not a credential — it sits in agent config and
// teacher-facing station lists) is bounded to a small number of guesses per
// hour.
const (
	// enrollIdentityLimit bounds attempts per enrollment code, not per PC:
	// every classroom PC in a school posts to /enroll with the same code, so
	// this bucket has to cover the whole rollout, not one machine. It used
	// to be sized at 20 back when a code was a disposable 2-hour window a
	// teacher reopened at will -- exhausting it just meant asking for a new
	// one. OpenInstallerKey changed that: the code is now idempotent and
	// licence-lived, so the bucket can no longer be reset by fetching again,
	// and rotating it to reset the bucket kills the copies already handed
	// out to the rest of the classroom. A school with 100 PCs (the rollout
	// size the enroll-code design itself was sized against -- see
	// enroll_code.go's EnrollCodeRow doc comment) each retrying up to
	// enrollSchedule's 4 attempts during a cold-boot install is 400 requests
	// against this one bucket in the worst case. 500 comfortably clears
	// that with room to spare, and it costs nothing in security: the tight
	// bound on *successful* enrolments is max_uses, sized to free seats and
	// enforced under the org row lock in EnrollStation, not this request
	// counter. A request-rate cap only needs to bound abuse volume, and 500
	// failed/successful attempts an hour against a single code is nowhere
	// near a meaningful DoS budget.
	enrollIdentityLimit = 500
	// enrollIPLimit is enroll's secondary dimension, sharing one NAT'd IP
	// with the whole rollout in the common case (a school's classroom PCs
	// sit behind one gateway). Since it sees roughly the same traffic as
	// enrollIdentityLimit's bucket for a single-school rollout, it is kept
	// at the same 1:3 identity:IP headroom ratio already used by
	// challenge/token below (20:60 before this change), scaled up with
	// enrollIdentityLimit so a big rollout does not trip the IP bucket the
	// moment the identity bucket was widened to let it through.
	enrollIPLimit = 1500

	// challengeIdentityLimit is keyed per station id, not per school, so
	// raising enrollIdentityLimit for fleet size does not change its math: a
	// live station needs roughly 5 challenge+token pairs an hour (see below),
	// and 200 is still ~40x that regardless of how many other stations exist.
	challengeIdentityLimit = 200
	// challengeIPLimit, unlike challengeIdentityLimit, is shared across every
	// station behind one NAT'd IP, so it does scale with fleet size. At ~5
	// challenge calls/hour/station (TTL 15 minutes, renewed with margin --
	// see keepTokenWarm in the station agent), a 100-PC school -- the size
	// the enroll-code design itself targets -- puts ~500/hour on this
	// bucket in steady state alone, before counting retries or a rollout's
	// extra initial-auth traffic. The old 600 left only ~100 of headroom
	// across the whole fleet for that. Raised to 1500 so one bad network
	// spell across many machines at once does not 429 an entire school.
	challengeIPLimit = 1500

	// tokenIdentityLimit, like challengeIdentityLimit, is keyed per station
	// id and unaffected by fleet size: ~5 needed/hour, 200 gives ~40x that.
	tokenIdentityLimit = 200
	// tokenFailedIdentityLimit gates how many failed signature checks a
	// single station id can accumulate in an hour before token issuance
	// locks out for that id (see recordTokenFailure). It was 20 in the
	// previous round, sized to also leave headroom for a station's own
	// legitimate traffic sharing the bucket. That headroom concern is gone:
	// a station's own successful renewals never touch this bucket, and a
	// successful issue now clears it outright (see the reset in
	// stationToken), so nothing legitimate accumulates here anymore -- only
	// sustained failure does. What is left to size for is purely how
	// expensive it is for someone who merely knows a station's id (not a
	// secret -- it sits in agent config and teacher-facing station lists)
	// to lock that station out of obtaining tokens for the rest of the
	// window. Raised to 100: still small enough to fail closed quickly
	// against a sustained attacker (an attacker without the station's
	// private key can never produce a passing signature no matter how many
	// tries it gets, so a higher ceiling does not buy it anything but
	// wasted requests), while making a deliberate lockout cost 5x what it
	// did before.
	tokenFailedIdentityLimit = 100
	// tokenIPLimit shares challengeIPLimit's fleet-size arithmetic (~5
	// token calls/hour/station, ~500/hour steady-state for a 100-PC school
	// behind one NAT'd IP) and is raised the same way and for the same
	// reason.
	tokenIPLimit = 1500
)

// hashIdentity turns a caller-supplied rate-limit identity (an enrollment
// code or a station id) into a fixed-length, non-reversible key segment. The
// enrollment code in particular is a short-lived shared secret; without this
// it would appear verbatim in Redis MONITOR output and the slowlog for the
// life of the enrollment window. Applied uniformly to every identity
// dimension so the keying stays consistent regardless of what the identity
// actually is.
func hashIdentity(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:16])
}

// clientIP returns the caller's address only when h.ClientIPs verified a
// signed trusted-proxy assertion for this request -- the same asserted
// check allow uses for its own IP dimension (see allow's doc comment for
// why an unasserted value cannot be trusted as the caller's IP). An
// unasserted result is not "no IP available": h.ClientIPs.Resolve would
// happily return one, but in this deployment that is nginx's own loopback
// peer address for every request until nginx is configured to send the
// assertion, so writing it as-is would populate b2b_station.last_ip with
// the same wrong value for every station on every login. The result is
// written through a ::inet cast via NULLIF($3,'')::inet, and a malformed
// value there fails that cast, which silently drops the rest of the
// telemetry UPDATE (last_seen_at, agent_version) along with it, so an
// unasserted or otherwise-unparseable IP yields an empty string -- turned
// into a NULL an operator can interpret, not a loopback address that looks
// like real data.
func (h *Handler) clientIP(r *http.Request) string {
	ip, asserted := h.ClientIPs.ResolveAsserted(r)
	if !asserted || net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

// allow applies a two-dimension fixed-window limit: identity first, IP
// second. identity is skipped when it is empty (no code/station id yet).
//
// The IP dimension is skipped unless h.ClientIPs verified a signed
// trusted-proxy assertion for this request (ResolveAsserted's asserted
// return). An *unasserted* IP is not "no IP" — h.clientIP(r) will happily
// return the TCP peer address — but in production that peer is nginx's own
// loopback address for every request until nginx is configured to send the
// assertion, so keying a limit on it would share one bucket across every
// caller on the platform: an attacker rotating identities (a fresh
// station_id per request) never trips their own identity bucket while
// draining the shared IP bucket dry, locking out every real station until
// the window rolls. Gating on asserted rather than on "IP is non-empty" is
// what closes that: the dimension turns on by itself, correctly, the moment
// nginx starts sending the header, with no code change here.
//
// If neither dimension applied — identity empty and IP unasserted — the
// request would otherwise leave with no limit at all, breaking allow's own
// fail-closed contract (reachable today via e.g. an enroll call with
// `{"code":""}`). That case falls back to one coarse action-wide bucket,
// sized the same as the dormant IP bucket: it can't attribute the request
// to a caller, but it still bounds the total rate of unkeyable requests.
//
// A missing limiter (tests) allows. Fails closed: a limiter error denies.
func (h *Handler) allow(r *http.Request, action, identity string, identityLimit, ipLimit int, window time.Duration) bool {
	if h.Lim.R == nil {
		return true
	}
	applied := false
	if identity != "" {
		key := "station:" + action + ":id:" + hashIdentity(identity)
		ok, err := h.Lim.Allow(r.Context(), key, identityLimit, window)
		if err != nil || !ok {
			return false
		}
		applied = true
	}
	if ip, asserted := h.ClientIPs.ResolveAsserted(r); asserted {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":ip:"+ip, ipLimit, window)
		if err != nil || !ok {
			return false
		}
		applied = true
	}
	if !applied {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":unkeyed", ipLimit, window)
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
	// Canonicalize it the same way Store.EnrollStation does before it becomes
	// a rate-limit key: otherwise "AVTO-ABCD-EFGH", "avto-abcd-efgh" and
	// " AVTO-ABCD-EFGH " are three independent buckets for what is really one
	// leaked code, and the limit is evaded by changing case or whitespace.
	identity := strings.ToUpper(strings.TrimSpace(body.Code))
	if !h.allow(r, "enroll", identity, enrollIdentityLimit, enrollIPLimit, time.Hour) {
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
	// or Redis touched yet, so the pre-work limiter still runs before any
	// I/O. This is a coarse volume cap only -- it counts every attempt,
	// including the station's own successful ones -- so it must not be the
	// only gate; see the constants' doc comment.
	if !h.allow(r, "token", stationID.String(), tokenIdentityLimit, tokenIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many token requests")
		return
	}
	// The failed-attempt bucket is what actually protects one station from
	// being locked out by someone who merely knows its id: it only grows
	// when a call turns out to be bogus, so a legitimate station's own
	// successful renewals can never fill it. Checked before doing any
	// verification work so an already-bounded attacker doesn't get a free
	// signature check per request.
	failKey := "station:token:fail:" + hashIdentity(stationID.String())
	if h.Lim.R != nil {
		if n, err := h.Lim.Count(r.Context(), failKey); err != nil || n >= tokenFailedIdentityLimit {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many failed token attempts")
			return
		}
	}
	sig, err := base64.StdEncoding.DecodeString(body.Sig)
	if err != nil {
		h.recordTokenFailure(r, failKey)
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
			h.recordTokenFailure(r, failKey)
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "token failed")
		return
	}
	// A successful Token call proves in.Sig verified against this station's
	// real key, so whatever is already sitting in failKey was noise --
	// clock drift, an expired nonce, a restart mid-handshake -- and not an
	// attacker grinding it up: nothing lacking the station's private key
	// can ever reach this line, so an attacker can never trigger this reset
	// to launder its own failed attempts. Clearing it here is what lets a
	// station that recovers keep working for the rest of the hour instead
	// of staying locked out by failures that happened before it recovered.
	// Best effort, like recordTokenFailure: the response is already a 200
	// by the time this runs, so a Redis error here does not need to change
	// it.
	if h.Lim.R != nil {
		_ = h.Lim.Reset(r.Context(), failKey)
	}
	httpx.Data(w, http.StatusOK, out)
}

// recordTokenFailure grows the failed-attempt bucket that gates further
// stationToken calls for this station id. Best effort: the response for
// this request is already decided (401 or 400) by the time this runs, so a
// limiter error here does not need to change it -- the pre-work bucket in
// stationToken already fails closed on every subsequent call regardless.
func (h *Handler) recordTokenFailure(r *http.Request, failKey string) {
	if h.Lim.R == nil {
		return
	}
	_, _ = h.Lim.Allow(r.Context(), failKey, tokenFailedIdentityLimit, time.Hour)
}

package b2b

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/stationctx"
)

// StationRoutes registers the route an enrolled classroom PC calls with its
// station token. r MUST already carry auth.Required: the station id is read off
// the request context, where only a verified JWT can have put it (see
// internal/auth/middleware.go and internal/stationctx), and never off the body.
func (h *Handler) StationRoutes(r chi.Router) {
	r.Post("/b2b/stations/diag", h.postDiagnostics)
}

// maxDiagBody bounds the request before any of it is parsed. The tail itself
// is capped at 32 KB by the store; this leaves generous room for JSON escaping
// of a log full of Uzbek text and Windows paths, and still refuses a body that
// could only be an attempt to fill the disk.
const maxDiagBody = 256 << 10

// diagIdentityLimit is how many reports one station may file per hour. The
// agent sends one at startup, one whenever its state changes, and a heartbeat
// every six hours, with its own five-minute debounce -- so a healthy PC uses a
// handful a day. Twenty leaves room for a machine that is genuinely flapping
// (which is itself worth seeing) while bounding what a leaked station key can
// write.
const diagIdentityLimit = 20

type diagBody struct {
	// EnrollCode and HWIDHash are read only by the unauthenticated
	// enrol-failure route; the authenticated one takes its identity from the
	// verified JWT and ignores whatever is in the body. Named apart from Code
	// on purpose -- one is the school's installer key, the other is the error
	// code being reported, and conflating them here would be a credential
	// ending up in a support field.
	EnrollCode         string `json:"enroll_code"`
	HWIDHash           string `json:"hwid_hash"`
	Label              string `json:"label"`
	AgentVersion       string `json:"agent_version"`
	Phase              string `json:"phase"`
	Code               string `json:"code"`
	Problem            string `json:"problem"`
	Detail             string `json:"detail"`
	OS                 string `json:"os"`
	ClockOffsetSeconds *int   `json:"clock_offset_seconds"`
	LogTail            string `json:"log_tail"`
}

func (h *Handler) postDiagnostics(w http.ResponseWriter, r *http.Request) {
	stationID, ok := stationctx.FromContext(r.Context())
	if !ok {
		// A learner token reaching this route is not an error worth
		// explaining: only a station has anything to report.
		httpx.Error(w, http.StatusForbidden, "station_only", "this endpoint is for classroom stations")
		return
	}
	if !h.allow(r, "diag", stationID.String(), diagIdentityLimit, diagIdentityLimit*3, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many diagnostic reports")
		return
	}

	var body diagBody
	if err := json.NewDecoder(io.LimitReader(r.Body, maxDiagBody)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}

	err := h.store().RecordStationDiagnostics(r.Context(), stationID, Diagnostics{
		AgentVersion:       body.AgentVersion,
		Phase:              body.Phase,
		Code:               body.Code,
		Problem:            body.Problem,
		Detail:             body.Detail,
		OS:                 body.OS,
		Label:              body.Label,
		ClockOffsetSeconds: body.ClockOffsetSeconds,
		LogTail:            body.LogTail,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// The token is valid but the row is gone -- the school was
			// purged while this PC was running. Nothing to record and
			// nothing the PC can do; say so plainly rather than 500.
			httpx.Error(w, http.StatusNotFound, "not_found", "station no longer exists")
			return
		}
		writeStoreErr(w, err, "could not record diagnostics")
		return
	}
	// No body: the agent has nothing to do with a response, and a report that
	// fails must never be worth retrying hard enough to matter.
	w.WriteHeader(http.StatusNoContent)
}

// enrollFailureIdentityLimit bounds reports per enrolment code per hour. A
// classroom rolling out thirty PCs where every one of them fails is thirty
// reports plus a retry or two; 200 clears that comfortably while bounding what
// a leaked installer key can write. The rows it can write are support records
// that grant nothing and are capped at five per machine.
const enrollFailureIdentityLimit = 200

// postEnrollFailure records why a PC could not enrol.
//
// Unauthenticated by necessity and by design: a machine that cannot enrol has
// no station token, and those are exactly the failures worth seeing -- the PC
// already registered to another school, the school with no free seats, the
// expired installer key. The enrolment code in the body proves which school
// the agent was aiming at, the same credential POST /enroll itself accepts,
// and unlike /enroll this route grants nothing at all.
func (h *Handler) postEnrollFailure(w http.ResponseWriter, r *http.Request) {
	var body diagBody
	if err := json.NewDecoder(io.LimitReader(r.Body, maxDiagBody)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	identity := strings.ToUpper(strings.TrimSpace(body.EnrollCode))
	if !h.allow(r, "diag_enroll", identity, enrollFailureIdentityLimit, enrollFailureIdentityLimit*3, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many diagnostic reports")
		return
	}

	err := h.store().RecordEnrollFailure(r.Context(), body.EnrollCode, body.HWIDHash, Diagnostics{
		AgentVersion:       body.AgentVersion,
		Phase:              body.Phase,
		Code:               body.Code,
		Problem:            body.Problem,
		Detail:             body.Detail,
		OS:                 body.OS,
		Label:              body.Label,
		ClockOffsetSeconds: body.ClockOffsetSeconds,
		LogTail:            body.LogTail,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
			// An unknown code or a malformed hwid is not worth a distinct
			// answer: this route must not become a way to test whether a
			// given installer key exists.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeStoreErr(w, err, "could not record diagnostics")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

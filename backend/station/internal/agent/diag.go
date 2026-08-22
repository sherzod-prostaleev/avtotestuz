package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"avtotest.uz/station/internal/netclient"
)

// ReportDiagnostics tells the backend what this PC is doing, so a school never
// has to read a log file to find out.
//
// The agent has no console window any more, and even when it had one, a
// classroom with thirty machines meant walking to each of them. This posts the
// same picture the kiosk shows locally -- phase, error code, the Uzbek
// sentence, the clock offset -- plus the tail of station.log, to a route
// authenticated with the station's own token.
//
// Failures are the caller's to ignore. A PC that cannot file a report is
// usually a PC that cannot reach the backend at all, which is itself the thing
// being reported; retrying hard would just add load to a network that is
// already failing.
type DiagReport struct {
	Phase              string `json:"phase"`
	Code               string `json:"code"`
	Problem            string `json:"problem"`
	Detail             string `json:"detail"`
	AgentVersion       string `json:"agent_version"`
	OS                 string `json:"os"`
	Label              string `json:"label"`
	ClockOffsetSeconds *int64 `json:"clock_offset_seconds"`
	LogTail            string `json:"log_tail"`

	// EnrollCode and HWIDHash are sent only by ReportEnrollFailure, where
	// there is no token and the school's installer key is the only thing that
	// says which school this PC was trying to join.
	EnrollCode string `json:"enroll_code,omitempty"`
	HWIDHash   string `json:"hwid_hash,omitempty"`
}

// diagLogTailBytes is how much of station.log travels with a report. The
// server caps it at 32 KB; asking for a little less here keeps a report from
// being silently trimmed mid-line.
const diagLogTailBytes = 28 << 10

// ReadLogTail returns the last diagLogTailBytes of the file at path, starting
// at the first newline inside that window so a report never opens mid-line.
// A missing or unreadable log is not an error: the rest of the report is still
// worth sending, and "there is no log" is itself a useful thing to see.
func ReadLogTail(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return ""
	}
	start := int64(0)
	if size > diagLogTailBytes {
		start = size - diagLogTailBytes
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return ""
	}
	if start > 0 {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 && i+1 < len(buf) {
			buf = buf[i+1:]
		}
	}
	return string(buf)
}

// ReportDiagnostics posts one report. It needs a live station token, so it can
// only run once this PC has authenticated at least once -- which is the point:
// an agent that has never enrolled has no identity to file under, and its
// operator is standing in front of the kiosk screen instead.
func (a *Agent) ReportDiagnostics(ctx context.Context, rep DiagReport) error {
	tok, err := a.Token(ctx)
	if err != nil {
		return err
	}
	rep.AgentVersion = a.Version
	rep.OS = runtime.GOOS + "/" + runtime.GOARCH
	if off, known := a.ClockOffset(); known {
		rep.ClockOffsetSeconds = &off
	}

	return a.postDiag(ctx, "/api/v1/b2b/stations/diag", rep, tok)
}

// newDiagClient is deliberately separate from Agent.client(): a log tail is up
// to 28 KB going out over a driving school's uplink, and it must never share a
// deadline with the 15-second control calls that keep the classroom working.
func newDiagClient() *http.Client {
	return netclient.New(60 * time.Second)
}

// ReportEnrollFailure posts a report for a PC that never became a station.
//
// This is the case that matters and the case the authenticated route cannot
// serve: a machine already registered to another school, a school with no free
// seats, an installer key that has expired. None of them can hold a station
// token, so before this existed every one of them failed in total silence and
// the only evidence was a log file on a PC nobody would open.
//
// The school's installer key travels with the report because it is the only
// thing that says which school to file it against. It is the same credential
// the enrolment attempt itself just sent, to the same origin, over the same
// TLS -- and unlike enrolment, this route grants nothing.
func (a *Agent) ReportEnrollFailure(ctx context.Context, code string, rep DiagReport) error {
	if code == "" {
		return errors.New("no installer key to report against")
	}
	rep.AgentVersion = a.Version
	rep.OS = runtime.GOOS + "/" + runtime.GOARCH
	rep.EnrollCode = code
	rep.HWIDHash = a.HWID
	if off, known := a.ClockOffset(); known {
		rep.ClockOffsetSeconds = &off
	}
	return a.postDiag(ctx, "/api/v1/b2b/stations/enroll-failure", rep, "")
}

// postDiag sends one report. token may be empty for the unauthenticated route.
func (a *Agent) postDiag(ctx context.Context, path string, rep DiagReport, token string) error {
	body, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.APIBase+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := newDiagClient().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode >= 400 {
		return &APIError{Path: path, Code: "http", Message: resp.Status}
	}
	return nil
}

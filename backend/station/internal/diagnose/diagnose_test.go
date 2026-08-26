package diagnose_test

import (
	"net/http"
	"strings"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/diagnose"
	"avtotest.uz/station/internal/status"
)

// TestThrottlingIsNamedOnBothPaths is the regression for what 55 classroom PCs
// were actually shown on 2026-08-26.
//
// nginx rate-limits the station endpoints and answers 429 with an HTML body of
// its own, so there is no error envelope and no code to switch on. Both
// classifiers fell through to their "unknown" arm, and the school spent the
// morning reading "Serverdan tushunarsiz javob keldi" ("the server sent
// something we don't understand") and "Server bilan aloqada xatolik" ("a
// problem communicating with the server") on the enrollment and token paths
// respectively -- neither of which says the one thing that was true and
// actionable: too many requests at once, wait, it is not your network.
func TestThrottlingIsNamedOnBothPaths(t *testing.T) {
	throttles := []struct {
		name string
		err  error
	}{
		{
			// What nginx's limit_req produces.
			name: "nginx 429 with no envelope",
			err:  &agent.APIError{Path: "/api/v1/b2b/stations/token", Status: http.StatusTooManyRequests},
		},
		{
			// What the Go API's own limiter produces.
			name: "api envelope rate_limited",
			err: &agent.APIError{
				Path: "/api/v1/b2b/stations/token", Status: http.StatusTooManyRequests,
				Code: "rate_limited", Message: "too many requests",
			},
		},
	}

	classifiers := []struct {
		name string
		run  func(error) diagnose.Result
	}{
		{"Enroll", diagnose.Enroll},
		{"Token", func(err error) diagnose.Result { return diagnose.Token(err, 0) }},
	}

	for _, tc := range throttles {
		for _, c := range classifiers {
			t.Run(tc.name+"/"+c.name, func(t *testing.T) {
				got := c.run(tc.err)
				if got.Code != "rate_limited" {
					t.Fatalf("code=%q, want rate_limited", got.Code)
				}
				if !got.Retryable {
					t.Fatal("retryable=false: throttling is the definition of try again later")
				}
				if got.Phase != status.PhaseWaiting {
					t.Fatalf("phase=%q, want %q: nobody at the school can fix this by hand",
						got.Phase, status.PhaseWaiting)
				}
				// The exact wording may change; what must not come back is the
				// message that told a school its server was unintelligible.
				for _, banned := range []string{"tushunarsiz", "kutilmagan"} {
					if strings.Contains(strings.ToLower(got.Problem), banned) {
						t.Fatalf("problem=%q still reads as an unclassified failure", got.Problem)
					}
				}
				if got.Problem == "" || got.Action == "" {
					t.Fatalf("problem=%q action=%q, both must be filled in", got.Problem, got.Action)
				}
				if got.Detail == "" {
					t.Fatal("detail is empty: support has nothing to go on")
				}
			})
		}
	}
}

// TestABareStatusIsStillClassifiedByCode guards the change that gave APIError a
// Status: a rejection that DOES carry an envelope must keep being classified by
// its code, not swallowed by the new status check.
func TestABareStatusIsStillClassifiedByCode(t *testing.T) {
	cases := []struct {
		code      string
		wantPhase status.Phase
		retryable bool
	}{
		{"hwid_other_org", status.PhaseBlocked, false},
		{"no_license", status.PhaseBlocked, false},
		{"seats_exhausted", status.PhaseBlocked, false},
		{"org_suspended", status.PhaseBlocked, false},
		{"not_found", status.PhaseBlocked, false},
		{"invalid", status.PhaseBlocked, false},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := diagnose.Enroll(&agent.APIError{
				Path: "/api/v1/b2b/stations/enroll", Status: http.StatusConflict,
				Code: tc.code, Message: "nope",
			})
			if got.Code != tc.code {
				t.Fatalf("code=%q, want %q", got.Code, tc.code)
			}
			if got.Phase != tc.wantPhase {
				t.Fatalf("phase=%q, want %q", got.Phase, tc.wantPhase)
			}
			if got.Retryable != tc.retryable {
				t.Fatalf("retryable=%v, want %v: retrying a refusal spends seats and fixes nothing",
					got.Retryable, tc.retryable)
			}
		})
	}
}

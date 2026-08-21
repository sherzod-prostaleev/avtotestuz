package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/status"
)

// TestEnrollWithRetryStopsOnPermanentRefusal is the incident this whole
// classification exists for. A PC that already belongs to another school gets
// 409 conflict, which no amount of retrying can change. The agent used to
// treat it exactly like a dropped packet: four attempts, then a console held
// for two minutes on "Press Enter to close" with the single word "conflict" on
// it. One attempt, and a sentence naming the school, is the whole fix.
func TestEnrollWithRetryStopsOnPermanentRefusal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{
		{"another school owns this PC", http.StatusConflict, "conflict"},
		{"installer key is dead", http.StatusNotFound, "not_found"},
		{"school has no licence", http.StatusBadRequest, "no_license"},
		{"every seat is taken", http.StatusConflict, "seats_exhausted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": tc.code, "message": tc.code},
				})
			}))
			t.Cleanup(srv.Close)

			dir := t.TempDir()
			ks, err := keystore.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			a := &agent.Agent{APIBase: srv.URL, StateDir: dir, Keys: ks, HWID: "hwid", Version: "test"}
			st := status.New("test", "", "")

			if err := enrollWithRetry(context.Background(), a, st, "AVTO-TEST-CODE", "PC-1", "Yangi Avtomaktab", fastEnrollRetry); err == nil {
				t.Fatal("enrollWithRetry() = nil, want the refusal to be returned")
			}
			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Fatalf("enroll was attempted %d times, want exactly 1: %s can never succeed by retrying", got, tc.code)
			}

			snap := st.Get()
			if snap.Phase != status.PhaseBlocked {
				t.Fatalf("phase = %q, want %q so the kiosk stops showing a spinner", snap.Phase, status.PhaseBlocked)
			}
			if snap.Problem == "" || snap.Action == "" {
				t.Fatalf("status carries no Uzbek problem/action: %+v", snap)
			}
			if snap.Code != tc.code {
				t.Fatalf("status code = %q, want %q", snap.Code, tc.code)
			}
		})
	}
}

// TestDifferentSchoolNeedsPositiveEvidence pins the conservative half of the
// check. Firing it on a working classroom because an older agent wrote a
// station.json without these fields would be worse than the bug it detects.
func TestDifferentSchoolNeedsPositiveEvidence(t *testing.T) {
	codeA, codeB := "AVTO-AAAA-AAAA", "AVTO-BBBB-BBBB"

	for _, tc := range []struct {
		name  string
		state agent.State
		cfg   resolved
		want  bool
	}{
		{
			name:  "same installer key",
			state: agent.State{StationID: "s1", CodeHash: agent.HashCode(codeA)},
			cfg:   resolved{Code: codeA},
			want:  false,
		},
		{
			name:  "a different school's installer",
			state: agent.State{StationID: "s1", CodeHash: agent.HashCode(codeA), Org: "Eski Maktab"},
			cfg:   resolved{Code: codeB, Org: "Yangi Maktab"},
			want:  true,
		},
		{
			name:  "key is case- and space-insensitive",
			state: agent.State{StationID: "s1", CodeHash: agent.HashCode(codeA)},
			cfg:   resolved{Code: "  avto-aaaa-aaaa  "},
			want:  false,
		},
		{
			name:  "state written by an older agent carries neither field",
			state: agent.State{StationID: "s1"},
			cfg:   resolved{Code: codeB, Org: "Yangi Maktab"},
			want:  false,
		},
		{
			name:  "org name alone still decides when there is no key hash",
			state: agent.State{StationID: "s1", Org: "Eski Maktab"},
			cfg:   resolved{Org: "Yangi Maktab"},
			want:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := differentSchool(tc.state, tc.cfg); got != tc.want {
				t.Fatalf("differentSchool() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHashCodeDoesNotStoreTheKey guards the reason CodeHash exists at all: an
// installer key can enrol further PCs, and station.json is a plain file on a
// shared classroom machine.
func TestHashCodeDoesNotStoreTheKey(t *testing.T) {
	const code = "AVTO-SECRET-KEY"
	h := agent.HashCode(code)
	if h == "" {
		t.Fatal("HashCode() = \"\", want a fingerprint")
	}
	if h == code || len(h) != 16 {
		t.Fatalf("HashCode() = %q, want a short digest that is not the key itself", h)
	}
	if agent.HashCode("") != "" {
		t.Fatal("HashCode(\"\") must stay empty so the comparison is skipped")
	}
}

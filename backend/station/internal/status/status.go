// Package status holds the one live picture of what this classroom PC is
// doing, so the agent can answer "what is wrong?" without anyone reading a
// console.
//
// The agent runs with no window (see cmd/avtotest-station's -H windowsgui
// build) and the only screen a school ever looks at is the kiosk page in the
// browser. That page therefore has to be able to ask the local agent how it is
// getting on, which is what internal/proxy serves at /__station/status from a
// Store kept here. Everything in the struct is deliberately safe to show on a
// classroom projector: no token, no installer key, no private key.
//
// Messages are stored ready-translated into Uzbek. The kiosk page renders
// whatever it is handed, because the failures worth reporting are operational
// ("this PC's clock is wrong", "this PC belongs to another school") and belong
// in the language of the person standing next to the machine, not in Go error
// prose.
package status

import (
	"sync"
	"time"
)

// Phase is the coarse state a school cares about.
type Phase string

const (
	// PhaseStarting is set before the first enrollment or token attempt has
	// finished. It is the only phase that legitimately resolves on its own
	// within a few seconds.
	PhaseStarting Phase = "starting"
	// PhaseReady means a station token is live and the kiosk works.
	PhaseReady Phase = "ready"
	// PhaseWaiting means something is temporarily wrong (no network yet, the
	// backend is down) and the agent is retrying on its own. Nobody needs to
	// do anything.
	PhaseWaiting Phase = "waiting"
	// PhaseBlocked means retrying cannot help: a human has to change
	// something, either on this PC or in the admin panel. Distinguishing this
	// from PhaseWaiting is the entire point of this package -- the old agent
	// showed the same endless spinner for both.
	PhaseBlocked Phase = "blocked"
)

// Snapshot is the JSON the kiosk page and a support call both read.
type Snapshot struct {
	Version   string `json:"version"`
	Phase     Phase  `json:"phase"`
	Org       string `json:"org"`
	StationID string `json:"station_id"`
	Label     string `json:"label"`
	Enrolled  bool   `json:"enrolled"`
	TokenOK   bool   `json:"token_ok"`

	// Problem is the Uzbek sentence to show the school; Action is what to do
	// about it. Both empty when there is nothing wrong.
	Problem string `json:"problem"`
	Action  string `json:"action"`
	// Detail is the raw Go/wire error, kept for a support call. The kiosk
	// shows it in small print rather than hiding it: it is what makes a
	// screenshot from a school actually useful.
	Detail string `json:"detail"`
	// Code is the backend's error code when there was one, e.g. "conflict".
	Code string `json:"code"`

	SinceUnix   int64  `json:"since_unix"`
	LogPath     string `json:"log_path"`
	ListenAddr  string `json:"listen_addr"`
	UpdateState string `json:"update_state"`
}

// Store is a concurrency-safe Snapshot. The zero value is not usable; call
// New.
type Store struct {
	mu   sync.RWMutex
	snap Snapshot
}

// New returns a Store describing a PC that has only just started.
func New(version, logPath, listenAddr string) *Store {
	return &Store{snap: Snapshot{
		Version:    version,
		Phase:      PhaseStarting,
		LogPath:    logPath,
		ListenAddr: listenAddr,
		SinceUnix:  time.Now().Unix(),
	}}
}

// Get returns a copy of the current snapshot.
func (s *Store) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// SetIdentity records which school and station this PC belongs to. Called
// after enrollment and after state is loaded from disk.
func (s *Store) SetIdentity(org, stationID, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if org != "" {
		s.snap.Org = org
	}
	s.snap.StationID = stationID
	s.snap.Label = label
	s.snap.Enrolled = stationID != ""
}

// SetReady clears any problem and marks the kiosk usable.
func (s *Store) SetReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap.Phase != PhaseReady {
		s.snap.SinceUnix = time.Now().Unix()
	}
	s.snap.Phase = PhaseReady
	s.snap.TokenOK = true
	s.snap.Problem = ""
	s.snap.Action = ""
	s.snap.Detail = ""
	s.snap.Code = ""
}

// SetProblem records something the school may need to see. phase distinguishes
// "the agent is handling it" (PhaseWaiting) from "a human must act"
// (PhaseBlocked); problem and action are Uzbek, detail is the raw error.
func (s *Store) SetProblem(phase Phase, code, problem, action, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap.Phase != phase || s.snap.Code != code {
		s.snap.SinceUnix = time.Now().Unix()
	}
	s.snap.Phase = phase
	s.snap.TokenOK = false
	s.snap.Code = code
	s.snap.Problem = problem
	s.snap.Action = action
	s.snap.Detail = detail
}

// SetUpdateState records what the self-updater is doing, e.g. "1.1.0 tayyor --
// keyingi yoqilishda o'rnatiladi".
func (s *Store) SetUpdateState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.UpdateState = state
}

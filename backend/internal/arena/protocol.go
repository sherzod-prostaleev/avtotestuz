// Package arena implements VIP-only live 1v1 duel transport, matchmaking,
// and match state (M4-03). Rating fill is M4-04; UI is M4-05 / J10.
//
// Match state has exactly one writer (its goroutine). Decision logic that
// can be pure lives in rules.go.
//
// A duel is self-paced: both players get the same questions in the same
// order, but each moves to the next one the moment they answer (or their
// own 15 s run out). Whoever finishes first waits for the other; the
// opponent's position is streamed as opponent.progress so the waiting is
// visible instead of silent.
package arena

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	ProtocolVersion = 1

	QuestionCount   = 10
	QuestionTimeSec = 15
	ReconnectGrace  = 20 * time.Second
	TicketTTL       = 30 * time.Second
	QueueTimeout    = 45 * time.Second
	AnswerGrace     = 400 * time.Millisecond
	// RevealFor is how long a player sees their own verdict (green/red)
	// before the server deals them the next question.
	RevealFor = 1200 * time.Millisecond
	Countdown = 3 * time.Second

	CloseReplaced      = 4001
	CloseTicketInvalid = 4002
	CloseShutdown      = 4003
)

// Match modes. Only ranked moves ELO (see migration 0074).
const (
	ModeRanked = "ranked"
	ModeFriend = "friend"
	ModeBot    = "bot"
)

// Round marks, one per question, as the client draws them.
const (
	MarkCorrect = "correct"
	MarkWrong   = "wrong"
	MarkSkipped = "skipped"
)

// Envelope is the versioned WS JSON frame.
type Envelope struct {
	V int             `json:"v"`
	T string          `json:"t"`
	D json.RawMessage `json:"d"`
}

func Encode(t string, d any) ([]byte, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{V: ProtocolVersion, T: t, D: raw})
}

func Decode(b []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return Envelope{}, err
	}
	if env.V != ProtocolVersion {
		return Envelope{}, fmt.Errorf("bad_protocol")
	}
	if env.T == "" {
		return Envelope{}, fmt.Errorf("bad_protocol")
	}
	return env, nil
}

type HelloData struct {
	ProfileID    uuid.UUID `json:"profile_id"`
	ServerTimeMs int64     `json:"server_time_ms"`
	Protocol     int       `json:"protocol"`
	Online       int       `json:"online"`
	// InMatch tells a reconnecting client that a match.state resync follows.
	InMatch bool `json:"in_match"`
	// Invite is the player's still-open invite code, if any.
	Invite *InviteCreatedData `json:"invite,omitempty"`
}

type QueueJoinedData struct {
	QueuedAtMs   int64 `json:"queued_at_ms"`
	TimeoutMs    int64 `json:"timeout_ms"`
	ServerTimeMs int64 `json:"server_time_ms"`
	Online       int   `json:"online"`
}

type QueueTimeoutData struct {
	WaitedMs int64 `json:"waited_ms"`
	Online   int   `json:"online"`
}

// PlayerCard is who a player is, as the other side is shown it.
type PlayerCard struct {
	Name   string `json:"name"`
	Rating int    `json:"rating"`
	Medal  string `json:"medal"`
	Bot    bool   `json:"bot,omitempty"`
}

type MatchFoundData struct {
	MatchID        uuid.UUID  `json:"match_id"`
	Mode           string     `json:"mode"`
	You            PlayerCard `json:"you"`
	Opponent       PlayerCard `json:"opponent"`
	QuestionCount  int        `json:"question_count"`
	QuestionTimeMs int64      `json:"question_time_ms"`
	StartsInMs     int64      `json:"starts_in_ms"`
	StartsAtMs     int64      `json:"starts_at_ms"`
	ServerTimeMs   int64      `json:"server_time_ms"`
}

type AnswerClientData struct {
	MatchID  uuid.UUID `json:"match_id"`
	Index    int       `json:"index"`
	AnswerID uuid.UUID `json:"answer_id"`
}

type MatchRejoinData struct {
	MatchID uuid.UUID `json:"match_id"`
}

type QuestionData struct {
	Index        int   `json:"index"`
	Total        int   `json:"total"`
	DeadlineMs   int64 `json:"deadline_ms"`
	ServerTimeMs int64 `json:"server_time_ms"`
	Question     any   `json:"question"`
}

// AnswerResultData is the verdict on one of the player's own questions: sent
// when they answer, or when their clock for it runs out (Answered=false).
type AnswerResultData struct {
	Index           int        `json:"index"`
	Answered        bool       `json:"answered"`
	Correct         bool       `json:"correct"`
	AnswerID        *uuid.UUID `json:"answer_id,omitempty"`
	CorrectAnswerID uuid.UUID  `json:"correct_answer_id"`
	Points          int        `json:"points"`
	Score           int        `json:"score"`
	ResponseMs      int64      `json:"response_ms"`
	NextInMs        int64      `json:"next_in_ms"`
	// Last is true on the final question: after the reveal the player waits.
	Last bool `json:"last"`
}

// OpponentProgressData is the opponent's position only — never which
// answers they got right, so a leader cannot be read off mid-duel.
type OpponentProgressData struct {
	Answered int  `json:"answered"`
	Total    int  `json:"total"`
	Finished bool `json:"finished"`
}

type OpponentStatusData struct {
	State string `json:"state"` // disconnected|connected
}

// MatchWaitingData is sent to a player who finished before the opponent.
type MatchWaitingData struct {
	Score   int `json:"score"`
	Correct int `json:"correct"`
}

// OpponentState is the opponent block of a resync snapshot.
type OpponentState struct {
	PlayerCard
	Answered  int  `json:"answered"`
	Finished  bool `json:"finished"`
	Connected bool `json:"connected"`
}

// MatchStateData is a full resync for a player who (re)connects mid-match:
// everything the client needs to redraw the duel from nothing.
type MatchStateData struct {
	MatchID        uuid.UUID         `json:"match_id"`
	Mode           string            `json:"mode"`
	Phase          string            `json:"phase"` // countdown|question|reveal|waiting
	Index          int               `json:"index"`
	Total          int               `json:"total"`
	QuestionTimeMs int64             `json:"question_time_ms"`
	StartsAtMs     int64             `json:"starts_at_ms"`
	DeadlineMs     int64             `json:"deadline_ms"`
	ServerTimeMs   int64             `json:"server_time_ms"`
	Question       any               `json:"question,omitempty"`
	LastResult     *AnswerResultData `json:"last_result,omitempty"`
	You            PlayerCard        `json:"you"`
	Marks          []string          `json:"marks"`
	Score          int               `json:"score"`
	Opponent       OpponentState     `json:"opponent"`
}

type ScorePair struct {
	You      int `json:"you"`
	Opponent int `json:"opponent"`
}

type MarksPair struct {
	You      []string `json:"you"`
	Opponent []string `json:"opponent"`
}

type MatchEndData struct {
	MatchID      uuid.UUID `json:"match_id"`
	Mode         string    `json:"mode"`
	Outcome      string    `json:"outcome"` // won|lost|draw
	Reason       string    `json:"reason"`
	Score        ScorePair `json:"score"`
	Correct      ScorePair `json:"correct"`
	Marks        MarksPair `json:"marks"`
	Total        int       `json:"total"`
	Rated        bool      `json:"rated"`
	RatingBefore int       `json:"rating_before"`
	RatingAfter  int       `json:"rating_after"`
	RatingDelta  int       `json:"rating_delta"`
	Medal        string    `json:"medal,omitempty"`
}

type InviteCreatedData struct {
	Code         string `json:"code"`
	ExpiresInSec int    `json:"expires_in_sec"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type QueueJoinClientData struct {
	Locale string `json:"locale"`
}

type InviteJoinClientData struct {
	Code   string `json:"code"`
	Locale string `json:"locale"`
}

type BotStartClientData struct {
	Locale string `json:"locale"`
}

func ms(t time.Time) int64 { return t.UTC().UnixMilli() }

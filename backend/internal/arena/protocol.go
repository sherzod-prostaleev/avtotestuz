// Package arena implements VIP-only live 1v1 duel transport, matchmaking,
// and match state (M4-03). Rating fill is M4-04; UI is M4-05 / J10.
//
// Match state has exactly one writer (its goroutine). Decision logic that
// can be pure lives in rules.go.
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

	CloseReplaced      = 4001
	CloseTicketInvalid = 4002
	CloseShutdown      = 4003
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
}

type QueueJoinedData struct {
	QueuedAtMs int64 `json:"queued_at_ms"`
	TimeoutMs  int64 `json:"timeout_ms"`
}

type QueueTimeoutData struct {
	WaitedMs int64 `json:"waited_ms"`
}

type MatchFoundData struct {
	MatchID  uuid.UUID `json:"match_id"`
	Opponent struct {
		Name string `json:"name"`
	} `json:"opponent"`
	QuestionCount  int   `json:"question_count"`
	QuestionTimeMs int64 `json:"question_time_ms"`
	StartsInMs     int64 `json:"starts_in_ms"`
}

type AnswerClientData struct {
	MatchID  uuid.UUID `json:"match_id"`
	Index    int       `json:"index"`
	AnswerID uuid.UUID `json:"answer_id"`
}

type MatchRejoinData struct {
	MatchID uuid.UUID `json:"match_id"`
}

type AnswerAckData struct {
	Index      int   `json:"index"`
	ResponseMs int64 `json:"response_ms"`
}

type QuestionData struct {
	Index        int   `json:"index"`
	Total        int   `json:"total"`
	DeadlineMs   int64 `json:"deadline_ms"`
	ServerTimeMs int64 `json:"server_time_ms"`
	Question     any   `json:"question"`
}

type PlayerRoundData struct {
	Answered bool `json:"answered"`
	Correct  bool `json:"correct"`
	Points   int  `json:"points"`
}

type QuestionResultData struct {
	Index           int             `json:"index"`
	CorrectAnswerID uuid.UUID       `json:"correct_answer_id"`
	You             PlayerRoundData `json:"you"`
	Opponent        PlayerRoundData `json:"opponent"`
	Score           struct {
		You      int `json:"you"`
		Opponent int `json:"opponent"`
	} `json:"score"`
}

type OpponentStatusData struct {
	State string `json:"state"` // disconnected|connected
}

type MatchStateData struct {
	MatchID    uuid.UUID `json:"match_id"`
	Index      int       `json:"index"`
	Phase      string    `json:"phase"`
	DeadlineMs int64     `json:"deadline_ms"`
	Score      struct {
		You      int `json:"you"`
		Opponent int `json:"opponent"`
	} `json:"score"`
	Correct struct {
		You      int `json:"you"`
		Opponent int `json:"opponent"`
	} `json:"correct"`
}

type MatchEndData struct {
	MatchID uuid.UUID `json:"match_id"`
	Outcome string    `json:"outcome"` // won|lost|draw
	Reason  string    `json:"reason"`
	Score   struct {
		You      int `json:"you"`
		Opponent int `json:"opponent"`
	} `json:"score"`
	Correct struct {
		You      int `json:"you"`
		Opponent int `json:"opponent"`
	} `json:"correct"`
	RatingDelta int    `json:"rating_delta,omitempty"`
	Medal       string `json:"medal,omitempty"`
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

func ms(t time.Time) int64 { return t.UTC().UnixMilli() }

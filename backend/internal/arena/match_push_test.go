package arena

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
)

type stubQuestionLoader struct {
	detail    content.QuestionDetailDTO
	fallback  bool
	err       error
	callCount int
}

func (s *stubQuestionLoader) LoadQuestionDetail(
	context.Context, uuid.UUID, string,
) (content.QuestionDetailDTO, bool, error) {
	s.callCount++
	return s.detail, s.fallback, s.err
}

func drainOut(t *testing.T, c *Conn) []capturedFrame {
	t.Helper()
	var out []capturedFrame
	for {
		select {
		case payload := <-c.out:
			env, err := Decode(payload)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			out = append(out, capturedFrame{t: env.T, d: env.D})
		default:
			return out
		}
	}
}

type capturedFrame struct {
	t string
	d json.RawMessage
}

func TestPushQuestionUsesDetailWhenFallbackFalse(t *testing.T) {
	// Regression: LoadQuestionDetail's bool is fallbackUsed. Native-locale
	// questions return false and must still be fully dealt to both players.
	qid := uuid.New()
	ansID := uuid.New()
	loader := &stubQuestionLoader{
		fallback: false,
		detail: content.QuestionDetailDTO{
			QuestionDTO: content.QuestionDTO{
				ID:   qid.String(),
				Text: "Qaysi belgi?",
				Answers: []content.AnswerDTO{
					{ID: ansID.String(), Position: 1, Text: "A"},
					{ID: uuid.New().String(), Position: 2, Text: "B"},
				},
			},
		},
	}
	a, b := uuid.New(), uuid.New()
	svc := &Service{
		Content: loader,
		Hub:     NewHub(),
		Now:     func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}
	cA := &Conn{ProfileID: a, out: make(chan []byte, 16)}
	svc.Hub.Register(a, cA)

	m := NewMatch(svc, uuid.New(), a, b, "uz-Latn", "uz-Latn", []uuid.UUID{qid}, map[uuid.UUID]uuid.UUID{qid: ansID})
	m.pushQuestion(context.Background(), a, "uz-Latn", qid, 0)

	if loader.callCount != 1 {
		t.Fatalf("loader calls=%d", loader.callCount)
	}
	frames := drainOut(t, cA)
	var questionFrames int
	for _, msg := range frames {
		if msg.t != "question" {
			continue
		}
		questionFrames++
		var qd QuestionData
		if err := json.Unmarshal(msg.d, &qd); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(qd.Question)
		var payload struct {
			ID      string `json:"id"`
			Text    string `json:"text"`
			Answers []any  `json:"answers"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Text == "" || len(payload.Answers) < 2 {
			t.Fatalf("expected full question payload, got %s", string(raw))
		}
	}
	if questionFrames != 1 {
		t.Fatalf("want 1 question frame, got %d (frames=%v)", questionFrames, frames)
	}
}

func TestPushQuestionSkipsEmptyWhenLoadFails(t *testing.T) {
	loader := &stubQuestionLoader{err: errors.New("db down"), fallback: false}
	a := uuid.New()
	svc := &Service{
		Content: loader,
		Hub:     NewHub(),
		Now:     func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}
	cA := &Conn{ProfileID: a, out: make(chan []byte, 16)}
	svc.Hub.Register(a, cA)
	qid := uuid.New()
	m := NewMatch(svc, uuid.New(), a, uuid.New(), "uz-Latn", "uz-Latn", []uuid.UUID{qid}, nil)
	m.pushQuestion(context.Background(), a, "uz-Latn", qid, 0)

	frames := drainOut(t, cA)
	var sawErr, sawQ bool
	for _, msg := range frames {
		switch msg.t {
		case "error":
			sawErr = true
		case "question":
			sawQ = true
			var qd QuestionData
			_ = json.Unmarshal(msg.d, &qd)
			raw, _ := json.Marshal(qd.Question)
			var payload struct {
				Answers []any `json:"answers"`
			}
			_ = json.Unmarshal(raw, &payload)
			if payload.Answers == nil {
				t.Fatalf("fallback question must include answers array, got %s", string(raw))
			}
		}
	}
	if !sawErr || !sawQ {
		t.Fatalf("want error+question frames, frames=%v", frames)
	}
}

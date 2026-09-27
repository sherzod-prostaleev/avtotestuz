package arena

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/content"
)

type capturedFrame struct {
	t string
	d json.RawMessage
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

func framesOf(frames []capturedFrame, typ string) []capturedFrame {
	var out []capturedFrame
	for _, f := range frames {
		if f.t == typ {
			out = append(out, f)
		}
	}
	return out
}

// duelRig is a match driven by hand: a fake clock, captured sockets, and the
// match's handlers called directly instead of through Run's goroutine.
type duelRig struct {
	t       *testing.T
	now     time.Time
	m       *Match
	a, b    uuid.UUID
	ca, cb  *Conn
	qids    []uuid.UUID
	correct map[uuid.UUID]uuid.UUID
	wrong   map[uuid.UUID]uuid.UUID
}

func newDuelRig(t *testing.T, n int, bot bool) *duelRig {
	t.Helper()
	r := &duelRig{t: t, now: time.Unix(1_700_000_000, 0).UTC(),
		correct: map[uuid.UUID]uuid.UUID{}, wrong: map[uuid.UUID]uuid.UUID{}}
	svc := &Service{Hub: NewHub(), Log: zap.NewNop(), Now: func() time.Time { return r.now }}
	var list []content.QuestionDetailDTO
	for i := 0; i < n; i++ {
		qid, ok, bad := uuid.New(), uuid.New(), uuid.New()
		r.qids = append(r.qids, qid)
		r.correct[qid] = ok
		r.wrong[qid] = bad
		list = append(list, content.QuestionDetailDTO{QuestionDTO: content.QuestionDTO{
			ID: qid.String(), Text: "Savol",
			Answers: []content.AnswerDTO{{ID: ok.String(), Position: 1, Text: "A"}, {ID: bad.String(), Position: 2, Text: "B"}},
		}})
	}
	r.a, r.b = uuid.New(), uuid.New()
	r.ca = &Conn{ProfileID: r.a, out: make(chan []byte, 256)}
	svc.Hub.Register(r.a, r.ca)
	specB := SeatSpec{ID: r.b, Locale: "uz-Latn", Card: PlayerCard{Name: "B", Rating: 1000}}
	mode := ModeRanked
	if bot {
		specB.Bot = true
		mode = ModeBot
	} else {
		r.cb = &Conn{ProfileID: r.b, out: make(chan []byte, 256)}
		svc.Hub.Register(r.b, r.cb)
	}
	r.m = NewMatch(svc, uuid.New(), mode,
		SeatSpec{ID: r.a, Locale: "uz-Latn", Card: PlayerCard{Name: "A", Rating: 1000}}, specB,
		r.qids, map[string][]content.QuestionDetailDTO{"uz-Latn": list}, r.correct)
	r.m.Start()
	return r
}

func (r *duelRig) advance(d time.Duration) {
	r.now = r.now.Add(d)
	r.m.onTimer(context.Background())
}

func (r *duelRig) answer(pid uuid.UUID, index int, right bool) {
	qid := r.qids[index]
	aid := r.wrong[qid]
	if right {
		aid = r.correct[qid]
	}
	r.m.handle(context.Background(), matchEvent{kind: "answer", profileID: pid, index: index, answerID: aid})
}

func TestDuelIsSelfPaced(t *testing.T) {
	r := newDuelRig(t, 3, false)
	r.advance(Countdown)
	if got := len(framesOf(drainOut(t, r.ca), "question")); got != 1 {
		t.Fatalf("A should get question 0 at start, got %d question frames", got)
	}
	drainOut(t, r.cb)

	r.now = r.now.Add(2 * time.Second)
	r.answer(r.a, 0, true)
	fa := drainOut(t, r.ca)
	res := framesOf(fa, "answer.result")
	if len(res) != 1 {
		t.Fatalf("A must get its verdict immediately, frames=%v", fa)
	}
	var ar AnswerResultData
	_ = json.Unmarshal(res[0].d, &ar)
	if !ar.Correct || ar.Points <= 0 || ar.CorrectAnswerID != r.correct[r.qids[0]] {
		t.Fatalf("bad verdict %+v", ar)
	}
	prog := framesOf(drainOut(t, r.cb), "opponent.progress")
	if len(prog) != 1 {
		t.Fatalf("B must see A move, got %d progress frames", len(prog))
	}
	// The opponent's frame carries position only, never correctness.
	if strings.Contains(string(prog[0].d), "correct") || strings.Contains(string(prog[0].d), "score") {
		t.Fatalf("opponent.progress leaks the result: %s", prog[0].d)
	}

	// A does not wait for B: after the reveal A is on question 1, B still on 0.
	r.advance(RevealFor)
	qs := framesOf(drainOut(t, r.ca), "question")
	if len(qs) != 1 {
		t.Fatalf("A should be dealt question 1 without waiting for B")
	}
	var qd QuestionData
	_ = json.Unmarshal(qs[0].d, &qd)
	if qd.Index != 1 {
		t.Fatalf("A on index %d, want 1", qd.Index)
	}
	if r.m.seatOf(r.b).index != 0 || r.m.seatOf(r.b).state != seatActive {
		t.Fatalf("B moved without answering: %+v", r.m.seatOf(r.b))
	}
}

func TestFinisherWaitsForOpponent(t *testing.T) {
	r := newDuelRig(t, 2, false)
	r.advance(Countdown)
	for i := 0; i < 2; i++ {
		r.now = r.now.Add(time.Second)
		r.answer(r.a, i, true)
		r.advance(RevealFor)
	}
	fa := drainOut(t, r.ca)
	if len(framesOf(fa, "match.waiting")) != 1 {
		t.Fatalf("A finished first and must be told to wait, frames=%v", fa)
	}
	if len(framesOf(fa, "match.end")) != 0 {
		t.Fatal("match ended while B was still playing")
	}
	if r.m.phase != "active" {
		t.Fatalf("phase=%s", r.m.phase)
	}
	prog := framesOf(drainOut(t, r.cb), "opponent.progress")
	var last OpponentProgressData
	_ = json.Unmarshal(prog[len(prog)-1].d, &last)
	if !last.Finished || last.Answered != 2 {
		t.Fatalf("B must see A finished, got %+v", last)
	}
}

func TestUnansweredQuestionTimesOutPerPlayer(t *testing.T) {
	r := newDuelRig(t, 2, false)
	r.advance(Countdown)
	r.now = r.now.Add(time.Second)
	r.answer(r.b, 0, true)
	drainOut(t, r.ca)
	// A's own clock runs out; B's progress is irrelevant to it.
	r.advance(time.Duration(QuestionTimeSec)*time.Second + AnswerGrace)
	res := framesOf(drainOut(t, r.ca), "answer.result")
	if len(res) != 1 {
		t.Fatalf("A must get a timeout verdict")
	}
	var ar AnswerResultData
	_ = json.Unmarshal(res[0].d, &ar)
	if ar.Answered || ar.Points != 0 || ar.AnswerID != nil {
		t.Fatalf("timeout verdict wrong: %+v", ar)
	}
	if got := r.m.seatOf(r.a).answers[0].mark(); got != MarkSkipped {
		t.Fatalf("mark=%s", got)
	}
}

func TestLateAndDuplicateAnswersAreRejected(t *testing.T) {
	r := newDuelRig(t, 2, false)
	r.advance(Countdown)
	r.answer(r.a, 0, true)
	drainOut(t, r.ca)
	r.answer(r.a, 0, false) // same question again during the reveal
	errs := framesOf(drainOut(t, r.ca), "error")
	if len(errs) != 1 || r.m.seatOf(r.a).score == 0 || r.m.seatOf(r.a).correctN != 1 {
		t.Fatalf("duplicate answer changed the result: errs=%d seat=%+v", len(errs), r.m.seatOf(r.a))
	}
	// An answer id from another question is refused.
	r.advance(RevealFor)
	drainOut(t, r.ca)
	r.m.handle(context.Background(), matchEvent{kind: "answer", profileID: r.a, index: 1, answerID: r.correct[r.qids[0]]})
	if len(framesOf(drainOut(t, r.ca), "error")) != 1 {
		t.Fatal("foreign answer id accepted")
	}
}

func TestResyncSnapshotRebuildsTheDuel(t *testing.T) {
	r := newDuelRig(t, 3, false)
	snap := r.m.stateSnapshot(r.m.seatOf(r.a))
	if snap.Phase != "countdown" || snap.StartsAtMs == 0 {
		t.Fatalf("pending snapshot %+v", snap)
	}
	r.advance(Countdown)
	r.answer(r.a, 0, false)
	r.answer(r.b, 0, true)
	r.advance(RevealFor)
	r.answer(r.b, 1, true)

	snap = r.m.stateSnapshot(r.m.seatOf(r.a))
	if snap.Phase != "question" || snap.Index != 1 || snap.Question == nil || snap.DeadlineMs == 0 {
		t.Fatalf("A snapshot %+v", snap)
	}
	if len(snap.Marks) != 1 || snap.Marks[0] != MarkWrong {
		t.Fatalf("A marks %v", snap.Marks)
	}
	if snap.Opponent.Answered != 2 || snap.Opponent.Name != "B" || !snap.Opponent.Connected {
		t.Fatalf("opponent block %+v", snap.Opponent)
	}
	bs := r.m.stateSnapshot(r.m.seatOf(r.b))
	if bs.Phase != "reveal" || bs.LastResult == nil || !bs.LastResult.Correct {
		t.Fatalf("B snapshot %+v", bs)
	}
}

func TestReconnectResendsState(t *testing.T) {
	r := newDuelRig(t, 2, false)
	r.advance(Countdown)
	drainOut(t, r.ca)
	drainOut(t, r.cb)
	ctx := context.Background()
	r.m.handle(ctx, matchEvent{kind: "disconnect", profileID: r.a})
	if st := framesOf(drainOut(t, r.cb), "opponent.status"); len(st) != 1 {
		t.Fatal("B not told A dropped")
	}
	r.m.handle(ctx, matchEvent{kind: "reconnect", profileID: r.a})
	if len(framesOf(drainOut(t, r.ca), "match.state")) != 1 {
		t.Fatal("A got no resync")
	}
	if len(framesOf(drainOut(t, r.cb), "opponent.status")) != 1 {
		t.Fatal("B not told A is back")
	}
}

func TestStaleForfeitCheckIsIgnored(t *testing.T) {
	r := newDuelRig(t, 2, false)
	r.advance(Countdown)
	ctx := context.Background()
	r.m.handle(ctx, matchEvent{kind: "disconnect", profileID: r.a}) // gen 1
	r.m.handle(ctx, matchEvent{kind: "reconnect", profileID: r.a})
	r.m.handle(ctx, matchEvent{kind: "disconnect", profileID: r.a}) // gen 2
	// The check scheduled by the first drop fires: A has had only part of the
	// grace for the second drop and must not be forfeited yet.
	r.m.handle(ctx, matchEvent{kind: "forfeit_check", profileID: r.a, gen: 1})
	if r.m.phase == "finished" {
		t.Fatal("stale forfeit check ended the match")
	}
}

func TestBotPlaysItsTurnsAndGetsNoFrames(t *testing.T) {
	r := newDuelRig(t, 3, true)
	r.advance(Countdown)
	bot := r.m.seatOf(r.b)
	if bot.state != seatActive || bot.botAt.IsZero() {
		t.Fatalf("bot not dealt its first question: %+v", bot)
	}
	window := time.Duration(QuestionTimeSec) * time.Second
	if d := bot.botAt.Sub(r.now); d < BotDelay(0, window) || d > BotDelay(0.999, window) {
		t.Fatalf("bot delay %v out of range", d)
	}
	drainOut(t, r.ca)
	r.now = bot.botAt
	r.m.onTimer(context.Background())
	if bot.state != seatReveal || !bot.answers[0].answered {
		t.Fatalf("bot did not answer: %+v", bot)
	}
	if prog := framesOf(drainOut(t, r.ca), "opponent.progress"); len(prog) != 1 {
		t.Fatal("human must see the bot move")
	}
	if at, ok := r.m.nextWake(); !ok || at.IsZero() {
		t.Fatal("no timer armed for the next step")
	}
}

func TestBotAccuracyExtremes(t *testing.T) {
	for _, tc := range []struct {
		acc  float64
		want string
	}{{1, MarkCorrect}, {0, MarkWrong}} {
		r := newDuelRig(t, 1, true)
		r.advance(Countdown)
		bot := r.m.seatOf(r.b)
		bot.botAccuracy = tc.acc
		r.now = bot.botAt
		r.m.onTimer(context.Background())
		if got := bot.answers[0].mark(); got != tc.want {
			t.Fatalf("accuracy %v gave %s", tc.acc, got)
		}
	}
}

func TestOutcomesAndRatedness(t *testing.T) {
	r := newDuelRig(t, 1, false)
	r.m.seats[0].score, r.m.seats[1].score = 50, 80
	r.m.endReason = "completed"
	if a, b := r.m.outcomes(); a != "lost" || b != "won" {
		t.Fatalf("completed %s/%s", a, b)
	}
	if !r.m.rated() {
		t.Fatal("completed ranked duel must be rated")
	}
	r.m.endReason, r.m.quitter = "forfeit", r.b
	if a, b := r.m.outcomes(); a != "won" || b != "lost" {
		t.Fatalf("forfeit %s/%s", a, b)
	}
	r.m.endReason = "both_disconnected"
	if r.m.rated() {
		t.Fatal("abandoned duel must not move rating")
	}
	r.m.endReason, r.m.mode = "completed", ModeFriend
	if r.m.rated() {
		t.Fatal("friend duel must not move rating")
	}
	r.m.mode = ModeBot
	if r.m.rated() {
		t.Fatal("bot duel must not move rating")
	}
}

func TestEndMarksDistinguishSkippedFromUnreached(t *testing.T) {
	r := newDuelRig(t, 3, false)
	r.advance(Countdown)
	r.answer(r.a, 0, true)
	a := r.m.seatOf(r.a)
	if got := a.marks(3); len(got) != 1 || got[0] != MarkCorrect {
		t.Fatalf("marks %v", got)
	}
	r.advance(RevealFor)
	// Question 1 is live but unanswered: not a mark yet.
	if got := a.marks(3); len(got) != 1 {
		t.Fatalf("live question counted: %v", got)
	}
}

func TestAssembleDuelSkipsQuestionsItCannotDeal(t *testing.T) {
	good1, noCorrect, missingRu, oneOption, good2 := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ids := []uuid.UUID{good1, noCorrect, missingRu, oneOption, good2}
	correct := map[uuid.UUID]uuid.UUID{good1: uuid.New(), missingRu: uuid.New(), oneOption: uuid.New(), good2: uuid.New()}
	two := []content.AnswerDTO{{ID: uuid.New().String()}, {ID: uuid.New().String()}}
	d := func(id uuid.UUID, ans []content.AnswerDTO) content.QuestionDetailDTO {
		return content.QuestionDetailDTO{
			QuestionDTO: content.QuestionDTO{ID: id.String(), Answers: ans},
			Explanation: &content.ExplanationDTO{},
		}
	}
	byLocale := map[string]map[uuid.UUID]content.QuestionDetailDTO{
		"uz-Latn": {good1: d(good1, two), noCorrect: d(noCorrect, two), missingRu: d(missingRu, two), oneOption: d(oneOption, two[:1]), good2: d(good2, two)},
		"ru":      {good1: d(good1, two), noCorrect: d(noCorrect, two), oneOption: d(oneOption, two[:1]), good2: d(good2, two)},
	}
	picked, payloads := assembleDuel(ids, correct, byLocale, 5)
	if len(picked) != 2 || picked[0] != good1 || picked[1] != good2 {
		t.Fatalf("picked %v", picked)
	}
	for loc, list := range payloads {
		if len(list) != 2 || list[0].ID != good1.String() || list[1].ID != good2.String() {
			t.Fatalf("%s payloads out of step with picks", loc)
		}
		if list[0].Explanation != nil {
			t.Fatalf("%s payload leaks the explanation", loc)
		}
	}
}

func TestClaimKeepsAPlayerInOneDuel(t *testing.T) {
	svc := &Service{Hub: NewHub(), matches: map[uuid.UUID]*Match{}}
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	ph, ok := svc.claim(a, b)
	if !ok {
		t.Fatal("first claim refused")
	}
	if _, ok := svc.claim(b, c); ok {
		t.Fatal("b claimed into a second duel")
	}
	if svc.Hub.InMatch(c) {
		t.Fatal("a refused claim left c marked in a match")
	}
	svc.release(ph, a, b)
	if svc.Hub.InMatch(a) || svc.Hub.InMatch(b) {
		t.Fatal("release did not free the players")
	}
	if _, ok := svc.claim(b, c); !ok {
		t.Fatal("released player could not be claimed again")
	}
}

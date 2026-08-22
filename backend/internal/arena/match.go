package arena

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Match is the single writer for one duel. All mutations happen on Run's goroutine.
type Match struct {
	svc         *Service
	id          uuid.UUID
	a, b        uuid.UUID
	localeA     string
	localeB     string
	questions   []uuid.UUID
	correct     map[uuid.UUID]uuid.UUID
	validAns    map[uuid.UUID]map[uuid.UUID]struct{}
	score       map[uuid.UUID]int
	correctN    map[uuid.UUID]int
	responseSum map[uuid.UUID]int64
	answers     map[uuid.UUID][]playerAnswer
	index       int
	deadline    time.Time
	phase       string // pending|active|reveal|finished
	endReason   string
	discSince   map[uuid.UUID]time.Time
	quitter     uuid.UUID

	inbox      chan matchEvent
	retimer    chan struct{}
	done       chan struct{}
	countdown  time.Duration
	qTime      time.Duration
	revealFor  time.Duration
	grace      time.Duration
	reconGrace time.Duration
}

type playerAnswer struct {
	answered   bool
	answerID   uuid.UUID
	correct    bool
	responseMs int64
	points     int
	at         time.Time
}

type matchEvent struct {
	kind      string
	profileID uuid.UUID
	index     int
	answerID  uuid.UUID
}

func NewMatch(
	svc *Service,
	id, a, b uuid.UUID,
	localeA, localeB string,
	questions []uuid.UUID,
	correct map[uuid.UUID]uuid.UUID,
) *Match {
	return &Match{
		svc:         svc,
		id:          id,
		a:           a,
		b:           b,
		localeA:     localeA,
		localeB:     localeB,
		questions:   questions,
		correct:     correct,
		validAns:    map[uuid.UUID]map[uuid.UUID]struct{}{},
		score:       map[uuid.UUID]int{a: 0, b: 0},
		correctN:    map[uuid.UUID]int{a: 0, b: 0},
		responseSum: map[uuid.UUID]int64{a: 0, b: 0},
		answers: map[uuid.UUID][]playerAnswer{
			a: make([]playerAnswer, len(questions)),
			b: make([]playerAnswer, len(questions)),
		},
		phase:      "pending",
		discSince:  map[uuid.UUID]time.Time{},
		inbox:      make(chan matchEvent, 64),
		retimer:    make(chan struct{}, 1),
		done:       make(chan struct{}),
		countdown:  3 * time.Second,
		qTime:      time.Duration(QuestionTimeSec) * time.Second,
		revealFor:  2500 * time.Millisecond,
		grace:      AnswerGrace,
		reconGrace: ReconnectGrace,
	}
}

func (m *Match) now() time.Time {
	if m.svc != nil && m.svc.Now != nil {
		return m.svc.Now()
	}
	return time.Now()
}

func (m *Match) kickTimer() {
	select {
	case m.retimer <- struct{}{}:
	default:
	}
}

func (m *Match) enqueue(ev matchEvent, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case m.inbox <- ev:
		return true
	case <-m.done:
		return false
	case <-timer.C:
		if m.svc != nil && m.svc.Log != nil {
			m.svc.Log.Error("arena match inbox saturated",
				zap.String("match_id", m.id.String()),
				zap.String("event", ev.kind),
			)
		}
		return false
	}
}

func (m *Match) SubmitAnswer(profileID uuid.UUID, index int, answerID uuid.UUID) bool {
	return m.enqueue(matchEvent{kind: "answer", profileID: profileID, index: index, answerID: answerID}, time.Second)
}

func (m *Match) NotifyDisconnect(profileID uuid.UUID) bool {
	return m.enqueue(matchEvent{kind: "disconnect", profileID: profileID}, time.Second)
}

func (m *Match) Rejoin(profileID uuid.UUID) bool {
	return m.enqueue(matchEvent{kind: "reconnect", profileID: profileID}, time.Second)
}

func (m *Match) Leave(profileID uuid.UUID) bool {
	return m.enqueue(matchEvent{kind: "leave", profileID: profileID}, time.Second)
}

func (m *Match) AbortShutdown(ctx context.Context) error {
	select {
	case m.inbox <- matchEvent{kind: "abort"}:
		return nil
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Match) Run() {
	defer close(m.done)
	next := time.NewTimer(m.countdown)
	defer next.Stop()

	// The loop itself must outlive every operation in it -- a match runs for
	// minutes -- so it keeps no context of its own. Each step takes a bounded
	// one instead (opContext), which is the granularity that actually matters:
	// a wedged query can cost this match one step, not the goroutine.
	step := func(fn func(context.Context)) {
		ctx, cancel := opContext()
		defer cancel()
		fn(ctx)
	}

	for m.phase != "finished" {
		select {
		case ev := <-m.inbox:
			step(func(ctx context.Context) { m.handle(ctx, ev) })
			if m.phase == "finished" {
				return
			}
		case <-m.retimer:
			m.resetTimer(next)
		case <-next.C:
			step(m.onTimer)
			if m.phase == "finished" {
				return
			}
			m.resetTimer(next)
		}
	}
}

func (m *Match) resetTimer(timer *time.Timer) {
	var d time.Duration
	switch m.phase {
	case "active":
		d = time.Until(m.deadline.Add(m.grace))
		if d < 0 {
			d = 0
		}
	case "reveal":
		d = m.revealFor
	case "pending":
		d = m.countdown
	default:
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

func (m *Match) onTimer(ctx context.Context) {
	switch m.phase {
	case "pending":
		m.startQuestion(ctx, 0)
	case "active":
		m.reveal(ctx)
	case "reveal":
		nextIdx := m.index + 1
		if nextIdx >= len(m.questions) {
			m.finish(ctx, "completed")
			return
		}
		m.startQuestion(ctx, nextIdx)
	}
}

func (m *Match) handle(ctx context.Context, ev matchEvent) {
	switch ev.kind {
	case "answer":
		m.onAnswer(ctx, ev)
	case "disconnect":
		m.discSince[ev.profileID] = m.now()
		opp := m.opponentOf(ev.profileID)
		_ = m.svc.sendJSON(opp, "opponent.status", OpponentStatusData{State: "disconnected"})
		if bothDisconnected(m) {
			m.finish(ctx, "both_disconnected")
			return
		}
		pid := ev.profileID
		go func() {
			time.Sleep(m.reconGrace)
			m.enqueue(matchEvent{kind: "forfeit_check", profileID: pid}, time.Second)
		}()
	case "forfeit_check":
		if _, still := m.discSince[ev.profileID]; still && m.phase != "finished" {
			m.quitter = ev.profileID
			m.finish(ctx, "forfeit")
		}
	case "reconnect":
		delete(m.discSince, ev.profileID)
		opp := m.opponentOf(ev.profileID)
		_ = m.svc.sendJSON(opp, "opponent.status", OpponentStatusData{State: "connected"})
		_ = m.svc.sendJSON(ev.profileID, "match.state", m.stateSnapshot(ev.profileID))
	case "leave":
		m.quitter = ev.profileID
		m.finish(ctx, "forfeit")
	case "abort":
		m.finish(ctx, "server_shutdown")
	}
}

func (m *Match) onAnswer(ctx context.Context, ev matchEvent) {
	if m.phase != "active" {
		_ = m.svc.sendJSON(ev.profileID, "error", ErrorData{Code: "wrong_question", Message: "not active"})
		return
	}
	if ev.index != m.index {
		_ = m.svc.sendJSON(ev.profileID, "error", ErrorData{Code: "wrong_question", Message: "index mismatch"})
		return
	}
	if m.answers[ev.profileID][m.index].answered {
		_ = m.svc.sendJSON(ev.profileID, "error", ErrorData{Code: "already_answered", Message: "already answered"})
		return
	}
	now := m.now()
	if now.After(m.deadline.Add(m.grace)) {
		_ = m.svc.sendJSON(ev.profileID, "error", ErrorData{Code: "too_late", Message: "deadline passed"})
		return
	}
	qid := m.questions[m.index]
	if set := m.validAns[qid]; set != nil {
		if _, ok := set[ev.answerID]; !ok {
			_ = m.svc.sendJSON(ev.profileID, "error", ErrorData{Code: "invalid_answer", Message: "answer not for question"})
			return
		}
	}
	resp := now.Sub(m.deadline.Add(-m.qTime)).Milliseconds()
	if resp < 0 {
		resp = 0
	}
	window := m.qTime.Milliseconds()
	if resp > window {
		resp = window
	}
	ok := m.correct[qid] == ev.answerID
	pts := AnswerPoints(ok, resp, window)
	m.answers[ev.profileID][m.index] = playerAnswer{
		answered: true, answerID: ev.answerID, correct: ok, responseMs: resp, points: pts, at: now,
	}
	m.score[ev.profileID] += pts
	if ok {
		m.correctN[ev.profileID]++
	}
	m.responseSum[ev.profileID] += resp
	_ = m.svc.sendJSON(ev.profileID, "answer.ack", AnswerAckData{Index: m.index, ResponseMs: resp})
	if m.answers[m.a][m.index].answered && m.answers[m.b][m.index].answered {
		m.reveal(ctx)
		m.kickTimer()
	}
}

func (m *Match) startQuestion(ctx context.Context, index int) {
	m.index = index
	m.phase = "active"
	m.deadline = m.now().Add(m.qTime)
	qid := m.questions[index]
	m.pushQuestion(ctx, m.a, m.localeA, qid, index)
	m.pushQuestion(ctx, m.b, m.localeB, qid, index)
}

func (m *Match) pushQuestion(ctx context.Context, profileID uuid.UUID, locale string, qid uuid.UUID, index int) {
	var detail any
	if m.svc.Content != nil {
		// LoadQuestionDetail's bool is fallbackUsed, NOT success. Gating on it
		// dropped every native-locale question to {"id":...} with no answers,
		// which crashed the Arena UI (error boundary) and ended matches as
		// both_disconnected within the first question tick.
		d, _, err := m.svc.Content.LoadQuestionDetail(ctx, qid, locale)
		if err == nil && len(d.Answers) > 0 {
			d.Explanation = nil
			detail = d
			if m.validAns[qid] == nil {
				set := map[uuid.UUID]struct{}{}
				for _, a := range d.Answers {
					if id, err := uuid.Parse(a.ID); err == nil {
						set[id] = struct{}{}
					}
				}
				m.validAns[qid] = set
			}
		} else if err != nil && m.svc.Log != nil {
			m.svc.Log.Warn("arena question load failed",
				zap.String("question_id", qid.String()),
				zap.String("locale", locale),
				zap.Error(err),
			)
		}
	}
	if detail == nil {
		// Prefer an explicit error frame over a half-empty question that the
		// client cannot render. Match continues on the timer so both players
		// stay in sync; answers will score as unanswered for this index.
		_ = m.svc.sendJSON(profileID, "error", ErrorData{
			Code: "question_unavailable", Message: "question payload unavailable",
		})
		detail = map[string]any{
			"id": qid.String(), "text": "", "answers": []any{},
		}
	}
	_ = m.svc.sendJSON(profileID, "question", QuestionData{
		Index: index, Total: len(m.questions),
		DeadlineMs: ms(m.deadline), ServerTimeMs: ms(m.now()),
		Question: detail,
	})
}

func (m *Match) reveal(ctx context.Context) {
	if m.phase != "active" {
		return
	}
	m.phase = "reveal"
	qid := m.questions[m.index]
	correctID := m.correct[qid]
	for _, pid := range []uuid.UUID{m.a, m.b} {
		opp := m.opponentOf(pid)
		_ = m.svc.sendJSON(pid, "question.result", QuestionResultData{
			Index: m.index, CorrectAnswerID: correctID,
			You: PlayerRoundData{
				Answered: m.answers[pid][m.index].answered,
				Correct:  m.answers[pid][m.index].correct,
				Points:   m.answers[pid][m.index].points,
			},
			Opponent: PlayerRoundData{
				Answered: m.answers[opp][m.index].answered,
				Correct:  m.answers[opp][m.index].correct,
				Points:   m.answers[opp][m.index].points,
			},
			Score: struct {
				You      int `json:"you"`
				Opponent int `json:"opponent"`
			}{You: m.score[pid], Opponent: m.score[opp]},
		})
	}
	_ = ctx
}

func (m *Match) finish(ctx context.Context, reason string) {
	if m.phase == "finished" {
		return
	}
	m.phase = "finished"
	m.endReason = reason
	outA, outB := OutcomeFromScores(m.score[m.a], m.score[m.b])
	switch reason {
	case "both_disconnected", "server_shutdown":
		outA, outB = "draw", "draw"
	case "forfeit":
		switch m.quitter {
		case m.a:
			outA, outB = "lost", "won"
		case m.b:
			outA, outB = "won", "lost"
		}
	}
	da, db := 0, 0
	ra, rb := 1000, 1000
	backoff := 100 * time.Millisecond
	for {
		var persistErr error
		if reason != "both_disconnected" && reason != "server_shutdown" && m.svc.Rating != nil {
			// Forfeit: quitter scores as loss for ELO (score 0 vs opponent+1).
			scoreA, scoreB := m.score[m.a], m.score[m.b]
			if reason == "forfeit" {
				switch m.quitter {
				case m.a:
					scoreA, scoreB = 0, 1
				case m.b:
					scoreA, scoreB = 1, 0
				}
			}
			da, db, persistErr = m.svc.Rating.ApplyResult(ctx, m.id, m.a, m.b, scoreA, scoreB)
			if persistErr == nil {
				ra, persistErr = m.svc.Rating.Rating(ctx, m.a)
			}
			if persistErr == nil {
				rb, persistErr = m.svc.Rating.Rating(ctx, m.b)
			}
		}
		if persistErr == nil {
			persistErr = m.svc.FinishPersist(ctx, m, da, db)
		}
		if persistErr == nil {
			break
		}
		m.svc.Log.Error("arena finish persistence failed; retrying",
			zap.String("match_id", m.id.String()),
			zap.Error(persistErr),
		)
		time.Sleep(backoff)
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
	_ = m.svc.sendJSON(m.a, "match.end", MatchEndData{
		MatchID: m.id, Outcome: outA, Reason: reason, RatingDelta: da,
		Medal: MedalForRating(ra),
		Score: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.score[m.a], m.score[m.b]},
		Correct: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.correctN[m.a], m.correctN[m.b]},
	})
	_ = m.svc.sendJSON(m.b, "match.end", MatchEndData{
		MatchID: m.id, Outcome: outB, Reason: reason, RatingDelta: db,
		Medal: MedalForRating(rb),
		Score: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.score[m.b], m.score[m.a]},
		Correct: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.correctN[m.b], m.correctN[m.a]},
	})
}

func (m *Match) opponentOf(id uuid.UUID) uuid.UUID {
	if id == m.a {
		return m.b
	}
	return m.a
}

func bothDisconnected(m *Match) bool {
	_, aGone := m.discSince[m.a]
	_, bGone := m.discSince[m.b]
	return aGone && bGone
}

func (m *Match) stateSnapshot(forProfile uuid.UUID) MatchStateData {
	opp := m.opponentOf(forProfile)
	return MatchStateData{
		MatchID:    m.id,
		Index:      m.index,
		Phase:      m.phase,
		DeadlineMs: ms(m.deadline),
		Score: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.score[forProfile], m.score[opp]},
		Correct: struct {
			You      int `json:"you"`
			Opponent int `json:"opponent"`
		}{m.correctN[forProfile], m.correctN[opp]},
	}
}

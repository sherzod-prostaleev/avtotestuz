package arena

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/content"
)

// Seat states. A seat walks active → reveal → active … → done on its own
// clock; the two seats of a match are never in lockstep.
const (
	seatActive = "active"
	seatReveal = "reveal"
	seatDone   = "done"
)

// Match is the single writer for one duel. All mutations happen on Run's goroutine.
type Match struct {
	svc       *Service
	id        uuid.UUID
	mode      string
	questions []uuid.UUID
	// payloads[locale][i] is question i as that locale's reader sees it. Loaded
	// before the match exists (see Service.loadPayloads), so dealing the next
	// question never waits on the database.
	payloads map[string][]content.QuestionDetailDTO
	correct  map[uuid.UUID]uuid.UUID
	validAns map[uuid.UUID]map[uuid.UUID]struct{}
	seats    [2]*seat
	// a and b are seats[0].id and seats[1].id; kept as fields because the
	// persistence code and its tests address players by them.
	a, b      uuid.UUID
	phase     string // pending|active|finished
	startAt   time.Time
	endReason string
	quitter   uuid.UUID

	inbox      chan matchEvent
	retimer    chan struct{}
	done       chan struct{}
	countdown  time.Duration
	qTime      time.Duration
	revealFor  time.Duration
	grace      time.Duration
	reconGrace time.Duration
	rng        *rand.Rand
}

// seat is one player's private progress through the shared question list.
type seat struct {
	id          uuid.UUID
	locale      string
	card        PlayerCard
	bot         bool
	botAccuracy float64

	index    int    // question the seat is on; == len(questions) once done
	state    string // active|reveal|done (meaningless while match is pending)
	deadline time.Time
	nextAt   time.Time
	botAt    time.Time
	last     *AnswerResultData

	answers     []playerAnswer
	score       int
	correctN    int
	responseSum int64

	disconnected bool
	discGen      int
}

type playerAnswer struct {
	answered   bool
	answerID   uuid.UUID
	correct    bool
	responseMs int64
	points     int
	at         time.Time
}

func (p playerAnswer) mark() string {
	switch {
	case !p.answered:
		return MarkSkipped
	case p.correct:
		return MarkCorrect
	default:
		return MarkWrong
	}
}

type matchEvent struct {
	kind      string
	profileID uuid.UUID
	index     int
	answerID  uuid.UUID
	gen       int
}

// SeatSpec describes one participant when a match is created.
type SeatSpec struct {
	ID     uuid.UUID
	Locale string
	Card   PlayerCard
	Bot    bool
}

func NewMatch(
	svc *Service,
	id uuid.UUID,
	mode string,
	a, b SeatSpec,
	questions []uuid.UUID,
	payloads map[string][]content.QuestionDetailDTO,
	correct map[uuid.UUID]uuid.UUID,
) *Match {
	m := &Match{
		svc:        svc,
		id:         id,
		mode:       mode,
		questions:  questions,
		payloads:   payloads,
		correct:    correct,
		validAns:   map[uuid.UUID]map[uuid.UUID]struct{}{},
		a:          a.ID,
		b:          b.ID,
		phase:      "pending",
		inbox:      make(chan matchEvent, 64),
		retimer:    make(chan struct{}, 1),
		done:       make(chan struct{}),
		countdown:  Countdown,
		qTime:      time.Duration(QuestionTimeSec) * time.Second,
		revealFor:  RevealFor,
		grace:      AnswerGrace,
		reconGrace: ReconnectGrace,
		rng:        rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
	for i, spec := range []SeatSpec{a, b} {
		s := &seat{
			id:      spec.ID,
			locale:  spec.Locale,
			card:    spec.Card,
			bot:     spec.Bot,
			answers: make([]playerAnswer, len(questions)),
		}
		m.seats[i] = s
	}
	for _, s := range m.seats {
		if s.bot {
			// The bot plays to its human's strength, not to a fixed level.
			s.botAccuracy = BotAccuracy(m.other(s).card.Rating)
		}
	}
	for _, byLocale := range payloads {
		for _, q := range byLocale {
			qid, err := uuid.Parse(q.ID)
			if err != nil || m.validAns[qid] != nil {
				continue
			}
			set := map[uuid.UUID]struct{}{}
			for _, ans := range q.Answers {
				if aid, err := uuid.Parse(ans.ID); err == nil {
					set[aid] = struct{}{}
				}
			}
			m.validAns[qid] = set
		}
	}
	return m
}

func (m *Match) now() time.Time {
	if m.svc != nil && m.svc.Now != nil {
		return m.svc.Now()
	}
	return time.Now()
}

func (m *Match) seatOf(id uuid.UUID) *seat {
	for _, s := range m.seats {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (m *Match) other(s *seat) *seat {
	if m.seats[0] == s {
		return m.seats[1]
	}
	return m.seats[0]
}

func (m *Match) send(s *seat, t string, d any) {
	if s.bot || m.svc == nil {
		return
	}
	_ = m.svc.sendJSON(s.id, t, d)
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

// Rejoin re-attaches a player's (new) socket and resends the full state.
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

// Start stamps the countdown so match.found can quote the same instant the
// loop will act on. Must be called once, before Run.
func (m *Match) Start() time.Time {
	m.startAt = m.now().Add(m.countdown)
	return m.startAt
}

func (m *Match) Run() {
	defer close(m.done)
	if m.startAt.IsZero() {
		m.Start()
	}
	next := time.NewTimer(time.Until(m.startAt))
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
			m.resetTimer(next)
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

// nextWake is the earliest instant any seat needs the loop to act.
func (m *Match) nextWake() (time.Time, bool) {
	switch m.phase {
	case "pending":
		return m.startAt, true
	case "active":
	default:
		return time.Time{}, false
	}
	var at time.Time
	for _, s := range m.seats {
		var t time.Time
		switch s.state {
		case seatActive:
			if s.bot {
				t = s.botAt
			} else {
				t = s.deadline.Add(m.grace)
			}
		case seatReveal:
			t = s.nextAt
		default:
			continue
		}
		if at.IsZero() || t.Before(at) {
			at = t
		}
	}
	return at, !at.IsZero()
}

func (m *Match) resetTimer(timer *time.Timer) {
	at, ok := m.nextWake()
	if !ok {
		return
	}
	d := time.Until(at)
	if m.svc != nil && m.svc.Now != nil {
		d = at.Sub(m.svc.Now())
	}
	if d < 0 {
		d = 0
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

// onTimer advances every seat whose moment has come. Seats are independent,
// so both are checked on every tick.
func (m *Match) onTimer(ctx context.Context) {
	now := m.now()
	if m.phase == "pending" {
		if now.Before(m.startAt) {
			return
		}
		m.phase = "active"
		for _, s := range m.seats {
			m.startQuestion(s, 0)
		}
		return
	}
	if m.phase != "active" {
		return
	}
	for _, s := range m.seats {
		switch s.state {
		case seatActive:
			if s.bot && !now.Before(s.botAt) {
				m.botAnswer(s)
			} else if !s.bot && !now.Before(s.deadline.Add(m.grace)) {
				m.resolve(s, playerAnswer{})
			}
		case seatReveal:
			if !now.Before(s.nextAt) {
				m.advance(ctx, s)
			}
		}
		if m.phase == "finished" {
			return
		}
	}
}

func (m *Match) handle(ctx context.Context, ev matchEvent) {
	if ev.kind == "abort" {
		m.finish(ctx, "server_shutdown")
		return
	}
	s := m.seatOf(ev.profileID)
	if s == nil || s.bot {
		return
	}
	switch ev.kind {
	case "answer":
		m.onAnswer(ev, s)
	case "disconnect":
		if s.disconnected {
			return
		}
		s.disconnected = true
		s.discGen++
		opp := m.other(s)
		m.send(opp, "opponent.status", OpponentStatusData{State: "disconnected"})
		if opp.disconnected && !opp.bot {
			m.finish(ctx, "both_disconnected")
			return
		}
		pid, gen := s.id, s.discGen
		grace := m.reconGrace
		go func() {
			time.Sleep(grace)
			m.enqueue(matchEvent{kind: "forfeit_check", profileID: pid, gen: gen}, time.Second)
		}()
	case "forfeit_check":
		// gen pins the check to the disconnect that scheduled it: a player who
		// dropped, came back and dropped again gets a fresh full grace.
		if s.disconnected && s.discGen == ev.gen && m.phase != "finished" {
			m.quitter = s.id
			m.finish(ctx, "forfeit")
		}
	case "reconnect":
		if s.disconnected {
			s.disconnected = false
			m.send(m.other(s), "opponent.status", OpponentStatusData{State: "connected"})
		}
		m.send(s, "match.state", m.stateSnapshot(s))
	case "leave":
		m.quitter = s.id
		m.finish(ctx, "forfeit")
	}
}

func (m *Match) onAnswer(ev matchEvent, s *seat) {
	if m.phase != "active" || s.state != seatActive {
		m.send(s, "error", ErrorData{Code: "wrong_question", Message: "not active"})
		return
	}
	if ev.index != s.index {
		m.send(s, "error", ErrorData{Code: "wrong_question", Message: "index mismatch"})
		return
	}
	now := m.now()
	if now.After(s.deadline.Add(m.grace)) {
		m.send(s, "error", ErrorData{Code: "too_late", Message: "deadline passed"})
		return
	}
	qid := m.questions[s.index]
	if set := m.validAns[qid]; set != nil {
		if _, ok := set[ev.answerID]; !ok {
			m.send(s, "error", ErrorData{Code: "invalid_answer", Message: "answer not for question"})
			return
		}
	}
	resp := now.Sub(s.deadline.Add(-m.qTime)).Milliseconds()
	m.resolve(s, m.scoreAnswer(qid, ev.answerID, resp, now))
}

func (m *Match) scoreAnswer(qid, answerID uuid.UUID, resp int64, at time.Time) playerAnswer {
	window := m.qTime.Milliseconds()
	if resp < 0 {
		resp = 0
	}
	if resp > window {
		resp = window
	}
	ok := m.correct[qid] == answerID
	return playerAnswer{
		answered: true, answerID: answerID, correct: ok, responseMs: resp,
		points: AnswerPoints(ok, resp, window), at: at,
	}
}

// botAnswer plays the bot's turn: right with its accuracy, otherwise a
// random wrong option.
func (m *Match) botAnswer(s *seat) {
	qid := m.questions[s.index]
	correctID := m.correct[qid]
	pick := correctID
	if m.rng.Float64() >= s.botAccuracy {
		var wrong []uuid.UUID
		for aid := range m.validAns[qid] {
			if aid != correctID {
				wrong = append(wrong, aid)
			}
		}
		if len(wrong) > 0 {
			pick = wrong[m.rng.IntN(len(wrong))]
		}
	}
	now := m.now()
	resp := now.Sub(s.deadline.Add(-m.qTime)).Milliseconds()
	m.resolve(s, m.scoreAnswer(qid, pick, resp, now))
}

// resolve records the seat's verdict on its current question, shows it to the
// player and moves the opponent's view of their progress.
func (m *Match) resolve(s *seat, ans playerAnswer) {
	now := m.now()
	i := s.index
	s.answers[i] = ans
	s.score += ans.points
	if ans.correct {
		s.correctN++
	}
	if ans.answered {
		s.responseSum += ans.responseMs
	}
	s.state = seatReveal
	s.nextAt = now.Add(m.revealFor)
	res := &AnswerResultData{
		Index:           i,
		Answered:        ans.answered,
		Correct:         ans.correct,
		CorrectAnswerID: m.correct[m.questions[i]],
		Points:          ans.points,
		Score:           s.score,
		ResponseMs:      ans.responseMs,
		NextInMs:        m.revealFor.Milliseconds(),
		Last:            i == len(m.questions)-1,
	}
	if ans.answered {
		aid := ans.answerID
		res.AnswerID = &aid
	}
	s.last = res
	m.send(s, "answer.result", res)
	m.send(m.other(s), "opponent.progress", OpponentProgressData{
		Answered: i + 1, Total: len(m.questions),
	})
	m.kickTimer()
}

// advance ends a seat's reveal: the next question, or the finish line.
func (m *Match) advance(ctx context.Context, s *seat) {
	s.index++
	if s.index < len(m.questions) {
		m.startQuestion(s, s.index)
		return
	}
	s.state = seatDone
	opp := m.other(s)
	m.send(opp, "opponent.progress", OpponentProgressData{
		Answered: len(m.questions), Total: len(m.questions), Finished: true,
	})
	if opp.state == seatDone {
		m.finish(ctx, "completed")
		return
	}
	m.send(s, "match.waiting", MatchWaitingData{Score: s.score, Correct: s.correctN})
}

func (m *Match) startQuestion(s *seat, index int) {
	now := m.now()
	s.index = index
	s.state = seatActive
	s.last = nil
	s.deadline = now.Add(m.qTime)
	if s.bot {
		s.botAt = now.Add(BotDelay(m.rng.Float64(), m.qTime))
		return
	}
	m.send(s, "question", QuestionData{
		Index: index, Total: len(m.questions),
		DeadlineMs: ms(s.deadline), ServerTimeMs: ms(now),
		Question: m.payloadFor(s, index),
	})
}

func (m *Match) payloadFor(s *seat, index int) any {
	if list := m.payloads[s.locale]; index < len(list) {
		return list[index]
	}
	for _, list := range m.payloads {
		if index < len(list) {
			return list[index]
		}
	}
	return nil
}

// outcomes is the verdict for seats[0] and seats[1].
func (m *Match) outcomes() (string, string) {
	a, b := m.seats[0], m.seats[1]
	switch m.endReason {
	case "both_disconnected", "server_shutdown":
		return "draw", "draw"
	case "forfeit":
		switch m.quitter {
		case a.id:
			return "lost", "won"
		case b.id:
			return "won", "lost"
		}
	}
	return OutcomeFromScores(a.score, b.score)
}

func (m *Match) rated() bool {
	if m.mode != ModeRanked {
		return false
	}
	return m.endReason == "completed" || m.endReason == "forfeit"
}

func (m *Match) finish(ctx context.Context, reason string) {
	if m.phase == "finished" {
		return
	}
	m.phase = "finished"
	m.endReason = reason
	outA, outB := m.outcomes()
	a, b := m.seats[0], m.seats[1]
	da, db := 0, 0
	ra, rb := a.card.Rating, b.card.Rating
	backoff := 100 * time.Millisecond
	for {
		var persistErr error
		if m.rated() && m.svc.Rating != nil {
			// Forfeit: quitter scores as loss for ELO (score 0 vs opponent+1).
			scoreA, scoreB := a.score, b.score
			if reason == "forfeit" {
				switch m.quitter {
				case a.id:
					scoreA, scoreB = 0, 1
				case b.id:
					scoreA, scoreB = 1, 0
				}
			}
			da, db, persistErr = m.svc.Rating.ApplyResult(ctx, m.id, a.id, b.id, scoreA, scoreB)
			if persistErr == nil {
				ra, persistErr = m.svc.Rating.Rating(ctx, a.id)
			}
			if persistErr == nil {
				rb, persistErr = m.svc.Rating.Rating(ctx, b.id)
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
	m.send(a, "match.end", m.endFor(a, b, outA, ra, da))
	m.send(b, "match.end", m.endFor(b, a, outB, rb, db))
}

func (m *Match) endFor(s, opp *seat, outcome string, ratingAfter, delta int) MatchEndData {
	return MatchEndData{
		MatchID: m.id, Mode: m.mode, Outcome: outcome, Reason: m.endReason,
		Score:        ScorePair{You: s.score, Opponent: opp.score},
		Correct:      ScorePair{You: s.correctN, Opponent: opp.correctN},
		Marks:        MarksPair{You: s.marks(len(m.questions)), Opponent: opp.marks(len(m.questions))},
		Total:        len(m.questions),
		Rated:        m.rated(),
		RatingBefore: ratingAfter - delta,
		RatingAfter:  ratingAfter,
		RatingDelta:  delta,
		Medal:        MedalForRating(ratingAfter),
	}
}

// marks lists the verdicts a seat has reached. A question the seat never got
// to (forfeit, disconnect) is left out, so the client can tell "skipped by
// the clock" from "never reached".
func (s *seat) marks(total int) []string {
	n := s.resolvedCount()
	if n > total {
		n = total
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s.answers[i].mark())
	}
	return out
}

// resolvedCount is how many questions have a verdict for this seat.
func (s *seat) resolvedCount() int {
	switch s.state {
	case seatReveal:
		return s.index + 1
	case seatDone:
		return len(s.answers)
	default:
		return s.index
	}
}

func (m *Match) stateSnapshot(s *seat) MatchStateData {
	opp := m.other(s)
	out := MatchStateData{
		MatchID:        m.id,
		Mode:           m.mode,
		Index:          s.index,
		Total:          len(m.questions),
		QuestionTimeMs: m.qTime.Milliseconds(),
		StartsAtMs:     ms(m.startAt),
		ServerTimeMs:   ms(m.now()),
		You:            s.card,
		Marks:          s.marks(len(m.questions)),
		Score:          s.score,
		Opponent: OpponentState{
			PlayerCard: opp.card,
			Answered:   opp.resolvedCount(),
			Finished:   opp.state == seatDone,
			Connected:  !opp.disconnected,
		},
	}
	if m.phase == "pending" {
		out.Phase = "countdown"
		return out
	}
	switch s.state {
	case seatActive:
		out.Phase = "question"
		out.DeadlineMs = ms(s.deadline)
		out.Question = m.payloadFor(s, s.index)
	case seatReveal:
		out.Phase = "reveal"
		out.Question = m.payloadFor(s, s.index)
		out.LastResult = s.last
	default:
		out.Phase = "waiting"
	}
	return out
}

package arena

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/flags"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
)

//go:embed arena_join.lua
var arenaJoinLua string

var (
	ErrRequiresVIP     = errors.New("vip_required")
	ErrAlreadyQueued   = errors.New("already_queued")
	ErrAlreadyInMatch  = errors.New("already_in_match")
	ErrTicketInvalid   = errors.New("ticket_invalid")
	ErrInviteInvalid   = errors.New("invite_invalid")
	ErrFeatureDisabled = errors.New("feature_disabled")
)

// RatingProvider supplies ratings for matchmaking (M4-04 fills real ELO).
type RatingProvider interface {
	Rating(ctx context.Context, profileID uuid.UUID) (int, error)
	ApplyResult(ctx context.Context, matchID uuid.UUID, a, b uuid.UUID, scoreA, scoreB int) (deltaA, deltaB int, err error)
}

// FixedRating is the M4-03 stub (everyone starts at 1000).
type FixedRating struct{ Value int }

func (f FixedRating) Rating(context.Context, uuid.UUID) (int, error) {
	if f.Value == 0 {
		return 1000, nil
	}
	return f.Value, nil
}
func (f FixedRating) ApplyResult(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, int) (int, int, error) {
	return 0, 0, nil
}

// EloStore persists and updates arena ratings in Redis (M4-04).
type EloStore struct {
	R *redis.Client
	K float64
}

func (e EloStore) key(id uuid.UUID) string { return "arena:rating:" + id.String() }

func (e EloStore) Rating(ctx context.Context, profileID uuid.UUID) (int, error) {
	v, err := e.R.Get(ctx, e.key(profileID)).Int()
	if err == redis.Nil {
		return 1000, nil
	}
	return v, err
}

func (e EloStore) ApplyResult(ctx context.Context, matchID uuid.UUID, a, b uuid.UUID, scoreA, scoreB int) (int, int, error) {
	return e.applyResult(ctx, matchID, a, b, scoreA, scoreB)
}

type eloAppliedResult struct {
	DeltaA int `json:"delta_a"`
	DeltaB int `json:"delta_b"`
}

// applyResult updates both ratings in one optimistic Redis transaction. A
// match-scoped result key makes retries idempotent, including the ambiguous
// case where EXEC succeeded but the network response was lost.
func (e EloStore) applyResult(ctx context.Context, matchID, a, b uuid.UUID, scoreA, scoreB int) (int, int, error) {
	resultKey := "arena:rating:match:" + matchID.String()
	var applied eloAppliedResult
	for attempts := 0; attempts < 8; attempts++ {
		err := e.R.Watch(ctx, func(tx *redis.Tx) error {
			if raw, err := tx.Get(ctx, resultKey).Bytes(); err == nil {
				return json.Unmarshal(raw, &applied)
			} else if err != redis.Nil {
				return err
			}
			ra, err := redisRating(ctx, tx, e.key(a))
			if err != nil {
				return err
			}
			rb, err := redisRating(ctx, tx, e.key(b))
			if err != nil {
				return err
			}
			var sa, sb float64
			switch {
			case scoreA > scoreB:
				sa, sb = 1, 0
			case scoreA < scoreB:
				sa, sb = 0, 1
			default:
				sa, sb = 0.5, 0.5
			}
			k := e.K
			if k <= 0 {
				k = 32
			}
			applied = eloAppliedResult{
				DeltaA: EloDelta(ra, rb, sa, k),
				DeltaB: EloDelta(rb, ra, sb, k),
			}
			payload, err := json.Marshal(applied)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, e.key(a), ra+applied.DeltaA, 0)
				pipe.Set(ctx, e.key(b), rb+applied.DeltaB, 0)
				pipe.Set(ctx, resultKey, payload, 90*24*time.Hour)
				return nil
			})
			return err
		}, resultKey, e.key(a), e.key(b))
		if err == nil {
			return applied.DeltaA, applied.DeltaB, nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return 0, 0, err
		}
	}
	return 0, 0, redis.TxFailedErr
}

func redisRating(ctx context.Context, tx *redis.Tx, key string) (int, error) {
	v, err := tx.Get(ctx, key).Int()
	if err == redis.Nil {
		return 1000, nil
	}
	return v, err
}

// QuestionLoader loads public question payloads for live duels.
// The bool from LoadQuestionDetail is locale fallbackUsed — never treat it as ok.
type QuestionLoader interface {
	LoadQuestionDetail(ctx context.Context, id uuid.UUID, loc string) (content.QuestionDetailDTO, bool, error)
}

// Service owns tickets, matchmaking, and match lifecycle.
type Service struct {
	Q        *sqlc.Queries
	Pool     *pgxpool.Pool
	R        *redis.Client
	Lim      auth.Limiter
	Billing  billing.Service
	Content  QuestionLoader
	Learning *learning.Service
	Progress *progress.Service
	Rating   RatingProvider
	Hub      *Hub
	Log      *zap.Logger
	Now      func() time.Time
	Instance string

	mu      sync.Mutex
	matches map[uuid.UUID]*Match
	locales sync.Map // profileID string → locale
}

func NewService(
	q *sqlc.Queries,
	pool *pgxpool.Pool,
	r *redis.Client,
	billingSvc billing.Service,
	contentH QuestionLoader,
	learningSvc *learning.Service,
	progressSvc *progress.Service,
	log *zap.Logger,
) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	host, _ := os.Hostname()
	return &Service{
		Q:        q,
		Pool:     pool,
		R:        r,
		Lim:      auth.Limiter{R: r},
		Billing:  billingSvc,
		Content:  contentH,
		Learning: learningSvc,
		Progress: progressSvc,
		Rating:   EloStore{R: r, K: 32},
		Hub:      NewHub(),
		Log:      log,
		Now:      time.Now,
		Instance: host,
		matches:  make(map[uuid.UUID]*Match),
	}
}

func (s *Service) ticketKey(tok string) string { return "arena:ticket:" + tok }

func (s *Service) MintTicket(ctx context.Context, profileID uuid.UUID) (string, int, error) {
	enabled, err := flags.Bool(ctx, s.Pool, flags.KeyArenaEnabled, true)
	if err != nil {
		return "", 0, err
	}
	if !enabled {
		return "", 0, ErrFeatureDisabled
	}
	ok, err := s.Lim.Allow(ctx, "arena:rl:ticket:"+profileID.String(), 30, time.Minute)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, fmt.Errorf("rate_limited")
	}
	tok, err := auth.NewRefreshToken()
	if err != nil {
		return "", 0, err
	}
	if err := s.R.Set(ctx, s.ticketKey(tok), profileID.String(), TicketTTL).Err(); err != nil {
		return "", 0, err
	}
	return tok, int(TicketTTL.Seconds()), nil
}

func (s *Service) RedeemTicket(ctx context.Context, tok string) (uuid.UUID, error) {
	val, err := s.R.GetDel(ctx, s.ticketKey(tok)).Result()
	if err == redis.Nil || val == "" {
		return uuid.Nil, ErrTicketInvalid
	}
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, ErrTicketInvalid
	}
	return id, nil
}

func (s *Service) requireVIP(ctx context.Context, profileID uuid.UUID) error {
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return err
	}
	if !active {
		return ErrRequiresVIP
	}
	return nil
}

func (s *Service) JoinQueue(ctx context.Context, profileID uuid.UUID, locale string) error {
	if err := s.requireVIP(ctx, profileID); err != nil {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "vip_required", Message: "VIP required"})
		return err
	}
	ok, err := s.Lim.Allow(ctx, "arena:rl:join:"+profileID.String(), 20, 5*time.Minute)
	if err != nil {
		return err
	}
	if !ok {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "rate_limited", Message: "join rate limited"})
		return fmt.Errorf("rate_limited")
	}
	if s.Hub.InMatch(profileID) {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "already_in_match", Message: "already in match"})
		return ErrAlreadyInMatch
	}
	if v, _ := s.R.Exists(ctx, "arena:match:"+profileID.String()).Result(); v > 0 {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "already_in_match", Message: "already in match"})
		return ErrAlreadyInMatch
	}
	if v, _ := s.R.Exists(ctx, "arena:queued:"+profileID.String()).Result(); v > 0 {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "already_queued", Message: "already queued"})
		return ErrAlreadyQueued
	}
	locale = normalizeLocale(locale)
	s.locales.Store(profileID.String(), locale)

	rating, err := s.Rating.Rating(ctx, profileID)
	if err != nil {
		return err
	}
	bucket := Bucket(rating)
	now := s.Now().UTC().UnixMilli()
	ownKey := fmt.Sprintf("arena:q:%d", bucket)
	keys := []string{ownKey}
	for _, b := range SearchBuckets(bucket, 0)[1:] {
		keys = append(keys, fmt.Sprintf("arena:q:%d", b))
	}
	res, err := s.R.Eval(ctx, arenaJoinLua, keys, profileID.String(), now, ownKey).Result()
	if err != nil {
		return err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 1 {
		return fmt.Errorf("bad_join_result")
	}
	kind, _ := arr[0].(string)
	switch kind {
	case "queued":
		_ = s.sendJSON(profileID, "queue.joined", QueueJoinedData{
			QueuedAtMs: now,
			TimeoutMs:  QueueTimeout.Milliseconds(),
		})
		go s.watchQueueTimeout(profileID, bucket, now)
		return nil
	case "paired":
		oppStr, _ := arr[1].(string)
		oppID, err := uuid.Parse(oppStr)
		if err != nil {
			return err
		}
		if !s.Hub.Alive(oppID) {
			// Stale opponent — re-queue self
			_ = s.R.Del(ctx, "arena:queued:"+oppID.String()).Err()
			return s.JoinQueue(ctx, profileID, locale)
		}
		return s.startMatch(ctx, profileID, oppID)
	default:
		return fmt.Errorf("unknown_join_kind")
	}
}

func (s *Service) LeaveQueue(ctx context.Context, profileID uuid.UUID) error {
	bucket, err := s.R.Get(ctx, "arena:queued:"+profileID.String()).Result()
	if err == redis.Nil {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "not_queued", Message: "not queued"})
		return nil
	}
	if err != nil {
		return err
	}
	_ = s.R.ZRem(ctx, "arena:q:"+bucket, profileID.String()).Err()
	_ = s.R.Del(ctx, "arena:queued:"+profileID.String()).Err()
	return nil
}

func (s *Service) CreateInvite(ctx context.Context, profileID uuid.UUID) error {
	if err := s.requireVIP(ctx, profileID); err != nil {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "vip_required", Message: "VIP required"})
		return err
	}
	rawCode, err := auth.NewRefreshToken()
	if err != nil {
		return err
	}
	code := strings.ToUpper(rawCode[:8])
	if err := s.R.Set(ctx, "arena:invite:"+code, profileID.String(), 10*time.Minute).Err(); err != nil {
		return err
	}
	return s.sendJSON(profileID, "invite.created", InviteCreatedData{Code: code, ExpiresInSec: 600})
}

func (s *Service) JoinInvite(ctx context.Context, profileID uuid.UUID, code, locale string) error {
	if err := s.requireVIP(ctx, profileID); err != nil {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "vip_required", Message: "VIP required"})
		return err
	}
	val, err := s.R.GetDel(ctx, "arena:invite:"+strings.ToUpper(code)).Result()
	if err == redis.Nil || val == "" {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "invite_invalid", Message: "invalid invite"})
		return ErrInviteInvalid
	}
	if err != nil {
		return err
	}
	hostID, err := uuid.Parse(val)
	if err != nil || hostID == profileID {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "invite_invalid", Message: "invalid invite"})
		return ErrInviteInvalid
	}
	if err := s.requireVIP(ctx, hostID); err != nil {
		_ = s.sendJSON(profileID, "error", ErrorData{Code: "vip_required", Message: "host not VIP"})
		return err
	}
	locale = normalizeLocale(locale)
	s.locales.Store(profileID.String(), locale)
	return s.startMatch(ctx, hostID, profileID)
}

func (s *Service) watchQueueTimeout(profileID uuid.UUID, bucket int, queuedAt int64) {
	deadline := time.UnixMilli(queuedAt).Add(QueueTimeout)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	<-timer.C
	ctx, cancel := opContext()
	defer cancel()
	still, _ := s.R.Get(ctx, "arena:queued:"+profileID.String()).Result()
	if still == "" {
		return
	}
	_ = s.R.ZRem(ctx, fmt.Sprintf("arena:q:%d", bucket), profileID.String()).Err()
	_ = s.R.Del(ctx, "arena:queued:"+profileID.String()).Err()
	_ = s.sendJSON(profileID, "queue.timeout", QueueTimeoutData{WaitedMs: QueueTimeout.Milliseconds()})
}

func (s *Service) localeOf(id uuid.UUID) string {
	if v, ok := s.locales.Load(id.String()); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "uz-Latn"
}

func (s *Service) startMatch(ctx context.Context, a, b uuid.UUID) error {
	ids, err := s.Q.RandomQuestionIDs(ctx, int32(QuestionCount))
	if err != nil || len(ids) < QuestionCount {
		_ = s.sendJSON(a, "error", ErrorData{Code: "not_enough_questions", Message: "bank too small"})
		_ = s.sendJSON(b, "error", ErrorData{Code: "not_enough_questions", Message: "bank too small"})
		return fmt.Errorf("not_enough_questions")
	}
	qids := make([]uuid.UUID, len(ids))
	copy(qids, ids)

	correctRows, err := s.Q.ListCorrectAnswerIDsForQuestions(ctx, qids)
	if err != nil {
		return err
	}
	correct := map[uuid.UUID]uuid.UUID{}
	for _, row := range correctRows {
		correct[row.QuestionID] = row.AnswerID
	}

	matchRow, err := s.Q.InsertArenaMatch(ctx, sqlc.InsertArenaMatchParams{
		QuestionIds:     qids,
		QuestionTimeSec: int16(QuestionTimeSec),
	})
	if err != nil {
		return err
	}

	ttl := time.Duration(QuestionCount)*(time.Duration(QuestionTimeSec)*time.Second+5*time.Second) + ReconnectGrace
	_ = s.R.Set(ctx, "arena:match:"+a.String(), matchRow.ID.String(), ttl).Err()
	_ = s.R.Set(ctx, "arena:match:"+b.String(), matchRow.ID.String(), ttl).Err()
	_ = s.R.Del(ctx, "arena:queued:"+a.String(), "arena:queued:"+b.String()).Err()

	nameA := leaderboard.DisplayName("", a.String())
	nameB := leaderboard.DisplayName("", b.String())
	if p, err := s.Q.GetProfileByID(ctx, a); err == nil {
		nameA = leaderboard.DisplayName(p.Name, a.String())
	}
	if p, err := s.Q.GetProfileByID(ctx, b); err == nil {
		nameB = leaderboard.DisplayName(p.Name, b.String())
	}

	foundA := MatchFoundData{
		MatchID: matchRow.ID, QuestionCount: QuestionCount,
		QuestionTimeMs: int64(QuestionTimeSec) * 1000, StartsInMs: 3000,
	}
	foundA.Opponent.Name = nameB
	foundB := foundA
	foundB.Opponent.Name = nameA
	_ = s.sendJSON(a, "match.found", foundA)
	_ = s.sendJSON(b, "match.found", foundB)

	m := NewMatch(s, matchRow.ID, a, b, s.localeOf(a), s.localeOf(b), qids, correct)
	s.mu.Lock()
	s.matches[matchRow.ID] = m
	s.Hub.SetMatch(a, matchRow.ID)
	s.Hub.SetMatch(b, matchRow.ID)
	s.mu.Unlock()
	go m.Run()
	return nil
}

func (s *Service) HandleClient(profileID uuid.UUID, env Envelope) {
	ctx, cancel := opContext()
	defer cancel()
	switch env.T {
	case "queue.join":
		locale := "uz-Latn"
		var d QueueJoinClientData
		if json.Unmarshal(env.D, &d) == nil && d.Locale != "" {
			locale = d.Locale
		}
		_ = s.JoinQueue(ctx, profileID, locale)
	case "queue.leave":
		_ = s.LeaveQueue(ctx, profileID)
	case "invite.create":
		_ = s.CreateInvite(ctx, profileID)
	case "invite.join":
		var d InviteJoinClientData
		if json.Unmarshal(env.D, &d) != nil || d.Code == "" {
			_ = s.sendJSON(profileID, "error", ErrorData{Code: "invite_invalid", Message: "code required"})
			return
		}
		_ = s.JoinInvite(ctx, profileID, d.Code, d.Locale)
	case "answer":
		var d AnswerClientData
		if json.Unmarshal(env.D, &d) != nil {
			return
		}
		s.mu.Lock()
		m := s.matches[d.MatchID]
		s.mu.Unlock()
		if m != nil {
			if !m.SubmitAnswer(profileID, d.Index, d.AnswerID) {
				_ = s.sendJSON(profileID, "error", ErrorData{Code: "server_busy", Message: "answer queue is busy; retry"})
			}
		}
	case "match.rejoin":
		var d MatchRejoinData
		if json.Unmarshal(env.D, &d) != nil {
			return
		}
		s.mu.Lock()
		m := s.matches[d.MatchID]
		s.mu.Unlock()
		if m != nil {
			if !m.Rejoin(profileID) {
				_ = s.sendJSON(profileID, "error", ErrorData{Code: "server_busy", Message: "rejoin queue is busy; retry"})
			}
		}
	case "match.leave":
		if mid, ok := s.Hub.MatchOf(profileID); ok {
			s.mu.Lock()
			m := s.matches[mid]
			s.mu.Unlock()
			if m != nil {
				if !m.Leave(profileID) {
					s.Log.Error("arena leave event could not be queued", zap.String("match_id", mid.String()), zap.String("profile_id", profileID.String()))
				}
			}
		}
	}
}

func (s *Service) OnDisconnect(profileID uuid.UUID) {
	ctx, cancel := opContext()
	defer cancel()
	_ = s.LeaveQueue(ctx, profileID)
	if mid, ok := s.Hub.MatchOf(profileID); ok {
		s.mu.Lock()
		m := s.matches[mid]
		s.mu.Unlock()
		if m != nil {
			if !m.NotifyDisconnect(profileID) {
				s.Log.Error("arena disconnect event could not be queued", zap.String("match_id", mid.String()), zap.String("profile_id", profileID.String()))
			}
		}
	}
}

func (s *Service) sendJSON(profileID uuid.UUID, t string, d any) error {
	b, err := Encode(t, d)
	if err != nil {
		return err
	}
	return s.Hub.Send(profileID, b)
}

func (s *Service) FinishPersist(ctx context.Context, m *Match, da, db int) error {
	outA, outB := OutcomeFromScores(m.score[m.a], m.score[m.b])
	switch m.endReason {
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
	if s.Pool == nil {
		return errors.New("arena transaction pool is not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM arena_match WHERE id = $1 FOR UPDATE`, m.id).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("arena match %s disappeared before persistence", m.id)
		}
		return err
	}
	if status == "finished" {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return s.cleanupFinishedMatch(ctx, m)
	}

	ra, rb := 1000, 1000
	if s.Rating != nil {
		ra, err = s.Rating.Rating(ctx, m.a)
		if err != nil {
			return err
		}
		rb, err = s.Rating.Rating(ctx, m.b)
		if err != nil {
			return err
		}
	}
	beforeA, beforeB := ra-da, rb-db
	q := sqlc.New(tx)
	if err := q.FinishArenaMatch(ctx, sqlc.FinishArenaMatchParams{
		ID: m.id, EndReason: pgtype.Text{String: m.endReason, Valid: true},
	}); err != nil {
		return err
	}
	if err := q.InsertArenaMatchPlayer(ctx, sqlc.InsertArenaMatchPlayerParams{
		MatchID: m.id, ProfileID: m.a, Slot: 1, Locale: m.localeA,
		Score: int32(m.score[m.a]), CorrectCount: int16(m.correctN[m.a]), TotalResponseMs: int32(m.responseSum[m.a]),
		Outcome:      pgtype.Text{String: outA, Valid: true},
		RatingBefore: pgtype.Int4{Int32: int32(beforeA), Valid: true},
		RatingAfter:  pgtype.Int4{Int32: int32(ra), Valid: true},
		RatingDelta:  pgtype.Int4{Int32: int32(da), Valid: true},
	}); err != nil {
		return err
	}
	if err := q.InsertArenaMatchPlayer(ctx, sqlc.InsertArenaMatchPlayerParams{
		MatchID: m.id, ProfileID: m.b, Slot: 2, Locale: m.localeB,
		Score: int32(m.score[m.b]), CorrectCount: int16(m.correctN[m.b]), TotalResponseMs: int32(m.responseSum[m.b]),
		Outcome:      pgtype.Text{String: outB, Valid: true},
		RatingBefore: pgtype.Int4{Int32: int32(beforeB), Valid: true},
		RatingAfter:  pgtype.Int4{Int32: int32(rb), Valid: true},
		RatingDelta:  pgtype.Int4{Int32: int32(db), Valid: true},
	}); err != nil {
		return err
	}

	var learningSvc *learning.Service
	if s.Learning != nil {
		learningSvc = learning.NewService(q)
	}
	for i, qid := range m.questions {
		for _, pid := range []uuid.UUID{m.a, m.b} {
			ans := m.answers[pid][i]
			var answerID uuid.NullUUID
			var resp pgtype.Int4
			var answeredAt pgtype.Timestamptz
			if ans.answered {
				answerID = uuid.NullUUID{UUID: ans.answerID, Valid: true}
				resp = pgtype.Int4{Int32: int32(ans.responseMs), Valid: true}
				answeredAt = pgtype.Timestamptz{Time: ans.at, Valid: true}
			}
			if err := q.InsertArenaAnswer(ctx, sqlc.InsertArenaAnswerParams{
				MatchID: m.id, ProfileID: pid, QuestionID: qid, Position: int16(i + 1),
				AnswerID: answerID, IsCorrect: ans.correct, ResponseMs: resp, Points: int16(ans.points), AnsweredAt: answeredAt,
			}); err != nil {
				return err
			}
			if ans.answered && !ans.correct && learningSvc != nil {
				if _, err := learningSvc.RecordReview(ctx, pid, qid, learning.Again); err != nil {
					return err
				}
			}
		}
	}
	if s.Progress != nil {
		progressSvc := progress.NewService(q)
		progressSvc.Learning = learningSvc
		if s.Progress.Billing.Q != nil {
			progressSvc.Billing = s.Progress.Billing
			progressSvc.Billing.Q = q
		}
		if _, err := progressSvc.RecordActivity(ctx, m.a); err != nil {
			return err
		}
		if _, err := progressSvc.RecordActivity(ctx, m.b); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.cleanupFinishedMatch(ctx, m)
}

func (s *Service) cleanupFinishedMatch(ctx context.Context, m *Match) error {
	if s.R != nil {
		if err := s.R.Del(ctx, "arena:match:"+m.a.String(), "arena:match:"+m.b.String()).Err(); err != nil {
			return err
		}
	}
	s.Hub.ClearMatch(m.a)
	s.Hub.ClearMatch(m.b)
	s.mu.Lock()
	delete(s.matches, m.id)
	s.mu.Unlock()
	return nil
}

func (s *Service) Drain(ctx context.Context) {
	s.mu.Lock()
	list := make([]*Match, 0, len(s.matches))
	for _, m := range s.matches {
		list = append(list, m)
	}
	s.mu.Unlock()
	for _, m := range list {
		if err := m.AbortShutdown(ctx); err != nil {
			s.Log.Error("arena shutdown abort could not be queued", zap.String("match_id", m.id.String()), zap.Error(err))
		}
	}
	for _, m := range list {
		select {
		case <-m.done:
		case <-ctx.Done():
			s.Log.Error("arena shutdown timed out waiting for persistence", zap.String("match_id", m.id.String()), zap.Error(ctx.Err()))
			return
		}
	}
	s.Hub.CloseAll(CloseShutdown)
}

func (s *Service) ListHistory(ctx context.Context, profileID uuid.UUID, limit int32) ([]sqlc.ListArenaMatchesForProfileRow, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.Q.ListArenaMatchesForProfile(ctx, sqlc.ListArenaMatchesForProfileParams{ProfileID: profileID, Limit: limit})
}

func normalizeLocale(loc string) string {
	switch loc {
	case "uz-Latn", "uz-Cyrl", "ru", "kaa":
		return loc
	default:
		return "uz-Latn"
	}
}

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

// QuestionLoader loads public question payloads for live duels, a whole
// match's worth per locale in one go. The bool is locale fallbackUsed —
// never treat it as ok.
type QuestionLoader interface {
	LoadQuestionDetails(ctx context.Context, ids []uuid.UUID, loc string, includeExplanations bool) (map[uuid.UUID]content.QuestionDetailDTO, bool, error)
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

// errorTo sends an error frame; a failed send only means the socket is gone.
func (s *Service) errorTo(profileID uuid.UUID, code, msg string) {
	_ = s.sendJSON(profileID, "error", ErrorData{Code: code, Message: msg})
}

// Online is how many players hold an arena socket right now.
func (s *Service) Online() int { return s.Hub.Count() }

// inMatch reports whether the player is in a live match. The Redis pointer
// outlives a crashed process by its TTL; on this single-instance deployment a
// pointer to a match this process does not run is stale and is dropped, so a
// restart cannot lock anyone out of the arena for minutes.
func (s *Service) inMatch(ctx context.Context, profileID uuid.UUID) bool {
	if s.Hub.InMatch(profileID) {
		return true
	}
	if s.R == nil {
		return false
	}
	val, err := s.R.Get(ctx, "arena:match:"+profileID.String()).Result()
	if err != nil || val == "" {
		return false
	}
	if mid, err := uuid.Parse(val); err == nil {
		s.mu.Lock()
		_, live := s.matches[mid]
		s.mu.Unlock()
		if live {
			return true
		}
	}
	_ = s.R.Del(ctx, "arena:match:"+profileID.String()).Err()
	return false
}

// arenaGate is the common admission check for every way into a duel.
func (s *Service) arenaGate(ctx context.Context, profileID uuid.UUID) error {
	if err := s.requireVIP(ctx, profileID); err != nil {
		if errors.Is(err, ErrRequiresVIP) {
			s.errorTo(profileID, "vip_required", "VIP required")
		}
		return err
	}
	ok, err := s.Lim.Allow(ctx, "arena:rl:join:"+profileID.String(), 30, 5*time.Minute)
	if err != nil {
		return err
	}
	if !ok {
		s.errorTo(profileID, "rate_limited", "join rate limited")
		return fmt.Errorf("rate_limited")
	}
	if s.inMatch(ctx, profileID) {
		s.errorTo(profileID, "already_in_match", "already in match")
		return ErrAlreadyInMatch
	}
	return nil
}

const queueMarkerTTL = 120 * time.Second

// queueSearchKeys lists the queue buckets a joining player scans, own bucket
// first and then outward. Before this widened, a player only ever looked in
// their own bucket: once ratings drifted to 984 and 1016 (buckets 9 and 10)
// two players could search side by side and never be paired.
func queueSearchKeys(bucket int) []string {
	keys := []string{}
	for _, b := range SearchBuckets(bucket, MaxSearchSteps*5*time.Second) {
		if b >= 0 {
			keys = append(keys, fmt.Sprintf("arena:q:%d", b))
		}
	}
	return keys
}

func (s *Service) JoinQueue(ctx context.Context, profileID uuid.UUID, locale string) error {
	if err := s.arenaGate(ctx, profileID); err != nil {
		return err
	}
	if v, _ := s.R.Exists(ctx, "arena:queued:"+profileID.String()).Result(); v > 0 {
		// A second click while searching: say so, the search is still on.
		s.errorTo(profileID, "already_queued", "already queued")
		return ErrAlreadyQueued
	}
	s.cancelInvite(ctx, profileID)
	locale = normalizeLocale(locale)
	s.locales.Store(profileID.String(), locale)

	rating, err := s.Rating.Rating(ctx, profileID)
	if err != nil {
		return err
	}
	keys := queueSearchKeys(Bucket(rating))
	ownKey := keys[0]
	// A candidate whose socket is gone is dropped and the search goes on; the
	// bound only keeps a pathological queue from spinning this goroutine.
	for attempt := 0; attempt < 8; attempt++ {
		now := s.Now().UTC().UnixMilli()
		res, err := s.R.Eval(ctx, arenaJoinLua, keys, profileID.String(), now, ownKey, int(queueMarkerTTL.Seconds())).Result()
		if err != nil {
			return err
		}
		arr, ok := res.([]interface{})
		if !ok || len(arr) < 2 {
			return fmt.Errorf("bad_join_result")
		}
		kind, _ := arr[0].(string)
		switch kind {
		case "queued":
			marker, _ := arr[1].(string)
			_ = s.sendJSON(profileID, "queue.joined", QueueJoinedData{
				QueuedAtMs:   now,
				TimeoutMs:    QueueTimeout.Milliseconds(),
				ServerTimeMs: now,
				Online:       s.Online(),
			})
			go s.watchQueueTimeout(profileID, ownKey, marker)
			return nil
		case "paired":
			oppStr, _ := arr[1].(string)
			oppID, err := uuid.Parse(oppStr)
			if err != nil || !s.Hub.Alive(oppID) || s.inMatch(ctx, oppID) {
				continue
			}
			err = s.startMatch(ctx, ModeRanked, profileID, oppID)
			if errors.Is(err, ErrAlreadyInMatch) {
				continue // the candidate got into another duel this instant
			}
			return err
		default:
			return fmt.Errorf("unknown_join_kind")
		}
	}
	s.errorTo(profileID, "server_busy", "matchmaking busy; retry")
	return fmt.Errorf("join_attempts_exhausted")
}

// dequeue removes the player's queue entry, if any, and reports whether one
// existed.
func (s *Service) dequeue(ctx context.Context, profileID uuid.UUID) (bool, error) {
	marker, err := s.R.Get(ctx, "arena:queued:"+profileID.String()).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bucket, _, _ := strings.Cut(marker, ":")
	if err := s.R.ZRem(ctx, "arena:q:"+bucket, profileID.String()).Err(); err != nil {
		return false, err
	}
	if err := s.R.Del(ctx, "arena:queued:"+profileID.String()).Err(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) LeaveQueue(ctx context.Context, profileID uuid.UUID) error {
	_, err := s.dequeue(ctx, profileID)
	return err
}

func inviteKey(code string) string        { return "arena:invite:" + code }
func inviteHostKey(host uuid.UUID) string { return "arena:invite:host:" + host.String() }

const inviteTTL = 10 * time.Minute

// cancelInvite withdraws the player's open invite code, if they have one.
func (s *Service) cancelInvite(ctx context.Context, host uuid.UUID) {
	code, err := s.R.GetDel(ctx, inviteHostKey(host)).Result()
	if err != nil || code == "" {
		return
	}
	// Only delete the code if it still points at this host.
	if val, err := s.R.Get(ctx, inviteKey(code)).Result(); err == nil && val == host.String() {
		_ = s.R.Del(ctx, inviteKey(code)).Err()
	}
}

func (s *Service) CreateInvite(ctx context.Context, profileID uuid.UUID, locale string) error {
	if err := s.arenaGate(ctx, profileID); err != nil {
		return err
	}
	if _, err := s.dequeue(ctx, profileID); err != nil {
		return err
	}
	s.cancelInvite(ctx, profileID)
	s.locales.Store(profileID.String(), normalizeLocale(locale))
	for attempt := 0; attempt < 5; attempt++ {
		code, err := NewInviteCode()
		if err != nil {
			return err
		}
		ok, err := s.R.SetNX(ctx, inviteKey(code), profileID.String(), inviteTTL).Result()
		if err != nil {
			return err
		}
		if !ok {
			continue // collision with a live code
		}
		if err := s.R.Set(ctx, inviteHostKey(profileID), code, inviteTTL).Err(); err != nil {
			return err
		}
		return s.sendJSON(profileID, "invite.created", InviteCreatedData{
			Code: code, ExpiresInSec: int(inviteTTL.Seconds()),
		})
	}
	return fmt.Errorf("invite_code_collision")
}

// OpenInvite is the player's live invite code, so a reconnecting host sees
// the code they already sent instead of losing it.
func (s *Service) OpenInvite(ctx context.Context, host uuid.UUID) *InviteCreatedData {
	if s.R == nil {
		return nil
	}
	code, err := s.R.Get(ctx, inviteHostKey(host)).Result()
	if err != nil || code == "" {
		return nil
	}
	ttl, err := s.R.TTL(ctx, inviteKey(code)).Result()
	if err != nil || ttl <= 0 {
		return nil
	}
	return &InviteCreatedData{Code: code, ExpiresInSec: int(ttl.Seconds())}
}

func (s *Service) CancelInvite(ctx context.Context, profileID uuid.UUID) {
	s.cancelInvite(ctx, profileID)
	_ = s.sendJSON(profileID, "invite.cancelled", struct{}{})
}

func (s *Service) JoinInvite(ctx context.Context, profileID uuid.UUID, rawCode, locale string) error {
	code := NormalizeInviteCode(rawCode)
	if code == "" {
		s.errorTo(profileID, "invite_invalid", "invalid invite")
		return ErrInviteInvalid
	}
	if err := s.arenaGate(ctx, profileID); err != nil {
		return err
	}
	val, err := s.R.Get(ctx, inviteKey(code)).Result()
	if err == redis.Nil || val == "" {
		s.errorTo(profileID, "invite_invalid", "invalid invite")
		return ErrInviteInvalid
	}
	if err != nil {
		return err
	}
	hostID, err := uuid.Parse(val)
	if err != nil {
		s.errorTo(profileID, "invite_invalid", "invalid invite")
		return ErrInviteInvalid
	}
	if hostID == profileID {
		s.errorTo(profileID, "invite_self", "own invite")
		return ErrInviteInvalid
	}
	if !s.Hub.Alive(hostID) || s.inMatch(ctx, hostID) {
		// The code stays valid: the host may just be reloading the page.
		s.errorTo(profileID, "invite_host_away", "host is not in the arena")
		return ErrInviteInvalid
	}
	if err := s.requireVIP(ctx, hostID); err != nil {
		s.errorTo(profileID, "invite_host_away", "host not VIP")
		return err
	}
	// Claim the code atomically: two friends typing it at once get one duel.
	if claimed, err := s.R.Del(ctx, inviteKey(code)).Result(); err != nil || claimed == 0 {
		s.errorTo(profileID, "invite_invalid", "invalid invite")
		return ErrInviteInvalid
	}
	_ = s.R.Del(ctx, inviteHostKey(hostID)).Err()
	if _, err := s.dequeue(ctx, profileID); err != nil {
		return err
	}
	if _, err := s.dequeue(ctx, hostID); err != nil {
		return err
	}
	s.locales.Store(profileID.String(), normalizeLocale(locale))
	err = s.startMatch(ctx, ModeFriend, hostID, profileID)
	if errors.Is(err, ErrAlreadyInMatch) {
		s.errorTo(profileID, "invite_host_away", "host is in another match")
	}
	return err
}

// StartBot starts an unrated practice duel against the server's bot.
func (s *Service) StartBot(ctx context.Context, profileID uuid.UUID, locale string) error {
	if err := s.arenaGate(ctx, profileID); err != nil {
		return err
	}
	if _, err := s.dequeue(ctx, profileID); err != nil {
		return err
	}
	s.cancelInvite(ctx, profileID)
	s.locales.Store(profileID.String(), normalizeLocale(locale))
	err := s.startMatch(ctx, ModeBot, profileID, uuid.New())
	if errors.Is(err, ErrAlreadyInMatch) {
		s.errorTo(profileID, "already_in_match", "already in match")
	}
	return err
}

func (s *Service) watchQueueTimeout(profileID uuid.UUID, ownKey, marker string) {
	timer := time.NewTimer(QueueTimeout)
	defer timer.Stop()
	<-timer.C
	ctx, cancel := opContext()
	defer cancel()
	// Only this queue entry may be timed out: a player who left and rejoined
	// has a new marker and a full new wait ahead of them.
	still, _ := s.R.Get(ctx, "arena:queued:"+profileID.String()).Result()
	if still != marker {
		return
	}
	_ = s.R.ZRem(ctx, ownKey, profileID.String()).Err()
	_ = s.R.Del(ctx, "arena:queued:"+profileID.String()).Err()
	_ = s.sendJSON(profileID, "queue.timeout", QueueTimeoutData{
		WaitedMs: QueueTimeout.Milliseconds(), Online: s.Online(),
	})
}

func (s *Service) localeOf(id uuid.UUID) string {
	if v, ok := s.locales.Load(id.String()); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "uz-Latn"
}

func (s *Service) playerCard(ctx context.Context, id uuid.UUID) (PlayerCard, error) {
	rating, err := s.Rating.Rating(ctx, id)
	if err != nil {
		return PlayerCard{}, err
	}
	name := leaderboard.DisplayName("", id.String())
	if p, err := s.Q.GetProfileByID(ctx, id); err == nil {
		name = leaderboard.DisplayName(p.Name, id.String())
	}
	return PlayerCard{Name: name, Rating: rating, Medal: MedalForRating(rating)}, nil
}

// BotName is the practice bot's display name; the client localizes a bot
// card by its Bot flag, this is the fallback.
const BotName = "AvtoBot"

// pickQuestions draws the duel's questions and their payloads in every
// locale the two players read. Both players get the same questions in the
// same order — the only way a score race is fair — so a question that cannot
// be dealt in full to either of them is replaced rather than sent half-empty.
func (s *Service) pickQuestions(ctx context.Context, locales []string) ([]uuid.UUID, map[string][]content.QuestionDetailDTO, map[uuid.UUID]uuid.UUID, error) {
	ids, err := s.Q.RandomQuestionIDs(ctx, int32(QuestionCount+6))
	if err != nil {
		return nil, nil, nil, err
	}
	correctRows, err := s.Q.ListCorrectAnswerIDsForQuestions(ctx, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	correct := map[uuid.UUID]uuid.UUID{}
	for _, row := range correctRows {
		correct[row.QuestionID] = row.AnswerID
	}
	byLocale := map[string]map[uuid.UUID]content.QuestionDetailDTO{}
	for _, loc := range locales {
		if _, done := byLocale[loc]; done {
			continue
		}
		details, _, err := s.Content.LoadQuestionDetails(ctx, ids, loc, false)
		if err != nil {
			return nil, nil, nil, err
		}
		byLocale[loc] = details
	}
	picked, payloads := assembleDuel(ids, correct, byLocale, QuestionCount)
	if len(picked) < QuestionCount {
		return nil, nil, nil, fmt.Errorf("not_enough_questions")
	}
	return picked, payloads, correct, nil
}

// assembleDuel keeps, in draw order, the first n questions that have a
// correct answer and a full payload (at least two options) in every locale.
func assembleDuel(ids []uuid.UUID, correct map[uuid.UUID]uuid.UUID, byLocale map[string]map[uuid.UUID]content.QuestionDetailDTO, n int) ([]uuid.UUID, map[string][]content.QuestionDetailDTO) {
	var picked []uuid.UUID
	payloads := map[string][]content.QuestionDetailDTO{}
	for _, id := range ids {
		if len(picked) == n {
			break
		}
		if _, ok := correct[id]; !ok {
			continue
		}
		usable := true
		for _, details := range byLocale {
			d, ok := details[id]
			if !ok || len(d.Answers) < 2 {
				usable = false
				break
			}
		}
		if !usable {
			continue
		}
		picked = append(picked, id)
		for loc, details := range byLocale {
			d := details[id]
			d.Explanation = nil
			d.Signs = nil
			payloads[loc] = append(payloads[loc], d)
		}
	}
	return picked, payloads
}

// claim marks the humans of a new match as in-match before any slow work, so
// a player cannot be pulled into two duels at once (e.g. paired by someone's
// queue join in the same instant they start a bot practice). It fails if
// either is already claimed. The placeholder id is replaced by the real one
// once the match row exists; release undoes a claim that did not start.
func (s *Service) claim(ids ...uuid.UUID) (uuid.UUID, bool) {
	placeholder := uuid.New()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if s.Hub.InMatch(id) {
			return uuid.Nil, false
		}
	}
	for _, id := range ids {
		s.Hub.SetMatch(id, placeholder)
	}
	return placeholder, true
}

func (s *Service) release(placeholder uuid.UUID, ids ...uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if cur, ok := s.Hub.MatchOf(id); ok && cur == placeholder {
			s.Hub.ClearMatch(id)
		}
	}
}

func (s *Service) startMatch(ctx context.Context, mode string, a, b uuid.UUID) error {
	humans := []uuid.UUID{a}
	if mode != ModeBot {
		humans = append(humans, b)
	}
	placeholder, ok := s.claim(humans...)
	if !ok {
		return ErrAlreadyInMatch
	}
	started := false
	defer func() {
		if !started {
			s.release(placeholder, humans...)
		}
	}()
	fail := func(code, msg string, err error) error {
		for _, id := range humans {
			s.errorTo(id, code, msg)
		}
		return err
	}
	cardA, err := s.playerCard(ctx, a)
	if err != nil {
		return fail("server_busy", "could not start match", err)
	}
	seatA := SeatSpec{ID: a, Locale: s.localeOf(a), Card: cardA}
	var seatB SeatSpec
	if mode == ModeBot {
		seatB = SeatSpec{ID: b, Locale: seatA.Locale, Bot: true, Card: PlayerCard{
			Name: BotName, Rating: cardA.Rating, Medal: MedalForRating(cardA.Rating), Bot: true,
		}}
	} else {
		cardB, err := s.playerCard(ctx, b)
		if err != nil {
			return fail("server_busy", "could not start match", err)
		}
		seatB = SeatSpec{ID: b, Locale: s.localeOf(b), Card: cardB}
	}

	qids, payloads, correct, err := s.pickQuestions(ctx, []string{seatA.Locale, seatB.Locale})
	if err != nil {
		if err.Error() == "not_enough_questions" {
			return fail("not_enough_questions", "bank too small", err)
		}
		return fail("server_busy", "could not start match", err)
	}

	matchRow, err := s.Q.InsertArenaMatch(ctx, sqlc.InsertArenaMatchParams{
		QuestionIds:     qids,
		QuestionTimeSec: int16(QuestionTimeSec),
		Mode:            mode,
	})
	if err != nil {
		return fail("server_busy", "could not start match", err)
	}

	m := NewMatch(s, matchRow.ID, mode, seatA, seatB, qids, payloads, correct)
	startsAt := m.Start()

	// Longest possible duel plus slack: every question to the last second,
	// every reveal, the countdown and a reconnect grace.
	perQ := time.Duration(QuestionTimeSec)*time.Second + AnswerGrace + RevealFor
	ttl := Countdown + time.Duration(QuestionCount)*perQ + 2*ReconnectGrace + time.Minute
	_ = s.R.Set(ctx, "arena:match:"+a.String(), matchRow.ID.String(), ttl).Err()
	if mode != ModeBot {
		_ = s.R.Set(ctx, "arena:match:"+b.String(), matchRow.ID.String(), ttl).Err()
	}

	s.mu.Lock()
	s.matches[matchRow.ID] = m
	for _, id := range humans {
		s.Hub.SetMatch(id, matchRow.ID)
	}
	s.mu.Unlock()
	started = true

	now := s.Now()
	found := func(you, opp PlayerCard) MatchFoundData {
		return MatchFoundData{
			MatchID: matchRow.ID, Mode: mode, You: you, Opponent: opp,
			QuestionCount: len(qids), QuestionTimeMs: int64(QuestionTimeSec) * 1000,
			StartsInMs: startsAt.Sub(now).Milliseconds(), StartsAtMs: ms(startsAt),
			ServerTimeMs: ms(now),
		}
	}
	_ = s.sendJSON(a, "match.found", found(seatA.Card, seatB.Card))
	if mode != ModeBot {
		_ = s.sendJSON(b, "match.found", found(seatB.Card, seatA.Card))
	}
	go m.Run()
	return nil
}

func (s *Service) matchOf(profileID uuid.UUID) *Match {
	mid, ok := s.Hub.MatchOf(profileID)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matches[mid]
}

func (s *Service) HandleClient(profileID uuid.UUID, env Envelope) {
	ctx, cancel := opContext()
	defer cancel()
	switch env.T {
	case "queue.join":
		var d QueueJoinClientData
		_ = json.Unmarshal(env.D, &d)
		_ = s.JoinQueue(ctx, profileID, d.Locale)
	case "queue.leave":
		if err := s.LeaveQueue(ctx, profileID); err != nil {
			s.Log.Warn("arena queue leave failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		}
		_ = s.sendJSON(profileID, "queue.left", struct{}{})
	case "invite.create":
		var d QueueJoinClientData
		_ = json.Unmarshal(env.D, &d)
		if err := s.CreateInvite(ctx, profileID, d.Locale); err != nil && s.Log != nil {
			s.Log.Warn("arena invite create failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		}
	case "invite.cancel":
		s.CancelInvite(ctx, profileID)
	case "invite.join":
		var d InviteJoinClientData
		if json.Unmarshal(env.D, &d) != nil || d.Code == "" {
			s.errorTo(profileID, "invite_invalid", "code required")
			return
		}
		_ = s.JoinInvite(ctx, profileID, d.Code, d.Locale)
	case "bot.start":
		var d BotStartClientData
		_ = json.Unmarshal(env.D, &d)
		_ = s.StartBot(ctx, profileID, d.Locale)
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
				s.errorTo(profileID, "server_busy", "answer queue is busy; retry")
			}
		}
	case "match.rejoin":
		if m := s.matchOf(profileID); m != nil {
			if !m.Rejoin(profileID) {
				s.errorTo(profileID, "server_busy", "rejoin queue is busy; retry")
			}
		}
	case "match.leave":
		if m := s.matchOf(profileID); m != nil {
			if !m.Leave(profileID) {
				s.Log.Error("arena leave event could not be queued", zap.String("match_id", m.id.String()), zap.String("profile_id", profileID.String()))
			}
		}
	default:
		s.errorTo(profileID, "bad_protocol", "unknown message type")
	}
}

// OnConnect runs after a socket is registered: a player who reloads the page
// or whose network blinked mid-duel is put straight back into their match.
func (s *Service) OnConnect(profileID uuid.UUID) {
	if m := s.matchOf(profileID); m != nil {
		if !m.Rejoin(profileID) {
			s.Log.Error("arena resume event could not be queued", zap.String("match_id", m.id.String()), zap.String("profile_id", profileID.String()))
		}
	}
}

// OnDisconnect runs when a socket closes. A socket that was replaced by a
// newer one for the same player (second tab, reconnect racing the old
// socket's close) is not a disconnect: the player is still here, and treating
// it as one would pull them out of the queue or forfeit their live match.
func (s *Service) OnDisconnect(profileID uuid.UUID, c *Conn) {
	if cur := s.Hub.Get(profileID); cur != nil && cur != c {
		return
	}
	ctx, cancel := opContext()
	defer cancel()
	if err := s.LeaveQueue(ctx, profileID); err != nil {
		s.Log.Warn("arena queue cleanup on disconnect failed", zap.String("profile_id", profileID.String()), zap.Error(err))
	}
	if m := s.matchOf(profileID); m != nil {
		if !m.NotifyDisconnect(profileID) {
			s.Log.Error("arena disconnect event could not be queued", zap.String("match_id", m.id.String()), zap.String("profile_id", profileID.String()))
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
	outA, outB := m.outcomes()
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

	q := sqlc.New(tx)
	if err := q.FinishArenaMatch(ctx, sqlc.FinishArenaMatchParams{
		ID: m.id, EndReason: pgtype.Text{String: m.endReason, Valid: true},
	}); err != nil {
		return err
	}

	var learningSvc *learning.Service
	if s.Learning != nil {
		learningSvc = learning.NewService(q)
		learningSvc.PassRates = s.Learning.PassRates
	}
	var progressSvc *progress.Service
	if s.Progress != nil {
		progressSvc = progress.NewService(q)
		progressSvc.Learning = learningSvc
		if s.Progress.Billing.Q != nil {
			progressSvc.Billing = s.Progress.Billing
			progressSvc.Billing.Q = q
		}
	}

	outcomes := [2]string{outA, outB}
	deltas := [2]int{da, db}
	for slot, st := range m.seats {
		if st.bot {
			continue // the bot has no profile row to hang results on
		}
		// The rating as it stands now (after ApplyResult for a rated duel);
		// before = now - delta, which is exact for unrated duels (delta 0).
		after := st.card.Rating
		if s.Rating != nil {
			if after, err = s.Rating.Rating(ctx, st.id); err != nil {
				return err
			}
		}
		if err := q.InsertArenaMatchPlayer(ctx, sqlc.InsertArenaMatchPlayerParams{
			MatchID: m.id, ProfileID: st.id, Slot: int16(slot + 1), Locale: st.locale,
			Score: int32(st.score), CorrectCount: int16(st.correctN), TotalResponseMs: int32(st.responseSum),
			Outcome:      pgtype.Text{String: outcomes[slot], Valid: true},
			RatingBefore: pgtype.Int4{Int32: int32(after - deltas[slot]), Valid: true},
			RatingAfter:  pgtype.Int4{Int32: int32(after), Valid: true},
			RatingDelta:  pgtype.Int4{Int32: int32(deltas[slot]), Valid: true},
		}); err != nil {
			return err
		}
		for i, qid := range m.questions {
			ans := st.answers[i]
			var answerID uuid.NullUUID
			var resp pgtype.Int4
			var answeredAt pgtype.Timestamptz
			if ans.answered {
				answerID = uuid.NullUUID{UUID: ans.answerID, Valid: true}
				resp = pgtype.Int4{Int32: int32(ans.responseMs), Valid: true}
				answeredAt = pgtype.Timestamptz{Time: ans.at, Valid: true}
			}
			if err := q.InsertArenaAnswer(ctx, sqlc.InsertArenaAnswerParams{
				MatchID: m.id, ProfileID: st.id, QuestionID: qid, Position: int16(i + 1),
				AnswerID: answerID, IsCorrect: ans.correct, ResponseMs: resp, Points: int16(ans.points), AnsweredAt: answeredAt,
			}); err != nil {
				return err
			}
			if ans.answered && !ans.correct && learningSvc != nil {
				if _, err := learningSvc.RecordReview(ctx, st.id, qid, learning.Again); err != nil {
					return err
				}
			}
		}
		if progressSvc != nil {
			if _, err := progressSvc.RecordActivity(ctx, st.id); err != nil {
				return err
			}
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
	for _, st := range m.seats {
		if !st.bot {
			s.Hub.ClearMatch(st.id)
		}
	}
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

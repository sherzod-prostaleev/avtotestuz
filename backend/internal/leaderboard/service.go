package leaderboard

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

type Service struct {
	Redis   *redis.Client
	Q       *sqlc.Queries
	Billing billing.Service
}

func NewService(r *redis.Client, q *sqlc.Queries, b billing.Service) *Service {
	return &Service{Redis: r, Q: q, Billing: b}
}

const dailyPointsConfigKey = "leaderboard_daily_points"

// RecordPoint credits profileID with one point (a correct answer) across
// all four periods, unless the profile has already hit its daily cap (see
// dailyPointsConfigKey / migration 0017). Safe to call from a hot path:
// callers should discard this error rather than fail the request that
// triggered it — leaderboard standing is a side effect, never the source
// of truth for anything else (session_answer already recorded the answer
// by the time this runs). See internal/session.Service.SubmitAnswer.
func (s *Service) RecordPoint(ctx context.Context, profileID uuid.UUID) error {
	now := time.Now().UTC()
	member := profileID.String()

	current, err := s.currentPointsAllPeriods(ctx, member, now)
	if err != nil {
		return err
	}

	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return err
	}
	cfg, err := s.Q.GetLimitConfig(ctx, dailyPointsConfigKey)
	if err != nil {
		return err
	}
	dailyCap := int(cfg.FreeValue)
	if active {
		dailyCap = int(cfg.VipValue)
	}
	if current[PeriodDaily] >= dailyCap {
		return nil
	}

	pipe := s.Redis.Pipeline()
	for _, p := range AllPeriods {
		key := RedisKey(p, now)
		score := EncodeScore(current[p]+1, now)
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
		if ttl := TTL(p); ttl > 0 {
			pipe.Expire(ctx, key, ttl)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// currentPointsAllPeriods reads the caller's current integer point total
// for all four periods in a single Redis round trip.
func (s *Service) currentPointsAllPeriods(ctx context.Context, member string, now time.Time) (map[Period]int, error) {
	pipe := s.Redis.Pipeline()
	cmds := make(map[Period]*redis.FloatCmd, len(AllPeriods))
	for _, p := range AllPeriods {
		cmds[p] = pipe.ZScore(ctx, RedisKey(p, now), member)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	out := make(map[Period]int, len(AllPeriods))
	for _, p := range AllPeriods {
		score, err := cmds[p].Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		out[p] = DecodePoints(score) // DecodePoints(0) == 0 for the redis.Nil (not-found) case
	}
	return out, nil
}

// Entry is one row of a leaderboard listing.
type Entry struct {
	Rank  int
	Name  string
	Score int
}

// Result is GetLeaderboard's full response: the requested period, the
// caller's own standing (YouRank is nil if the caller has no score yet in
// this period), the top TopN entries, and — only when the caller falls
// outside Top — their AroundRadius neighborhood.
type Result struct {
	Period    Period
	YouRank   *int
	YouScore  int
	YouName   string
	Top       []Entry
	AroundYou []Entry
}

func (s *Service) GetLeaderboard(ctx context.Context, profileID uuid.UUID, p Period) (Result, error) {
	now := time.Now().UTC()
	key := RedisKey(p, now)
	member := profileID.String()

	topZ, err := s.Redis.ZRevRangeWithScores(ctx, key, 0, TopN-1).Result()
	if err != nil {
		return Result{}, err
	}

	var youRank *int
	youScore := 0
	rank, err := s.Redis.ZRevRank(ctx, key, member).Result()
	switch {
	case err == nil:
		r := int(rank) + 1 // ZRevRank is 0-indexed
		youRank = &r
		scoreVal, scoreErr := s.Redis.ZScore(ctx, key, member).Result()
		if scoreErr != nil && !errors.Is(scoreErr, redis.Nil) {
			return Result{}, scoreErr
		}
		youScore = DecodePoints(scoreVal)
	case errors.Is(err, redis.Nil):
		// No score yet — youRank stays nil, youScore stays 0.
	default:
		return Result{}, err
	}

	var aroundZ []redis.Z
	var aroundStart int64
	if youRank != nil && *youRank > TopN {
		aroundStart = int64(*youRank - 1 - AroundRadius)
		if aroundStart < 0 {
			aroundStart = 0
		}
		stop := int64(*youRank - 1 + AroundRadius)
		aroundZ, err = s.Redis.ZRevRangeWithScores(ctx, key, aroundStart, stop).Result()
		if err != nil {
			return Result{}, err
		}
	}

	ids := []uuid.UUID{profileID}
	seen := map[uuid.UUID]bool{profileID: true}
	collectIDs := func(zs []redis.Z) {
		for _, z := range zs {
			memberStr, _ := z.Member.(string)
			id, parseErr := uuid.Parse(memberStr)
			if parseErr != nil || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	collectIDs(topZ)
	collectIDs(aroundZ)

	names := map[uuid.UUID]string{}
	rows, err := s.Q.ListProfileNamesByIDs(ctx, ids)
	if err != nil {
		return Result{}, err
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}

	toEntries := func(zs []redis.Z, rankOffset int) []Entry {
		out := make([]Entry, 0, len(zs))
		for i, z := range zs {
			memberStr, _ := z.Member.(string)
			id, _ := uuid.Parse(memberStr)
			out = append(out, Entry{
				Rank:  rankOffset + i + 1,
				Name:  DisplayName(names[id], memberStr),
				Score: DecodePoints(z.Score),
			})
		}
		return out
	}

	return Result{
		Period:    p,
		YouRank:   youRank,
		YouScore:  youScore,
		YouName:   DisplayName(names[profileID], profileID.String()),
		Top:       toEntries(topZ, 0),
		AroundYou: toEntries(aroundZ, int(aroundStart)),
	}, nil
}

// RebuildPeriod recomputes period p's Redis sorted set entirely from
// session_answer (the durable source of truth), as of instant at. Safe to
// call at any time — e.g. after a Redis flush/restart, or on a periodic
// reconciliation schedule — since it fully overwrites each affected
// member's score rather than incrementing.
func (s *Service) RebuildPeriod(ctx context.Context, p Period, at time.Time) error {
	from := PeriodStart(p, at)
	to := PeriodEnd(p, at)
	dayRows, err := s.Q.CountCorrectAnswersByProfileByDayInRange(ctx, sqlc.CountCorrectAnswersByProfileByDayInRangeParams{
		FromTs: pgtype.Timestamptz{Time: from, Valid: true},
		ToTs:   pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return err
	}

	key := RedisKey(p, at)
	pipe := s.Redis.Pipeline()
	pipe.Del(ctx, key)
	if len(dayRows) == 0 {
		_, err = pipe.Exec(ctx)
		return err
	}

	cfg, err := s.Q.GetLimitConfig(ctx, dailyPointsConfigKey)
	if err != nil {
		return err
	}

	// Reapply the same daily cap RecordPoint enforces live, per day, before
	// summing across the period — otherwise a rebuild would retroactively
	// un-cap any profile that hit its daily limit before Redis was lost.
	// Uses each profile's CURRENT VIP status and the CURRENT cap value as
	// an approximation (neither is tracked historically in this schema);
	// see docs/superpowers/specs/2026-07-25-m4-01-leaderboard-design.md
	// section 4/7 for why this bounded approximation — not perfect
	// historical fidelity — is the accepted trade-off.
	type profileTotal struct {
		points int
		lastAt time.Time
	}
	totals := make(map[uuid.UUID]*profileTotal)
	vipCache := make(map[uuid.UUID]bool)
	for _, row := range dayRows {
		active, cached := vipCache[row.ProfileID]
		if !cached {
			active, _, err = s.Billing.Status(ctx, row.ProfileID)
			if err != nil {
				return err
			}
			vipCache[row.ProfileID] = active
		}
		dailyCap := int(cfg.FreeValue)
		if active {
			dailyCap = int(cfg.VipValue)
		}
		dayCount := int(row.CorrectCount)
		if dayCount > dailyCap {
			dayCount = dailyCap
		}
		t, ok := totals[row.ProfileID]
		if !ok {
			t = &profileTotal{}
			totals[row.ProfileID] = t
		}
		t.points += dayCount
		if row.LastAnsweredAt.Time.After(t.lastAt) {
			t.lastAt = row.LastAnsweredAt.Time
		}
	}

	for profileID, t := range totals {
		score := EncodeScore(t.points, t.lastAt)
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: profileID.String()})
	}
	if ttl := TTL(p); ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

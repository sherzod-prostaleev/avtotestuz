package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
)

const (
	KindSupportBroadcast   = "support_broadcast"
	defaultMaxRecipients   = 100_000
	expandPageSize         = 500
	claimBatchSize         = 50
	maxAttempts            = 5
	workerPollInterval     = 2 * time.Second
	defaultProcessingLease = 2 * time.Minute
)

var (
	ErrNotFound      = errors.New("broadcast not found")
	ErrNotConfirm    = errors.New("confirm required")
	ErrTooMany       = errors.New("audience exceeds max recipients")
	ErrInvalidCancel = errors.New("campaign cannot be cancelled")
	ErrRateLimited   = errors.New("broadcast rate limited")
)

// Config controls broadcast safety rails.
type Config struct {
	MaxRecipients int
	ImageHosts    []string
	// ProcessingLease is how long a recipient may stay in status=processing
	// before another worker may reclaim it (crash recovery). Zero → 2m.
	ProcessingLease time.Duration
}

func (c Config) maxRecipients() int {
	if c.MaxRecipients <= 0 {
		return defaultMaxRecipients
	}
	return c.MaxRecipients
}

func (c Config) processingLease() time.Duration {
	if c.ProcessingLease <= 0 {
		return defaultProcessingLease
	}
	return c.ProcessingLease
}

// Service owns campaign create/list and outbox processing.
type Service struct {
	Pool *pgxpool.Pool
	Q    *sqlc.Queries
	Push *push.Service
	Cfg  Config
	Lim  interface {
		Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	}
}

// CreateInput is a live send request.
type CreateInput struct {
	AdminID        uuid.UUID
	Title          string
	Body           string
	ImageURL       string
	ActionURL      string
	Audience       string
	Channels       string
	Confirm        bool
	IdempotencyKey string
}

// Create queues a campaign for the worker (HTTP must not fan out).
func (s *Service) Create(ctx context.Context, in CreateInput) (sqlc.BroadcastCampaign, error) {
	if !in.Confirm {
		return sqlc.BroadcastCampaign{}, ErrNotConfirm
	}
	title, body, err := ValidateContent(in.Title, in.Body)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	audience, err := NormalizeAudience(in.Audience)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	channels, err := NormalizeChannels(in.Channels)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	actionURL, err := SanitizeActionURL(in.ActionURL)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	imageURL, err := SanitizeImageURL(in.ImageURL, s.Cfg.ImageHosts)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}

	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		existing, err := s.Q.GetBroadcastCampaignByIdempotencyKey(ctx, pgtype.Text{String: key, Valid: true})
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return sqlc.BroadcastCampaign{}, err
		}
	}

	counts, err := CountAudience(ctx, s.Pool, audience)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	if counts.Recipients > s.Cfg.maxRecipients() {
		return sqlc.BroadcastCampaign{}, fmt.Errorf("%w: %d > %d", ErrTooMany, counts.Recipients, s.Cfg.maxRecipients())
	}

	if s.Lim != nil {
		ok, limErr := s.Lim.Allow(ctx, "broadcast:admin:"+in.AdminID.String(), 5, 10*time.Minute)
		if limErr != nil {
			return sqlc.BroadcastCampaign{}, limErr
		}
		if !ok {
			return sqlc.BroadcastCampaign{}, ErrRateLimited
		}
	}

	idem := pgtype.Text{}
	if key != "" {
		idem = pgtype.Text{String: key, Valid: true}
	}
	camp, err := s.Q.InsertBroadcastCampaign(ctx, sqlc.InsertBroadcastCampaignParams{
		CreatedByAdmin: in.AdminID,
		Title:          title,
		Body:           body,
		ImageUrl:       imageURL,
		ActionUrl:      actionURL,
		Audience:       audience,
		Channels:       channels,
		Status:         "queued",
		IdempotencyKey: idem,
	})
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	return camp, nil
}

func (s *Service) DryRun(ctx context.Context, audience, channels string) (DryRunCounts, error) {
	a, err := NormalizeAudience(audience)
	if err != nil {
		return DryRunCounts{}, err
	}
	if _, err := NormalizeChannels(channels); err != nil {
		return DryRunCounts{}, err
	}
	return CountAudience(ctx, s.Pool, a)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (sqlc.BroadcastCampaign, error) {
	camp, err := s.Q.GetBroadcastCampaignByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.BroadcastCampaign{}, ErrNotFound
	}
	return camp, err
}

func (s *Service) List(ctx context.Context, page, limit int) ([]sqlc.BroadcastCampaign, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	total, err := s.Q.CountBroadcastCampaigns(ctx)
	if err != nil {
		return nil, 0, err
	}
	offset64 := int64(page-1) * int64(limit)
	if offset64 > math.MaxInt32 {
		return nil, 0, fmt.Errorf("page out of range")
	}
	items, err := s.Q.ListBroadcastCampaigns(ctx, sqlc.ListBroadcastCampaignsParams{
		Limit:  int32(limit),
		Offset: int32(offset64),
	})
	return items, int(total), err
}

func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (sqlc.BroadcastCampaign, error) {
	camp, err := s.Q.CancelBroadcastCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.BroadcastCampaign{}, ErrInvalidCancel
	}
	return camp, err
}

// Retract removes in-app notifications for a campaign and stops further delivery.
// Web push already shown by the OS cannot be recalled.
func (s *Service) Retract(ctx context.Context, id uuid.UUID) (sqlc.BroadcastCampaign, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.Q.WithTx(tx)

	if _, err := qtx.GetBroadcastCampaignByID(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.BroadcastCampaign{}, ErrNotFound
		}
		return sqlc.BroadcastCampaign{}, err
	}
	if _, err := qtx.DeleteInappNotificationsByCampaign(ctx, uuid.NullUUID{UUID: id, Valid: true}); err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	if _, err := qtx.FailPendingRecipientsForCampaign(ctx, id); err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	camp, err := qtx.RetractBroadcastCampaign(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.BroadcastCampaign{}, ErrNotFound
		}
		return sqlc.BroadcastCampaign{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.BroadcastCampaign{}, err
	}
	return camp, nil
}

// ProcessOnce runs one reclaim/expand/claim cycle (used by in-API worker and CLI).
func (s *Service) ProcessOnce(ctx context.Context) error {
	if err := s.reclaimStaleProcessing(ctx); err != nil {
		return err
	}
	if err := s.expandQueued(ctx); err != nil {
		return err
	}
	return s.processRecipients(ctx)
}

// reclaimStaleProcessing returns crash-abandoned "processing" rows to the
// claimable pool. At-least-once delivery is safe because in-app inserts are
// idempotent on (campaign_id, profile_id).
func (s *Service) reclaimStaleProcessing(ctx context.Context) error {
	leaseMs := s.Cfg.processingLease().Milliseconds()
	if leaseMs < 1 {
		leaseMs = 1
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE broadcast_recipient
		SET status = CASE
		      WHEN attempt_count >= $2 THEN 'failed'
		      ELSE 'pending'
		    END,
		    next_attempt_at = now(),
		    last_error = CASE
		      WHEN attempt_count >= $2 THEN 'stale processing; max attempts exceeded'
		      ELSE 'stale processing lease expired'
		    END,
		    updated_at = now(),
		    processed_at = CASE
		      WHEN attempt_count >= $2 THEN COALESCE(processed_at, now())
		      ELSE processed_at
		    END
		WHERE status = 'processing'
		  AND updated_at < now() - ($1::bigint * interval '1 millisecond')`,
		leaseMs, maxAttempts)
	return err
}

func (s *Service) expandQueued(ctx context.Context) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, audience
		FROM broadcast_campaign
		WHERE status IN ('queued', 'expanding')
		ORDER BY created_at
		LIMIT 5`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id       uuid.UUID
		audience string
	}
	var camps []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.audience); err != nil {
			return err
		}
		camps = append(camps, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range camps {
		if err := s.expandOne(ctx, c.id, c.audience); err != nil {
			_, _ = s.Pool.Exec(ctx, `
				UPDATE broadcast_campaign
				SET status = 'failed', error_summary = $2, finished_at = now()
				WHERE id = $1 AND status IN ('queued', 'expanding')`,
				c.id, truncateErr(err))
			return err
		}
	}
	return nil
}

func (s *Service) expandOne(ctx context.Context, campaignID uuid.UUID, audience string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE broadcast_campaign
		SET status = 'expanding', started_at = COALESCE(started_at, now())
		WHERE id = $1 AND status IN ('queued', 'expanding')`, campaignID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	var status string
	if err := s.Pool.QueryRow(ctx, `SELECT status FROM broadcast_campaign WHERE id = $1`, campaignID).Scan(&status); err != nil {
		return err
	}
	if status == "cancelled" {
		return nil
	}

	cursor := uuid.Nil
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.Pool.QueryRow(ctx, `SELECT status FROM broadcast_campaign WHERE id = $1`, campaignID).Scan(&status); err != nil {
			return err
		}
		if status == "cancelled" {
			return nil
		}
		ids, err := ListAudiencePage(ctx, s.Pool, audience, cursor, expandPageSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		if total+len(ids) > s.Cfg.maxRecipients() {
			return fmt.Errorf("%w during expand", ErrTooMany)
		}
		batch := &pgx.Batch{}
		for _, id := range ids {
			batch.Queue(`
				INSERT INTO broadcast_recipient (campaign_id, profile_id, status, push_status)
				VALUES ($1, $2, 'pending', 'pending')
				ON CONFLICT (campaign_id, profile_id) DO NOTHING`, campaignID, id)
		}
		br := s.Pool.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			return err
		}
		total += len(ids)
		cursor = ids[len(ids)-1]
		if len(ids) < expandPageSize {
			break
		}
	}

	_, err = s.Pool.Exec(ctx, `
		UPDATE broadcast_campaign c
		SET recipient_total = COALESCE(r.cnt, 0),
		    pending_count = COALESCE(r.pending, 0),
		    status = CASE
		      WHEN c.status = 'cancelled' THEN 'cancelled'
		      WHEN COALESCE(r.cnt, 0) = 0 THEN 'completed'
		      ELSE 'sending'
		    END,
		    finished_at = CASE
		      WHEN c.status = 'cancelled' OR COALESCE(r.cnt, 0) = 0 THEN COALESCE(c.finished_at, now())
		      ELSE c.finished_at
		    END
		FROM (
		  SELECT
		    count(*)::int AS cnt,
		    count(*) FILTER (WHERE status IN ('pending', 'processing', 'failed'))::int AS pending
		  FROM broadcast_recipient
		  WHERE campaign_id = $1
		) r
		WHERE c.id = $1 AND c.status IN ('expanding', 'sending', 'cancelled')`, campaignID)
	return err
}

type claimRow struct {
	ID         uuid.UUID
	CampaignID uuid.UUID
	ProfileID  uuid.UUID
	Attempts   int
}

func (s *Service) processRecipients(ctx context.Context) error {
	rows, err := s.Pool.Query(ctx, `
		UPDATE broadcast_recipient
		SET status = 'processing',
		    attempt_count = attempt_count + 1,
		    updated_at = now()
		WHERE id IN (
		  SELECT r.id
		  FROM broadcast_recipient r
		  JOIN broadcast_campaign c ON c.id = r.campaign_id
		  WHERE r.status IN ('pending', 'failed')
		    AND r.next_attempt_at <= now()
		    AND c.status = 'sending'
		    AND r.attempt_count < $2
		  ORDER BY r.created_at
		  FOR UPDATE OF r SKIP LOCKED
		  LIMIT $1
		)
		RETURNING id, campaign_id, profile_id, attempt_count`, claimBatchSize, maxAttempts)
	if err != nil {
		return err
	}
	defer rows.Close()
	var claims []claimRow
	for rows.Next() {
		var c claimRow
		if err := rows.Scan(&c.ID, &c.CampaignID, &c.ProfileID, &c.Attempts); err != nil {
			return err
		}
		claims = append(claims, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range claims {
		if err := s.deliverOne(ctx, c); err != nil {
			_ = s.failRecipient(ctx, c, err)
		}
	}
	return s.refreshCampaignStats(ctx)
}

func (s *Service) deliverOne(ctx context.Context, c claimRow) error {
	var title, body, imageURL, actionURL, channels, status string
	err := s.Pool.QueryRow(ctx, `
		SELECT title, body, image_url, action_url, channels, status
		FROM broadcast_campaign WHERE id = $1`, c.CampaignID).
		Scan(&title, &body, &imageURL, &actionURL, &channels, &status)
	if err != nil {
		return err
	}
	if status == "cancelled" {
		_, err = s.Pool.Exec(ctx, `
			UPDATE broadcast_recipient
			SET status = 'failed', last_error = 'campaign cancelled', updated_at = now(), processed_at = now()
			WHERE id = $1`, c.ID)
		return err
	}

	payload := map[string]any{
		"title": title,
		"body":  body,
		"kind":  KindSupportBroadcast,
	}
	if imageURL != "" {
		payload["image_url"] = imageURL
	}
	url := actionURL
	if url == "" {
		url = "/uz-Latn/dashboard"
	}
	payload["url"] = url
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	notifID, err := s.ensureInapp(ctx, c.CampaignID, c.ProfileID, raw)
	if err != nil {
		return err
	}

	pushStatus := "skipped"
	if channels == ChannelsBoth {
		if s.Push == nil || !s.Push.Cfg.Configured() {
			pushStatus = "failed"
		} else {
			sent, pushErr := s.Push.DeliverToProfile(ctx, c.ProfileID, push.NotifyPayload{
				Title: title,
				Body:  body,
				URL:   url,
				Data:  map[string]any{"kind": KindSupportBroadcast, "campaign_id": c.CampaignID.String()},
			})
			switch {
			case errors.Is(pushErr, push.ErrNoSubs), errors.Is(pushErr, push.ErrUnconfigured):
				if errors.Is(pushErr, push.ErrNoSubs) {
					pushStatus = "no_subscription"
				} else {
					pushStatus = "failed"
				}
			case pushErr != nil:
				pushStatus = "failed"
			case sent > 0:
				pushStatus = "sent"
			default:
				pushStatus = "no_subscription"
			}
		}
	}

	_, err = s.Pool.Exec(ctx, `
		UPDATE broadcast_recipient
		SET status = 'sent',
		    notification_id = $2,
		    push_status = $3,
		    last_error = '',
		    updated_at = now(),
		    processed_at = now()
		WHERE id = $1`, c.ID, notifID, pushStatus)
	return err
}

func (s *Service) ensureInapp(ctx context.Context, campaignID, profileID uuid.UUID, raw []byte) (uuid.UUID, error) {
	existing, err := s.Q.GetInappNotificationByCampaignProfile(ctx, sqlc.GetInappNotificationByCampaignProfileParams{
		CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
		ProfileID:  profileID,
	})
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	row, err := s.Q.InsertInappNotification(ctx, sqlc.InsertInappNotificationParams{
		ProfileID:  profileID,
		Kind:       KindSupportBroadcast,
		Payload:    raw,
		CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
	})
	if err != nil {
		// Race: unique index — fetch winner.
		existing, getErr := s.Q.GetInappNotificationByCampaignProfile(ctx, sqlc.GetInappNotificationByCampaignProfileParams{
			CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			ProfileID:  profileID,
		})
		if getErr == nil {
			return existing.ID, nil
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (s *Service) failRecipient(ctx context.Context, c claimRow, deliverErr error) error {
	delay := backoff(c.Attempts)
	nextStatus := "failed"
	finished := c.Attempts >= maxAttempts
	_, err := s.Pool.Exec(ctx, `
		UPDATE broadcast_recipient
		SET status = $2,
		    last_error = $3,
		    next_attempt_at = $4,
		    updated_at = now(),
		    processed_at = CASE WHEN $5 THEN now() ELSE processed_at END
		WHERE id = $1`,
		c.ID, nextStatus, truncateErr(deliverErr), time.Now().UTC().Add(delay), finished)
	return err
}

func (s *Service) refreshCampaignStats(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE broadcast_campaign c
		SET pending_count = COALESCE(r.pending, 0),
		    sent_count = COALESCE(r.sent, 0),
		    failed_count = COALESCE(r.failed_final, 0),
		    push_sent_count = COALESCE(r.push_sent, 0),
		    push_failed_count = COALESCE(r.push_failed, 0),
		    status = CASE
		      WHEN c.status = 'cancelled' THEN 'cancelled'
		      WHEN COALESCE(r.pending, 0) > 0 THEN 'sending'
		      WHEN COALESCE(r.failed_final, 0) > 0 THEN 'completed_with_errors'
		      ELSE 'completed'
		    END,
		    finished_at = CASE
		      WHEN c.status = 'cancelled' THEN COALESCE(c.finished_at, now())
		      WHEN COALESCE(r.pending, 0) = 0 THEN COALESCE(c.finished_at, now())
		      ELSE NULL
		    END
		FROM (
		  SELECT
		    campaign_id,
		    count(*) FILTER (
		      WHERE status IN ('pending', 'processing')
		         OR (status = 'failed' AND attempt_count < $1)
		    )::int AS pending,
		    count(*) FILTER (WHERE status = 'sent')::int AS sent,
		    count(*) FILTER (WHERE status = 'failed' AND attempt_count >= $1)::int AS failed_final,
		    count(*) FILTER (WHERE push_status = 'sent')::int AS push_sent,
		    count(*) FILTER (WHERE push_status = 'failed')::int AS push_failed
		  FROM broadcast_recipient
		  GROUP BY campaign_id
		) r
		WHERE c.id = r.campaign_id
		  AND c.status IN ('sending', 'completed', 'completed_with_errors')`,
		maxAttempts)
	return err
}

func backoff(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 30 * time.Second
	case attempt == 2:
		return 2 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func truncateErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

package push

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/flags"
)

const (
	// KindFSRSDue is the notification.kind for retention digests.
	KindFSRSDue = "fsrs_due"

	// DigestCooldown skips profiles that already received an FSRS due push
	// recently (ops cron is daily; 20h leaves slack for late/early runs).
	DigestCooldown = 20 * time.Hour

	defaultDigestLimit = 500
	maxReviewCount     = 20
)

// DigestCandidate is a profile eligible for an FSRS due retention push.
type DigestCandidate struct {
	ProfileID  uuid.UUID
	LocalePref string
	DueCount   int
}

// DigestOpts controls a digest run.
type DigestOpts struct {
	// Limit caps how many profiles are considered (default 500).
	Limit int
	// DryRun lists candidates but does not call Notify.
	DryRun bool
}

// DigestResult summarizes a digest run for ops logs.
type DigestResult struct {
	Candidates int `json:"candidates"`
	Notified   int `json:"notified"`
	Deliveries int `json:"deliveries"`
	Skipped    int `json:"skipped"`
	Errors     int `json:"errors"`
}

// ListFSRSDueCandidates returns active profiles that have ≥1 due FSRS card
// and ≥1 push subscription, excluding those already notified within DigestCooldown.
func (s *Service) ListFSRSDueCandidates(ctx context.Context, limit int) ([]DigestCandidate, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("push digest requires a pool")
	}
	if limit <= 0 {
		limit = defaultDigestLimit
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, p.locale_pref, COUNT(qm.question_id)::int AS due_count
		FROM profile p
		JOIN push_subscription ps ON ps.profile_id = p.id
		JOIN question_memory qm ON qm.profile_id = p.id AND qm.due_at <= now()
		JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
		WHERE p.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM notification n
		    WHERE n.profile_id = p.id
		      AND n.kind = $1
		      AND n.channel = 'webpush'
		      AND n.created_at > now() - ($2::text)::interval
		  )
		GROUP BY p.id, p.locale_pref
		HAVING COUNT(qm.question_id) > 0
		ORDER BY due_count DESC, p.id
		LIMIT $3`,
		KindFSRSDue,
		fmt.Sprintf("%f seconds", DigestCooldown.Seconds()),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DigestCandidate
	for rows.Next() {
		var c DigestCandidate
		if err := rows.Scan(&c.ProfileID, &c.LocalePref, &c.DueCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RunFSRSDueDigest selects eligible profiles and optionally delivers pushes.
// Per-profile delivery errors are counted; the batch continues.
// Gone/expired endpoints are pruned inside Notify.
func (s *Service) RunFSRSDueDigest(ctx context.Context, opts DigestOpts) (DigestResult, error) {
	if !opts.DryRun {
		enabled, err := flags.Bool(ctx, s.Pool, flags.KeyWebPushDigest, true)
		if err != nil {
			return DigestResult{}, err
		}
		if !enabled {
			return DigestResult{}, ErrFeatureDisabled
		}
	}
	if !opts.DryRun && !s.Cfg.Configured() {
		return DigestResult{}, ErrUnconfigured
	}
	candidates, err := s.ListFSRSDueCandidates(ctx, opts.Limit)
	if err != nil {
		return DigestResult{}, err
	}
	res := DigestResult{Candidates: len(candidates)}
	if opts.DryRun {
		return res, nil
	}
	for _, c := range candidates {
		payload := fsrsDuePayload(c.LocalePref, c.DueCount)
		sent, err := s.Notify(ctx, c.ProfileID, KindFSRSDue, payload)
		if err != nil {
			res.Errors++
			continue
		}
		if sent == 0 {
			res.Skipped++
			continue
		}
		res.Notified++
		res.Deliveries += sent
	}
	return res, nil
}

func fsrsDuePayload(locale string, dueCount int) NotifyPayload {
	locale = normalizeLocale(locale)
	count := dueCount
	if count < 1 {
		count = 1
	}
	if count > maxReviewCount {
		count = maxReviewCount
	}
	title, body := fsrsDueCopy(locale, dueCount)
	return NotifyPayload{
		Title: title,
		Body:  body,
		URL:   fmt.Sprintf("/%s/session/start?mode=review&count=%d", locale, count),
		Data: map[string]any{
			"kind":      KindFSRSDue,
			"due_count": dueCount,
		},
	}
}

func normalizeLocale(locale string) string {
	switch strings.TrimSpace(locale) {
	case "uz-Cyrl", "ru", "uz-Latn":
		return strings.TrimSpace(locale)
	default:
		return "uz-Latn"
	}
}

func fsrsDueCopy(locale string, dueCount int) (title, body string) {
	switch locale {
	case "ru":
		title = "Время повторения"
		body = fmt.Sprintf("У вас %d вопрос(ов) к повторению. Умное повторение вернёт слабые места вовремя.", dueCount)
	case "uz-Cyrl":
		title = "Такрорлаш вақти"
		body = fmt.Sprintf("Сизда %d та савол такрорлаш учун тайёр. Ақлли такрорлаш билан машқни очинг.", dueCount)
	default:
		title = "Takrorlash vaqti"
		body = fmt.Sprintf("Sizda %d ta savol takrorlash uchun tayyor. Aqlli takrorlash bilan mashqni oching.", dueCount)
	}
	return title, body
}

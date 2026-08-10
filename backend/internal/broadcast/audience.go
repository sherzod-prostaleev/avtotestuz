package broadcast

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DryRunCounts is the result of a read-only audience count.
type DryRunCounts struct {
	Recipients   int `json:"recipients"`
	PushEligible int `json:"push_eligible"`
}

func audienceVIPClause(audience string) (string, error) {
	switch audience {
	case AudienceAllActive:
		return "", nil
	case AudienceVIP:
		return `AND EXISTS (
			SELECT 1 FROM entitlement e
			WHERE e.profile_id = p.id AND e.ends_at > now()
		)`, nil
	case AudienceNonVIP:
		return `AND NOT EXISTS (
			SELECT 1 FROM entitlement e
			WHERE e.profile_id = p.id AND e.ends_at > now()
		)`, nil
	default:
		return "", fmt.Errorf("unknown audience %q", audience)
	}
}

// CountAudience returns active user profiles matching audience + push-eligible subset.
func CountAudience(ctx context.Context, pool *pgxpool.Pool, audience string) (DryRunCounts, error) {
	vip, err := audienceVIPClause(audience)
	if err != nil {
		return DryRunCounts{}, err
	}
	q := fmt.Sprintf(`
		SELECT
		  COUNT(*)::int AS recipients,
		  COUNT(*) FILTER (
		    WHERE EXISTS (
		      SELECT 1 FROM push_subscription ps WHERE ps.profile_id = p.id
		    )
		  )::int AS push_eligible
		FROM profile p
		WHERE p.status = 'active' AND p.kind = 'user'
		%s`, vip)
	var out DryRunCounts
	err = pool.QueryRow(ctx, q).Scan(&out.Recipients, &out.PushEligible)
	return out, err
}

// ListAudiencePage returns profile IDs after cursor (exclusive), ordered by id.
func ListAudiencePage(ctx context.Context, pool *pgxpool.Pool, audience string, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 500
	}
	vip, err := audienceVIPClause(audience)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(`
		SELECT p.id
		FROM profile p
		WHERE p.status = 'active' AND p.kind = 'user'
		  AND p.id > $1
		  %s
		ORDER BY p.id
		LIMIT $2`, vip)
	rows, err := pool.Query(ctx, q, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

package admin

import (
	"context"
	"fmt"
	"time"
)

// AnalyticsOverview is honest SQL aggregates — not a BI funnel product.
type AnalyticsOverview struct {
	GeneratedAt           time.Time            `json:"generated_at"`
	ProfilesTotal         int64                `json:"profiles_total"`
	ProfilesCreated24h    int64                `json:"profiles_created_24h"`
	ProfilesCreated7d     int64                `json:"profiles_created_7d"`
	PaymentsPaidTotal     int64                `json:"payments_paid_total"`
	PaymentsPaid7d        int64                `json:"payments_paid_7d"`
	RevenuePaidUzsTotal   int64                `json:"revenue_paid_uzs_total"`
	RevenuePaidUzs7d      int64                `json:"revenue_paid_uzs_7d"`
	EntitlementsActive    int64                `json:"entitlements_active"`
	EventsLast7d          int64                `json:"events_last_7d"`
	TopEventNames7d       []AnalyticsNameCount `json:"top_event_names_7d"`
	SignupsByDay14d       []AnalyticsDayCount  `json:"signups_by_day_14d"`
	RevenueByDay14d       []AnalyticsDayCount  `json:"revenue_by_day_14d"`
	Note                  string               `json:"note"`
}

// AnalyticsDayCount is a calendar-day series point for charts.
type AnalyticsDayCount struct {
	Day   string `json:"day"` // YYYY-MM-DD UTC
	Count int64  `json:"count"`
}

// AnalyticsNameCount is a simple name→count tile.
type AnalyticsNameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// GetAnalyticsOverview returns product/revenue tiles from live tables.
func (s Store) GetAnalyticsOverview(ctx context.Context) (AnalyticsOverview, error) {
	out := AnalyticsOverview{
		GeneratedAt:     time.Now().UTC(),
		Note:            "Honest SQL aggregates from profile/payment/entitlement/event. Not MRR/churn/LTV BI.",
		TopEventNames7d: []AnalyticsNameCount{},
		SignupsByDay14d: []AnalyticsDayCount{},
		RevenueByDay14d: []AnalyticsDayCount{},
	}
	if s.Pool == nil {
		return out, fmt.Errorf("analytics: nil pool")
	}

	err := s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM profile),
		  (SELECT COUNT(*) FROM profile WHERE created_at >= now() - interval '24 hours'),
		  (SELECT COUNT(*) FROM profile WHERE created_at >= now() - interval '7 days'),
		  (SELECT COUNT(*) FROM payment WHERE status = 'paid'),
		  (SELECT COUNT(*) FROM payment WHERE status = 'paid' AND COALESCE(paid_at, created_at) >= now() - interval '7 days'),
		  (SELECT COALESCE(SUM(amount_uzs), 0) FROM payment WHERE status = 'paid'),
		  (SELECT COALESCE(SUM(amount_uzs), 0) FROM payment WHERE status = 'paid' AND COALESCE(paid_at, created_at) >= now() - interval '7 days'),
		  (SELECT COUNT(*) FROM entitlement WHERE starts_at <= now() AND ends_at > now()),
		  (SELECT COUNT(*) FROM event WHERE ts >= now() - interval '7 days')
	`).Scan(
		&out.ProfilesTotal,
		&out.ProfilesCreated24h,
		&out.ProfilesCreated7d,
		&out.PaymentsPaidTotal,
		&out.PaymentsPaid7d,
		&out.RevenuePaidUzsTotal,
		&out.RevenuePaidUzs7d,
		&out.EntitlementsActive,
		&out.EventsLast7d,
	)
	if err != nil {
		return out, fmt.Errorf("analytics overview: %w", err)
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT name, COUNT(*)::bigint AS c
		FROM event
		WHERE ts >= now() - interval '7 days'
		GROUP BY name
		ORDER BY c DESC, name ASC
		LIMIT 8`)
	if err != nil {
		return out, fmt.Errorf("analytics events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row AnalyticsNameCount
		if err := rows.Scan(&row.Name, &row.Count); err != nil {
			return out, err
		}
		out.TopEventNames7d = append(out.TopEventNames7d, row)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	signupRows, err := s.Pool.Query(ctx, `
		SELECT to_char(d::date, 'YYYY-MM-DD') AS day, COUNT(p.id)::bigint
		FROM generate_series(
		  (timezone('UTC', now()))::date - 13,
		  (timezone('UTC', now()))::date,
		  interval '1 day'
		) AS d
		LEFT JOIN profile p ON p.created_at::date = d::date
		GROUP BY d
		ORDER BY d`)
	if err != nil {
		return out, fmt.Errorf("analytics signups series: %w", err)
	}
	defer signupRows.Close()
	for signupRows.Next() {
		var row AnalyticsDayCount
		if err := signupRows.Scan(&row.Day, &row.Count); err != nil {
			return out, err
		}
		out.SignupsByDay14d = append(out.SignupsByDay14d, row)
	}
	if err := signupRows.Err(); err != nil {
		return out, err
	}

	revRows, err := s.Pool.Query(ctx, `
		SELECT to_char(d::date, 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(pay.amount_uzs), 0)::bigint
		FROM generate_series(
		  (timezone('UTC', now()))::date - 13,
		  (timezone('UTC', now()))::date,
		  interval '1 day'
		) AS d
		LEFT JOIN payment pay
		  ON pay.status = 'paid'
		 AND COALESCE(pay.paid_at, pay.created_at)::date = d::date
		GROUP BY d
		ORDER BY d`)
	if err != nil {
		return out, fmt.Errorf("analytics revenue series: %w", err)
	}
	defer revRows.Close()
	for revRows.Next() {
		var row AnalyticsDayCount
		if err := revRows.Scan(&row.Day, &row.Count); err != nil {
			return out, err
		}
		out.RevenueByDay14d = append(out.RevenueByDay14d, row)
	}
	return out, revRows.Err()
}

package billing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PANMigrationResult struct {
	ManualCards     int
	ReferralPayouts int
}

type storedPANRow struct {
	ID    uuid.UUID
	Value string
}

// EncryptStoredPANs rewrites legacy plaintext PAN fields in one transaction.
// It never deletes rows and is idempotent: enc:v1 envelopes are skipped.
func EncryptStoredPANs(ctx context.Context, pool *pgxpool.Pool, secret []byte) (PANMigrationResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return PANMigrationResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `LOCK TABLE manual_pay_card, referral_payout IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return PANMigrationResult{}, err
	}

	load := func(query string) ([]storedPANRow, error) {
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []storedPANRow
		for rows.Next() {
			var row storedPANRow
			if err := rows.Scan(&row.ID, &row.Value); err != nil {
				return nil, err
			}
			if PANNeedsEncryption(row.Value) {
				out = append(out, row)
			}
		}
		return out, rows.Err()
	}

	manual, err := load(`SELECT id, pan_full FROM manual_pay_card ORDER BY id FOR UPDATE`)
	if err != nil {
		return PANMigrationResult{}, err
	}
	payouts, err := load(`SELECT id, card_number FROM referral_payout ORDER BY id FOR UPDATE`)
	if err != nil {
		return PANMigrationResult{}, err
	}
	svc := Service{Secret: secret}
	result := PANMigrationResult{}
	for _, row := range manual {
		ciphertext, last4, err := svc.EncryptPAN(row.Value)
		if err != nil {
			return PANMigrationResult{}, fmt.Errorf("manual card %s: %w", row.ID, err)
		}
		tag, err := tx.Exec(ctx, `UPDATE manual_pay_card SET pan_full=$2, pan_last4=$3, updated_at=now() WHERE id=$1`, row.ID, ciphertext, last4)
		if err != nil {
			return PANMigrationResult{}, fmt.Errorf("manual card %s update: %w", row.ID, err)
		}
		if affected := tag.RowsAffected(); affected != 1 {
			return PANMigrationResult{}, fmt.Errorf("manual card %s update: affected=%d", row.ID, affected)
		}
		result.ManualCards++
	}
	for _, row := range payouts {
		ciphertext, _, err := svc.EncryptPAN(row.Value)
		if err != nil {
			return PANMigrationResult{}, fmt.Errorf("referral payout %s: %w", row.ID, err)
		}
		tag, err := tx.Exec(ctx, `UPDATE referral_payout SET card_number=$2 WHERE id=$1`, row.ID, ciphertext)
		if err != nil {
			return PANMigrationResult{}, fmt.Errorf("referral payout %s update: %w", row.ID, err)
		}
		if affected := tag.RowsAffected(); affected != 1 {
			return PANMigrationResult{}, fmt.Errorf("referral payout %s update: affected=%d", row.ID, affected)
		}
		result.ReferralPayouts++
	}
	if err := tx.Commit(ctx); err != nil {
		return PANMigrationResult{}, err
	}
	return result, nil
}

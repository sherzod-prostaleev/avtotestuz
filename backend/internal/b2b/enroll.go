package b2b

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// EnrollInput is one classroom PC presenting an org code and the public half
// of a keypair it generated locally. The private half never leaves that
// machine, which is what makes the binding uncopyable.
type EnrollInput struct {
	Code         string
	PublicKey    ed25519.PublicKey
	HWIDHash     string
	Label        string
	AgentVersion string
}

// EnrollResult is what the agent persists after a successful enrollment.
type EnrollResult struct {
	StationID     uuid.UUID `json:"station_id"`
	OrgID         uuid.UUID `json:"org_id"`
	ProfileID     uuid.UUID `json:"-"`
	Label         string    `json:"label"`
	LicenseEndsAt time.Time `json:"license_ends_at"`
}

// maxLabelLen keeps operator-supplied PC names from bloating list responses.
const maxLabelLen = 64

// hwidHashLen is the hex-encoded length of a SHA-256 digest (32 bytes -> 64
// hex chars). The station agent derives hwid_hash by hashing local machine
// identifiers; anything else is not a hardware identity we can trust.
const hwidHashLen = 64

func (in EnrollInput) validate() error {
	if strings.TrimSpace(in.Code) == "" {
		return fmt.Errorf("%w: code required", ErrInvalid)
	}
	if len(in.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: public_key must be %d bytes", ErrInvalid, ed25519.PublicKeySize)
	}
	if len(in.HWIDHash) != hwidHashLen || strings.ToLower(in.HWIDHash) != in.HWIDHash {
		return fmt.Errorf("%w: hwid_hash must be a %d-character lowercase sha256 hex digest", ErrInvalid, hwidHashLen)
	}
	if _, err := hex.DecodeString(in.HWIDHash); err != nil {
		return fmt.Errorf("%w: hwid_hash must be hex-encoded", ErrInvalid)
	}
	return nil
}

// EnrollStation binds one machine to an org under a live enrollment window.
//
// The whole check-and-insert runs inside one transaction that takes
// `SELECT ... FROM b2b_org FOR UPDATE` first. That lock is not decoration:
// seat accounting reads b2b_station and writes a new row, two statements with
// no row in common, so under READ COMMITTED a GPO rollout starting 100 agents
// at once would have every one of them read the same pre-rollout count and
// sail past a 30-seat cap. Serializing on the org row is what makes the cap
// hold.
func (s Store) EnrollStation(ctx context.Context, in EnrollInput) (EnrollResult, error) {
	if err := in.validate(); err != nil {
		return EnrollResult{}, err
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	hwid := strings.TrimSpace(in.HWIDHash)
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = "PC"
	}
	if utf8.RuneCountInString(label) > maxLabelLen {
		// Truncate on runes, not bytes: Cyrillic labels (Uzbek/Russian PC
		// names) are 2 bytes per rune, so byte slicing can cut mid-rune and
		// produce invalid UTF-8 that the profile insert then rejects.
		runes := []rune(label)
		label = string(runes[:maxLabelLen])
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EnrollResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		codeID    uuid.UUID
		orgID     uuid.UUID
		maxUses   int
		usedCount int
	)
	err = tx.QueryRow(ctx, `
		SELECT id, org_id, max_uses, used_count
		FROM b2b_org_enroll_code
		WHERE code = $1 AND revoked_at IS NULL AND expires_at > now()`,
		code).Scan(&codeID, &orgID, &maxUses, &usedCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollResult{}, ErrNotFound
		}
		return EnrollResult{}, err
	}

	// Lock the org before any seat arithmetic.
	var orgStatus string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&orgStatus); err != nil {
		return EnrollResult{}, err
	}
	if orgStatus != "active" {
		return EnrollResult{}, ErrOrgSuspended
	}

	// Re-read the counter under the lock; a concurrent enroll may have used
	// it. Re-check revoked_at/expires_at too: under a stampede a transaction
	// can sit blocked on the org lock for a long time, and a teacher
	// revoking a leaked code -- the emergency stop -- must still cut off
	// every transaction already queued behind the lock, not just future ones.
	if err := tx.QueryRow(ctx, `
		SELECT used_count FROM b2b_org_enroll_code
		WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`, codeID).Scan(&usedCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollResult{}, ErrNotFound
		}
		return EnrollResult{}, err
	}
	if usedCount >= maxUses {
		return EnrollResult{}, ErrCodeExhausted
	}

	var seats int64
	var licenseEnds time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0), COALESCE(MAX(ends_at), now())
		FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats, &licenseEnds)
	if err != nil {
		return EnrollResult{}, err
	}
	if seats <= 0 {
		return EnrollResult{}, ErrNoLicense
	}

	// Re-imaging the same machine reuses its seat instead of burning a new
	// one -- but only within the enrolling org. hwid_hash is not a secret
	// (every agent transmits it), so a valid code for org A must never let
	// it silently revoke and take over a PC bound to org B: that's a
	// cross-tenant denial-of-service with no audit trail.
	revokeTag, err := tx.Exec(ctx, `
		UPDATE b2b_station SET status = 'revoked'
		WHERE hwid_hash = $1 AND status = 'active' AND org_id = $2`, hwid, orgID)
	if err != nil {
		return EnrollResult{}, err
	}
	if revokeTag.RowsAffected() == 0 {
		// No active binding in this org. If the hwid is actively bound to a
		// DIFFERENT org, refuse rather than silently transferring it --
		// a deliberate cross-org rebind is a support action, not something
		// any code-holder can trigger.
		var crossOrgActive bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM b2b_station WHERE hwid_hash = $1 AND status = 'active'
			)`, hwid).Scan(&crossOrgActive); err != nil {
			return EnrollResult{}, err
		}
		if crossOrgActive {
			return EnrollResult{}, ErrConflict
		}
	}

	var active int64
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&active); err != nil {
		return EnrollResult{}, err
	}
	if active >= seats {
		return EnrollResult{}, ErrSeatsExhausted
	}

	var profileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), $1, 'station')
		RETURNING id`, label).Scan(&profileID); err != nil {
		return EnrollResult{}, fmt.Errorf("create shadow profile: %w", err)
	}

	var stationID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO b2b_station
		  (org_id, public_key, hwid_hash, label, status, activated_by, agent_version, station_profile_id)
		VALUES ($1, $2, $3, $4, 'active', 'enroll', $5, $6)
		RETURNING id`,
		orgID, []byte(in.PublicKey), hwid, label, in.AgentVersion, profileID).Scan(&stationID); err != nil {
		if isUniqueViolation(err) {
			// A concurrent enrollment of the same hwid_hash under a
			// different org isn't serialized by the org lock (different
			// orgs, different lock rows) and can race past the cross-org
			// check above; the global partial unique index is the last
			// line of defense. Map it to the same conflict the check above
			// returns instead of leaking a raw pgx 23505 as a 500.
			return EnrollResult{}, ErrConflict
		}
		return EnrollResult{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET used_count = used_count + 1 WHERE id = $1`, codeID); err != nil {
		return EnrollResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollResult{}, err
	}

	return EnrollResult{
		StationID:     stationID,
		OrgID:         orgID,
		ProfileID:     profileID,
		Label:         label,
		LicenseEndsAt: licenseEnds.UTC(),
	}, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), mirroring internal/auth/service.go's helper of
// the same name (unexported, package-local -- no shared identifier to collide).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

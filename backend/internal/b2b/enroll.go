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

// staleSeatAfter is how long a station must have been silent before a new
// enrollment on the same machine may take its seat.
//
// A running classroom PC renews its token every 15 minutes at the outside
// (stationTokenTTL less keepTokenWarm's margin) and touches last_seen_at on
// every renewal, so a gap this long means the row describes a PC that is off,
// gone, or re-imaged -- never one with a student sitting at it. That is the
// whole point: seat reclamation must be unable to interrupt a lesson, which
// is exactly what the old unconditional revoke did.
const staleSeatAfter = 30 * time.Minute

// reclaimStaleSeat frees one seat by revoking the least recently seen station
// in this org that shares hwid and has been silent for staleSeatAfter. It
// reports whether a seat was actually freed.
//
// Scoped to a single hwid on purpose: a full licence is not a licence to
// revoke whichever PC happens to look idlest, only to let a machine that is
// being re-imaged step back into the seat it already had. The caller holds
// the org row lock, so the count it read stays true through this update.
func reclaimStaleSeat(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, hwid string) (bool, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE b2b_station SET status = 'revoked'
		WHERE id = (
			SELECT id FROM b2b_station
			WHERE org_id = $1 AND hwid_hash = $2 AND status = 'active'
			  AND last_seen_at < now() - make_interval(secs => $3)
			ORDER BY last_seen_at ASC
			LIMIT 1
		)`, orgID, hwid, staleSeatAfter.Seconds())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// maxAgentVersionLen bounds the attacker-controlled agent_version string
// (accepted on both /b2b/stations/enroll and /b2b/stations/token) before it
// reaches the otherwise-unbounded text column. A future version string
// longer than this is not worth failing an enrollment or a token renewal
// over, so it is truncated rather than rejected -- see truncateRunes.
const maxAgentVersionLen = 32

// hwidHashLen is the hex-encoded length of a SHA-256 digest (32 bytes -> 64
// hex chars). The station agent derives hwid_hash by hashing local machine
// identifiers; anything else is not a hardware identity we can trust.
const hwidHashLen = 64

// truncateRunes caps s at maxLen runes, truncating on rune boundaries rather
// than bytes: Cyrillic input (Uzbek/Russian PC names, agent version tags)
// is 2 bytes per rune, so a byte-slice cut can land mid-rune and produce
// invalid UTF-8 that a later insert then rejects.
func truncateRunes(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen])
}

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
	label = truncateRunes(label, maxLabelLen)
	agentVersion := truncateRunes(strings.TrimSpace(in.AgentVersion), maxAgentVersionLen)

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
	// can sit blocked on the org lock for a long time, and an admin
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

	// Enrollment revokes nothing outright, and an hwid already bound to
	// another org is no reason to refuse.
	//
	// It used to do both. The theory was that one hwid_hash meant one physical
	// machine, so an hwid arriving again had to be that machine being
	// re-imaged: revoke the old row, reuse its seat, and treat the same hwid
	// under a different org as a cross-tenant takeover attempt. Windows does
	// not cooperate. hwid_hash is sha256 of MachineGuid, which sysprep
	// regenerates but a raw disk clone does not -- and a raw disk clone is
	// how a room of fifty identical PCs is actually built. On 2026-08-26 a
	// 55-seat school enrolled 43 stations presenting six hwids between them:
	// every install silently revoked the PC installed before it, and a revoked
	// station loses VIP the instant it loses status = 'active' (ActiveStationVIP
	// requires it), dropping a whole classroom to the 30-question free tier.
	// Six PCs survived -- one per master image.
	//
	// Dropping the revoke also removes the reason the cross-org check existed:
	// that check protected org B from having its PC revoked out from under it
	// by anyone holding org A's code, and nothing here revokes across orgs
	// anymore. Two schools that bought from the same shop now enrol the same
	// image without one of them being refused.
	//
	// What still cannot happen is one machine quietly eating a licence: seats
	// are counted below, and a seat is only ever reclaimed from a station that
	// has stopped talking to us.
	var active int64
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&active); err != nil {
		return EnrollResult{}, err
	}
	if active >= seats {
		// Full. Before refusing, try to reclaim the seat of a station that
		// shares this machine's hwid and has gone quiet -- the genuine
		// re-image, where the old row describes a PC that no longer exists.
		// A classroom PC that is switched on renews its token every 15
		// minutes (keepTokenWarm), so a live one can never be picked.
		reclaimed, err := reclaimStaleSeat(ctx, tx, orgID, hwid)
		if err != nil {
			return EnrollResult{}, err
		}
		if !reclaimed {
			return EnrollResult{}, ErrSeatsExhausted
		}
	}

	// bypass_variant_progress is true for every station.
	//
	// A learner unlocks bilet N+1 by finishing bilet N, which works because
	// the progress belongs to one person. A classroom PC has one profile and
	// thirty students: the first student's progress would decide which bilets
	// the next twenty-nine are allowed to open, and a school that has paid for
	// the whole catalogue would find most of it shut. The flag already exists
	// for exactly this "progress gating does not apply to this account" case
	// (QA/ops), and IsVariantUnlocked still requires an active entitlement, so
	// this opens the catalogue to licensed classrooms only -- not to anyone
	// who points an agent at the API.
	var profileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind, bypass_variant_progress)
		VALUES ('st:' || gen_random_uuid(), $1, 'station', TRUE)
		RETURNING id`, label).Scan(&profileID); err != nil {
		return EnrollResult{}, fmt.Errorf("create shadow profile: %w", err)
	}

	var stationID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO b2b_station
		  (org_id, public_key, hwid_hash, label, status, activated_by, agent_version, station_profile_id)
		VALUES ($1, $2, $3, $4, 'active', 'enroll', $5, $6)
		RETURNING id`,
		orgID, []byte(in.PublicKey), hwid, label, agentVersion, profileID).Scan(&stationID); err != nil {
		if isUniqueViolation(err) {
			// b2b_station_active_hwid_key_uidx: this exact installation --
			// same machine, same keypair -- already holds an active row.
			// Unreachable in normal operation, because the agent only
			// enrolls when it has no saved station id (see connect.go), and
			// a keypair is generated per installation. It survives as the
			// last line of defence against a concurrent double-enroll, which
			// the org lock does not serialize when the two calls arrive under
			// different orgs. Map it rather than leak a raw pgx 23505 as a 500.
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

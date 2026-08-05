package b2b

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/auth"
)

// ErrStationAuth is the single error every station-login failure returns.
// The reasons (bad signature, wrong hardware, replayed nonce, revoked
// station) are deliberately indistinguishable to the caller: telling an
// attacker which half of the credential was wrong is free reconnaissance.
var ErrStationAuth = errors.New("station auth failed")

const (
	// stationNonceTTL bounds how long a challenge stays answerable.
	stationNonceTTL = time.Minute
	// stationTokenTTL is short because renewal is free for a live agent and
	// expensive for anyone holding a stolen token.
	stationTokenTTL = 15 * time.Minute
	// stationClockSkew is how far the agent's clock may drift from ours.
	stationClockSkew = 2 * time.Minute
	// signedMessagePrefix domain-separates these signatures from any other
	// use of the same key.
	signedMessagePrefix = "avtotest-station-v1"
)

// StationAuth verifies station identity and mints access tokens.
type StationAuth struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Secret []byte
}

// ChallengeResult is a one-shot nonce for the agent to sign.
type ChallengeResult struct {
	Nonce     string `json:"nonce"`
	ExpiresIn int    `json:"expires_in"`
}

// TokenInput is a signed challenge answer.
type TokenInput struct {
	StationID    uuid.UUID
	Nonce        string
	TS           int64
	Sig          []byte
	HWIDHash     string
	AgentVersion string
	IP           string
}

// TokenResult is a live station session.
type TokenResult struct {
	AccessToken   string    `json:"access_token"`
	ExpiresIn     int       `json:"expires_in"`
	LicenseEndsAt time.Time `json:"license_ends_at"`
	OrgName       string    `json:"org_name"`
}

// SignedMessage builds the exact bytes both the server and the agent sign.
// Any change here breaks every deployed agent, so it is versioned by prefix.
func SignedMessage(stationID uuid.UUID, nonce string, ts int64) []byte {
	return []byte(signedMessagePrefix + "|" + stationID.String() + "|" + nonce + "|" + strconv.FormatInt(ts, 10))
}

func nonceKey(stationID uuid.UUID, nonce string) string {
	return "station:nonce:" + stationID.String() + ":" + nonce
}

// Challenge issues a nonce bound to one station.
func (a StationAuth) Challenge(ctx context.Context, stationID uuid.UUID) (ChallengeResult, error) {
	if stationID == uuid.Nil {
		return ChallengeResult{}, ErrStationAuth
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return ChallengeResult{}, err
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)
	if err := a.Redis.Set(ctx, nonceKey(stationID, nonce), "1", stationNonceTTL).Err(); err != nil {
		return ChallengeResult{}, fmt.Errorf("store station nonce: %w", err)
	}
	return ChallengeResult{Nonce: nonce, ExpiresIn: int(stationNonceTTL.Seconds())}, nil
}

// Token verifies a signed challenge and mints a station access token.
func (a StationAuth) Token(ctx context.Context, in TokenInput) (TokenResult, error) {
	if in.StationID == uuid.Nil || in.Nonce == "" || len(in.Sig) != ed25519.SignatureSize {
		return TokenResult{}, ErrStationAuth
	}

	skew := time.Since(time.Unix(in.TS, 0))
	if skew < -stationClockSkew || skew > stationClockSkew {
		return TokenResult{}, ErrStationAuth
	}

	// DEL returns the number of keys removed, so claiming the nonce and
	// checking it existed is one atomic step — two agents replaying the same
	// nonce cannot both pass.
	claimed, err := a.Redis.Del(ctx, nonceKey(in.StationID, in.Nonce)).Result()
	if err != nil {
		return TokenResult{}, fmt.Errorf("claim station nonce: %w", err)
	}
	if claimed != 1 {
		return TokenResult{}, ErrStationAuth
	}

	var (
		pub       []byte
		hwid      string
		profileID uuid.UUID
		orgName   string
		ends      time.Time
	)
	err = a.Pool.QueryRow(ctx, `
		SELECT s.public_key, s.hwid_hash, s.station_profile_id, o.name, MAX(l.ends_at)
		FROM b2b_station s
		JOIN b2b_org o ON o.id = s.org_id AND o.status = 'active'
		JOIN b2b_org_license l ON l.org_id = s.org_id
		  AND l.starts_at <= now() AND l.ends_at > now()
		WHERE s.id = $1 AND s.status = 'active' AND s.station_profile_id IS NOT NULL
		GROUP BY s.public_key, s.hwid_hash, s.station_profile_id, o.name`,
		in.StationID).Scan(&pub, &hwid, &profileID, &orgName, &ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenResult{}, ErrStationAuth
		}
		return TokenResult{}, err
	}

	// Both checks always run, in this order, so that whether the hwid
	// matched cannot be inferred from whether the (much costlier) signature
	// verification ran: an unauthenticated Challenge caller could otherwise
	// use response latency to distinguish "hwid wrong" from "hwid right,
	// signature wrong" for free. ed25519.Verify panics on a public key that
	// is not exactly 32 bytes rather than returning false, so the length is
	// guarded first; the column has no length constraint, so a bad row must
	// not be able to turn a rejection into a panic.
	sigOK := len(pub) == ed25519.PublicKeySize &&
		ed25519.Verify(ed25519.PublicKey(pub), SignedMessage(in.StationID, in.Nonce, in.TS), in.Sig)
	hwidOK := subtle.ConstantTimeCompare([]byte(hwid), []byte(in.HWIDHash)) == 1
	if !sigOK || !hwidOK {
		return TokenResult{}, ErrStationAuth
	}

	token, err := auth.IssueStationAccess(a.Secret, in.StationID, profileID, stationTokenTTL)
	if err != nil {
		return TokenResult{}, err
	}

	// agent_version is attacker-controlled (sent by whoever holds the
	// station's key) and would otherwise be unbounded text; cap it the same
	// way EnrollStation does at enrollment time. Truncate rather than
	// reject: a future agent version string is not worth failing a token
	// renewal over.
	agentVersion := truncateRunes(strings.TrimSpace(in.AgentVersion), maxAgentVersionLen)
	_, _ = a.Pool.Exec(ctx, `
		UPDATE b2b_station
		SET last_seen_at = now(),
		    agent_version = COALESCE(NULLIF($2, ''), agent_version),
		    last_ip = NULLIF($3, '')::inet
		WHERE id = $1`, in.StationID, agentVersion, in.IP)

	return TokenResult{
		AccessToken:   token,
		ExpiresIn:     int(stationTokenTTL.Seconds()),
		LicenseEndsAt: ends.UTC(),
		OrgName:       orgName,
	}, nil
}

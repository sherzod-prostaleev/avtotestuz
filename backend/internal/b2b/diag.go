package b2b

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Diagnostics is one classroom PC describing its own state.
//
// It exists because the only record of an agent's trouble used to be
// station.log on the machine itself. A driving school with thirty PCs cannot
// be asked to walk to each one and read a file, and by the time anyone did,
// the school had usually reinstalled and destroyed the evidence. Now the agent
// says what is wrong while it is still wrong.
//
// Every field is the agent's own account of itself and therefore untrusted:
// whoever holds a station key, or a school's installer key, can post anything
// here. Nothing is used for authorisation -- it is a support record, sized and
// truncated on arrival. The identity it is filed under always comes from the
// verified JWT or from the enrolment code lookup, never from this struct.
type Diagnostics struct {
	AgentVersion string
	Phase        string
	Code         string
	Problem      string
	Detail       string
	OS           string
	Label        string
	// ClockOffsetSeconds is how far the PC's clock is ahead of ours, or nil
	// when the agent has not measured it.
	ClockOffsetSeconds *int
	// LogTail is the end of station.log.
	LogTail string
}

const (
	// maxDiagFieldLen bounds each short field. Long enough for a full Uzbek
	// sentence with an error appended, short enough that a fleet list stays
	// cheap to read.
	maxDiagFieldLen = 2000
	// maxDiagLogBytes bounds the log tail. The agent sends the end of the
	// file, and every failure this exists to explain is in the last few
	// hundred lines; a whole file would cost a lot and add nothing.
	maxDiagLogBytes = 32 << 10
	// diagHistoryPerMachine is how many reports are kept per PC. Enough to see
	// "it was fine this morning and broke at noon", bounded so a fleet
	// reporting for a year cannot become a storage problem.
	diagHistoryPerMachine = 5
	// maxEnrollFailuresPerOrg is the ceiling on unenrolled-machine reports for
	// one school, and it exists for a reason the per-machine cap does not
	// cover. Retention is keyed on (org, hwid_hash), so a caller holding a
	// leaked installer key could mint an unbounded number of buckets simply by
	// varying the hwid it claims -- five rows each, 32 KB apiece, forever. A
	// school rolling out even a hundred PCs where every one of them fails
	// produces a few hundred rows at most, so 500 is far above anything real
	// and still a hard bound on what one key can write.
	maxEnrollFailuresPerOrg = 500
	// maxClockOffsetSeconds clamps an implausible clock report. A year of skew
	// is already far past anything actionable, and it keeps a hostile agent
	// from writing an integer that overflows the column.
	maxClockOffsetSeconds = 366 * 24 * 60 * 60
)

// truncateBytes caps s at max bytes without splitting a rune, keeping the END
// of the string.
//
// The end, not the beginning: this is a log tail, and the last line before a
// failure is the whole point. Cutting on a rune boundary matters because the
// agent's own messages are Uzbek, two bytes per rune, and a byte-slice cut
// lands mid-rune and produces invalid UTF-8 that Postgres then rejects.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[len(s)-max:]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[1:]
	}
	return s
}

func (in Diagnostics) sanitized() Diagnostics {
	in.AgentVersion = truncateRunes(strings.TrimSpace(in.AgentVersion), maxAgentVersionLen)
	in.Phase = truncateRunes(strings.TrimSpace(in.Phase), 32)
	in.Code = truncateRunes(strings.TrimSpace(in.Code), 64)
	in.Problem = truncateRunes(strings.TrimSpace(in.Problem), maxDiagFieldLen)
	in.Detail = truncateRunes(strings.TrimSpace(in.Detail), maxDiagFieldLen)
	in.OS = truncateRunes(strings.TrimSpace(in.OS), 128)
	in.Label = truncateRunes(strings.TrimSpace(in.Label), maxLabelLen)
	in.LogTail = truncateBytes(in.LogTail, maxDiagLogBytes)
	if in.ClockOffsetSeconds != nil {
		v := *in.ClockOffsetSeconds
		if v > maxClockOffsetSeconds {
			v = maxClockOffsetSeconds
		}
		if v < -maxClockOffsetSeconds {
			v = -maxClockOffsetSeconds
		}
		in.ClockOffsetSeconds = &v
	}
	return in
}

// RecordStationDiagnostics stores a report from an enrolled PC and refreshes
// that station's summary.
//
// Both happen in one transaction so the fleet list and the history can never
// disagree about what a PC last said, and rows past diagHistoryPerMachine are
// swept in the same transaction rather than by a cron nobody would remember to
// run.
func (s Store) RecordStationDiagnostics(ctx context.Context, stationID uuid.UUID, in Diagnostics) error {
	if stationID == uuid.Nil {
		return ErrInvalid
	}
	d := in.sanitized()

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID uuid.UUID
	var hwid string
	if err := tx.QueryRow(ctx,
		`SELECT org_id, hwid_hash FROM b2b_station WHERE id = $1`, stationID).Scan(&orgID, &hwid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO b2b_station_diag
		  (org_id, station_id, hwid_hash, label, agent_version, phase, code,
		   problem, detail, clock_offset_seconds, os, log_tail)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		orgID, stationID, hwid, d.Label, d.AgentVersion, d.Phase, d.Code,
		d.Problem, d.Detail, d.ClockOffsetSeconds, d.OS, d.LogTail); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM b2b_station_diag
		WHERE station_id = $1 AND id NOT IN (
		  SELECT id FROM b2b_station_diag
		  WHERE station_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2
		)`, stationID, diagHistoryPerMachine); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE b2b_station
		SET last_phase = $2,
		    last_code = $3,
		    last_problem = $4,
		    clock_offset_seconds = $5,
		    last_diag_at = now(),
		    agent_version = COALESCE(NULLIF($6, ''), agent_version)
		WHERE id = $1`,
		stationID, d.Phase, d.Code, d.Problem, d.ClockOffsetSeconds, d.AgentVersion); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RecordEnrollFailure stores a report from a PC that could not become a
// station at all, filed against the school it was trying to join.
//
// This is the half that matters. A machine already registered to another
// school, a school whose seats are full, an installer key that expired -- none
// of those can hold a station token, so none of them could ever file through
// the authenticated route, and they are precisely the failures that used to be
// invisible. The enrolment code proves which school the agent was aiming at;
// it is a bearer credential the school already holds, and nothing here grants
// anything, so accepting it is no wider than accepting an enrolment attempt.
func (s Store) RecordEnrollFailure(ctx context.Context, code, hwidHash string, in Diagnostics) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	hwidHash = strings.TrimSpace(hwidHash)
	if code == "" {
		return ErrInvalid
	}
	// The hwid is what distinguishes one unenrolled machine from another, so
	// it is validated the same way EnrollStation validates it rather than
	// being accepted as free text.
	if len(hwidHash) != hwidHashLen || strings.ToLower(hwidHash) != hwidHash {
		return ErrInvalid
	}
	d := in.sanitized()

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Deliberately not requiring the code to be unexpired or unrevoked: "the
	// key you are holding is dead" is one of the states worth seeing, and
	// refusing the report would hide exactly that. It must still be a code
	// this platform issued, so an arbitrary string cannot write rows.
	var orgID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT org_id FROM b2b_org_enroll_code WHERE code = $1`, code).Scan(&orgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO b2b_station_diag
		  (org_id, station_id, hwid_hash, label, agent_version, phase, code,
		   problem, detail, clock_offset_seconds, os, log_tail)
		VALUES ($1,NULL,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		orgID, hwidHash, d.Label, d.AgentVersion, d.Phase, d.Code,
		d.Problem, d.Detail, d.ClockOffsetSeconds, d.OS, d.LogTail); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM b2b_station_diag
		WHERE station_id IS NULL AND org_id = $1 AND hwid_hash = $2 AND id NOT IN (
		  SELECT id FROM b2b_station_diag
		  WHERE station_id IS NULL AND org_id = $1 AND hwid_hash = $2
		  ORDER BY created_at DESC, id DESC LIMIT $3
		)`, orgID, hwidHash, diagHistoryPerMachine); err != nil {
		return err
	}

	// The per-machine sweep above cannot bound a caller that invents a new
	// hwid every time, so the org is bounded too.
	if _, err := tx.Exec(ctx, `
		DELETE FROM b2b_station_diag
		WHERE station_id IS NULL AND org_id = $1 AND id NOT IN (
		  SELECT id FROM b2b_station_diag
		  WHERE station_id IS NULL AND org_id = $1
		  ORDER BY created_at DESC, id DESC LIMIT $2
		)`, orgID, maxEnrollFailuresPerOrg); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DiagRow is one stored report, for the admin panel.
type DiagRow struct {
	CreatedAt time.Time  `json:"created_at"`
	StationID *uuid.UUID `json:"station_id,omitempty"`
	// HWIDHash identifies a machine that never enrolled. Shown so an operator
	// can tell two failing PCs apart before either has a name.
	HWIDHash           string `json:"hwid_hash"`
	Label              string `json:"label"`
	AgentVersion       string `json:"agent_version"`
	Phase              string `json:"phase"`
	Code               string `json:"code"`
	Problem            string `json:"problem"`
	Detail             string `json:"detail"`
	ClockOffsetSeconds *int   `json:"clock_offset_seconds,omitempty"`
	OS                 string `json:"os"`
	LogTail            string `json:"log_tail"`
}

const diagQuerySelect = `
	SELECT created_at, station_id, hwid_hash, label, agent_version, phase, code,
	       problem, detail, clock_offset_seconds, os, log_tail
	FROM b2b_station_diag`

// StationDiagnostics returns the stored reports for one enrolled station,
// newest first. orgID scopes the lookup so one school's admin can never read
// another's by guessing a station id.
func (s Store) StationDiagnostics(ctx context.Context, orgID, stationID uuid.UUID) ([]DiagRow, error) {
	rows, err := s.Pool.Query(ctx, diagQuerySelect+`
		WHERE org_id = $1 AND station_id = $2
		ORDER BY created_at DESC, id DESC`, orgID, stationID)
	if err != nil {
		return nil, err
	}
	return scanDiagRows(rows)
}

// OrgEnrollFailures returns reports from machines in this org that never
// became stations, newest first.
func (s Store) OrgEnrollFailures(ctx context.Context, orgID uuid.UUID, limit int) ([]DiagRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, diagQuerySelect+`
		WHERE org_id = $1 AND station_id IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	return scanDiagRows(rows)
}

func scanDiagRows(rows pgx.Rows) ([]DiagRow, error) {
	defer rows.Close()
	out := make([]DiagRow, 0, diagHistoryPerMachine)
	for rows.Next() {
		var r DiagRow
		if err := rows.Scan(&r.CreatedAt, &r.StationID, &r.HWIDHash, &r.Label,
			&r.AgentVersion, &r.Phase, &r.Code, &r.Problem, &r.Detail,
			&r.ClockOffsetSeconds, &r.OS, &r.LogTail); err != nil {
			return nil, err
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

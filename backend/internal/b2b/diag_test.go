package b2b_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

func intPtr(v int) *int { return &v }

// openCode mints (or returns) the org's live installer key, the same way the
// admin panel does.
func openCode(t *testing.T, store b2b.Store, orgID uuid.UUID) string {
	t.Helper()
	row, err := store.OpenInstallerKey(context.Background(), orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	return row.Code
}

// enrolledStation creates an org with a live code and one enrolled PC.
func enrolledStation(t *testing.T, store b2b.Store, orgID uuid.UUID, hwid string) (uuid.UUID, string) {
	t.Helper()
	code := openCode(t, store, orgID)
	res, err := store.EnrollStation(context.Background(), b2b.EnrollInput{
		Code: code, PublicKey: newPub(t), HWIDHash: hwid,
		Label: "PC-1", AgentVersion: "1.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.StationID, code
}

// TestEnrollFailureIsRecordedWithoutAStation is the point of the whole
// feature. A PC that cannot enrol has no station token, so it can never use
// the authenticated route -- and "could not enrol" is exactly the failure a
// school needs explained. The report has to land against the school anyway.
func TestEnrollFailureIsRecordedWithoutAStation(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code := openCode(t, store, orgID)
	hwid := testHWID("never-enrolled")

	err := store.RecordEnrollFailure(ctx, code, hwid, b2b.Diagnostics{
		Phase: "blocked", Code: "hwid_other_org",
		Problem: "Bu kompyuter allaqachon BOSHQA avtomaktabga ro'yxatdan o'tgan.",
		Detail:  "/api/v1/b2b/stations/enroll: conflict",
		Label:   "WIN-TEST", AgentVersion: "1.1.0",
		LogTail: "line one\nline two\n",
	})
	if err != nil {
		t.Fatalf("RecordEnrollFailure() = %v", err)
	}

	rows, err := store.OrgEnrollFailures(ctx, orgID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d reports, want 1", len(rows))
	}
	got := rows[0]
	if got.StationID != nil {
		t.Fatalf("station_id = %v, want nil for a PC that never enrolled", got.StationID)
	}
	if got.HWIDHash != hwid {
		t.Fatalf("hwid = %q, want %q -- it is the only way to tell two failing PCs apart", got.HWIDHash, hwid)
	}
	if got.Code != "hwid_other_org" || got.LogTail == "" {
		t.Fatalf("report lost its content: %+v", got)
	}
}

// TestEnrollFailureAcceptsADeadCode. "The key you are holding no longer works"
// is one of the states worth seeing; refusing the report because the key is
// revoked would hide exactly the failure being reported.
func TestEnrollFailureAcceptsADeadCode(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code := openCode(t, store, orgID)
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET revoked_at = now() WHERE code = $1`, code); err != nil {
		t.Fatal(err)
	}

	if err := store.RecordEnrollFailure(ctx, code, testHWID("dead-code"), b2b.Diagnostics{
		Phase: "blocked", Code: "not_found",
	}); err != nil {
		t.Fatalf("RecordEnrollFailure() = %v, want a revoked key's report to still be filed", err)
	}
	rows, err := store.OrgEnrollFailures(ctx, orgID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d reports, want the revoked-key report to be stored", len(rows))
	}
}

// TestEnrollFailureRejectsAnUnknownCode keeps the unauthenticated route from
// becoming a way to write rows against arbitrary schools.
func TestEnrollFailureRejectsAnUnknownCode(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}

	err := store.RecordEnrollFailure(context.Background(), "AVTO-NOPE-NOPE",
		testHWID("x"), b2b.Diagnostics{Phase: "blocked"})
	if !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("RecordEnrollFailure() = %v, want ErrNotFound", err)
	}
}

// TestEnrollFailureRejectsAMalformedHWID. The hwid is what distinguishes one
// unenrolled machine from another, so free text would let a caller fragment or
// collide the history at will.
func TestEnrollFailureRejectsAMalformedHWID(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	orgID := seatedOrg(t, pool, 1)
	code := openCode(t, store, orgID)

	for _, bad := range []string{"", "not-a-digest", strings.ToUpper(testHWID("upper"))} {
		if err := store.RecordEnrollFailure(context.Background(), code, bad,
			b2b.Diagnostics{Phase: "blocked"}); !errors.Is(err, b2b.ErrInvalid) {
			t.Fatalf("hwid %q: got %v, want ErrInvalid", bad, err)
		}
	}
}

// TestStationDiagnosticsUpdatesTheFleetSummary: the station list has to be
// able to answer "which PCs are stuck and why" without reading every report.
func TestStationDiagnosticsUpdatesTheFleetSummary(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	stationID, _ := enrolledStation(t, store, orgID, testHWID("fleet"))

	if err := store.RecordStationDiagnostics(ctx, stationID, b2b.Diagnostics{
		Phase: "blocked", Code: "clock",
		Problem:            "Bu kompyuterning soati noto'g'ri.",
		ClockOffsetSeconds: intPtr(900),
		AgentVersion:       "1.1.0",
		LogTail:            "tail\n",
	}); err != nil {
		t.Fatalf("RecordStationDiagnostics() = %v", err)
	}

	list, err := store.ListStations(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range list {
		if s.ID != stationID {
			continue
		}
		found = true
		if s.LastPhase != "blocked" || s.LastCode != "clock" {
			t.Fatalf("summary = %q/%q, want blocked/clock", s.LastPhase, s.LastCode)
		}
		if s.ClockOffsetSeconds == nil || *s.ClockOffsetSeconds != 900 {
			t.Fatalf("clock offset = %v, want 900", s.ClockOffsetSeconds)
		}
		if s.AgentVersion != "1.1.0" {
			t.Fatalf("agent_version = %q, want 1.1.0", s.AgentVersion)
		}
		if s.LastDiagAt == nil {
			t.Fatal("last_diag_at is nil after a report")
		}
	}
	if !found {
		t.Fatal("the station vanished from the list")
	}
}

// TestDiagnosticsHistoryIsBounded keeps a fleet reporting for a year from
// becoming a storage problem, and keeps the sweep in the same transaction as
// the insert rather than in a cron nobody remembers.
func TestDiagnosticsHistoryIsBounded(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	stationID, _ := enrolledStation(t, store, orgID, testHWID("bounded"))

	for i := 0; i < 12; i++ {
		if err := store.RecordStationDiagnostics(ctx, stationID, b2b.Diagnostics{
			Phase: "waiting", Code: "network", Detail: string(rune('a' + i)),
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := store.StationDiagnostics(ctx, orgID, stationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("kept %d reports, want the newest 5", len(rows))
	}
}

// TestStationDiagnosticsAreScopedToTheOwningOrg. An admin who can read one
// school's stations must not be able to read another's by guessing an id.
func TestStationDiagnosticsAreScopedToTheOwningOrg(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgA := seatedOrg(t, pool, 1)
	orgB := seatedOrg(t, pool, 1)
	stationA, _ := enrolledStation(t, store, orgA, testHWID("scoped-a"))
	if err := store.RecordStationDiagnostics(ctx, stationA, b2b.Diagnostics{
		Phase: "ready", LogTail: "secret-of-school-a",
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := store.StationDiagnostics(ctx, orgB, stationA)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("school B read %d of school A's reports", len(rows))
	}
}

// TestLogTailKeepsTheEndAndStaysValidUTF8. The last line before a failure is
// the whole point, and the agent's messages are Uzbek -- a byte-slice cut
// lands mid-rune and Postgres rejects the insert.
func TestLogTailKeepsTheEndAndStaysValidUTF8(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	stationID, _ := enrolledStation(t, store, orgID, testHWID("tail"))

	huge := strings.Repeat("ў", 40000) + "\nOXIRGI QATOR\n"
	if err := store.RecordStationDiagnostics(ctx, stationID, b2b.Diagnostics{
		Phase: "waiting", LogTail: huge,
	}); err != nil {
		t.Fatalf("RecordStationDiagnostics() = %v (a mid-rune cut would fail here)", err)
	}
	rows, err := store.StationDiagnostics(ctx, orgID, stationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if !strings.HasSuffix(rows[0].LogTail, "OXIRGI QATOR\n") {
		t.Fatal("the END of the log was not kept -- the last line before a failure is the point")
	}
	if len(rows[0].LogTail) > 32<<10 {
		t.Fatalf("log tail is %d bytes, want it capped", len(rows[0].LogTail))
	}
}

// TestEnrollFailuresAreBoundedPerOrg closes the hole the per-machine cap
// leaves open. Retention is keyed on (org, hwid_hash), so a caller holding a
// leaked installer key could mint an unbounded number of buckets by varying
// the hwid it claims — five rows each, 32 KB apiece, forever.
func TestEnrollFailuresAreBoundedPerOrg(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code := openCode(t, store, orgID)

	// Every report claims a different machine, which defeats the per-machine
	// sweep entirely.
	for i := 0; i < 520; i++ {
		if err := store.RecordEnrollFailure(ctx, code, testHWID(fmt.Sprintf("flood-%d", i)),
			b2b.Diagnostics{Phase: "blocked", Code: "hwid_other_org"}); err != nil {
			t.Fatal(err)
		}
	}

	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM b2b_station_diag WHERE org_id = $1 AND station_id IS NULL`,
		orgID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows > 500 {
		t.Fatalf("stored %d unenrolled reports for one school, want the org capped at 500", rows)
	}
	if rows == 0 {
		t.Fatal("the cap swept everything; the newest reports must survive")
	}
}

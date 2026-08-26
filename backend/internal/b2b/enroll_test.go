package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

// testHWID turns any seed string into a deterministic 64-character lowercase
// sha256 hex digest, the shape EnrollInput.validate requires of hwid_hash.
// Distinct seeds always hash to distinct digests, so tests that need several
// unique station identities (or one reused identity) can keep using plain,
// readable seeds like "hw-1" or "hw-same".
func testHWID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// freshHWID hashes a fresh random UUID, for tests that need many distinct
// runtime-generated identities (e.g. a concurrency stampede) rather than a
// fixed seed.
func freshHWID() string {
	sum := sha256.Sum256([]byte(uuid.NewString()))
	return hex.EncodeToString(sum[:])
}

// seatedOrg inserts an active org with `seats` classroom seats live for 30 days.
func seatedOrg(t *testing.T, pool *pgxpool.Pool, seats int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, $2, now(), now() + interval '30 days', 'test')`, orgID, seats); err != nil {
		t.Fatal(err)
	}
	return orgID
}

func newPub(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestEnrollStationBindsAndCapsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-1"),
		Label: "PC-1", AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.OrgID != orgID || first.StationID == uuid.Nil || first.ProfileID == uuid.Nil {
		t.Fatalf("bad result: %+v", first)
	}

	// The shadow profile is a station, not a learner.
	var kind, phone string
	if err := pool.QueryRow(ctx,
		`SELECT kind, phone FROM profile WHERE id = $1`, first.ProfileID).Scan(&kind, &phone); err != nil {
		t.Fatal(err)
	}
	if kind != "station" || !strings.HasPrefix(phone, "st:") {
		t.Fatalf("shadow profile kind=%q phone=%q", kind, phone)
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-2"), Label: "PC-2",
	}); err != nil {
		t.Fatal(err)
	}

	// Third machine: both seats are taken. The licence is what stops it --
	// max_uses now carries headroom over the seat count precisely so that it
	// is not the thing a school runs into first, and neither of the two live
	// stations may be evicted to make room (they were seen just now).
	_, err = store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-3"), Label: "PC-3",
	})
	if !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}
}

func TestEnrollStationRejectsBadCodes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: "AVTO-ZZZZ-ZZZZ", PublicKey: newPub(t), HWIDHash: testHWID("hw-x"), Label: "PC",
	}); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("unknown code err=%v, want ErrNotFound", err)
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET expires_at = now() - interval '1 minute' WHERE id = $1`,
		code.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-y"), Label: "PC",
	}); err == nil {
		t.Fatal("expired code must be refused")
	}

	code2, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET revoked_at = now() WHERE id = $1`, code2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code2.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-z"), Label: "PC",
	}); err == nil {
		t.Fatal("revoked code must be refused")
	}
}

func TestEnrollStationRejectsMalformedHWIDHash(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"empty":                       "",
		"too short":                   "hw-1",
		"right length, non-hex chars": strings.Repeat("z", 64),
	}
	for name, hwid := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := store.EnrollStation(ctx, b2b.EnrollInput{
				Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC",
			})
			if !errors.Is(err, b2b.ErrInvalid) {
				t.Fatalf("hwid_hash=%q err=%v, want ErrInvalid", hwid, err)
			}
		})
	}
}

// TestEnrollStationKeepsClonedMachinesOnTheSameLicence is the regression for
// Avtomotohavaskorlar BUXORO, 2026-08-26.
//
// A lab is built by cloning one master disk, and a raw clone carries the
// master's MachineGuid, so every PC in the room reports the same hwid_hash.
// Enrollment used to read that as "the same machine, re-imaged" and revoke the
// previous row, which meant installing PC n silently knocked PC n-1 off the
// licence -- and off VIP with it, since ActiveStationVIP requires
// status = 'active'. The school ended the day with 43 enrolled stations, six
// of them active: one per master image.
//
// Each PC does generate its own keypair on first run, which is what tells two
// clones apart here.
func TestEnrollStationKeepsClonedMachinesOnTheSameLicence(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	const lab = 20
	orgID := seatedOrg(t, pool, 55)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	cloned := testHWID("one-master-image")
	ids := make(map[uuid.UUID]bool, lab)
	for i := 0; i < lab; i++ {
		res, err := store.EnrollStation(ctx, b2b.EnrollInput{
			Code: code.Code, PublicKey: newPub(t), HWIDHash: cloned, Label: "HPStoreBukhara",
		})
		if err != nil {
			t.Fatalf("PC %d of %d could not enroll: %v", i+1, lab, err)
		}
		ids[res.StationID] = true
	}
	if len(ids) != lab {
		t.Fatalf("got %d distinct station ids, want %d", len(ids), lab)
	}

	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != lab {
		t.Fatalf("used=%d err=%v, want %d active stations", used, err, lab)
	}

	// The point of staying active is VIP: a revoked station falls back to the
	// free tier's 30 questions a day for the whole classroom.
	var revoked int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'revoked'`,
		orgID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked != 0 {
		t.Fatalf("revoked=%d, want 0: no PC may be evicted by a classmate", revoked)
	}
}

// TestEnrollStationReclaimsASeatOnlyFromAMachineThatWentQuiet covers the one
// case where a seat is still taken back: the licence is full and this hwid
// already holds a seat whose station stopped reporting, i.e. a genuine
// re-image. Without this a school that re-images its lab would have to have an
// admin revoke every row by hand before a single PC could come back.
func TestEnrollStationReclaimsASeatOnlyFromAMachineThatWentQuiet(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	hwid := testHWID("re-imaged")
	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Silent for longer than staleSeatAfter: this PC is gone, not busy.
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_station SET last_seen_at = now() - interval '2 hours' WHERE id = $1`,
		first.StationID); err != nil {
		t.Fatal(err)
	}

	second, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatalf("re-imaged machine must reclaim its own seat: %v", err)
	}
	if second.StationID == first.StationID {
		t.Fatal("re-enroll must create a new station row")
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 1 {
		t.Fatalf("used=%d err=%v, want the one seat reused, not double-spent", used, err)
	}
	var firstStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, first.StationID).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "revoked" {
		t.Fatalf("first station status=%q, want revoked", firstStatus)
	}
}

// TestEnrollStationRefusesRatherThanEvictALivePC is the other half of seat
// reclamation, and the more important half: a full licence must fail the new
// PC, never interrupt one that is mid-lesson. A station that renewed its token
// recently is by definition switched on.
func TestEnrollStationRefusesRatherThanEvictALivePC(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	hwid := testHWID("busy-classroom")
	live, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-2",
	}); !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, live.StationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("live station status=%q, want it left alone", status)
	}
}

func TestEnrollStationConcurrentStampedeRespectsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	// Mint the code while the org has 20 seats, so max_uses == 20. Then
	// shrink the license to 5. The two counters now diverge on purpose:
	// used_count will never reach max_uses (20) in this test, so the only
	// thing that can stop the stampede at 5 is the *active station* seat
	// check (enroll.go's `active >= seats`), independent of the code's own
	// max_uses. Without this divergence every fixture in this file mints a
	// code with max_uses == seats, so the used_count check always fires
	// first and the seat-cap branch is never exercised by any test.
	const mintedSeats = 20
	const seats = 5
	const machines = 20
	orgID := seatedOrg(t, pool, mintedSeats)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_license SET seats = $1 WHERE org_id = $2`, seats, orgID); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]error, machines)
	for i := 0; i < machines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = store.EnrollStation(ctx, b2b.EnrollInput{
				Code:      code.Code,
				PublicKey: newPub(t),
				HWIDHash:  freshHWID(),
				Label:     "PC",
			})
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != seats {
		t.Fatalf("%d enrollments succeeded, want exactly %d", ok, seats)
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != seats {
		t.Fatalf("used=%d err=%v, want %d", used, err, seats)
	}
}

// TestEnrollStationRejectsCodeRevokedWhileQueuedBehindLock proves the
// emergency-stop scenario from the finding: a code revoked *after* an
// in-flight enrollment already did its pre-lock read, but *before* that
// enrollment reaches the seat arithmetic, must still be refused.
//
// The interleaving is made deterministic without touching production code:
// this test takes the org lock itself first, so the enrolling transaction's
// own `FOR UPDATE` necessarily blocks. Because a single transaction executes
// its statements strictly in program order, once Postgres reports that
// transaction as queued *waiting* on the org lock (polled via pg_locks), its
// pre-lock enroll-code read is guaranteed to have already completed. Only
// then does this test revoke the code and release the lock -- so the
// enrolling transaction can only ever observe the revocation at the re-check
// added under the lock, never at the original pre-lock read.
func TestEnrollStationRejectsCodeRevokedWhileQueuedBehindLock(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(ctx) }()
	var status string
	if err := blocker.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&status); err != nil {
		t.Fatal(err)
	}

	enrollDone := make(chan error, 1)
	go func() {
		_, err := store.EnrollStation(ctx, b2b.EnrollInput{
			Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-queued"), Label: "PC",
		})
		enrollDone <- err
	}()

	// A blocked `SELECT ... FOR UPDATE` waits on the blocking transaction's
	// xid (locktype='transactionid' in pg_locks, not a relation-level lock),
	// so poll pg_stat_activity's wait_event_type instead -- it flips to
	// 'Lock' for exactly this backend once it's parked behind the blocker,
	// and its query text still contains the SQL as sent (unsubstituted
	// placeholders), which is enough to identify it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND query LIKE '%FROM b2b_org%FOR UPDATE%'
			)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the enroll transaction to queue behind the org lock")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The enroll transaction is now provably parked on the org lock, past
	// its pre-lock read. Revoke the code -- the emergency stop -- then
	// release the lock and let it proceed.
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET revoked_at = now() WHERE id = $1`, code.ID); err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	err = <-enrollDone
	if !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound (code revoked while queued behind the lock)", err)
	}
}

// TestEnrollStationRefusesCrossOrgHWIDTakeover covers both halves of the
// cross-org fix: a same-org re-image still reuses the seat (unchanged
// behavior, see TestEnrollStationRebindsSameMachine), and a different org
// presenting the same hwid_hash is refused rather than silently stealing the
// machine out from under the first school.
// TestEnrollStationLetsTwoSchoolsRunTheSameImage is the cross-org half of the
// clone problem. Two schools that bought their PCs from the same shop get the
// same factory image and therefore the same hwid_hash, and neither of them may
// be refused because the other enrolled first.
//
// This used to answer ErrConflict, which reached the classroom as "this
// computer is already registered to ANOTHER driving school" -- an instruction
// to go find an admin at a school that has nothing to do with this one. The
// refusal existed to stop org A's code revoking org B's PC; enrollment no
// longer revokes anything across orgs, so there is nothing left to protect
// against. Each school still pays for, and is capped by, its own seats.
func TestEnrollStationLetsTwoSchoolsRunTheSameImage(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgA := seatedOrg(t, pool, 5)
	codeA, err := store.OpenEnrollWindow(ctx, orgA, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	hwid := testHWID("shared-hw")
	stationA, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: codeA.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-A",
	})
	if err != nil {
		t.Fatal(err)
	}

	orgB := seatedOrg(t, pool, 5)
	codeB, err := store.OpenEnrollWindow(ctx, orgB, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: codeB.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-B",
	}); err != nil {
		t.Fatalf("second school with the same factory image was refused: %v", err)
	}

	// Neither school's seat count may be disturbed by the other.
	usedA, err := store.CountActiveStations(ctx, orgA)
	if err != nil || usedA != 1 {
		t.Fatalf("orgA used=%d err=%v, want 1 active station", usedA, err)
	}
	usedB, err := store.CountActiveStations(ctx, orgB)
	if err != nil || usedB != 1 {
		t.Fatalf("orgB used=%d err=%v, want 1 active station", usedB, err)
	}
	var statusA string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, stationA.StationID).Scan(&statusA); err != nil {
		t.Fatal(err)
	}
	if statusA != "active" {
		t.Fatalf("orgA station status=%q, want active", statusA)
	}
}

// TestEnrollStationTruncatesLongCyrillicLabelOnRunes covers the finding that
// byte-slicing a label at maxLabelLen can cut a multi-byte Cyrillic rune in
// half, producing invalid UTF-8 that then fails the profile insert.
func TestEnrollStationTruncatesLongCyrillicLabelOnRunes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	// A single 1-byte 'A' prefix, then all 2-byte Cyrillic runes: this
	// shifts every subsequent rune boundary to an odd byte offset, so a
	// byte-slice cut at the (even) 64-byte mark is guaranteed to land
	// mid-rune -- exactly what corrupted UTF-8 under the old `label[:64]`.
	// "Компьютер" is 9 runes; 1 + 9*8 = 73 runes total, past maxLabelLen.
	longLabel := "A" + strings.Repeat("Компьютер", 8)
	result, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-cyrillic"), Label: longLabel,
	})
	if err != nil {
		t.Fatalf("enroll with long Cyrillic label: %v", err)
	}
	if !utf8.ValidString(result.Label) {
		t.Fatalf("stored label is not valid UTF-8: %q", result.Label)
	}
	if n := utf8.RuneCountInString(result.Label); n != 64 {
		t.Fatalf("label rune count=%d, want 64 (maxLabelLen)", n)
	}

	// Also confirm what's actually persisted (not just the returned struct)
	// round-trips as valid UTF-8 through Postgres.
	var storedLabel string
	if err := pool.QueryRow(ctx,
		`SELECT label FROM b2b_station WHERE id = $1`, result.StationID).Scan(&storedLabel); err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(storedLabel) {
		t.Fatalf("persisted label is not valid UTF-8: %q", storedLabel)
	}
}

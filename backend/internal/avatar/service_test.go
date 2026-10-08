package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

const (
	testToken = "777000:avatar-test-secret-token"
	mediaBase = "https://drivergo.test/media"
)

// fakeTelegram is api.telegram.org for the three calls a photo takes.
type fakeTelegram struct {
	mu          sync.Mutex
	noPhotos    bool
	fileSize    int64
	body        []byte
	contentType string
	status      int
	// blockFile, when set, holds getFile until the channel is closed.
	blockFile chan struct{}
	// profileStatus/profileCode/profileDesc, when set, make
	// getUserProfilePhotos answer ok:false.
	profileCode int
	profileDesc string

	photoCalls atomic.Int32
}

func (f *fakeTelegram) set(fn func(*fakeTelegram)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeTelegram) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		noPhotos, fileSize, body, ct, status, block := f.noPhotos, f.fileSize, f.body, f.contentType, f.status, f.blockFile
		pcode, pdesc := f.profileCode, f.profileDesc
		f.mu.Unlock()
		switch r.URL.Path {
		case "/bot" + testToken + "/getUserProfilePhotos":
			f.photoCalls.Add(1)
			if pcode != 0 {
				w.WriteHeader(pcode)
				_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":%d,"description":%q}`, pcode, pdesc)
				return
			}
			if noPhotos {
				_, _ = w.Write([]byte(`{"ok":true,"result":{"total_count":0,"photos":[]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"total_count":1,"photos":[[
				{"file_id":"s","width":160,"height":160},
				{"file_id":"m","width":320,"height":320},
				{"file_id":"l","width":640,"height":640}]]}}`))
		case "/bot" + testToken + "/getFile":
			if block != nil {
				<-block
			}
			res, _ := json.Marshal(map[string]any{"file_id": "m", "file_path": "photos/file_1.jpg", "file_size": fileSize})
			_, _ = w.Write([]byte(`{"ok":true,"result":` + string(res) + `}`))
		case "/file/bot" + testToken + "/photos/file_1.jpg":
			w.Header().Set("Content-Type", ct)
			if status != 0 {
				w.WriteHeader(status)
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type fixture struct {
	svc   *Service
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	tg    *fakeTelegram
	root  string
	logs  *observer.ObservedLogs
	clock atomic.Pointer[time.Time]
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testdb.New(t)
	f := &fixture{q: sqlc.New(pool), pool: pool, root: t.TempDir()}
	f.tg = &fakeTelegram{body: encodeJPEG(t, 320, 320), contentType: "image/jpeg"}
	srv := f.tg.server(t)
	core, logs := observer.New(zapcore.DebugLevel)
	f.logs = logs
	f.svc = New(f.q, blob.NewLocalDir(f.root), bot.NewClient(srv.URL, testToken, srv.Client()), mediaBase, zap.New(core))
	f.svc.now = func() time.Time {
		if p := f.clock.Load(); p != nil {
			return *p
		}
		return time.Now()
	}
	t.Cleanup(func() {
		f.svc.Wait()
		for _, e := range logs.All() {
			line := e.Message + fmt.Sprint(e.ContextMap())
			if strings.Contains(line, "avatar-test-secret-token") || strings.Contains(line, "/file/bot") {
				t.Errorf("log leaks the bot token or a file URL: %s", line)
			}
		}
	})
	return f
}

var phoneSeq atomic.Int64

func (f *fixture) learner(t *testing.T, verified bool) (uuid.UUID, int64) {
	t.Helper()
	n := phoneSeq.Add(1)
	p, err := f.q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: fmt.Sprintf("+9989055%05d", n)})
	if err != nil {
		t.Fatal(err)
	}
	tg := 880000 + n
	if err := f.q.UpsertTelegramAccount(context.Background(), sqlc.UpsertTelegramAccountParams{
		ProfileID: p.ID, TgUserID: tg, Username: "u" + strconv.FormatInt(n, 10), PhoneVerified: verified,
	}); err != nil {
		t.Fatal(err)
	}
	return p.ID, tg
}

func (f *fixture) state(t *testing.T, id uuid.UUID) sqlc.GetProfileAvatarStateRow {
	t.Helper()
	st, err := f.q.GetProfileAvatarState(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func (f *fixture) objects(t *testing.T) []string {
	t.Helper()
	var keys []string
	_ = filepath.Walk(f.root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(f.root, p)
			keys = append(keys, filepath.ToSlash(rel))
		}
		return nil
	})
	return keys
}

func (f *fixture) url(t *testing.T, id uuid.UUID) string {
	t.Helper()
	return f.svc.AvatarURL(context.Background(), id, "user")
}

func TestVerifiedLinkStoresAvatarAndServesURL(t *testing.T) {
	f := newFixture(t)
	id, tg := f.learner(t, true)

	f.svc.TelegramLinked(id)
	f.svc.Wait()

	st := f.state(t, id)
	if !st.AvatarKey.Valid || !st.AvatarUpdatedAt.Valid {
		t.Fatalf("state = %+v, want a stored avatar", st)
	}
	key := st.AvatarKey.String
	if !strings.HasPrefix(key, KeyPrefix) || !strings.HasSuffix(key, ".jpg") {
		t.Fatalf("key %q outside %s*.jpg", key, KeyPrefix)
	}
	if strings.Contains(key, id.String()) || strings.Contains(key, strconv.FormatInt(tg, 10)) {
		t.Fatalf("key %q is derived from an identifier", key)
	}
	if got := f.objects(t); len(got) != 1 || got[0] != key {
		t.Fatalf("objects = %v, want exactly %s", got, key)
	}
	if got, want := f.url(t, id), mediaBase+"/"+key; got != want {
		t.Fatalf("AvatarURL = %q, want %q", got, want)
	}

	// A refresh replaces the object under a new key and deletes the old one,
	// so a cached copy of an old photo can never be served under the new URL.
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	next := f.state(t, id).AvatarKey.String
	if next == key {
		t.Fatal("refresh reused the key")
	}
	if got := f.objects(t); len(got) != 1 || got[0] != next {
		t.Fatalf("objects after refresh = %v, want only %s", got, next)
	}
}

func TestLegacyLinkAndStationNeverTouchTelegram(t *testing.T) {
	f := newFixture(t)
	legacy, _ := f.learner(t, false)
	station, _ := f.learner(t, true)
	if _, err := f.pool.Exec(context.Background(), `UPDATE profile SET kind = 'station' WHERE id = $1`, station); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{legacy, station} {
		f.svc.TelegramLinked(id)
		f.svc.Wait()
		if got := f.svc.AvatarURL(context.Background(), id, "user"); got != "" {
			t.Errorf("AvatarURL = %q", got)
		}
	}
	// A station is refused before the database is even asked.
	if got := f.svc.AvatarURL(context.Background(), station, "station"); got != "" {
		t.Errorf("station AvatarURL = %q", got)
	}
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 0 {
		t.Fatalf("Telegram asked %d times for unverified/station profiles", n)
	}
	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("objects stored: %v", objs)
	}
}

func TestNoPhotoClearsAndRecordsCheck(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	if !f.state(t, id).AvatarKey.Valid {
		t.Fatal("setup: no avatar")
	}

	// The learner hides their photo from bots: Telegram returns none.
	f.tg.set(func(t *fakeTelegram) { t.noPhotos = true })
	f.svc.TelegramLinked(id)
	f.svc.Wait()

	st := f.state(t, id)
	if st.AvatarKey.Valid {
		t.Fatalf("avatar kept after the photo was hidden: %+v", st)
	}
	if !st.AvatarUpdatedAt.Valid {
		t.Fatal("check not recorded; the weekly gate would re-ask on every page load")
	}
	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("old object not deleted: %v", objs)
	}
	if got := f.url(t, id); got != "" {
		t.Fatalf("AvatarURL = %q", got)
	}
	f.svc.Wait()
}

func TestUnusablePhotosAreRejected(t *testing.T) {
	cases := map[string]func(*fakeTelegram){
		"html body":  func(t *fakeTelegram) { t.body = []byte("<!doctype html><p>hi</p>") },
		"text type":  func(t *fakeTelegram) { t.contentType = "text/html" },
		"over 2 MiB": func(t *fakeTelegram) { t.fileSize = MaxDownloadBytes + 1 },
		"long body":  func(t *fakeTelegram) { t.body = append(t.body, make([]byte, MaxDownloadBytes)...) },
		"gif": func(t *fakeTelegram) {
			t.body = []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
			t.contentType = "image/gif"
		},
		"truncated jpg": func(t *fakeTelegram) { t.body = t.body[:60] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			id, _ := f.learner(t, true)
			f.svc.TelegramLinked(id)
			f.svc.Wait()

			f.tg.set(mutate)
			f.svc.TelegramLinked(id)
			f.svc.Wait()
			st := f.state(t, id)
			if st.AvatarKey.Valid {
				t.Fatalf("unusable photo left an avatar: %+v", st)
			}
			if !st.AvatarUpdatedAt.Valid {
				t.Fatal("rejection not recorded as a check")
			}
			if objs := f.objects(t); len(objs) != 0 {
				t.Fatalf("objects = %v, want none", objs)
			}
		})
	}
}

func TestUnlinkClearsAvatarAndDeletesObject(t *testing.T) {
	f := newFixture(t)
	id, tg := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()

	if _, err := f.q.DeleteTelegramAccountByTgUserID(context.Background(), tg); err != nil {
		t.Fatal(err)
	}
	f.svc.TelegramUnlinked(id)
	f.svc.Wait()

	st := f.state(t, id)
	if st.AvatarKey.Valid || st.AvatarUpdatedAt.Valid {
		t.Fatalf("state after unlink = %+v, want all cleared", st)
	}
	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("object survived unlink: %v", objs)
	}
}

// Unlinked is a reconcile, not a delete: a profile that still has its
// verified link keeps the photo.
func TestUnlinkedKeepsPhotoWhileLinkIsVerified(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	key := f.state(t, id).AvatarKey

	f.svc.TelegramUnlinked(id)
	f.svc.Wait()
	if got := f.state(t, id).AvatarKey; got != key {
		t.Fatalf("key = %+v, want unchanged %+v", got, key)
	}
}

// Whatever path removed the link, GET /me never shows the photo of a profile
// without a verified link, and it starts the cleanup itself.
func TestAvatarURLHidesAndCleansStalePhotoOfUnlinkedProfile(t *testing.T) {
	f := newFixture(t)
	id, tg := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	if _, err := f.q.DeleteTelegramAccountByTgUserID(context.Background(), tg); err != nil {
		t.Fatal(err)
	}

	if got := f.url(t, id); got != "" {
		t.Fatalf("AvatarURL = %q for an unlinked profile", got)
	}
	f.svc.Wait()
	if st := f.state(t, id); st.AvatarKey.Valid {
		t.Fatalf("lazy cleanup did not run: %+v", st)
	}
	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("objects = %v", objs)
	}
}

func TestRefreshAtMostWeekly(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)

	// Never checked: the first GET /me fetches; concurrent ones share it.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); f.url(t, id) }()
	}
	wg.Wait()
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("first load asked Telegram %d times, want 1", n)
	}

	// Six days later: still fresh.
	six := time.Now().Add(6 * 24 * time.Hour)
	f.clock.Store(&six)
	f.url(t, id)
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("refreshed before 7 days (%d calls)", n)
	}

	// Eight days later: one refresh, and the URL keeps working meanwhile.
	eight := time.Now().Add(8 * 24 * time.Hour)
	f.clock.Store(&eight)
	if got := f.url(t, id); got == "" {
		t.Fatal("stale avatar hidden while refreshing")
	}
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 2 {
		t.Fatalf("calls after 8 days = %d, want 2", n)
	}
}

// A Telegram outage keeps the photo the learner has, logs a warning without
// the token, and does not turn every page load into another Telegram call.
func TestTelegramFailureKeepsPhotoAndBacksOff(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	key := f.state(t, id).AvatarKey

	f.tg.set(func(t *fakeTelegram) { t.status = http.StatusBadGateway; t.contentType = "text/html" })
	later := time.Now().Add(8 * 24 * time.Hour)
	f.clock.Store(&later)
	f.url(t, id)
	f.svc.Wait()
	calls := f.tg.photoCalls.Load()
	for range 5 {
		f.url(t, id)
	}
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != calls {
		t.Fatalf("retried during back-off: %d -> %d calls", calls, n)
	}
	if got := f.state(t, id).AvatarKey; got != key {
		t.Fatalf("outage changed the avatar: %+v -> %+v", key, got)
	}
	warned := false
	for _, e := range f.logs.All() {
		if e.Level == zapcore.WarnLevel && e.Message == "avatar.telegram_fetch_failed" {
			warned = true
		}
	}
	if !warned {
		t.Fatal("no warning logged for the failed fetch")
	}
}

// The link is removed while the photo is being fetched: the upload that
// finishes afterwards must not attach to the profile or linger in storage.
func TestUnlinkDuringFetchLeavesNothingBehind(t *testing.T) {
	f := newFixture(t)
	id, tg := f.learner(t, true)
	block := make(chan struct{})
	f.tg.set(func(t *fakeTelegram) { t.blockFile = block })

	f.svc.TelegramLinked(id)
	deadline := time.Now().Add(5 * time.Second)
	for f.tg.photoCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := f.q.DeleteTelegramAccountByTgUserID(context.Background(), tg); err != nil {
		t.Fatal(err)
	}
	close(block)
	f.svc.Wait()

	if st := f.state(t, id); st.AvatarKey.Valid {
		t.Fatalf("avatar attached after unlink: %+v", st)
	}
	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("orphaned upload: %v", objs)
	}
}

func TestNilServiceIsInert(t *testing.T) {
	var s *Service
	s.TelegramLinked(uuid.New())
	s.TelegramUnlinked(uuid.New())
	if got := s.AvatarURL(context.Background(), uuid.New(), "user"); got != "" {
		t.Fatalf("nil service AvatarURL = %q", got)
	}
	s.Wait()
}

func waitPhotoCalls(t *testing.T, f *fixture, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.tg.photoCalls.Load() < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if f.tg.photoCalls.Load() < n {
		t.Fatalf("Telegram was not called %d times in time", n)
	}
}

// A new link is followed at once by GET /me (the row still has no check
// time). That load must not start a second fetch: it would replace the key
// the first one just handed out and delete its object.
func TestNewLinkAndImmediateLoadFetchOnce(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	block := make(chan struct{})
	f.tg.set(func(t *fakeTelegram) { t.blockFile = block })

	f.svc.TelegramLinked(id)
	waitPhotoCalls(t, f, 1)
	midFlight := f.url(t, id)
	close(block)
	f.svc.Wait()

	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("Telegram asked %d times, want 1", n)
	}
	final := f.url(t, id)
	f.svc.Wait()
	if final == "" {
		t.Fatal("no avatar after the fetch")
	}
	for _, u := range []string{midFlight, final} {
		if u == "" {
			continue
		}
		key := strings.TrimPrefix(u, mediaBase+"/")
		if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(key))); err != nil {
			t.Fatalf("URL %s handed out does not resolve: %v", u, err)
		}
	}
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("Telegram asked %d times after the reload, want 1", n)
	}
}

// A queued lazy refresh whose row was meanwhile refreshed by someone else is
// skipped, not repeated.
func TestLazyFetchSkippedWhenRowIsFresh(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	f.svc.enqueue(id, jobLazy)
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("lazy pass on a fresh row asked Telegram (%d calls)", n)
	}
}

// A format this server cannot decode (WebP) says nothing about whether the
// learner has a photo: the current one stays, and Telegram is not re-asked
// on every page load.
func TestUndecodableImageKeepsExistingAvatar(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	key := f.state(t, id).AvatarKey

	f.tg.set(func(t *fakeTelegram) {
		t.contentType = "image/webp"
		t.body = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	})
	later := time.Now().Add(8 * 24 * time.Hour)
	f.clock.Store(&later)
	f.url(t, id)
	f.svc.Wait()
	calls := f.tg.photoCalls.Load()
	for range 3 {
		f.url(t, id)
	}
	f.svc.Wait()

	if got := f.state(t, id).AvatarKey; got != key {
		t.Fatalf("WebP cleared the avatar: %+v -> %+v", key, got)
	}
	if len(f.objects(t)) != 1 {
		t.Fatalf("objects = %v, want the original only", f.objects(t))
	}
	if n := f.tg.photoCalls.Load(); n != calls {
		t.Fatalf("retried during back-off: %d -> %d", calls, n)
	}
}

// Telegram answering 400 (user not found) is a final "no photo", recorded so
// the weekly gate applies instead of an hourly warning loop.
func TestPermanentTelegram400RecordsNoPhoto(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.tg.set(func(t *fakeTelegram) { t.profileCode = 400; t.profileDesc = "Bad Request: user not found" })

	f.svc.TelegramLinked(id)
	f.svc.Wait()

	st := f.state(t, id)
	if st.AvatarKey.Valid || !st.AvatarUpdatedAt.Valid {
		t.Fatalf("state = %+v, want no key and a recorded check", st)
	}
	f.url(t, id)
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("asked %d times, want 1", n)
	}
	for _, e := range f.logs.All() {
		if e.Level >= zapcore.WarnLevel {
			t.Fatalf("unexpected warning: %s", e.Message)
		}
	}
}

// A bad token (401) is ours to fix, not "no photo".
func TestTelegram401IsNotRecordedAsNoPhoto(t *testing.T) {
	f := newFixture(t)
	id, _ := f.learner(t, true)
	f.tg.set(func(t *fakeTelegram) { t.profileCode = 401; t.profileDesc = "Unauthorized" })
	f.svc.TelegramLinked(id)
	f.svc.Wait()
	if st := f.state(t, id); st.AvatarUpdatedAt.Valid {
		t.Fatalf("a 401 was recorded as a check: %+v", st)
	}
}

// slowStore writes the object, then holds Put until the job is cancelled —
// the shutdown that lands mid-upload.
type slowStore struct {
	ObjectStore
	started chan struct{}
}

func (s slowStore) Put(ctx context.Context, key, ct string, data []byte) error {
	if err := s.ObjectStore.Put(ctx, key, ct, data); err != nil {
		return err
	}
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestShutdownCancelsInFlightJobAndDeletesFreshObject(t *testing.T) {
	f := newFixture(t)
	started := make(chan struct{})
	f.svc.store = slowStore{ObjectStore: f.svc.store, started: started}
	id, _ := f.learner(t, true)

	f.svc.TelegramLinked(id)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("upload never started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // grace period already over
	f.svc.Shutdown(ctx)

	if objs := f.objects(t); len(objs) != 0 {
		t.Fatalf("object left behind by the cancelled job: %v", objs)
	}
	if st := f.state(t, id); st.AvatarKey.Valid {
		t.Fatalf("cancelled job attached an avatar: %+v", st)
	}
	f.svc.TelegramLinked(id) // after shutdown: no new work
	f.svc.Wait()
	if n := f.tg.photoCalls.Load(); n != 1 {
		t.Fatalf("work started after shutdown (%d calls)", n)
	}
}

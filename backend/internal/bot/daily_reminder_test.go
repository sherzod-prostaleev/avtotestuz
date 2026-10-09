package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// fakeClock never sleeps for real: Sleep moves time forward and records how
// long the caller asked to wait.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	slept []time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.slept = append(c.slept, d)
	return nil
}

func (c *fakeClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func tashkentAt(day, h, m int) time.Time {
	return time.Date(2026, 10, day, h, m, 0, 0, tashkent)
}

type dailyCall struct {
	Method string
	ChatID int64
	Body   map[string]any
}

// dailyTG is a fake Bot API for the reminder: per-chat failures, one-shot
// 429s and a hook that runs after each request (used to simulate a crash).
type dailyTG struct {
	mu        sync.Mutex
	calls     []dailyCall
	failCode  map[int64]int    // chat -> permanent error code on every send
	flood     map[int64]int    // chat -> 429s left to return
	photoFail bool             // every sendPhoto answers 400
	after     func(c dailyCall) // runs after a call is recorded
}

func newDailyTG(t *testing.T) (*dailyTG, *Client) {
	t.Helper()
	f := &dailyTG{failCode: map[int64]int{}, flood: map[int64]int{}}
	var msgID int64 = 1000
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndexByte(r.URL.Path, '/')+1:]
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		chat, _ := body["chat_id"].(float64)
		c := dailyCall{Method: method, ChatID: int64(chat), Body: body}
		f.mu.Lock()
		if n := f.flood[c.ChatID]; n > 0 {
			f.flood[c.ChatID] = n - 1
			f.mu.Unlock()
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 3","parameters":{"retry_after":3}}`))
			return
		}
		code := f.failCode[c.ChatID]
		photoFail := f.photoFail && method == "sendPhoto"
		f.calls = append(f.calls, c)
		msgID++
		id := msgID
		hook := f.after
		f.mu.Unlock()
		switch {
		case code == 403:
			_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
		case code == 400:
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
		case photoFail:
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: wrong file identifier/HTTP URL specified"}`))
		case method == "sendPoll":
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"poll":{"id":"dp-%d"}}}`, id, id)
		default:
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, id)
		}
		if hook != nil {
			hook(c)
		}
	}))
	t.Cleanup(srv.Close)
	return f, NewClient(srv.URL, "test-token", srv.Client())
}

func (f *dailyTG) byChat(method string) map[int64]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]int{}
	for _, c := range f.calls {
		if c.Method == method {
			out[c.ChatID]++
		}
	}
	return out
}

func (f *dailyTG) callsFor(chat int64) []dailyCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []dailyCall
	for _, c := range f.calls {
		if c.ChatID == chat {
			out = append(out, c)
		}
	}
	return out
}

func (f *dailyTG) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func setDailyFlag(t *testing.T, pool *pgxpool.Pool, on bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE feature_flag SET value_json = to_jsonb($1::bool) WHERE key = 'telegram_daily_reminder'`, on); err != nil {
		t.Fatal(err)
	}
}

func addBotUsers(t *testing.T, q *sqlc.Queries, lang string, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if err := q.UpsertTelegramBotUser(context.Background(), sqlc.UpsertTelegramBotUserParams{
			TgUserID: id, FirstName: "U", LanguageCode: lang,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

type reminderFixture struct {
	pool  *pgxpool.Pool
	q     *sqlc.Queries
	tg    *dailyTG
	clock *fakeClock
	r     *DailyReminder
}

func newReminderFixture(t *testing.T) *reminderFixture {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	tg, client := newDailyTG(t)
	clock := &fakeClock{now: tashkentAt(9, 19, 0)}
	setDailyFlag(t, pool, true)
	// feature_flag survives testdb truncation; leave it as the migration did.
	t.Cleanup(func() { setDailyFlag(t, pool, false) })
	return &reminderFixture{pool: pool, q: q, tg: tg, clock: clock, r: &DailyReminder{
		Q: q, Pool: pool, TG: client, Clock: clock,
		MediaBaseURL:  "http://media.test",
		PublicBaseURL: "https://drivergo.test",
		WebAppURL:     "https://drivergo.test/uz-Latn/tg",
	}}
}

func TestDailyReminderDoesNothingBefore19(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 1, 2)
	f.clock.set(tashkentAt(9, 18, 59))
	res, err := f.r.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Ran || f.tg.total() != 0 {
		t.Fatalf("ran=%v calls=%d before 19:00", res.Ran, f.tg.total())
	}
}

func TestDailyReminderSendsBundleOncePerDay(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	seedQuizQuestion(t, f.pool, false)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 11, 12, 13)

	res, err := f.r.Tick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ran || res.Sent != 3 || res.Eligible != 3 {
		t.Fatalf("result = %+v, want 3 sent of 3 eligible", res)
	}
	for _, id := range []int64{11, 12, 13} {
		calls := f.tg.callsFor(id)
		if len(calls) != 2 || calls[0].Method != "sendPoll" || calls[1].Method != "sendMessage" {
			t.Fatalf("chat %d calls = %+v, want poll then text", id, calls)
		}
		if _, ok := calls[0].Body["open_period"]; ok {
			t.Fatal("the daily poll must stay open")
		}
		if calls[0].Body["type"] != "quiz" {
			t.Fatalf("poll type = %v", calls[0].Body["type"])
		}
	}

	f.clock.set(tashkentAt(9, 19, 30))
	if res, err := f.r.Tick(ctx); err != nil || res.Ran {
		t.Fatalf("second tick same day: ran=%v err=%v", res.Ran, err)
	}
	if got := f.tg.total(); got != 6 {
		t.Fatalf("calls after second tick = %d, want 6", got)
	}

	// A restarted process has no memory of the run; the DB claim is what
	// stops the second pass.
	fresh := *f.r
	fresh.doneDay = ""
	if res, err := fresh.Tick(ctx); err != nil || res.Sent != 0 {
		t.Fatalf("restart same day: %+v %v", res, err)
	}
	if got := f.tg.total(); got != 6 {
		t.Fatalf("restart re-sent: calls = %d", got)
	}

	first, err := f.q.GetDailyQuestion(ctx, pgtype.Date{Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.set(tashkentAt(10, 19, 0))
	if res, err := f.r.Tick(ctx); err != nil || res.Sent != 3 {
		t.Fatalf("next day: %+v %v", res, err)
	}
	second, err := f.q.GetDailyQuestion(ctx, pgtype.Date{Time: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("the next day repeated yesterday's question while an unused one exists")
	}
}

func TestDailyReminderFlagOffOrNoTokenSendsNothing(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 21)
	setDailyFlag(t, f.pool, false)
	if res, err := f.r.Tick(context.Background()); err != nil || res.Ran {
		t.Fatalf("flag off: ran=%v err=%v", res.Ran, err)
	}
	setDailyFlag(t, f.pool, true)
	noToken := *f.r
	noToken.TG = NewClient("http://127.0.0.1:1", "", nil)
	if res, err := noToken.Tick(context.Background()); err != nil || res.Ran {
		t.Fatalf("no token: ran=%v err=%v", res.Ran, err)
	}
	if f.tg.total() != 0 {
		t.Fatalf("calls = %d", f.tg.total())
	}
}

func TestDailyReminderResumesAfterCrashWithoutDuplicates(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	ids := []int64{31, 32, 33, 34, 35, 36}
	addBotUsers(t, f.q, "", ids...)

	ctx, crash := context.WithCancel(context.Background())
	polls := 0
	f.tg.after = func(c dailyCall) {
		if c.Method == "sendPoll" {
			polls++
			if polls == 3 {
				crash() // the process dies right after the third poll went out
			}
		}
	}
	_, _ = f.r.Tick(ctx)
	f.tg.mu.Lock()
	f.tg.after = nil
	f.tg.mu.Unlock()

	restarted := *f.r
	restarted.doneDay = ""
	if _, err := restarted.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := f.tg.byChat("sendPoll")
	missing := 0
	for _, id := range ids {
		switch got[id] {
		case 0:
			missing++
		case 1:
		default:
			t.Fatalf("chat %d got %d polls — double send after restart", id, got[id])
		}
	}
	// At most the one user claimed at the instant of the crash goes without.
	if missing > 1 {
		t.Fatalf("%d users missed the reminder, want at most 1 (byChat=%v)", missing, got)
	}
}

func TestDailyReminderStopsAt21(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 41, 42, 43, 44)
	f.r.MinInterval = 30 * time.Second // two messages per user = one minute each
	f.clock.set(tashkentAt(9, 20, 59))
	res, err := f.r.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 1 {
		t.Fatalf("sent = %d, want 1 before the 21:00 cut-off", res.Sent)
	}
	if f.clock.Now().After(tashkentAt(9, 21, 1)) {
		t.Fatalf("still sending at %s", f.clock.Now())
	}
	counts, err := f.q.CountTelegramReminderAudience(context.Background(),
		pgtype.Date{Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Pending != 3 {
		t.Fatalf("pending = %d, want 3 left unclaimed", counts.Pending)
	}
}

func TestDailyReminderAdvisoryLockBlocksSecondPass(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 51, 52)

	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, dailyReminderLockKey); err != nil {
		t.Fatal(err)
	}
	res, err := f.r.Tick(ctx)
	_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, dailyReminderLockKey)
	conn.Release()
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 0 || f.tg.total() != 0 || res.Skipped != "locked" {
		t.Fatalf("result = %+v calls=%d, want skipped while another pass holds the lock", res, f.tg.total())
	}
	// Not marked done: the next minute tries again.
	if res, err := f.r.Tick(ctx); err != nil || res.Sent != 2 {
		t.Fatalf("after unlock: %+v %v", res, err)
	}
}

func TestDailyReminderConcurrentPassesSendOnce(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	ids := []int64{61, 62, 63, 64, 65, 66, 67, 68}
	addBotUsers(t, f.q, "", ids...)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		r := &DailyReminder{Q: f.r.Q, Pool: f.r.Pool, TG: f.r.TG, Clock: &fakeClock{now: tashkentAt(9, 19, 5)},
			PublicBaseURL: f.r.PublicBaseURL}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Tick(context.Background())
		}()
	}
	wg.Wait()
	got := f.tg.byChat("sendPoll")
	for _, id := range ids {
		if got[id] > 1 {
			t.Fatalf("chat %d got %d polls", id, got[id])
		}
	}
}

func TestDailyReminder403MarksBlockedAndContinues(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 71, 72, 73)
	f.tg.failCode[71] = 403
	f.tg.failCode[72] = 400 // chat not found: never opened the bot
	res, err := f.r.Tick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocked != 2 || res.Sent != 1 {
		t.Fatalf("result = %+v, want 2 blocked 1 sent", res)
	}
	for _, id := range []int64{71, 72} {
		u, err := f.q.GetTelegramBotUser(ctx, id)
		if err != nil || !u.BlockedAt.Valid {
			t.Fatalf("chat %d blocked_at not set (%v)", id, err)
		}
		if n := len(f.tg.callsFor(id)); n != 1 {
			t.Fatalf("chat %d: %d calls, want to stop after the first refusal", id, n)
		}
	}
}

func TestDailyReminder429WaitsRetryAfter(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 81)
	f.tg.flood[81] = 1
	res, err := f.r.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 1 {
		t.Fatalf("result = %+v", res)
	}
	waited := false
	f.clock.mu.Lock()
	for _, d := range f.clock.slept {
		if d == 3*time.Second {
			waited = true
		}
	}
	f.clock.mu.Unlock()
	if !waited {
		t.Fatalf("slept %v, want the 3s retry_after honoured", f.clock.slept)
	}
	if got := f.tg.byChat("sendPoll")[81]; got != 1 {
		t.Fatalf("polls = %d, want exactly 1 after the retry", got)
	}
}

func TestDailyReminderPacesAt25PerSecond(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 91, 92, 93)
	start := f.clock.Now()
	if _, err := f.r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 6 messages at >= 40ms spacing: at least 200ms between first and last.
	if elapsed := f.clock.Now().Sub(start); elapsed < 200*time.Millisecond {
		t.Fatalf("6 sends took %v of clock time, want >= 200ms (25 msg/s)", elapsed)
	}
}

func TestDailyReminderSkipsOptedOutUsers(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 101, 102)
	if err := f.q.DisableTelegramReminders(ctx, 101); err != nil {
		t.Fatal(err)
	}
	res, err := f.r.Tick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.tg.callsFor(101)) != 0 {
		t.Fatal("an opted-out user received the reminder")
	}
	if res.OptedOut != 1 || res.Sent != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestDailyReminderImageQuestionIsPhotoThenPoll(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, true)
	addBotUsers(t, f.q, "", 111)
	if _, err := f.r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := f.tg.callsFor(111)
	if len(calls) != 2 || calls[0].Method != "sendPhoto" || calls[1].Method != "sendPoll" {
		t.Fatalf("calls = %+v, want photo then poll (2 messages max)", calls)
	}
	caption, _ := calls[0].Body["caption"].(string)
	if !strings.HasPrefix(caption, "🧠 Kun savoli") || !strings.Contains(caption, "Ro'yxatdan o'ting") {
		t.Fatalf("caption = %q", caption)
	}
	if calls[0].Body["reply_markup"] == nil {
		t.Fatal("the photo carries the buttons")
	}
	if !strings.HasPrefix(calls[0].Body["photo"].(string), "http://media.test/q/") {
		t.Fatalf("photo = %v", calls[0].Body["photo"])
	}
	if calls[1].Body["reply_to_message_id"] == nil {
		t.Fatal("poll must reply to its photo")
	}
}

// A broken image must not cost the learner the question.
func TestDailyReminderFallsBackToTextWhenPhotoRejected(t *testing.T) {
	f := newReminderFixture(t)
	seedQuizQuestion(t, f.pool, true)
	addBotUsers(t, f.q, "", 121, 122)
	f.tg.photoFail = true
	res, err := f.r.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 2 {
		t.Fatalf("result = %+v", res)
	}
	if photos := f.tg.byChat("sendPhoto"); photos[122] != 0 {
		t.Fatal("after one rejected photo the run must stop trying the image")
	}
	if polls := f.tg.byChat("sendPoll"); polls[121] != 1 || polls[122] != 1 {
		t.Fatalf("polls = %v", polls)
	}
}

func TestDailyReminderRussianUserGetsRussianPollAndLine(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	qID := seedQuizQuestion(t, f.pool, false)
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO question_translation (question_id, locale, text, status, source)
		VALUES ($1, 'ru', 'Что означает этот знак?', 'verified', '')`, qID); err != nil {
		t.Fatal(err)
	}
	addBotUsers(t, f.q, "ru-RU", 131)
	addBotUsers(t, f.q, "uz", 132)
	if _, err := f.r.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ru := f.tg.callsFor(131)
	if ru[0].Body["question"] != "Что означает этот знак?" {
		t.Fatalf("ru poll question = %v", ru[0].Body["question"])
	}
	if text, _ := ru[1].Body["text"].(string); !strings.Contains(text, "Вопрос дня") {
		t.Fatalf("ru follow-up = %q", text)
	}
	if uz := f.tg.callsFor(132); uz[0].Body["question"] != "Yo'l belgisi nimani anglatadi?" {
		t.Fatalf("uz poll question = %v", uz[0].Body["question"])
	}
}

func TestDailyReminderStreakLineForLinkedLearner(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	seedQuizQuestion(t, f.pool, false)
	profileID := createProfile(t, f.q, "+998901234500")
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO telegram_account (profile_id, tg_user_id, username, phone_verified_at)
		VALUES ($1, 141, 'u', now())`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO streak (profile_id, current, best, last_active_date) VALUES ($1, 6, 6, $2)`,
		profileID, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	addBotUsers(t, f.q, "", 141)
	if _, err := f.r.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	calls := f.tg.callsFor(141)
	if text, _ := calls[1].Body["text"].(string); !strings.Contains(text, "🔥 6 kunlik seriyangiz bor") {
		t.Fatalf("follow-up = %q", text)
	}
}

func TestDailyQuestionSkipsQuestionsOverPollLimits(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	long := seedQuizQuestion(t, f.pool, false)
	if _, err := f.pool.Exec(ctx,
		`UPDATE question_translation SET text = $2 WHERE question_id = $1`, long, strings.Repeat("s", 301)); err != nil {
		t.Fatal(err)
	}
	longOpt := seedQuizQuestionWithAnswers(t, f.pool, false, []string{strings.Repeat("a", 101), "b"})
	ok := seedQuizQuestion(t, f.pool, false)
	day := dayOf(tashkentAt(9, 19, 0))
	pick, err := f.r.dailyQuestion(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if pick.id != ok {
		t.Fatalf("picked %v, want the only fitting question %v (not %v / %v)", pick.id, ok, long, longOpt)
	}
}

func TestDailyReminderDryRunCountsSegmentsAndSendsNothing(t *testing.T) {
	f := newReminderFixture(t)
	ctx := context.Background()
	qID := seedQuizQuestion(t, f.pool, false)
	addBotUsers(t, f.q, "", 151, 152, 153)
	if err := f.q.DisableTelegramReminders(ctx, 153); err != nil {
		t.Fatal(err)
	}
	p := createProfile(t, f.q, "+998901234501")
	if _, err := f.pool.Exec(ctx, `INSERT INTO telegram_account (profile_id, tg_user_id, username) VALUES ($1, 152, 'x')`, p); err != nil {
		t.Fatal(err)
	}
	rep, err := f.r.DryRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f.tg.total() != 0 {
		t.Fatal("dry-run must not call Telegram")
	}
	if rep.Eligible != 2 || rep.OptedOut != 1 || rep.Segments["unlinked"] != 1 || rep.Segments["inactive"] != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.QuestionID != qID || !rep.QuestionFits {
		t.Fatalf("question = %v fits=%v, want %v", rep.QuestionID, rep.QuestionFits, qID)
	}
	if _, err := f.q.GetDailyQuestion(ctx, dayOf(f.clock.Now())); err == nil {
		t.Fatal("dry-run must not record the day's question")
	}
}

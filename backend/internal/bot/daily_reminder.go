package bot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/flags"
)

// The daily «Kun savoli» reminder (.superpowers/sdd/daily-brief.md): at
// 19:00 Tashkent every bot user who has not opted out or blocked the bot
// gets one quiz poll plus a personal line with buttons. It runs inside the
// api process on its own goroutine, so it never touches the webhook path.

// Clock is injected so tests drive the send window, the pacing and 429
// back-off without waiting in real time.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

const (
	// dailyReminderLockKey guards a pass across replicas and restarts. A
	// session-level lock dies with its connection, so a crashed pass never
	// leaves it held. Arbitrary, only has to be unique among our locks.
	dailyReminderLockKey int64 = 0x4476_4B53_0001

	// defaultReminderInterval keeps the whole run at 25 messages/second,
	// under Telegram's ~30/s broadcast ceiling.
	defaultReminderInterval = 40 * time.Millisecond

	// defaultReminderSendTimeout bounds one Telegram call. The bot client is
	// http.DefaultClient (no timeout), and this pass holds the advisory lock:
	// without a deadline one stalled connection would hang the scheduler
	// goroutine, and every later pass, silently and for good.
	defaultReminderSendTimeout = 20 * time.Second

	// bundleMessages is how many pacing slots one recipient's bundle needs.
	bundleMessages = 2

	maxTransientRetries = 2 // 5xx / transport / timeout, then skip the user for today
	maxFloodRetries     = 5 // 429s honoured for one message before giving up
	flagRecheckEvery    = 25
	candidatePoolSize   = 20
)

var errWindowClosed = errors.New("daily reminder: send window closed")

// DailyReminder sends the daily bundle. Tick is the scheduler's entry point;
// DryRun reports what a run would do without sending or writing anything.
type DailyReminder struct {
	Q             *sqlc.Queries
	Pool          *pgxpool.Pool
	TG            *Client
	MediaBaseURL  string
	PublicBaseURL string
	WebAppURL     string
	Clock         Clock
	Log           *zap.Logger
	// MinInterval spaces every Telegram call; zero means 25/s.
	MinInterval time.Duration
	// SendTimeout bounds each Telegram call; zero means 20 s.
	SendTimeout time.Duration

	// doneDay is the Tashkent day whose pass finished in this process. Only
	// an optimisation (skips a no-op pass every minute): the DB claim is
	// what prevents double sends after a restart.
	doneDay string
}

// ReminderRunResult is one Tick. Ran is false when nothing was attempted
// (Skipped says why).
type ReminderRunResult struct {
	Ran         bool
	Skipped     string
	Day         string
	QuestionID  uuid.UUID
	Eligible    int
	Pending     int
	Sent        int
	Partial     int // first message landed, the second did not
	Uncertain   int // a send timed out: Telegram may or may not have delivered it
	Blocked     int
	OptedOut    int
	Errors      int
	Interrupted string
	Duration    time.Duration
}

func (r *DailyReminder) clock() Clock {
	if r.Clock != nil {
		return r.Clock
	}
	return realClock{}
}

func (r *DailyReminder) logger() *zap.Logger {
	if r.Log != nil {
		return r.Log
	}
	return zap.NewNop()
}

func (r *DailyReminder) interval() time.Duration {
	if r.MinInterval > 0 {
		return r.MinInterval
	}
	return defaultReminderInterval
}

func (r *DailyReminder) sendTimeout() time.Duration {
	if r.SendTimeout > 0 {
		return r.SendTimeout
	}
	return defaultReminderSendTimeout
}

func (r *DailyReminder) tokenSet() bool {
	return r.TG != nil && strings.TrimSpace(r.TG.Token) != ""
}

func dayOf(t time.Time) pgtype.Date {
	return pgtype.Date{Time: dayOfTime(t), Valid: true}
}

// RunDailyReminderScheduler wakes every minute and lets Tick decide whether
// to send. It returns when ctx ends. A panic is logged and contained: the
// reminder is never worth taking the api down.
func RunDailyReminderScheduler(ctx context.Context, r *DailyReminder) {
	tick := func() {
		defer func() {
			if p := recover(); p != nil {
				r.logger().Error("telegram daily reminder: panic", zap.Any("panic", p))
			}
		}()
		if _, err := r.Tick(ctx); err != nil && ctx.Err() == nil {
			r.logger().Error("telegram daily reminder: run failed", zap.Error(err))
		}
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

// Tick runs today's pass if it is inside the send window, the bot has a
// token, the flag is on and this process has not finished today already.
func (r *DailyReminder) Tick(ctx context.Context) (ReminderRunResult, error) {
	now := r.clock().Now()
	if !inSendWindow(now) {
		return ReminderRunResult{Skipped: "outside_window"}, nil
	}
	day := dayOf(now)
	key := day.Time.Format(time.DateOnly)
	if r.doneDay == key {
		return ReminderRunResult{Skipped: "done", Day: key}, nil
	}
	if !r.tokenSet() {
		return ReminderRunResult{Skipped: "no_token", Day: key}, nil
	}
	on, err := flags.Bool(ctx, r.Pool, flags.KeyTelegramDailyReminder, false)
	if err != nil {
		return ReminderRunResult{}, err
	}
	if !on {
		// Not marked done: switching the flag on at 19:30 still sends today.
		return ReminderRunResult{Skipped: "flag_off", Day: key}, nil
	}
	res, err := r.runPass(ctx, day)
	// No question fits a poll today: retrying every minute would only repeat
	// the same error until 21:00. The run line already logged it once.
	if (err == nil && res.Ran && res.Interrupted == "") || errors.Is(err, errNoDailyQuestion) {
		r.doneDay = key
	}
	return res, err
}

// pass is the state of one run: the day's poll and the pacing clock.
type pass struct {
	pick    dailyPick
	imageOK bool
	next    time.Time
}

func (r *DailyReminder) runPass(ctx context.Context, day pgtype.Date) (res ReminderRunResult, err error) {
	start := r.clock().Now()
	res = ReminderRunResult{Day: day.Time.Format(time.DateOnly)}
	// Every pass that got anywhere ends in exactly one run line, failures
	// included; only "another pass holds the lock" stays quiet, since that
	// replica's own pass logs the run.
	defer func() {
		if res.Skipped != "" {
			return
		}
		res.Duration = r.clock().Now().Sub(start)
		r.logRun(res, err)
	}()

	conn, err := r.Pool.Acquire(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, dailyReminderLockKey).Scan(&locked); err != nil {
		return res, err
	}
	if !locked {
		res.Skipped = "locked"
		return res, nil
	}
	defer func() {
		// Unlocked even when ctx is already cancelled; if that fails, closing
		// the session is the only other way to drop the lock before the
		// connection goes back to the pool.
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(uctx, `SELECT pg_advisory_unlock($1)`, dailyReminderLockKey); err != nil {
			_ = conn.Conn().Close(uctx)
		}
	}()

	counts, err := r.Q.CountTelegramReminderAudience(ctx, day)
	if err != nil {
		return res, err
	}
	res.Ran = true
	res.Eligible, res.Pending, res.OptedOut = int(counts.Eligible), int(counts.Pending), int(counts.OptedOut)

	p := &pass{imageOK: true}
	if res.Pending > 0 {
		pick, err := r.dailyQuestion(ctx, day)
		if err != nil {
			return res, err
		}
		p.pick = pick
		res.QuestionID = pick.id
	}
	today := day.Time

	for i := 0; res.Pending > 0; i++ {
		if ctx.Err() != nil {
			res.Interrupted = "cancelled"
			break
		}
		// The bundle's first message goes out at the pacer's next slot, not
		// now, and it needs bundleMessages slots in all: a bundle that cannot
		// finish before 21:00 is not started, rather than cut in half.
		at := r.clock().Now()
		if p.next.After(at) {
			at = p.next
		}
		if !inSendWindow(at) || !inSendWindow(at.Add(bundleMessages*r.interval())) {
			res.Interrupted = "window_closed"
			break
		}
		if i > 0 && i%flagRecheckEvery == 0 {
			if on, err := flags.Bool(ctx, r.Pool, flags.KeyTelegramDailyReminder, false); err == nil && !on {
				res.Interrupted = "flag_off"
				break
			}
		}
		tgUserID, err := r.Q.ClaimTelegramReminderRecipient(ctx, day)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				res.Interrupted = "cancelled"
				break
			}
			return res, err
		}
		rows, err := r.Q.ListTelegramReminderAudience(ctx, pgtype.Int8{Int64: tgUserID, Valid: true})
		if err != nil {
			res.Errors++
			r.logger().Warn("telegram daily reminder: recipient lookup failed", zap.Error(err))
			continue
		}
		if len(rows) == 0 {
			continue // opted out or blocked between claim and lookup
		}
		d := r.deliver(ctx, p, rows[0], today)
		switch d.outcome {
		case outcomeSent:
			res.Sent++
		case outcomePartial:
			res.Partial++
		case outcomeUncertain:
			res.Uncertain++
		case outcomeBlocked:
			res.Blocked++
			if err := r.Q.MarkTelegramBotUserBlocked(ctx, tgUserID); err != nil {
				r.logger().Warn("telegram daily reminder: mark blocked failed", zap.Error(err))
			}
		case outcomeNotStarted:
			// Nothing reached the user (window closed before the first slot).
		default:
			res.Errors++
		}
		if d.lineDelivered && d.line == lineSignup {
			if err := r.Q.MarkTelegramSignupPitch(ctx, sqlc.MarkTelegramSignupPitchParams{Day: day, TgUserID: tgUserID}); err != nil {
				r.logger().Warn("telegram daily reminder: record signup pitch failed", zap.Error(err))
			}
		}
		if d.windowClosed {
			res.Interrupted = "window_closed"
			break
		}
	}
	return res, nil
}

// logRun is the run's single structured line. Counts only — no user ids.
func (r *DailyReminder) logRun(res ReminderRunResult, err error) {
	fields := []zap.Field{
		zap.String("day", res.Day),
		zap.String("question_id", res.QuestionID.String()),
		zap.Int("eligible", res.Eligible),
		zap.Int("pending", res.Pending),
		zap.Int("sent", res.Sent),
		zap.Int("partial", res.Partial),
		zap.Int("uncertain", res.Uncertain),
		zap.Int("blocked", res.Blocked),
		zap.Int("opted_out", res.OptedOut),
		zap.Int("errors", res.Errors),
		zap.String("interrupted", res.Interrupted),
		zap.Duration("duration", res.Duration),
	}
	if err != nil {
		r.logger().Error("telegram daily reminder: run", append(fields, zap.Error(err))...)
		return
	}
	r.logger().Info("telegram daily reminder: run", fields...)
}

type deliveryOutcome int

const (
	outcomeSent deliveryOutcome = iota
	outcomeBlocked
	outcomeFailed
	outcomePartial    // the first message landed, the second did not
	outcomeNotStarted // the window closed before the first message
	outcomeUncertain  // a send timed out; delivery unknown, never retried
)

// delivery is one recipient's result. lineDelivered says whether the
// message carrying the personal line landed (the signup pitch is recorded
// only then); windowClosed ends the pass.
type delivery struct {
	outcome       deliveryOutcome
	line          personalLine
	lineDelivered bool
	windowClosed  bool
}

// unreachable is Telegram saying this chat cannot receive anything: the
// user blocked the bot or deactivated (403), or never opened a chat with it
// (400 chat not found — e.g. an audience row seeded from a Mini App login).
func unreachable(err error) bool {
	var api *APIError
	if !errors.As(err, &api) {
		return false
	}
	return api.Code == 403 ||
		(api.Code == 400 && strings.Contains(strings.ToLower(api.Description), "chat not found"))
}

// timedOut is a send whose outcome is unknown: the deadline passed or the
// client timed out, possibly after Telegram had already accepted the call.
func timedOut(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

func permanent(err error) bool {
	var api *APIError
	return errors.As(err, &api) && api.Permanent()
}

// deliver sends one user's bundle: photo (caption = line + buttons) then
// the poll replying to it, or poll then the line as text. Two messages.
func (r *DailyReminder) deliver(ctx context.Context, p *pass, a sqlc.ListTelegramReminderAudienceRow, today time.Time) delivery {
	l := langOf(a.LanguageCode)
	pl := pickPersonalLine(a, today)
	line := personalLineText(pl, l, a, today)
	markup := dailyKeyboard(r.WebAppURL, r.PublicBaseURL, l)
	req := p.pick.poll(l)
	chat := a.TgUserID
	d := delivery{line: pl}

	// fail classifies an error; landed is how many of the bundle's messages
	// already reached the user.
	fail := func(step string, landed int, err error) delivery {
		d.windowClosed = errors.Is(err, errWindowClosed)
		switch {
		case unreachable(err):
			d.outcome = outcomeBlocked
			return d
		case timedOut(err):
			// Telegram may have delivered it; the user stays claimed for
			// today because a retry could post a second poll.
			d.outcome = outcomeUncertain
			r.logger().Warn("telegram daily reminder: send timed out, delivery uncertain",
				zap.String("step", step), zap.Error(err))
			return d
		case landed > 0:
			d.outcome = outcomePartial
			r.logger().Warn("telegram daily reminder: bundle half delivered",
				zap.String("step", step), zap.Error(err))
			return d
		case d.windowClosed:
			d.outcome = outcomeNotStarted
			return d
		}
		d.outcome = outcomeFailed
		r.logger().Warn("telegram daily reminder: send failed", zap.String("step", step), zap.Error(err))
		return d
	}

	if p.pick.imageURL != "" && p.imageOK {
		var photoID int64
		err := r.send(ctx, p, func(ctx context.Context) error {
			var e error
			photoID, e = r.TG.SendPhoto(ctx, chat, p.pick.imageURL, dailyCaption(l, line), markup)
			return e
		})
		switch {
		case err == nil:
			d.lineDelivered = true
			req.ReplyTo = photoID
			if err := r.send(ctx, p, func(ctx context.Context) error {
				_, _, e := r.TG.SendPoll(ctx, chat, req)
				return e
			}); err != nil {
				return fail("poll", 1, err)
			}
			d.outcome = outcomeSent
			return d
		case permanent(err) && !unreachable(err):
			// Telegram could not use the image; it will not for anyone else
			// either, so the rest of the run goes text-only.
			p.imageOK = false
			r.logger().Warn("telegram daily reminder: image rejected, sending text-only", zap.Error(err))
		default:
			return fail("photo", 0, err)
		}
	}

	if err := r.send(ctx, p, func(ctx context.Context) error {
		_, _, e := r.TG.SendPoll(ctx, chat, req)
		return e
	}); err != nil {
		return fail("poll", 0, err)
	}
	if err := r.send(ctx, p, func(ctx context.Context) error {
		_, e := r.TG.SendText(ctx, chat, dailyFollowUp(l, line), markup)
		return e
	}); err != nil {
		return fail("text", 1, err)
	}
	d.outcome, d.lineDelivered = outcomeSent, true
	return d
}

// send paces one Telegram call and retries it: 429 waits retry_after,
// transport errors that prove non-delivery and 5xx back off up to maxTransientRetries, permanent
// 4xx return at once.
func (r *DailyReminder) send(ctx context.Context, p *pass, call func(context.Context) error) error {
	transient, flood := 0, 0
	for {
		if err := r.pace(ctx, p); err != nil {
			return err
		}
		// Each call gets its own deadline (see defaultReminderSendTimeout).
		// A timeout is NOT retried: sendPoll/sendPhoto/sendMessage are not
		// idempotent and Telegram may have delivered the call.
		cctx, cancel := context.WithTimeout(ctx, r.sendTimeout())
		err := call(cctx)
		cancel()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil || timedOut(err) {
			return err
		}
		var api *APIError
		if errors.As(err, &api) && api.Code == 429 {
			flood++
			if flood > maxFloodRetries {
				return err
			}
			wait := time.Duration(api.RetryAfter) * time.Second
			if wait <= 0 {
				wait = time.Second
			}
			if err := r.clock().Sleep(ctx, wait); err != nil {
				return err
			}
			continue
		}
		if permanent(err) {
			return err
		}
		transient++
		if transient > maxTransientRetries {
			return err
		}
		if err := r.clock().Sleep(ctx, time.Duration(transient)*time.Second); err != nil {
			return err
		}
	}
}

// pace waits for the next send slot. It refuses a slot outside the window,
// so a pass that runs late can never send at or after 21:00.
func (r *DailyReminder) pace(ctx context.Context, p *pass) error {
	now := r.clock().Now()
	if now.Before(p.next) {
		if err := r.clock().Sleep(ctx, p.next.Sub(now)); err != nil {
			return err
		}
		now = r.clock().Now()
	}
	if !inSendWindow(now) {
		return errWindowClosed
	}
	p.next = now.Add(r.interval())
	return nil
}

// dailyPick is the day's question rendered as a poll in both languages.
type dailyPick struct {
	id       uuid.UUID
	uz, ru   PollRequest
	imageURL string
	explUz   bool
	explRu   bool
}

// poll returns a copy, so a per-user ReplyTo never leaks to the next user.
func (d dailyPick) poll(l lang) PollRequest {
	req := d.uz
	if l == langRu {
		req = d.ru
	}
	req.Options = append([]string(nil), req.Options...)
	return req
}

// dailyQuestion returns the day's question, choosing and recording it on
// first use. Whoever records first wins (ON CONFLICT DO NOTHING), so every
// replica sends the same question.
//
// A recorded question can stop fitting a poll mid-evening (edited, its
// translation unverified). It is then replaced once — logged, the day's row
// overwritten — and the rest of the evening gets the replacement: a
// different question for late recipients beats an error every minute.
func (r *DailyReminder) dailyQuestion(ctx context.Context, day pgtype.Date) (dailyPick, error) {
	id, err := r.Q.GetDailyQuestion(ctx, day)
	switch {
	case err == nil:
		pick, err := r.buildPick(ctx, id)
		if !errors.Is(err, errQuestionUnusable) {
			return pick, err
		}
		r.logger().Warn("telegram daily reminder: stored question unusable, replacing",
			zap.String("question_id", id.String()), zap.Error(err))
		return r.recordDailyQuestion(ctx, day, id)
	case errors.Is(err, pgx.ErrNoRows):
		return r.recordDailyQuestion(ctx, day, uuid.Nil)
	default:
		return dailyPick{}, err
	}
}

// recordDailyQuestion picks a question and stores it for day: a new row, or
// in place of broken. Then it uses whatever the row holds, which is another
// replica's pick if that one got there first.
func (r *DailyReminder) recordDailyQuestion(ctx context.Context, day pgtype.Date, broken uuid.UUID) (dailyPick, error) {
	pick, err := r.previewQuestion(ctx, day)
	if err != nil {
		return dailyPick{}, err
	}
	if broken == uuid.Nil {
		err = r.Q.InsertDailyQuestion(ctx, sqlc.InsertDailyQuestionParams{Day: day, QuestionID: pick.id})
	} else {
		err = r.Q.ReplaceDailyQuestion(ctx, sqlc.ReplaceDailyQuestionParams{
			Day: day, OldQuestionID: broken, NewQuestionID: pick.id,
		})
	}
	if err != nil {
		return dailyPick{}, err
	}
	winner, err := r.Q.GetDailyQuestion(ctx, day)
	if err != nil {
		return dailyPick{}, err
	}
	if winner != pick.id {
		return r.buildPick(ctx, winner)
	}
	return pick, nil
}

var (
	errNoDailyQuestion = errors.New("daily reminder: no question fits Telegram's poll limits")
	// errQuestionUnusable is buildPick refusing the question itself (gone,
	// untranslated, does not fit a poll), as opposed to a database error,
	// after which the same question is worth retrying.
	errQuestionUnusable = errors.New("daily reminder: question unusable")
)

// previewQuestion picks the day's question without recording it (DryRun
// uses it as is). The SQL pre-filters on length; buildPollRequest is the
// final word, and a candidate it rejects is skipped, never truncated.
func (r *DailyReminder) previewQuestion(ctx context.Context, day pgtype.Date) (dailyPick, error) {
	ids, err := r.Q.ListDailyQuestionCandidates(ctx, sqlc.ListDailyQuestionCandidatesParams{
		MaxOptionLen:      pollOptionMaxChars,
		MaxExplanationLen: pollExplanationMax,
		Day:               day,
		MaxQuestionLen:    pollQuestionMaxChars,
		MaxOptions:        pollMaxOptions,
		LimitCount:        candidatePoolSize,
	})
	if err != nil {
		return dailyPick{}, err
	}
	for _, c := range ids {
		pick, err := r.buildPick(ctx, c.ID)
		if err == nil {
			return pick, nil
		}
		r.logger().Warn("telegram daily reminder: candidate skipped",
			zap.String("question_id", c.ID.String()), zap.Error(err))
	}
	return dailyPick{}, errNoDailyQuestion
}

func (r *DailyReminder) buildPick(ctx context.Context, id uuid.UUID) (dailyPick, error) {
	pick := dailyPick{id: id}
	for _, l := range []lang{langUz, langRu} {
		locale := l.appLocale()
		detail, err := r.Q.GetQuestion(ctx, sqlc.GetQuestionParams{ID: id, Locale: locale})
		if errors.Is(err, pgx.ErrNoRows) {
			return dailyPick{}, fmt.Errorf("%w: question %s (%s) not found", errQuestionUnusable, id, locale)
		}
		if err != nil {
			return dailyPick{}, fmt.Errorf("question %s (%s): %w", id, locale, err)
		}
		answers, err := r.Q.ListQuizAnswers(ctx, sqlc.ListQuizAnswersParams{QuestionID: id, Locale: locale})
		if err != nil {
			return dailyPick{}, err
		}
		expl := ""
		ex, err := r.Q.GetVerifiedExplanation(ctx, sqlc.GetVerifiedExplanationParams{QuestionID: id, Locale: locale})
		switch {
		case err == nil:
			expl = explanationForPoll(ex.Blocks)
		case !errors.Is(err, pgx.ErrNoRows):
			return dailyPick{}, err
		}
		req, err := buildPollRequest(detail.Text, answers, expl, 0, 0)
		if err != nil {
			return dailyPick{}, fmt.Errorf("%w: question %s (%s) does not fit a poll: %v", errQuestionUnusable, id, locale, err)
		}
		if l == langRu {
			pick.ru, pick.explRu = req, expl != ""
		} else {
			pick.uz, pick.explUz = req, expl != ""
			if detail.ImageKey.Valid {
				pick.imageURL = mediaURLFor(r.MediaBaseURL, detail.ImageKey.String)
			}
		}
	}
	return pick, nil
}

// ReminderDryRun is what `tgdigest --dry-run` prints.
type ReminderDryRun struct {
	Day            string
	Now            time.Time
	InWindow       bool
	FlagOn         bool
	TokenSet       bool
	Total          int
	Eligible       int
	Pending        int
	OptedOut       int
	Blocked        int
	Segments       map[string]int
	QuestionID     uuid.UUID
	QuestionStored bool // already recorded for today (a run started)
	QuestionFits   bool
	QuestionErr    string
	HasImage       bool
	ExplanationUz  bool
	ExplanationRu  bool
}

// DryRun counts today's audience by personal-line segment and shows the
// question a run would send. It sends nothing and writes nothing.
func (r *DailyReminder) DryRun(ctx context.Context) (ReminderDryRun, error) {
	now := r.clock().Now()
	day := dayOf(now)
	rep := ReminderDryRun{
		Day: day.Time.Format(time.DateOnly), Now: now.In(tashkent),
		InWindow: inSendWindow(now), TokenSet: r.tokenSet(), Segments: map[string]int{},
	}
	var err error
	if rep.FlagOn, err = flags.Bool(ctx, r.Pool, flags.KeyTelegramDailyReminder, false); err != nil {
		return rep, err
	}
	counts, err := r.Q.CountTelegramReminderAudience(ctx, day)
	if err != nil {
		return rep, err
	}
	rep.Total, rep.Eligible, rep.Pending = int(counts.Total), int(counts.Eligible), int(counts.Pending)
	rep.OptedOut, rep.Blocked = int(counts.OptedOut), int(counts.Blocked)

	rows, err := r.Q.ListTelegramReminderAudience(ctx, pgtype.Int8{})
	if err != nil {
		return rep, err
	}
	today := day.Time
	for _, a := range rows {
		rep.Segments[pickPersonalLine(a, today).segment()]++
	}

	var pick dailyPick
	id, err := r.Q.GetDailyQuestion(ctx, day)
	switch {
	case err == nil:
		rep.QuestionStored, rep.QuestionID = true, id
		pick, err = r.buildPick(ctx, id)
	case errors.Is(err, pgx.ErrNoRows):
		pick, err = r.previewQuestion(ctx, day)
	}
	if err != nil {
		rep.QuestionErr = err.Error()
		return rep, nil
	}
	rep.QuestionID, rep.QuestionFits = pick.id, true
	rep.HasImage, rep.ExplanationUz, rep.ExplanationRu = pick.imageURL != "", pick.explUz, pick.explRu
	return rep, nil
}

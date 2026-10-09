package bot

import (
	"context"
	"errors"
	"fmt"
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

	maxTransientRetries = 2 // 5xx / transport, then skip the user for today
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
	if err == nil && res.Ran && res.Interrupted == "" {
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

func (r *DailyReminder) runPass(ctx context.Context, day pgtype.Date) (ReminderRunResult, error) {
	start := r.clock().Now()
	res := ReminderRunResult{Day: day.Time.Format(time.DateOnly)}

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
	today := r.clock().Now().UTC().Truncate(24 * time.Hour)

	for i := 0; res.Pending > 0; i++ {
		if ctx.Err() != nil {
			res.Interrupted = "cancelled"
			break
		}
		// The next send happens at the pacer's next slot, not now.
		at := r.clock().Now()
		if p.next.After(at) {
			at = p.next
		}
		if !inSendWindow(at) {
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
			r.logRun(res, start)
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
		switch r.deliver(ctx, p, rows[0], today) {
		case outcomeSent:
			res.Sent++
		case outcomeBlocked:
			res.Blocked++
			if err := r.Q.MarkTelegramBotUserBlocked(ctx, tgUserID); err != nil {
				r.logger().Warn("telegram daily reminder: mark blocked failed", zap.Error(err))
			}
		case outcomeWindowClosed:
			res.Interrupted = "window_closed"
		default:
			res.Errors++
		}
		if res.Interrupted != "" {
			break
		}
	}
	res.Duration = r.clock().Now().Sub(start)
	r.logRun(res, start)
	return res, nil
}

// logRun is the run's single structured line. Counts only — no user ids.
func (r *DailyReminder) logRun(res ReminderRunResult, start time.Time) {
	r.logger().Info("telegram daily reminder: run",
		zap.String("day", res.Day),
		zap.String("question_id", res.QuestionID.String()),
		zap.Int("eligible", res.Eligible),
		zap.Int("pending", res.Pending),
		zap.Int("sent", res.Sent),
		zap.Int("blocked", res.Blocked),
		zap.Int("opted_out", res.OptedOut),
		zap.Int("errors", res.Errors),
		zap.String("interrupted", res.Interrupted),
		zap.Duration("duration", r.clock().Now().Sub(start)),
	)
}

type deliveryOutcome int

const (
	outcomeSent deliveryOutcome = iota
	outcomeBlocked
	outcomeFailed
	outcomeWindowClosed
)

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

func permanent(err error) bool {
	var api *APIError
	return errors.As(err, &api) && api.Permanent()
}

// deliver sends one user's bundle: photo (caption = line + buttons) then
// the poll replying to it, or poll then the line as text. Two messages.
func (r *DailyReminder) deliver(ctx context.Context, p *pass, a sqlc.ListTelegramReminderAudienceRow, today time.Time) deliveryOutcome {
	l := langOf(a.LanguageCode)
	line := personalLineText(pickPersonalLine(a, today), l, a)
	markup := dailyKeyboard(r.WebAppURL, r.PublicBaseURL, l)
	req := p.pick.poll(l)
	chat := a.TgUserID

	classify := func(step string, err error) deliveryOutcome {
		switch {
		case errors.Is(err, errWindowClosed):
			return outcomeWindowClosed
		case unreachable(err):
			return outcomeBlocked
		}
		r.logger().Warn("telegram daily reminder: send failed", zap.String("step", step), zap.Error(err))
		return outcomeFailed
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
			req.ReplyTo = photoID
			if err := r.send(ctx, p, func(ctx context.Context) error {
				_, _, e := r.TG.SendPoll(ctx, chat, req)
				return e
			}); err != nil {
				return classify("poll", err)
			}
			return outcomeSent
		case permanent(err) && !unreachable(err):
			// Telegram could not use the image; it will not for anyone else
			// either, so the rest of the run goes text-only.
			p.imageOK = false
			r.logger().Warn("telegram daily reminder: image rejected, sending text-only", zap.Error(err))
		default:
			return classify("photo", err)
		}
	}

	if err := r.send(ctx, p, func(ctx context.Context) error {
		_, _, e := r.TG.SendPoll(ctx, chat, req)
		return e
	}); err != nil {
		return classify("poll", err)
	}
	if err := r.send(ctx, p, func(ctx context.Context) error {
		_, e := r.TG.SendText(ctx, chat, dailyFollowUp(l, line), markup)
		return e
	}); err != nil {
		return classify("text", err)
	}
	return outcomeSent
}

// send paces one Telegram call and retries it: 429 waits retry_after,
// transport errors and 5xx back off up to maxTransientRetries, permanent
// 4xx return at once.
func (r *DailyReminder) send(ctx context.Context, p *pass, call func(context.Context) error) error {
	transient, flood := 0, 0
	for {
		if err := r.pace(ctx, p); err != nil {
			return err
		}
		err := call(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
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
func (r *DailyReminder) dailyQuestion(ctx context.Context, day pgtype.Date) (dailyPick, error) {
	id, err := r.Q.GetDailyQuestion(ctx, day)
	if err == nil {
		return r.buildPick(ctx, id)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dailyPick{}, err
	}
	pick, err := r.previewQuestion(ctx, day)
	if err != nil {
		return dailyPick{}, err
	}
	if err := r.Q.InsertDailyQuestion(ctx, sqlc.InsertDailyQuestionParams{Day: day, QuestionID: pick.id}); err != nil {
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

var errNoDailyQuestion = errors.New("daily reminder: no question fits Telegram's poll limits")

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
			return dailyPick{}, fmt.Errorf("question %s (%s) does not fit a poll: %w", id, locale, err)
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
	today := now.UTC().Truncate(24 * time.Hour)
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

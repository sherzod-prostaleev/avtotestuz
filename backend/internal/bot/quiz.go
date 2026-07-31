package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/flags"
)

const (
	quizLocale  = "uz-Latn"
	quizIdleTTL = 30 * time.Minute

	cbNext = "n"
	cbStop = "x"
)

// quizMinInterval throttles /quiz and "Keyingi savol" taps. Overridable in tests.
var quizMinInterval = 3 * time.Second

// QuizService runs on-demand image-first quiz sessions in groups and DMs.
type QuizService struct {
	Q             *sqlc.Queries
	Pool          *pgxpool.Pool
	TG            *Client
	MediaBaseURL  string
	PublicBaseURL string
	WinnerSticker string // optional file_id; empty skips the sticker
	Log           *zap.Logger

	// Advance schedules the next question. Injected so tests drive the clock
	// instead of sleeping through a real countdown.
	Advance func(chatID int64, after time.Duration)
}

func (s *QuizService) logger() *zap.Logger {
	if s != nil && s.Log != nil {
		return s.Log
	}
	return zap.NewNop()
}

func (s *QuizService) ctaURL() string {
	base := strings.TrimRight(s.PublicBaseURL, "/")
	if base == "" {
		base = "https://avtotest.uz"
	}
	return base + "/uz-Latn"
}

func (s *QuizService) mediaURL(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || s.MediaBaseURL == "" {
		return ""
	}
	return strings.TrimRight(s.MediaBaseURL, "/") + "/" + strings.TrimLeft(key, "/")
}

// StartOrNext begins a session (or continues one) and sends the next question.
func (s *QuizService) StartOrNext(ctx context.Context, chatID, tgUserID int64) error {
	if s == nil || s.TG == nil || s.Q == nil {
		return fmt.Errorf("quiz service not configured")
	}
	enabled, err := flags.Bool(ctx, s.Pool, flags.KeyTelegramQuiz, true)
	if err != nil {
		return err
	}
	if !enabled {
		_, err := s.TG.SendText(ctx, chatID, "Quiz hozircha o'chirilgan. Keyinroq qayta urinib ko'ring.", nil)
		return err
	}

	session, err := s.ensureActiveSession(ctx, chatID, tgUserID)
	if err != nil {
		return err
	}
	return s.continueSession(ctx, session)
}

// ContinueScheduledGame advances chatID's quiz if a game is still active,
// and does nothing otherwise. This, not StartOrNext, is the entry point a
// scheduled auto-advance timer must call: StartOrNext creates a fresh
// session when none is active — exactly what a live /quiz or /next tap
// wants — but a timer that outlives /stop, an idle timeout, or the quiz
// flag being switched off mid-game must never resurrect a game the room
// already ended. The scheduler has no way to cancel a timer that is already
// in flight (see NewAdvanceScheduler), so this has to be the entry point
// itself, not a check before scheduling.
func (s *QuizService) ContinueScheduledGame(ctx context.Context, chatID int64) error {
	if s == nil || s.TG == nil || s.Q == nil {
		return fmt.Errorf("quiz service not configured")
	}
	enabled, err := flags.Bool(ctx, s.Pool, flags.KeyTelegramQuiz, true)
	if err != nil {
		return err
	}
	if !enabled {
		// Unlike StartOrNext, a scheduled advance says nothing when the
		// feature is off: there was no user action to acknowledge, and a
		// stray "Quiz hozircha o'chirilgan" arriving from a timer with no
		// prompting message would be confusing.
		return nil
	}
	session, err := s.Q.GetActiveQuizSessionByChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	return s.continueSession(ctx, session)
}

// continueSession is the shared tail of StartOrNext and
// ContinueScheduledGame: given a session that is known to exist, finish the
// game if it just ran out of questions, wait out the anti-spam throttle if
// the last question was very recent, and otherwise ask the next one.
func (s *QuizService) continueSession(ctx context.Context, session sqlc.TelegramQuizSession) error {
	if session.QuestionNo >= session.TotalQuestions {
		return s.finishGame(ctx, session)
	}
	if session.AskedCount > 0 && session.LastActivityAt.Valid {
		if elapsed := time.Since(session.LastActivityAt.Time); elapsed < quizMinInterval {
			wait := quizMinInterval - elapsed
			// Acknowledge the tap, then actually deliver the next question after
			// the throttle window. Returning here used to leave the chat stuck
			// with only "Biroz kuting…" and no follow-up.
			if _, err := s.TG.SendText(ctx, session.ChatID, "Biroz kuting — keyingi savol hozir chiqadi.", nil); err != nil {
				return err
			}
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
			refreshed, err := s.Q.GetActiveQuizSessionByChat(ctx, session.ChatID)
			if err != nil {
				return err
			}
			// AwaitingAnswer used to gate this: it went false the moment the
			// one expected inline-button reply came in, so re-checking it
			// caught a session that had been stopped mid-wait. Since polls
			// replaced that path, the column stays true for the whole game —
			// there is no single reply that clears it any more — so it no
			// longer tells us anything the Active check below doesn't.
			if !refreshed.Active {
				return nil
			}
			session = refreshed
		}
	}
	return s.sendNextQuestion(ctx, session)
}

// Stop ends the active quiz session for the chat and reports scores so far.
func (s *QuizService) Stop(ctx context.Context, chatID int64) error {
	session, err := s.Q.GetActiveQuizSessionByChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, sendErr := s.TG.SendText(ctx, chatID, "Faol quiz yo'q. Boshlash: /quiz", nil)
			return sendErr
		}
		return err
	}
	return s.finishGame(ctx, session)
}

// finishGame closes the session and reports the ranking. It is the only
// place a game ends, so /stop and the last question share one code path.
func (s *QuizService) finishGame(ctx context.Context, session sqlc.TelegramQuizSession) error {
	rows, err := s.Q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		return err
	}
	if err := s.Q.DeactivateQuizSession(ctx, session.ID); err != nil {
		return err
	}

	body := rankingText(rows, session.Mode, session.TotalQuestions)
	body += "\n\nDriver Go — rasmiy formatda bepul mashq"

	msgID, err := s.sendFinalMessage(ctx, session, body)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		s.celebrate(ctx, session, msgID)
	}
	return nil
}

// Telegram's 🎉 message effect. Private chats only — passing it to a group
// is an API error, which is why sendFinalMessage branches on mode.
const effectCelebrate = "5046509860389126442"

func (s *QuizService) sendFinalMessage(ctx context.Context, session sqlc.TelegramQuizSession, body string) (int64, error) {
	if session.Mode == "group" {
		return s.TG.SendText(ctx, session.ChatID, body, s.ctaMarkup())
	}
	return s.TG.SendTextWithEffect(ctx, session.ChatID, body, effectCelebrate, s.ctaMarkup())
}

// celebrate adds the decoration that survives a group chat. A reaction always
// works; the sticker is optional because an unverified file_id would fail on
// every finished game. Neither failure is worth aborting the result message.
func (s *QuizService) celebrate(ctx context.Context, session sqlc.TelegramQuizSession, msgID int64) {
	if session.Mode != "group" || msgID == 0 {
		return
	}
	if err := s.TG.SetMessageReaction(ctx, session.ChatID, msgID, "🎉"); err != nil {
		s.logger().Debug("quiz: reaction failed", zap.Error(err))
	}
	if sticker := s.winnerStickerID(); sticker != "" {
		if err := s.TG.SendSticker(ctx, session.ChatID, sticker); err != nil {
			s.logger().Debug("quiz: sticker failed", zap.Error(err))
		}
	}
}

// HandleCallback processes the next / stop taps that follow a finished game.
func (s *QuizService) HandleCallback(ctx context.Context, cq CallbackQuery) error {
	if s == nil || s.TG == nil {
		return nil
	}
	_ = s.TG.AnswerCallbackQuery(ctx, cq.ID, "", false)
	if cq.Message == nil {
		return nil
	}
	chatID := cq.Message.Chat.ID
	switch strings.TrimSpace(cq.Data) {
	case cbNext:
		return s.StartOrNext(ctx, chatID, cq.From.ID)
	case cbStop:
		return s.Stop(ctx, chatID)
	default:
		return nil
	}
}

func (s *QuizService) ensureActiveSession(ctx context.Context, chatID, tgUserID int64) (sqlc.TelegramQuizSession, error) {
	session, err := s.Q.GetActiveQuizSessionByChat(ctx, chatID)
	switch {
	case err == nil:
		if session.LastActivityAt.Valid && time.Since(session.LastActivityAt.Time) > quizIdleTTL {
			if err := s.Q.DeactivateQuizSession(ctx, session.ID); err != nil {
				return sqlc.TelegramQuizSession{}, err
			}
			return s.Q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
				ChatID: chatID, StartedByTgUserID: tgUserID,
			})
		}
		return session, nil
	case errors.Is(err, pgx.ErrNoRows):
		return s.Q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
			ChatID: chatID, StartedByTgUserID: tgUserID,
		})
	default:
		return sqlc.TelegramQuizSession{}, err
	}
}

// StartGame begins a quiz, recording whether the chat is a group so the
// final message can pick its format. Chat type decides the mode — a group
// with one participant is still a group.
func (s *QuizService) StartGame(ctx context.Context, chatID, tgUserID int64, chatType string) error {
	if s == nil || s.TG == nil || s.Q == nil {
		return fmt.Errorf("quiz service not configured")
	}
	enabled, err := flags.Bool(ctx, s.Pool, flags.KeyTelegramQuiz, true)
	if err != nil {
		return err
	}
	if !enabled {
		_, err := s.TG.SendText(ctx, chatID, "Quiz hozircha o'chirilgan. Keyinroq qayta urinib ko'ring.", nil)
		return err
	}

	session, err := s.ensureActiveSession(ctx, chatID, tgUserID)
	if err != nil {
		return err
	}
	if session.QuestionNo != 0 && session.QuestionNo >= session.TotalQuestions {
		// A finished game whose finishGame never ran (crashed process, lost
		// advance timer) is still "active" in the row. question_no != 0
		// would otherwise skip the intro below and fall straight into
		// sendNextQuestion, asking question after question forever. /quiz
		// on a dead game should behave like /quiz on no game: start a fresh
		// one.
		if err := s.Q.DeactivateQuizSession(ctx, session.ID); err != nil {
			return err
		}
		session, err = s.Q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
			ChatID: chatID, StartedByTgUserID: tgUserID,
		})
		if err != nil {
			return err
		}
	}
	if session.QuestionNo == 0 {
		mode := "solo"
		if IsGroupChat(chatType) {
			mode = "group"
		}
		total := s.quizQuestionCount(ctx)
		if err := s.Q.SetQuizSessionMode(ctx, sqlc.SetQuizSessionModeParams{
			ID: session.ID, Mode: mode, TotalQuestions: int32(total),
		}); err != nil {
			return err
		}
		session.Mode = mode
		session.TotalQuestions = int32(total)
		intro := fmt.Sprintf("🚦 Quiz boshlandi!\n%d savol · har biriga %d sekund",
			total, s.quizSeconds(ctx))
		if _, err := s.TG.SendText(ctx, chatID, intro, nil); err != nil {
			return err
		}
	}
	return s.sendNextQuestion(ctx, session)
}

func (s *QuizService) sendNextQuestion(ctx context.Context, session sqlc.TelegramQuizSession) error {
	qID, err := s.pickPollableQuestionID(ctx)
	if err != nil {
		return err
	}
	if qID == uuid.Nil {
		_, err := s.TG.SendText(ctx, session.ChatID, "Hozircha mos savol topilmadi. Keyinroq qayta urinib ko'ring.", nil)
		return err
	}

	detail, err := s.Q.GetQuestion(ctx, sqlc.GetQuestionParams{ID: qID, Locale: quizLocale})
	if err != nil {
		return err
	}
	answers, err := s.Q.ListQuizAnswers(ctx, sqlc.ListQuizAnswersParams{
		QuestionID: qID, Locale: quizLocale,
	})
	if err != nil {
		return err
	}

	questionNo, err := s.Q.AdvanceQuizSessionQuestion(ctx, session.ID)
	if err != nil {
		return err
	}

	// The photo goes first and the poll replies to it: Telegram polls cannot
	// carry an image, so one question is two messages.
	var photoMsgID int64
	if detail.ImageKey.Valid {
		if url := s.mediaURL(detail.ImageKey.String); url != "" {
			caption := fmt.Sprintf("Savol %d/%d", questionNo, session.TotalQuestions)
			photoMsgID, err = s.TG.SendPhoto(ctx, session.ChatID, url, caption, nil)
			if err != nil {
				s.logger().Warn("quiz: sendPhoto failed, continuing with poll only",
					zap.Error(err), zap.Int64("chat_id", session.ChatID))
				photoMsgID = 0
			}
		}
	}

	req, err := buildPollRequest(detail.Text, answers, "", s.quizSeconds(ctx), photoMsgID)
	if err != nil {
		// The corpus filter should have excluded this question; say so and
		// move on rather than stranding the chat.
		s.logger().Warn("quiz: question is not pollable, skipping",
			zap.Error(err), zap.String("question_id", qID.String()))
		_, sendErr := s.TG.SendText(ctx, session.ChatID, "Bu savol o'tkazib yuborildi. /next bilan davom eting.", nil)
		return sendErr
	}

	msgID, pollID, err := s.TG.SendPoll(ctx, session.ChatID, req)
	if err != nil {
		return err
	}

	if err := s.Q.CreateQuizPoll(ctx, sqlc.CreateQuizPollParams{
		PollID:     pollID,
		SessionID:  session.ID,
		QuestionID: uuid.NullUUID{UUID: qID, Valid: true},
		QuestionNo: questionNo,
		CorrectIdx: int32(req.CorrectIdx),
	}); err != nil {
		return err
	}

	if err := s.Q.SetQuizSessionQuestion(ctx, sqlc.SetQuizSessionQuestionParams{
		ID:              session.ID,
		QuestionID:      uuid.NullUUID{UUID: qID, Valid: true},
		AnswerMessageID: msgID,
	}); err != nil {
		return err
	}

	s.scheduleAdvance(session.ChatID, s.quizSeconds(ctx))
	return nil
}

// scheduleAdvance moves the game on after the poll closes. Telegram closes
// the poll itself; this only decides when to ask the next question. A lost
// timer (deploy, restart) is recoverable with /next — it is deliberately not
// durable state.
func (s *QuizService) scheduleAdvance(chatID int64, seconds int) {
	if s.Advance == nil {
		return
	}
	s.Advance(chatID, time.Duration(seconds+1)*time.Second)
}

func (s *QuizService) ctaMarkup() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: "Ilovada ochish", URL: s.ctaURL()},
		}},
	}
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max < 1 {
		return ""
	}
	return string(runes[:max-1]) + "…"
}

// IsGroupChat reports Telegram group-like chats.
func IsGroupChat(chatType string) bool {
	switch chatType {
	case "group", "supergroup":
		return true
	default:
		return false
	}
}

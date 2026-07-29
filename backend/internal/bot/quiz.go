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
	quizLocale          = "uz-Latn"
	quizIdleTTL         = 30 * time.Minute
	quizCaptionMaxRunes = 1000
	quizButtonMaxRunes  = 60

	cbAnswerPrefix = "a:"
	cbNext         = "n"
	cbStop         = "x"
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
	if session.AwaitingAnswer {
		_, err := s.TG.SendText(ctx, chatID, "Avval joriy savolga javob bering, keyin davom etamiz.", nil)
		return err
	}
	if session.AskedCount > 0 && session.LastActivityAt.Valid {
		if elapsed := time.Since(session.LastActivityAt.Time); elapsed < quizMinInterval {
			wait := quizMinInterval - elapsed
			// Acknowledge the tap, then actually deliver the next question after
			// the throttle window. Returning here used to leave the chat stuck
			// with only "Biroz kuting…" and no follow-up.
			if _, err := s.TG.SendText(ctx, chatID, "Biroz kuting — keyingi savol hozir chiqadi.", nil); err != nil {
				return err
			}
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
			session, err = s.Q.GetActiveQuizSessionByChat(ctx, chatID)
			if err != nil {
				return err
			}
			if !session.Active || session.AwaitingAnswer {
				return nil
			}
		}
	}
	return s.sendNextQuestion(ctx, session)
}

// Stop ends the active quiz session for the chat.
func (s *QuizService) Stop(ctx context.Context, chatID int64) error {
	session, err := s.Q.GetActiveQuizSessionByChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, sendErr := s.TG.SendText(ctx, chatID, "Faol quiz yo'q. Boshlash: /quiz", nil)
			return sendErr
		}
		return err
	}
	if err := s.Q.DeactivateQuizSession(ctx, session.ID); err != nil {
		return err
	}
	summary := fmt.Sprintf(
		"Quiz to'xtatildi.\nNatija: %d/%d to'g'ri.\n\nDriver Go — rasmiy formatda bepul mashq\n%s",
		session.CorrectCount, session.AskedCount, s.ctaURL(),
	)
	_, err = s.TG.SendText(ctx, chatID, summary, s.ctaMarkup())
	return err
}

// HandleCallback processes inline answer / next / stop taps.
func (s *QuizService) HandleCallback(ctx context.Context, cq CallbackQuery) error {
	if s == nil || s.TG == nil {
		return nil
	}
	_ = s.TG.AnswerCallbackQuery(ctx, cq.ID, "", false)
	if cq.Message == nil {
		return nil
	}
	chatID := cq.Message.Chat.ID
	data := strings.TrimSpace(cq.Data)

	switch {
	case data == cbNext:
		return s.StartOrNext(ctx, chatID, cq.From.ID)
	case data == cbStop:
		return s.Stop(ctx, chatID)
	case strings.HasPrefix(data, cbAnswerPrefix):
		idx, ok := parseAnswerIndex(data)
		if !ok {
			return nil
		}
		return s.handleAnswer(ctx, chatID, cq.Message.MessageID, idx)
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

func (s *QuizService) sendNextQuestion(ctx context.Context, session sqlc.TelegramQuizSession) error {
	qID, err := s.pickQuestionID(ctx)
	if err != nil {
		return err
	}
	if qID == uuid.Nil {
		_, err := s.TG.SendText(ctx, session.ChatID, "Hozircha savol topilmadi. Keyinroq qayta urinib ko'ring.", nil)
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
	if len(answers) < 2 {
		_, err := s.TG.SendText(ctx, session.ChatID, "Bu savolda javob variantlari yetarli emas. /quiz bilan qayta urinib ko'ring.", nil)
		return err
	}

	text := strings.TrimSpace(detail.Text)
	if text == "" {
		text = "Savol"
	}
	caption := truncateRunes(text, quizCaptionMaxRunes)
	markup := answerMarkup(answers)

	var msgID int64
	photoURL := ""
	if detail.ImageKey.Valid {
		photoURL = s.mediaURL(detail.ImageKey.String)
	}
	if photoURL != "" {
		msgID, err = s.TG.SendPhoto(ctx, session.ChatID, photoURL, caption, markup)
		if err != nil {
			s.logger().Warn("quiz: sendPhoto failed, falling back to text",
				zap.Error(err), zap.Int64("chat_id", session.ChatID))
			msgID, err = s.TG.SendText(ctx, session.ChatID, caption, markup)
		}
	} else {
		msgID, err = s.TG.SendText(ctx, session.ChatID, caption, markup)
	}
	if err != nil {
		return err
	}

	return s.Q.SetQuizSessionQuestion(ctx, sqlc.SetQuizSessionQuestionParams{
		ID:              session.ID,
		QuestionID:      uuid.NullUUID{UUID: qID, Valid: true},
		AnswerMessageID: msgID,
	})
}

func (s *QuizService) pickQuestionID(ctx context.Context) (uuid.UUID, error) {
	ids, err := s.Q.RandomQuestionIDsByImagePresence(ctx, sqlc.RandomQuestionIDsByImagePresenceParams{
		HasImage: true, LimitCount: 1,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if len(ids) > 0 {
		return ids[0], nil
	}
	ids, err = s.Q.RandomQuestionIDs(ctx, 1)
	if err != nil {
		return uuid.Nil, err
	}
	if len(ids) == 0 {
		return uuid.Nil, nil
	}
	return ids[0], nil
}

func (s *QuizService) handleAnswer(ctx context.Context, chatID, messageID int64, answerIdx int) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := sqlc.New(tx)

	session, err := qtx.GetActiveQuizSessionByChatForUpdate(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !session.AwaitingAnswer || !session.QuestionID.Valid {
		return nil
	}
	if session.AnswerMessageID != 0 && messageID != 0 && session.AnswerMessageID != messageID {
		// Tap on a stale keyboard — ignore quietly.
		return nil
	}

	answers, err := qtx.ListQuizAnswers(ctx, sqlc.ListQuizAnswersParams{
		QuestionID: session.QuestionID.UUID, Locale: quizLocale,
	})
	if err != nil {
		return err
	}
	if answerIdx < 0 || answerIdx >= len(answers) {
		return nil
	}
	chosen := answers[answerIdx]
	correctDelta := int32(0)
	if chosen.IsCorrect {
		correctDelta = 1
	}
	n, err := qtx.MarkQuizSessionAnswered(ctx, sqlc.MarkQuizSessionAnsweredParams{
		CorrectDelta: correctDelta,
		ID:           session.ID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return nil // double-tap race loser
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	_ = s.TG.EditMessageReplyMarkup(ctx, chatID, session.AnswerMessageID, nil)

	var correctText string
	for _, a := range answers {
		if a.IsCorrect {
			correctText = strings.TrimSpace(a.Text)
			break
		}
	}

	var body string
	if chosen.IsCorrect {
		body = "✅ To'g'ri!"
	} else {
		body = "❌ Noto'g'ri."
		if correctText != "" {
			body += "\nTo'g'ri javob: " + truncateRunes(correctText, 200)
		}
	}
	body += fmt.Sprintf("\n\nNatija: %d/%d", session.CorrectCount+correctDelta, session.AskedCount)
	body += "\n\nDriver Go — rasmiy formatda bepul mashq"

	_, err = s.TG.SendText(ctx, chatID, body, s.afterAnswerMarkup())
	return err
}

func (s *QuizService) ctaMarkup() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: "Ilovada ochish", URL: s.ctaURL()},
		}},
	}
}

func (s *QuizService) afterAnswerMarkup() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "Keyingi savol", CallbackData: cbNext},
				{Text: "To'xtatish", CallbackData: cbStop},
			},
			{
				{Text: "Ilovada ochish", URL: s.ctaURL()},
			},
		},
	}
}

func answerMarkup(answers []sqlc.ListQuizAnswersRow) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(answers))
	for i, a := range answers {
		label := strings.TrimSpace(a.Text)
		if label == "" {
			label = fmt.Sprintf("%d-javob", i+1)
		}
		label = truncateRunes(label, quizButtonMaxRunes)
		rows = append(rows, []InlineKeyboardButton{{
			Text:         label,
			CallbackData: fmt.Sprintf("%s%d", cbAnswerPrefix, i),
		}})
	}
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func parseAnswerIndex(data string) (int, bool) {
	raw := strings.TrimPrefix(data, cbAnswerPrefix)
	if raw == "" {
		return 0, false
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
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

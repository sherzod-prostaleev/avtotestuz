package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
)

const (
	msgStartUnlinked = "Salom! Bu Driver Go boti.\n\n" +
		"Mashq: /quiz\nTo'xtatish: /stop\n\n" +
		"Hisobingizni ulash uchun: saytda profilingizga kiring va " +
		"\"Telegram bilan bog'lash\" tugmasini bosing."
	msgStartLinkedFmt = "Salom, %s!\n\nMashq: /quiz\nHolat: /status\nUzish: /unlink"
	msgStartGroup     = "Driver Go quiz boti guruhda.\n\n" +
		"Boshlash: /quiz\nKeyingi: /next\nTo'xtatish: /stop\n\n" +
		"Rasmiy formatdagi savollar — bepul sinab ko'ring."
	msgLinkUsage       = "Havoladagi token topilmadi. /link <token> ko'rinishida yozing yoki saytdan yangi havola oling."
	msgLinkSuccess     = "Hisobingiz muvaffaqiyatli ulandi! /status buyrug'i bilan tekshiring. Mashq: /quiz"
	msgLinkAlreadyOK   = "Bu Telegram hisobi allaqachon shu profilga ulangan."
	msgLinkExpired     = "Havola muddati tugagan. Saytdan yangi havola oling."
	msgLinkUsed        = "Bu havola allaqachon ishlatilgan. Saytdan yangi havola oling."
	msgLinkNotFound    = "Havola noto'g'ri yoki muddati o'tgan. Saytdan yangi havola oling."
	msgLinkElsewhere   = "Bu Telegram hisobi boshqa profilga ulangan. Avval o'sha profildan uzing (/unlink) yoki qo'llab-quvvatlashga yozing."
	msgLinkInternal    = "Ulashda xatolik yuz berdi. Birozdan keyin qayta urinib ko'ring."
	msgStatusUnlinked  = "Hisobingiz hali ulanmagan. Ulash uchun /start buyrug'ini bosing va ko'rsatmalarga amal qiling."
	msgUnlinkOK        = "Telegram hisobi uzildi. Qayta ulash uchun saytdan yangi havola oling."
	msgUnlinkNone      = "Bu Telegram hisobi hech qaysi profilga ulanmagan."
	msgUnknown         = "Noma'lum buyruq. Mavjud: /quiz, /next, /stop, /start, /link, /status, /unlink"
	msgQuizUnavailable = "Quiz hozircha ishlamayapti. Keyinroq qayta urinib ko'ring."
)

// Bot dispatches inbound Telegram updates. Link redeem stays in-process
// (M4-06); quiz sessions are handled by QuizService (M4-07).
type Bot struct {
	Link          *LinkService
	Quiz          *QuizService
	Billing       billing.Service
	Progress      *progress.Service
	TG            *Client
	PublicBaseURL string
	Log           *zap.Logger
}

func (b *Bot) logger() *zap.Logger {
	if b.Log != nil {
		return b.Log
	}
	return zap.NewNop()
}

// HandleUpdate processes one Telegram update. Infra failures return an
// error; bad user input always gets a reply so webhooks can stay 200.
func (b *Bot) HandleUpdate(ctx context.Context, u Update) error {
	if u.MyChatMember != nil {
		return b.handleMyChatMember(ctx, u.MyChatMember)
	}
	if u.PollAnswer != nil {
		if b.Quiz == nil {
			return nil
		}
		if err := b.Quiz.HandlePollAnswer(ctx, *u.PollAnswer); err != nil {
			b.logger().Error("bot: poll answer failed", zap.Error(err))
			return err
		}
		return nil
	}
	if u.CallbackQuery != nil {
		if b.Quiz == nil {
			return nil
		}
		if err := b.Quiz.HandleCallback(ctx, *u.CallbackQuery); err != nil {
			b.logger().Error("bot: quiz callback failed", zap.Error(err))
			return err
		}
		return nil
	}
	if u.Message == nil || u.Message.From == nil {
		return nil
	}
	chatID := u.Message.Chat.ID
	tgUserID := u.Message.From.ID
	username := u.Message.From.Username
	chatType := u.Message.Chat.Type

	fields := strings.Fields(strings.TrimSpace(u.Message.Text))
	if len(fields) == 0 {
		return nil
	}
	cmd, arg := normalizeCommand(fields[0]), ""
	if len(fields) > 1 {
		arg = fields[1]
	}

	switch cmd {
	case "/quiz":
		if b.Quiz == nil {
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
		if err := b.Quiz.StartGame(ctx, chatID, tgUserID, chatType); err != nil {
			b.logger().Error("bot: quiz start failed", zap.Error(err), zap.Int64("chat_id", chatID))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgQuizUnavailable))
		}
		return nil
	case "/next":
		if b.Quiz == nil {
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
		if err := b.Quiz.StartOrNext(ctx, chatID, tgUserID); err != nil {
			b.logger().Error("bot: quiz next failed", zap.Error(err), zap.Int64("chat_id", chatID))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgQuizUnavailable))
		}
		return nil
	case "/stop":
		if b.Quiz == nil {
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
		if err := b.Quiz.Stop(ctx, chatID); err != nil {
			b.logger().Error("bot: quiz stop failed", zap.Error(err))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
		}
		return nil
	case "/unlink":
		reply, err := b.handleUnlink(ctx, tgUserID)
		if err != nil {
			b.logger().Error("bot: unlink failed", zap.Error(err))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
		}
		return b.TG.SendMessage(ctx, chatID, reply)
	case "/start":
		if IsGroupChat(chatType) && arg == "" {
			markup := &InlineKeyboardMarkup{}
			if b.Quiz != nil {
				markup = b.Quiz.ctaMarkup()
			}
			_, err := b.TG.SendText(ctx, chatID, msgStartGroup, markup)
			return err
		}
		reply, err := b.dispatchLegacy(ctx, cmd, arg, tgUserID, username)
		if err != nil {
			b.logger().Error("bot: dispatch failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
		}
		if reply == "" {
			return nil
		}
		return b.TG.SendMessage(ctx, chatID, reply)
	case "/link", "/status":
		reply, err := b.dispatchLegacy(ctx, cmd, arg, tgUserID, username)
		if err != nil {
			b.logger().Error("bot: dispatch failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
		}
		if reply == "" {
			return nil
		}
		return b.TG.SendMessage(ctx, chatID, reply)
	default:
		return b.TG.SendMessage(ctx, chatID, msgUnknown)
	}
}

func (b *Bot) dispatchLegacy(ctx context.Context, cmd, arg string, tgUserID int64, username string) (string, error) {
	switch cmd {
	case "/start":
		if arg == "" {
			return b.handleStart(ctx, tgUserID)
		}
		return b.handleLink(ctx, arg, tgUserID, username)
	case "/link":
		if arg == "" {
			return msgLinkUsage, nil
		}
		return b.handleLink(ctx, arg, tgUserID, username)
	case "/status":
		return b.handleStatus(ctx, tgUserID)
	default:
		return msgUnknown, nil
	}
}

func (b *Bot) handleMyChatMember(ctx context.Context, upd *ChatMemberUpd) error {
	if b.Quiz == nil || b.Quiz.Q == nil || upd == nil {
		return nil
	}
	status := upd.NewChatMember.Status
	if status == "" {
		status = "member"
	}
	return b.Quiz.Q.UpsertTelegramChat(ctx, sqlc.UpsertTelegramChatParams{
		ChatID:    upd.Chat.ID,
		Title:     upd.Chat.Title,
		ChatType:  upd.Chat.Type,
		BotStatus: status,
	})
}

func (b *Bot) handleUnlink(ctx context.Context, tgUserID int64) (string, error) {
	if err := b.Link.Unlink(ctx, tgUserID); err != nil {
		if errors.Is(err, ErrNotLinked) {
			return msgUnlinkNone, nil
		}
		return "", err
	}
	return msgUnlinkOK, nil
}

// normalizeCommand strips a "@BotUsername" suffix, which Telegram appends
// to commands in group chats (e.g. "/start@AvtoTestBot").
func normalizeCommand(cmd string) string {
	if i := strings.IndexByte(cmd, '@'); i != -1 {
		cmd = cmd[:i]
	}
	return strings.ToLower(cmd)
}

func (b *Bot) handleStart(ctx context.Context, tgUserID int64) (string, error) {
	account, err := b.Link.Q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil:
		name := account.Username
		if name == "" {
			name = "do'stim"
		}
		return fmt.Sprintf(msgStartLinkedFmt, name), nil
	case errors.Is(err, pgx.ErrNoRows):
		return msgStartUnlinked, nil
	default:
		return "", err
	}
}

func (b *Bot) handleLink(ctx context.Context, token string, tgUserID int64, username string) (string, error) {
	res, err := b.Link.RedeemLinkToken(ctx, token, tgUserID, username)
	if err != nil {
		switch {
		case isLinkUserError(err):
			return linkErrorMessage(err), nil
		default:
			return "", err
		}
	}
	if res.AlreadyLinked {
		return msgLinkAlreadyOK, nil
	}
	return msgLinkSuccess, nil
}

func isLinkUserError(err error) bool {
	return errors.Is(err, ErrLinkTokenNotFound) || errors.Is(err, ErrLinkTokenExpired) ||
		errors.Is(err, ErrLinkTokenAlreadyUsed) || errors.Is(err, ErrTelegramAccountLinkedElsewhere)
}

func linkErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrLinkTokenExpired):
		return msgLinkExpired
	case errors.Is(err, ErrLinkTokenAlreadyUsed):
		return msgLinkUsed
	case errors.Is(err, ErrTelegramAccountLinkedElsewhere):
		return msgLinkElsewhere
	default:
		return msgLinkNotFound
	}
}

func (b *Bot) handleStatus(ctx context.Context, tgUserID int64) (string, error) {
	account, err := b.Link.Q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil:
		// linked — continue below
	case errors.Is(err, pgx.ErrNoRows):
		return msgStatusUnlinked, nil
	default:
		return "", err
	}

	active, until, err := b.Billing.Status(ctx, account.ProfileID)
	if err != nil {
		return "", err
	}
	streak, err := b.Progress.GetStreak(ctx, account.ProfileID)
	if err != nil {
		return "", err
	}

	vipLine := "VIP: yo'q"
	if active && until != nil {
		vipLine = fmt.Sprintf("VIP: faol (%s gacha)", until.Format("2006-01-02"))
	}
	streakLine := fmt.Sprintf("Streak: %d kun (rekord: %d)", streak.Current, streak.Best)
	return strings.Join([]string{vipLine, streakLine, "Mashq: /quiz"}, "\n"), nil
}

// deepLink builds the t.me deep link a client hands to a freshly generated
// LinkToken. Shared with handlers.go's web endpoint.
func deepLink(botUsername, token string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botUsername, token)
}

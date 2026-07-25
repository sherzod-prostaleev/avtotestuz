package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/progress"
)

const (
	msgStartUnlinked = "Salom! Bu AvtoTest botining poydevor versiyasi.\n\n" +
		"Hisobingizni ulash uchun: saytda/ilovada profilingizga kiring va " +
		"\"Telegram bilan bog'lash\" tugmasini bosing — sizga shu botga " +
		"o'tadigan bir martalik havola beriladi."
	msgStartLinkedFmt = "Salom, %s! Hisobingiz allaqachon ulangan. /status buyrug'i bilan holatingizni ko'rishingiz mumkin."
	msgLinkUsage      = "Havoladagi token topilmadi. /link <token> ko'rinishida yozing yoki saytdan yangi havola oling."
	msgLinkSuccess    = "Hisobingiz muvaffaqiyatli ulandi! /status buyrug'i bilan tekshiring."
	msgLinkAlreadyOK  = "Bu Telegram hisobi allaqachon shu profilga ulangan."
	msgLinkExpired    = "Havola muddati tugagan. Saytdan yangi havola oling."
	msgLinkUsed       = "Bu havola allaqachon ishlatilgan. Saytdan yangi havola oling."
	msgLinkNotFound   = "Havola noto'g'ri yoki muddati o'tgan. Saytdan yangi havola oling."
	msgLinkElsewhere  = "Bu Telegram hisobi boshqa profilga ulangan. Avval o'sha profildan uzing yoki qo'llab-quvvatlash xizmatiga murojaat qiling."
	msgLinkInternal   = "Ulashda xatolik yuz berdi. Birozdan keyin qayta urinib ko'ring."
	msgStatusUnlinked = "Hisobingiz hali ulanmagan. Ulash uchun /start buyrug'ini bosing va ko'rsatmalarga amal qiling."
	msgUnknown        = "Noma'lum buyruq. Mavjud buyruqlar: /start, /link <token>, /status"
)

// Bot dispatches inbound Telegram updates to the M4-06 command set. It is
// the one place §3.2's "redeem happens in-process" design decision lives:
// HandleUpdate calls Link.RedeemLinkToken directly, never over HTTP.
type Bot struct {
	Link     *LinkService
	Billing  billing.Service
	Progress *progress.Service
	TG       *Client
	Log      *zap.Logger
}

func (b *Bot) logger() *zap.Logger {
	if b.Log != nil {
		return b.Log
	}
	return zap.NewNop()
}

// HandleUpdate processes one Telegram update. It only returns an error for
// infra-level failures (Telegram API unreachable, DB down) — bad user input
// always gets a reply, never a returned error, so a webhook caller can
// always respond 200 and a long-poll loop never gets stuck retrying a
// message a user typo'd.
func (b *Bot) HandleUpdate(ctx context.Context, u Update) error {
	if u.Message == nil || u.Message.From == nil {
		return nil // nothing this bot handles yet (edited messages, etc.)
	}
	chatID := u.Message.Chat.ID
	tgUserID := u.Message.From.ID
	username := u.Message.From.Username

	reply, err := b.dispatch(ctx, u.Message.Text, tgUserID, username)
	if err != nil {
		b.logger().Error("bot: dispatch failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
		reply = msgLinkInternal
	}
	if reply == "" {
		return nil
	}
	return b.TG.SendMessage(ctx, chatID, reply)
}

// dispatch returns the reply text for a command, and an error only for
// infra failures the caller should log (not show verbatim to the user).
func (b *Bot) dispatch(ctx context.Context, text string, tgUserID int64, username string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return msgUnknown, nil
	}
	cmd, arg := normalizeCommand(fields[0]), ""
	if len(fields) > 1 {
		arg = fields[1]
	}

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
	return strings.Join([]string{vipLine, streakLine}, "\n"), nil
}

// deepLink builds the t.me deep link a client hands to a freshly generated
// LinkToken. Shared with handlers.go's web endpoint.
func deepLink(botUsername, token string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botUsername, token)
}

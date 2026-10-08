package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/auth"
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
	msgLinkUsage         = "Havoladagi token topilmadi. /link <token> ko'rinishida yozing yoki saytdan yangi havola oling."
	msgLinkSuccess       = "Hisobingiz muvaffaqiyatli ulandi! /status buyrug'i bilan tekshiring. Mashq: /quiz"
	msgLinkAlreadyOK     = "Bu Telegram hisobi allaqachon shu profilga ulangan."
	msgLinkExpired       = "Havola muddati tugagan. Saytdan yangi havola oling."
	msgLinkUsed          = "Bu havola allaqachon ishlatilgan. Saytdan yangi havola oling."
	msgLinkNotFound      = "Havola noto'g'ri yoki muddati o'tgan. Saytdan yangi havola oling."
	msgLinkElsewhere     = "Bu Telegram hisobi boshqa profilga ulangan. Avval o'sha profildan uzing (/unlink) yoki qo'llab-quvvatlashga yozing."
	msgLinkInternal      = "Ulashda xatolik yuz berdi. Birozdan keyin qayta urinib ko'ring."
	msgStatusUnlinked    = "Hisobingiz hali ulanmagan. Ulash uchun /start buyrug'ini bosing va ko'rsatmalarga amal qiling."
	msgUnlinkOK          = "Telegram hisobi uzildi. Qayta ulash uchun saytdan yangi havola oling."
	msgUnlinkNone        = "Bu Telegram hisobi hech qaysi profilga ulanmagan."
	msgUnknown           = "Noma'lum buyruq. Mavjud: /quiz, /next, /stop, /start, /link, /status, /unlink"
	msgQuizUnavailable   = "Quiz hozircha ishlamayapti. Keyinroq qayta urinib ko'ring."
	msgResetNeedContact  = "Parolni tiklash uchun shu Telegram akkauntning telefon raqamini yuboring. Raqam hisobdagi telefon bilan bir xil bo'lishi kerak."
	msgResetShareContact = "Telefon raqamini yuborish"
	msgResetVerified     = "Tasdiqlandi. Brauzerdagi Driver Go sahifasiga qayting va yangi parolni kiriting."
	msgResetBackToSite   = "Brauzerdagi Driver Go sahifasiga qayting."
	msgResetInvalid      = "Havola noto'g'ri yoki muddati o'tgan. Saytdan yangi tiklash so'rang."
	msgResetConfirmFmt   = "Parolni tiklash so'raldi: %s. Bu siz bo'lsangiz «Ha, men» ni bosing. Agar siz so'ramagan bo'lsangiz, «Yo'q» ni bosing — «Ha, men» boshqa qurilmada parolni o'zgartirishga ruxsat beradi."
	msgResetConfirmYes   = "Ha, men"
	msgResetConfirmNo    = "Yo'q"
	msgResetCancelled    = "Parolni tiklash bekor qilindi. Parolingiz o'zgartirilmadi."
	msgResetConfirmStale = "Bu so'rov endi amal qilmaydi."
)

// Password-reset confirm buttons: "pwr:y:<nonce>" / "pwr:n:<nonce>". The
// nonce is per reset question (auth.TelegramResetBegin.ConfirmNonce), never
// the reset token itself.
const (
	cbResetPrefix = "pwr:"
	cbResetYes    = cbResetPrefix + "y:"
	cbResetNo     = cbResetPrefix + "n:"
)

// Bot dispatches inbound Telegram updates. Link redeem stays in-process
// (M4-06); quiz sessions are handled by QuizService (M4-07).
type Bot struct {
	// WebAppURL is the Mini App entry point; empty means no launcher button.
	WebAppURL     string
	Link          *LinkService
	Quiz          *QuizService
	Billing       billing.Service
	Progress      *progress.Service
	TG            *Client
	Auth          *auth.Service
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
		if strings.HasPrefix(u.CallbackQuery.Data, cbResetPrefix) {
			return b.handlePasswordResetCallback(ctx, *u.CallbackQuery)
		}
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

	if u.Message.Contact != nil {
		if IsGroupChat(chatType) {
			return nil
		}
		return b.handlePasswordResetContact(ctx, chatID, tgUserID, u.Message.Contact)
	}

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
		if raw, ok := auth.ParsePasswordResetStartPayload(arg); ok {
			if IsGroupChat(chatType) {
				return b.TG.SendMessage(ctx, chatID, msgResetInvalid)
			}
			return b.handlePasswordResetStart(ctx, chatID, tgUserID, raw)
		}
		reply, err := b.dispatchLegacy(ctx, cmd, arg, tgUserID, username)
		if err != nil {
			b.logger().Error("bot: dispatch failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
			return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
		}
		if reply == "" {
			return nil
		}
		// Only the plain /start (no link or reset payload) gets the launcher;
		// the group case returned above, because Telegram rejects web_app
		// buttons outside private chats.
		if arg == "" && b.WebAppURL != "" {
			_, err := b.TG.SendText(ctx, chatID, reply, &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
				{Text: "📱 DriverGo'ni ochish", WebApp: &WebAppInfo{URL: b.WebAppURL}},
			}}})
			return err
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

func contactRequestKeyboard() ReplyKeyboardMarkup {
	return ReplyKeyboardMarkup{
		Keyboard: [][]KeyboardButton{{
			{Text: msgResetShareContact, RequestContact: true},
		}},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

func (b *Bot) handlePasswordResetStart(ctx context.Context, chatID, tgUserID int64, rawToken string) error {
	if b.Auth == nil {
		return b.TG.SendMessage(ctx, chatID, msgResetInvalid)
	}
	res, err := b.Auth.BeginTelegramPasswordReset(ctx, rawToken, tgUserID)
	if err != nil {
		b.logger().Error("bot: password reset begin failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
		return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
	}
	switch res.Outcome {
	case auth.TelegramResetNeedContact:
		_, err := b.TG.SendChatText(ctx, chatID, msgResetNeedContact, contactRequestKeyboard())
		return err
	case auth.TelegramResetNeedConfirm:
		return b.askPasswordResetConfirm(ctx, chatID, res)
	case auth.TelegramResetVerified:
		_, err := b.TG.SendChatText(ctx, chatID, msgResetVerified, ReplyKeyboardRemove{RemoveKeyboard: true})
		return err
	default:
		return b.TG.SendMessage(ctx, chatID, msgResetInvalid)
	}
}

func (b *Bot) handlePasswordResetContact(ctx context.Context, chatID, tgUserID int64, contact *Contact) error {
	if contact == nil || b.Auth == nil {
		return nil
	}
	res, err := b.Auth.ConfirmTelegramPasswordResetContact(ctx, tgUserID, contact.UserID, contact.PhoneNumber)
	if err != nil {
		b.logger().Error("bot: password reset contact failed", zap.Error(err), zap.Int64("tg_user_id", tgUserID))
		return errors.Join(err, b.TG.SendMessage(ctx, chatID, msgLinkInternal))
	}
	switch res.Outcome {
	case auth.TelegramResetNeedConfirm:
		return b.askPasswordResetConfirm(ctx, chatID, res)
	case auth.TelegramResetNone:
		// Most likely the Mini App's "share phone" sheet, which also posts
		// the contact here; nobody is waiting for a reset answer.
		return nil
	}
	return b.TG.SendMessage(ctx, chatID, msgResetInvalid)
}

// askPasswordResetConfirm sends the explicit «Ha, men» / «Yo'q» question. A
// linked account or a matching contact only proves who is in this chat; the
// tap proves they asked for the reset.
func (b *Bot) askPasswordResetConfirm(ctx context.Context, chatID int64, res auth.TelegramResetBegin) error {
	_, err := b.TG.SendText(ctx, chatID, fmt.Sprintf(msgResetConfirmFmt, res.MaskedPhone), &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: msgResetConfirmYes, CallbackData: cbResetYes + res.ConfirmNonce},
			{Text: msgResetConfirmNo, CallbackData: cbResetNo + res.ConfirmNonce},
		}},
	})
	return err
}

// handlePasswordResetCallback answers a «Ha, men» / «Yo'q» tap. The user id
// comes from Telegram's callback (not the message), so a forwarded question
// tapped by someone else is rejected by the auth layer.
func (b *Bot) handlePasswordResetCallback(ctx context.Context, cq CallbackQuery) error {
	var nonce string
	accept := false
	switch {
	case strings.HasPrefix(cq.Data, cbResetYes):
		nonce, accept = strings.TrimPrefix(cq.Data, cbResetYes), true
	case strings.HasPrefix(cq.Data, cbResetNo):
		nonce = strings.TrimPrefix(cq.Data, cbResetNo)
	}
	if b.Auth == nil || nonce == "" || cq.Message == nil || IsGroupChat(cq.Message.Chat.Type) {
		return b.TG.AnswerCallbackQuery(ctx, cq.ID, msgResetConfirmStale, false)
	}
	res, err := b.Auth.AnswerTelegramPasswordResetConfirm(ctx, cq.From.ID, nonce, accept)
	if err != nil {
		b.logger().Error("bot: password reset confirm failed", zap.Error(err), zap.Int64("tg_user_id", cq.From.ID))
		return errors.Join(err, b.TG.AnswerCallbackQuery(ctx, cq.ID, msgLinkInternal, false))
	}
	var text string
	switch res.Outcome {
	case auth.TelegramResetVerified:
		text = msgResetVerified
	case auth.TelegramResetCancelled:
		text = msgResetCancelled
	default:
		return b.TG.AnswerCallbackQuery(ctx, cq.ID, msgResetConfirmStale, false)
	}
	// The state change is committed, so a failed ack must not fail the update:
	// Telegram would redeliver it and the replay only answers "stale".
	if err := b.TG.AnswerCallbackQuery(ctx, cq.ID, "", false); err != nil {
		b.logger().Warn("bot: password reset callback ack failed", zap.Error(err))
	}
	chatID := cq.Message.Chat.ID
	// The one-time share-contact reply keyboard (contact path) outlives the
	// inline question; an inline edit cannot carry a ReplyKeyboardRemove, so
	// verification closes it with a follow-up message.
	var remove any
	if res.Outcome == auth.TelegramResetVerified {
		remove = ReplyKeyboardRemove{RemoveKeyboard: true}
	}
	// A failed edit only leaves stale buttons, which the auth layer already
	// treats as no-ops.
	if err := b.TG.EditMessageText(ctx, chatID, cq.Message.MessageID, text, nil); err != nil {
		b.logger().Warn("bot: password reset confirm edit failed", zap.Error(err))
		_, sendErr := b.TG.SendChatText(ctx, chatID, text, remove)
		return sendErr
	}
	if remove != nil {
		if _, err := b.TG.SendChatText(ctx, chatID, msgResetBackToSite, remove); err != nil {
			b.logger().Warn("bot: reply keyboard cleanup failed", zap.Error(err))
		}
	}
	return nil
}

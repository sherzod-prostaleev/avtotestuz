package bot

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

// «Telegram orqali kirish»: the bot half of auth's Telegram login request.
// /start login_<token> asks the learner to approve; the approval itself
// (and every rule about who may approve what) lives in auth.

// Login confirm buttons: "tgl:y:<nonce>" / "tgl:n:<nonce>". The nonce is per
// question (auth.TelegramLoginBegin.ConfirmNonce), never the login token.
const (
	cbLoginPrefix = "tgl:"
	cbLoginYes    = cbLoginPrefix + "y:"
	cbLoginNo     = cbLoginPrefix + "n:"
)

type loginTexts struct {
	header, unknownDevice                string
	askedFmt                             string // device
	confirmHint, accountFmt              string
	contactHint, shareButton             string
	yes, no                              string
	done, cancelled, invalid, blocked    string
	notOwn, foreign, rateLimited, failed string
	stale, groupOnly                     string
}

var loginCopy = map[lang]loginTexts{
	langUz: {
		header:        "🔐 Driver Go'ga kirish",
		unknownDevice: "Noma'lum qurilma",
		askedFmt:      "%s orqali kirish so'raldi.",
		confirmHint:   "Siz bo'lsangiz «✅ Kirish» ni bosing. Siz so'ramagan bo'lsangiz — e'tibor bermang.",
		accountFmt:    "Hisob: %s",
		contactHint: "Siz bo'lsangiz, pastdagi «📱 Raqamni yuborish» tugmasini bosing — raqamingiz kirishni tasdiqlaydi. " +
			"Siz so'ramagan bo'lsangiz — hech narsa yubormang.",
		shareButton: "📱 Raqamni yuborish",
		yes:         "✅ Kirish",
		no:          "✖️ Bekor qilish",
		done:        "✅ Kirildi. Brauzerga qayting.",
		cancelled:   "Kirish bekor qilindi. Hech kim hisobingizga kirmadi.",
		invalid:     "Kirish havolasi eskirgan yoki ishlatilgan. Saytda «Telegram orqali kirish» ni qayta bosing.",
		blocked:     "Bu hisob bloklangan. Savollar bo'lsa, qo'llab-quvvatlash xizmatiga yozing.",
		notOwn:      "Faqat o'zingizning raqamingizni yuboring: pastdagi «📱 Raqamni yuborish» tugmasini bosing.",
		foreign:     "Kechirasiz, faqat O'zbekiston raqamlari (+998) bilan kirish mumkin.",
		rateLimited: "Juda ko'p urinish. Birozdan keyin qayta urinib ko'ring.",
		failed:      "Bu raqam bilan kirib bo'lmadi. Qo'llab-quvvatlash xizmatiga yozing.",
		stale:       "Bu so'rov endi amal qilmaydi.",
		groupOnly:   "Kirish faqat bot bilan shaxsiy chatda tasdiqlanadi.",
	},
	langRu: {
		header:        "🔐 Вход в Driver Go",
		unknownDevice: "Неизвестное устройство",
		askedFmt:      "Запрошен вход: %s.",
		confirmHint:   "Если это вы — нажмите «✅ Войти». Если вы не запрашивали вход — просто проигнорируйте.",
		accountFmt:    "Аккаунт: %s",
		contactHint: "Если это вы — нажмите кнопку «📱 Отправить номер» ниже: номер подтвердит вход. " +
			"Если вы не запрашивали вход — ничего не отправляйте.",
		shareButton: "📱 Отправить номер",
		yes:         "✅ Войти",
		no:          "✖️ Отмена",
		done:        "✅ Вход выполнен. Вернитесь в браузер.",
		cancelled:   "Вход отменён. Никто не вошёл в ваш аккаунт.",
		invalid:     "Ссылка для входа устарела или уже использована. Нажмите «Войти через Telegram» на сайте ещё раз.",
		blocked:     "Этот аккаунт заблокирован. Если есть вопросы, напишите в поддержку.",
		notOwn:      "Отправьте свой номер: нажмите кнопку «📱 Отправить номер» ниже.",
		foreign:     "К сожалению, войти можно только с номером Узбекистана (+998).",
		rateLimited: "Слишком много попыток. Попробуйте немного позже.",
		failed:      "Не удалось войти с этим номером. Напишите в поддержку.",
		stale:       "Этот запрос больше не действует.",
		groupOnly:   "Вход подтверждается только в личном чате с ботом.",
	},
}

func loginTextsFor(code string) loginTexts { return loginCopy[langOf(code)] }

func (t loginTexts) asked(device string) string {
	if strings.TrimSpace(device) == "" {
		device = t.unknownDevice
	}
	return t.header + "\n" + strings.Replace(t.askedFmt, "%s", device, 1)
}

func (t loginTexts) shareKeyboard() ReplyKeyboardMarkup {
	return ReplyKeyboardMarkup{
		Keyboard:        [][]KeyboardButton{{{Text: t.shareButton, RequestContact: true}}},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

func telegramLoginUser(u *User) auth.TelegramLoginUser {
	return auth.TelegramLoginUser{ID: u.ID, Username: u.Username, FirstName: u.FirstName, LanguageCode: u.LanguageCode}
}

// handleTelegramLoginStart answers /start login_<token>. The private-chat
// check is the caller's: a group never gets the question.
func (b *Bot) handleTelegramLoginStart(ctx context.Context, chatID int64, from *User, rawToken string) error {
	t := loginTextsFor(from.LanguageCode)
	if b.Auth == nil {
		return b.replyErr(b.TG.SendMessage(ctx, chatID, t.invalid))
	}
	res, err := b.Auth.BeginTelegramLogin(ctx, rawToken, telegramLoginUser(from))
	if err != nil {
		b.logger().Error("bot: telegram login begin failed", zap.Error(err), zap.Int64("tg_user_id", from.ID))
		return errors.Join(err, b.replyErr(b.TG.SendMessage(ctx, chatID, msgLinkInternal)))
	}
	switch res.Outcome {
	case auth.TelegramLoginNeedContact:
		_, err := b.TG.SendChatText(ctx, chatID, t.asked(res.Device)+"\n\n"+t.contactHint, t.shareKeyboard())
		return b.replyErr(err)
	case auth.TelegramLoginNeedConfirm:
		text := t.asked(res.Device) + "\n" + strings.Replace(t.accountFmt, "%s", res.MaskedPhone, 1) + "\n\n" + t.confirmHint
		_, err := b.TG.SendText(ctx, chatID, text, &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{{
				{Text: t.yes, CallbackData: cbLoginYes + res.ConfirmNonce},
				{Text: t.no, CallbackData: cbLoginNo + res.ConfirmNonce},
			}},
		})
		return b.replyErr(err)
	}
	return b.replyErr(b.TG.SendMessage(ctx, chatID, loginOutcomeText(t, res.Outcome)))
}

func loginOutcomeText(t loginTexts, outcome string) string {
	switch outcome {
	case auth.TelegramLoginApproved:
		return t.done
	case auth.TelegramLoginCancelled:
		return t.cancelled
	case auth.TelegramLoginBlocked:
		return t.blocked
	case auth.TelegramLoginNotOwnContact:
		return t.notOwn
	case auth.TelegramLoginForeignPhone:
		return t.foreign
	case auth.TelegramLoginRateLimited:
		return t.rateLimited
	case auth.TelegramLoginFailed:
		return t.failed
	case auth.TelegramLoginStale:
		return t.stale
	default:
		return t.invalid
	}
}

// handleTelegramLoginContact offers a shared contact to a waiting login
// request. handled=false means no login was waiting for this user: the
// contact belongs to someone else (the password reset, or the Mini App's
// own phone share) and the caller passes it on.
func (b *Bot) handleTelegramLoginContact(ctx context.Context, chatID int64, from *User, contact *Contact) (handled bool, err error) {
	if b.Auth == nil || contact == nil {
		return false, nil
	}
	t := loginTextsFor(from.LanguageCode)
	res, err := b.Auth.ConfirmTelegramLoginContact(ctx, telegramLoginUser(from), contact.UserID, contact.PhoneNumber)
	if err != nil {
		b.logger().Error("bot: telegram login contact failed", zap.Error(err), zap.Int64("tg_user_id", from.ID))
		return true, errors.Join(err, b.replyErr(b.TG.SendMessage(ctx, chatID, msgLinkInternal)))
	}
	switch res.Outcome {
	case auth.TelegramLoginNone:
		return false, nil
	case auth.TelegramLoginNotOwnContact, auth.TelegramLoginForeignPhone:
		// Still waiting: keep the share button up.
		_, err := b.TG.SendChatText(ctx, chatID, loginOutcomeText(t, res.Outcome), t.shareKeyboard())
		return true, b.replyErr(err)
	}
	_, err = b.TG.SendChatText(ctx, chatID, loginOutcomeText(t, res.Outcome), ReplyKeyboardRemove{RemoveKeyboard: true})
	return true, b.replyErr(err)
}

// handleTelegramLoginCallback answers «✅ Kirish» / «✖️ Bekor qilish». The
// user id is Telegram's (callback From), so a forwarded question tapped by
// someone else is stale in auth. As with the reset callback, only an
// internal failure is returned; Telegram API failures are logged.
func (b *Bot) handleTelegramLoginCallback(ctx context.Context, cq CallbackQuery) error {
	t := loginTextsFor(cq.From.LanguageCode)
	var nonce string
	accept := false
	switch {
	case strings.HasPrefix(cq.Data, cbLoginYes):
		nonce, accept = strings.TrimPrefix(cq.Data, cbLoginYes), true
	case strings.HasPrefix(cq.Data, cbLoginNo):
		nonce = strings.TrimPrefix(cq.Data, cbLoginNo)
	}
	if b.Auth == nil || nonce == "" || cq.Message == nil || IsGroupChat(cq.Message.Chat.Type) {
		b.ackResetCallback(ctx, cq.ID, t.stale)
		return nil
	}
	res, err := b.Auth.AnswerTelegramLoginConfirm(ctx, cq.From.ID, nonce, accept)
	if err != nil {
		b.logger().Error("bot: telegram login confirm failed", zap.Error(err), zap.Int64("tg_user_id", cq.From.ID))
		b.ackResetCallback(ctx, cq.ID, msgLinkInternal)
		return err
	}
	if res.Outcome == auth.TelegramLoginStale {
		b.ackResetCallback(ctx, cq.ID, t.stale)
		return nil
	}
	b.ackResetCallback(ctx, cq.ID, "")
	text := loginOutcomeText(t, res.Outcome)
	if err := b.TG.EditMessageText(ctx, cq.Message.Chat.ID, cq.Message.MessageID, text, nil); err != nil {
		b.logger().Warn("bot: telegram login edit failed", zap.Error(err))
		if err := b.TG.SendMessage(ctx, cq.Message.Chat.ID, text); err != nil {
			b.logger().Warn("bot: telegram login send failed", zap.Error(err))
		}
	}
	return nil
}

// rememberReferral keeps a /start ref_<CODE> for this Telegram user until
// their first profile is created (auth applies it, new profiles only). A
// failure is logged: the welcome still goes out, only the bonus is lost.
func (b *Bot) rememberReferral(ctx context.Context, tgUserID int64, code string) {
	if b.BotUsers == nil {
		return
	}
	if err := b.BotUsers.SetTelegramBotUserPendingReferral(ctx, sqlc.SetTelegramBotUserPendingReferralParams{
		TgUserID:            tgUserID,
		PendingReferralCode: pgtype.Text{String: code, Valid: true},
	}); err != nil {
		b.logger().Warn("bot: pending referral not stored", zap.Error(err))
	}
}

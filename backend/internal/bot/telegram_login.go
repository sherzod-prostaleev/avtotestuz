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
	header, unknownDevice string
	askedFmt              string // %s = device; the phone-share prompt
	// askedAccountFmt is the «✅ Kirish» question: {device} asks to sign in to
	// {phone} (masked). Naming the account is what lets a learner with two
	// numbers, or one who opened somebody else's link, see what they approve.
	askedAccountFmt                      string
	confirmHint                          string
	contactHint, shareButton             string
	yes, no                              string
	done, cancelled, invalid, blocked    string
	notOwn, foreign, rateLimited, failed string
	stale, groupOnly                     string
	taken, disabled, answered            string
}

var loginCopy = map[lang]loginTexts{
	langUz: {
		header:          "🔐 Driver Go'ga kirish",
		unknownDevice:   "Noma'lum qurilma",
		askedFmt:        "%s orqali kirish so'raldi.",
		askedAccountFmt: "{device} orqali {phone} hisobiga kirish so'raldi.",
		confirmHint:     "Siz bo'lsangiz «✅ Kirish» ni bosing. Siz so'ramagan bo'lsangiz — «✖️ Bekor qilish» ni bosing.",
		contactHint: "Siz bo'lsangiz, pastdagi «📱 Raqamni yuborish» tugmasini bosing, so'ng «✅ Kirish» ni bosing. " +
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
		taken:       "Bu havola boshqa foydalanuvchi tomonidan ochilgan. O'zingiz kirmoqchi bo'lsangiz, saytda «Telegram orqali kirish» ni bosing.",
		disabled:    "Telegram orqali kirish vaqtincha o'chirilgan. Saytda telefon raqam va parol bilan kiring.",
		answered:    "Javobingiz qabul qilindi.",
	},
	langRu: {
		header:          "🔐 Вход в Driver Go",
		unknownDevice:   "Неизвестное устройство",
		askedFmt:        "Запрошен вход: %s.",
		askedAccountFmt: "Запрошен вход в аккаунт {phone}: {device}.",
		confirmHint:     "Если это вы — нажмите «✅ Войти». Если вы не запрашивали вход — нажмите «✖️ Отмена».",
		contactHint: "Если это вы — нажмите кнопку «📱 Отправить номер» ниже, а затем «✅ Войти». " +
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
		taken:       "Эта ссылка уже открыта другим пользователем. Чтобы войти самому, нажмите «Войти через Telegram» на сайте.",
		disabled:    "Вход через Telegram временно отключён. Войдите на сайте по номеру телефона и паролю.",
		answered:    "Ответ принят.",
	},
}

func loginTextsFor(code string) loginTexts { return loginCopy[langOf(code)] }

func (t loginTexts) device(device string) string {
	if strings.TrimSpace(device) == "" {
		return t.unknownDevice
	}
	return device
}

func (t loginTexts) asked(device string) string {
	return t.header + "\n" + strings.Replace(t.askedFmt, "%s", t.device(device), 1)
}

// question is the «✅ Kirish» message, the same for a linked account and
// after a phone share: which device asks, which account it would open.
func (t loginTexts) question(device, maskedPhone string) string {
	line := strings.NewReplacer("{device}", t.device(device), "{phone}", maskedPhone).Replace(t.askedAccountFmt)
	return t.header + "\n" + line + "\n\n" + t.confirmHint
}

func (t loginTexts) shareKeyboard() ReplyKeyboardMarkup {
	return ReplyKeyboardMarkup{
		Keyboard:        [][]KeyboardButton{{{Text: t.shareButton, RequestContact: true}}},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

func telegramLoginUser(u *User) auth.TelegramLoginUser {
	return auth.TelegramLoginUser{
		ID: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName, LanguageCode: u.LanguageCode,
	}
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
		return b.askTelegramLoginConfirm(ctx, chatID, t, res)
	}
	return b.replyErr(b.TG.SendMessage(ctx, chatID, loginOutcomeText(t, res.Outcome)))
}

// askTelegramLoginConfirm sends the «✅ Kirish» / «✖️ Bekor qilish» question.
// Only a tap on it approves a login — never a phone share by itself.
func (b *Bot) askTelegramLoginConfirm(ctx context.Context, chatID int64, t loginTexts, res auth.TelegramLoginBegin) error {
	_, err := b.TG.SendText(ctx, chatID, t.question(res.Device, res.MaskedPhone), &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: t.yes, CallbackData: cbLoginYes + res.ConfirmNonce},
			{Text: t.no, CallbackData: cbLoginNo + res.ConfirmNonce},
		}},
	})
	return b.replyErr(err)
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
	case auth.TelegramLoginTaken:
		return t.taken
	case auth.TelegramLoginDisabled:
		return t.disabled
	default:
		return t.invalid
	}
}

// handleTelegramLoginContact offers a shared contact to a waiting login
// request. handled=false means no login was waiting for a contact from this
// user: it belongs to someone else (the password reset, or the Mini App's
// own phone share) and the caller passes it on. The user's own number earns
// the «✅ Kirish» question and nothing more.
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
	case auth.TelegramLoginNeedConfirm:
		return true, b.askTelegramLoginConfirm(ctx, chatID, t, res)
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
	// Private chat only: a question forwarded or posted anywhere else is dead.
	if b.Auth == nil || nonce == "" || cq.Message == nil || !IsPrivateChat(cq.Message.Chat.Type) {
		b.ackResetCallback(ctx, cq.ID, t.stale)
		return nil
	}
	res, err := b.Auth.AnswerTelegramLoginConfirm(ctx, telegramLoginUser(&cq.From), nonce, accept)
	if err != nil {
		b.logger().Error("bot: telegram login confirm failed", zap.Error(err), zap.Int64("tg_user_id", cq.From.ID))
		b.ackResetCallback(ctx, cq.ID, msgLinkInternal)
		return err
	}
	switch res.Outcome {
	case auth.TelegramLoginStale, auth.TelegramLoginRateLimited, auth.TelegramLoginDisabled:
		// Nothing changed; the answer is a toast and the question stays as it is.
		b.ackResetCallback(ctx, cq.ID, loginOutcomeText(t, res.Outcome))
		return nil
	}
	b.ackResetCallback(ctx, cq.ID, "")
	chatID, text := cq.Message.Chat.ID, loginOutcomeText(t, res.Outcome)
	if !res.ViaContact {
		if err := b.TG.EditMessageText(ctx, chatID, cq.Message.MessageID, text, nil); err != nil {
			b.logger().Warn("bot: telegram login edit failed", zap.Error(err))
			if err := b.TG.SendMessage(ctx, chatID, text); err != nil {
				b.logger().Warn("bot: telegram login send failed", zap.Error(err))
			}
		}
		return nil
	}
	// The phone-share step left its one-time reply keyboard up, and an inline
	// edit cannot carry a ReplyKeyboardRemove: the question is closed in place
	// and the outcome goes out as a message that takes the keyboard down.
	if err := b.TG.EditMessageText(ctx, chatID, cq.Message.MessageID, t.answered, nil); err != nil {
		b.logger().Warn("bot: telegram login edit failed", zap.Error(err))
	}
	if _, err := b.TG.SendChatText(ctx, chatID, text, ReplyKeyboardRemove{RemoveKeyboard: true}); err != nil {
		b.logger().Warn("bot: telegram login send failed", zap.Error(err))
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

package bot

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

// First-password notice. An account made through Telegram has no password;
// POST /me/password/set gives it one from any signed-in session — including a
// phished one. The owner is told here, in the chat that is the account's
// identity, so a password they did not set does not go unnoticed (audit I4).
// The other sessions are ended by the handler; this only informs.
var passwordSetNotice = map[lang]string{
	langUz: "🔐 Hisobingizga parol o'rnatildi. Boshqa qurilmalardagi seanslar yopildi.\n\n" +
		"Bu siz bo'lmasangiz — /start orqali yordamga yozing.",
	langRu: "🔐 Для вашего аккаунта установлен пароль. Сеансы на других устройствах завершены.\n\n" +
		"Если это были не вы — напишите в поддержку через /start.",
}

// PasswordNotifier implements account.PasswordNotifier with the bot.
type PasswordNotifier struct {
	Q   *sqlc.Queries
	TG  *Client
	Log *zap.Logger
}

// FirstPasswordSet messages the Telegram account linked to profileID, in the
// language that user last used with the bot (uz when unknown). Best-effort:
// no link, a blocked bot or a Telegram hiccup is logged and nothing more —
// the password is already set and the caller must not fail over a notice.
func (n *PasswordNotifier) FirstPasswordSet(ctx context.Context, profileID uuid.UUID) {
	if n == nil || n.Q == nil || n.TG == nil {
		return
	}
	log := n.Log
	if log == nil {
		log = zap.NewNop()
	}
	account, err := n.Q.GetTelegramAccountByProfileID(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Warn("bot: password notice lookup failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		}
		return
	}
	code := ""
	if u, err := n.Q.GetTelegramBotUser(ctx, account.TgUserID); err == nil {
		code = u.LanguageCode
	}
	if err := n.TG.SendMessage(ctx, account.TgUserID, passwordSetNotice[langOf(code)]); err != nil {
		log.Warn("bot: password notice not delivered", zap.String("profile_id", profileID.String()), zap.Error(err))
	}
}

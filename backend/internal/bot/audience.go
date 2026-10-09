package bot

import (
	"context"

	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

// The daily reminder's audience registry (telegram_bot_user) and the two
// ways a user controls it: the bundle's opt-out button and /eslatma.

// trackPrivateUser records the sender of a private-chat message or button
// tap. Groups are not the audience. A failure is logged and the update goes
// on: missing one last_seen bump is not worth failing the user's reply.
func (b *Bot) trackPrivateUser(ctx context.Context, u Update) {
	if b.BotUsers == nil {
		return
	}
	var from *User
	switch {
	case u.Message != nil && u.Message.From != nil && u.Message.Chat.Type == "private":
		from = u.Message.From
	case u.CallbackQuery != nil && u.CallbackQuery.Message != nil && u.CallbackQuery.Message.Chat.Type == "private":
		from = &u.CallbackQuery.From
	default:
		return
	}
	if err := b.BotUsers.UpsertTelegramBotUser(ctx, sqlc.UpsertTelegramBotUserParams{
		TgUserID:     from.ID,
		FirstName:    truncateRunes(from.FirstName, 64),
		Username:     truncateRunes(from.Username, 64),
		LanguageCode: truncateRunes(from.LanguageCode, 16),
	}); err != nil {
		b.logger().Warn("bot: audience upsert failed", zap.Error(err))
	}
}

// trackPrivateMembership follows the user blocking (kicked) and unblocking
// (member) the bot. Unlike trackPrivateUser this is the whole point of the
// update, so a DB failure is returned for a redelivery.
func (b *Bot) trackPrivateMembership(ctx context.Context, upd *ChatMemberUpd) error {
	if b.BotUsers == nil || upd.Chat.Type != "private" {
		return nil
	}
	switch upd.NewChatMember.Status {
	case "kicked", "left":
		return b.BotUsers.MarkTelegramBotUserBlocked(ctx, upd.Chat.ID)
	case "member":
		return b.BotUsers.UpsertTelegramBotUser(ctx, sqlc.UpsertTelegramBotUserParams{
			TgUserID:     upd.Chat.ID,
			FirstName:    truncateRunes(upd.From.FirstName, 64),
			Username:     truncateRunes(upd.From.Username, 64),
			LanguageCode: truncateRunes(upd.From.LanguageCode, 16),
		})
	}
	return nil
}

// handleEslatma toggles the daily reminder (private chats only).
func (b *Bot) handleEslatma(ctx context.Context, chatID int64, from *User, chatType string) error {
	l := langOf(from.LanguageCode)
	if IsGroupChat(chatType) {
		return b.replyErr(b.TG.SendMessage(ctx, chatID, remindersGroupText(l)))
	}
	if b.BotUsers == nil {
		return b.replyErr(b.TG.SendMessage(ctx, chatID, msgLinkInternal))
	}
	on, err := b.BotUsers.ToggleTelegramReminders(ctx, from.ID)
	if err != nil {
		b.logger().Error("bot: reminder toggle failed", zap.Error(err))
		return err
	}
	text := remindersOffText(l)
	if on {
		text = remindersOnText(l)
	}
	return b.replyErr(b.TG.SendMessage(ctx, chatID, text))
}

// handleReminderOff is the bundle's «🔕» button. The message is edited into
// the confirmation (caption for a photo bundle), keeping only the practice
// button. Telegram failures are logged, not returned: the opt-out is
// already committed and a redelivery could only repeat it.
func (b *Bot) handleReminderOff(ctx context.Context, cq CallbackQuery) error {
	l := langOf(cq.From.LanguageCode)
	if b.BotUsers == nil {
		b.ackReminderCallback(ctx, cq.ID, "")
		return nil
	}
	if err := b.BotUsers.DisableTelegramReminders(ctx, cq.From.ID); err != nil {
		b.logger().Error("bot: reminder opt-out failed", zap.Error(err))
		b.ackReminderCallback(ctx, cq.ID, msgLinkInternal)
		return err
	}
	text := remindersOffText(l)
	b.ackReminderCallback(ctx, cq.ID, "🔕")
	if cq.Message == nil {
		return nil
	}
	chatID, msgID := cq.Message.Chat.ID, cq.Message.MessageID
	keep := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{practiceButton(b.WebAppURL, b.PublicBaseURL, l)},
	}}
	var err error
	if cq.Message.Text == "" {
		err = b.TG.EditMessageCaption(ctx, chatID, msgID, text, keep)
	} else {
		err = b.TG.EditMessageText(ctx, chatID, msgID, text, keep)
	}
	if err != nil {
		b.logger().Warn("bot: reminder opt-out edit failed", zap.Error(err))
		if err := b.TG.SendMessage(ctx, chatID, text); err != nil {
			b.logger().Warn("bot: reminder opt-out reply failed", zap.Error(err))
		}
	}
	return nil
}

func (b *Bot) ackReminderCallback(ctx context.Context, id, text string) {
	if err := b.TG.AnswerCallbackQuery(ctx, id, text, false); err != nil {
		b.logger().Warn("bot: reminder callback ack failed", zap.Error(err))
	}
}

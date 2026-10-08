package bot

import (
	"context"
	"errors"
)

const (
	scopeAllPrivateChats = "all_private_chats"
	scopeAllGroupChats   = "all_group_chats"
)

// commandSet is one setMyCommands call: the menu for a scope in a language
// ("" is Telegram's fallback for every language without its own list).
type commandSet struct {
	Scope        string
	LanguageCode string
	Commands     []BotCommand
}

// commandSets is the "/" menu Telegram shows. Every command listed here must
// be handled by HandleUpdate for that chat type (TestAdvertisedCommandsAreHandled);
// /link, /unlink and the group /start stay unlisted on purpose: they are
// deep-link or rare actions, and /help names them.
func commandSets() []commandSet {
	return []commandSet{
		{Scope: scopeAllPrivateChats, Commands: []BotCommand{
			{Command: "start", Description: "Bosh menyu"},
			{Command: "quiz", Description: "Tezkor test"},
			{Command: "status", Description: "Hisob holati"},
			{Command: "help", Description: "Yordam"},
		}},
		{Scope: scopeAllPrivateChats, LanguageCode: "ru", Commands: []BotCommand{
			{Command: "start", Description: "Главное меню"},
			{Command: "quiz", Description: "Быстрый тест"},
			{Command: "status", Description: "Статус аккаунта"},
			{Command: "help", Description: "Помощь"},
		}},
		{Scope: scopeAllGroupChats, Commands: []BotCommand{
			{Command: "quiz", Description: "Guruh quizini boshlash"},
			{Command: "next", Description: "Keyingi savol"},
			{Command: "stop", Description: "Quizni to'xtatish"},
		}},
		{Scope: scopeAllGroupChats, LanguageCode: "ru", Commands: []BotCommand{
			{Command: "quiz", Description: "Начать квиз в группе"},
			{Command: "next", Description: "Следующий вопрос"},
			{Command: "stop", Description: "Остановить квиз"},
		}},
	}
}

// SyncCommands publishes the command menus on every start, alongside
// SyncMenuButton. Like the menu button they live on Telegram's side, so they
// are set whenever a bot token exists regardless of TELEGRAM_BOT_MODE. Each
// set is attempted even if an earlier one fails, so one rejected list does
// not leave the other menus stale.
func SyncCommands(ctx context.Context, c *Client) error {
	var errs []error
	for _, set := range commandSets() {
		if err := c.SetMyCommands(ctx, set.Commands, set.Scope, set.LanguageCode); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

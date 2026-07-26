package bot

// Update is a Telegram Bot API update. Only the fields this bot reads are
// modeled — Telegram's payload has many more, all ignored on decode.
type Update struct {
	UpdateID      int64           `json:"update_id"`
	Message       *Message        `json:"message,omitempty"`
	CallbackQuery *CallbackQuery  `json:"callback_query,omitempty"`
	MyChatMember  *ChatMemberUpd  `json:"my_chat_member,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"` // private | group | supergroup | channel
	Title string `json:"title"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data"`
}

type ChatMemberUpd struct {
	Chat          Chat       `json:"chat"`
	From          User       `json:"from"`
	NewChatMember ChatMember `json:"new_chat_member"`
	OldChatMember ChatMember `json:"old_chat_member"`
}

type ChatMember struct {
	Status string `json:"status"` // member | administrator | left | kicked | ...
	User   User   `json:"user"`
}

// Inline keyboard types mirror Telegram Bot API for answer buttons + CTA.

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

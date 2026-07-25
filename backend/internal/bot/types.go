package bot

// Update is a Telegram Bot API update. Only the fields this bot reads are
// modeled — Telegram's payload has many more, all ignored on decode.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID int64 `json:"id"`
}

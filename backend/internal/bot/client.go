package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Client is a minimal Telegram Bot API client — only the handful of methods
// this package needs, deliberately not a general-purpose SDK. Same shape as
// auth.TelegramSender: a plain REST wrapper, no external bot library
// dependency.
type Client struct {
	BaseURL string // e.g. https://api.telegram.org
	Token   string
	HC      *http.Client
}

func NewClient(baseURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{BaseURL: baseURL, Token: token, HC: hc}
}

// redactToken strips the bot token from error strings. Telegram's API puts
// the token in the URL path (/bot<TOKEN>/method), and net/http's *url.Error
// embeds that full URL — so an unredacted transport failure would write the
// live bot credential into application logs (and anything that scrapes them).
func (c *Client) redactToken(err error) error {
	if err == nil || c.Token == "" {
		return err
	}
	msg := err.Error()
	if !strings.Contains(msg, c.Token) {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(msg, c.Token, "<redacted>"))
}

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/bot%s/%s", c.BaseURL, c.Token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return c.redactToken(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HC.Do(req)
	if err != nil {
		return c.redactToken(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: decode response: %w", method, err)
	}
	if !envelope.OK {
		return fmt.Errorf("telegram %s: %s", method, envelope.Description)
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
	}
	return nil
}

// SendMessage delivers plain text to a chat (a direct-message chat ID is
// the same as the user's tg_user_id).
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := c.SendText(ctx, chatID, text, nil)
	return err
}

// SendText sends a text message and returns Telegram's message_id.
func (c *Client) SendText(ctx context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) (int64, error) {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	var msg Message
	if err := c.call(ctx, "sendMessage", payload, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// SendPhoto posts a publicly reachable image URL with an optional caption
// and inline keyboard. Caption is capped by Telegram at 1024 characters —
// callers should truncate before invoking.
func (c *Client) SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, markup *InlineKeyboardMarkup) (int64, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"photo":   photoURL,
	}
	if caption != "" {
		payload["caption"] = caption
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	var msg Message
	if err := c.call(ctx, "sendPhoto", payload, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// EditMessageReplyMarkup replaces or clears the inline keyboard on a message.
// Pass nil markup to remove buttons after an answer is graded.
func (c *Client) EditMessageReplyMarkup(ctx context.Context, chatID, messageID int64, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	if markup != nil {
		payload["reply_markup"] = markup
	} else {
		payload["reply_markup"] = InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{}}
	}
	return c.call(ctx, "editMessageReplyMarkup", payload, nil)
}

// AnswerCallbackQuery acknowledges a button tap (stops the client spinner).
func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID, text string, showAlert bool) error {
	payload := map[string]any{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = text
	}
	if showAlert {
		payload["show_alert"] = true
	}
	return c.call(ctx, "answerCallbackQuery", payload, nil)
}

// GetUpdates long-polls for new updates, starting at offset (the caller is
// expected to pass lastUpdateID+1 on the next call — Telegram's own
// "confirm receipt by asking past it" convention). Used only by the dev
// long-poll runner; webhook mode never calls this.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":  offset,
		"timeout": timeoutSec,
		"allowed_updates": []string{
			"message", "callback_query", "my_chat_member", "poll_answer",
		},
	}, &updates)
	return updates, err
}

// SetWebhook registers url as the bot's webhook target, with secretToken
// round-tripped on every call as X-Telegram-Bot-Api-Secret-Token — that
// header is how webhook.Handler verifies a request actually came from
// Telegram. Not called automatically anywhere in this codebase; it's a
// one-time operational step (design §7).
func (c *Client) SetWebhook(ctx context.Context, url, secretToken string) error {
	return c.call(ctx, "setWebhook", map[string]any{
		"url":          url,
		"secret_token": secretToken,
		"allowed_updates": []string{
			"message", "callback_query", "my_chat_member", "poll_answer",
		},
	}, nil)
}

// DeleteWebhook removes the webhook, e.g. before switching a deployment to
// long-poll mode (the two are mutually exclusive on Telegram's side).
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", map[string]any{}, nil)
}

// Telegram Bot API limits for polls. Exceeding any of them is a 400 from
// Telegram, so they are checked here where the caller gets a usable error.
const (
	pollQuestionMaxChars = 300
	pollOptionMaxChars   = 100
	pollExplanationMax   = 200
	pollMinOptions       = 2
	pollMaxOptions       = 10
	pollMinOpenPeriod    = 5
	pollMaxOpenPeriod    = 600
)

// SendPoll sends a quiz-type poll and returns the message id and the poll id.
// The poll id is what inbound poll_answer updates carry — it is the only
// handle back to the question that was asked.
func (c *Client) SendPoll(ctx context.Context, chatID int64, req PollRequest) (int64, string, error) {
	if n := utf8.RuneCountInString(req.Question); n < 1 || n > pollQuestionMaxChars {
		return 0, "", fmt.Errorf("poll question must be 1..%d chars, got %d", pollQuestionMaxChars, n)
	}
	if len(req.Options) < pollMinOptions || len(req.Options) > pollMaxOptions {
		return 0, "", fmt.Errorf("poll needs %d..%d options, got %d", pollMinOptions, pollMaxOptions, len(req.Options))
	}
	for i, opt := range req.Options {
		if n := utf8.RuneCountInString(opt); n < 1 || n > pollOptionMaxChars {
			return 0, "", fmt.Errorf("poll option %d must be 1..%d chars, got %d", i, pollOptionMaxChars, n)
		}
	}
	if req.CorrectIdx < 0 || req.CorrectIdx >= len(req.Options) {
		return 0, "", fmt.Errorf("correct index %d out of range", req.CorrectIdx)
	}
	if req.OpenPeriod < pollMinOpenPeriod || req.OpenPeriod > pollMaxOpenPeriod {
		return 0, "", fmt.Errorf("open_period must be %d..%d, got %d", pollMinOpenPeriod, pollMaxOpenPeriod, req.OpenPeriod)
	}

	payload := map[string]any{
		"chat_id":           chatID,
		"question":          req.Question,
		"options":           req.Options,
		"type":              "quiz",
		"correct_option_id": req.CorrectIdx,
		"is_anonymous":      false,
		"open_period":       req.OpenPeriod,
	}
	if req.Explanation != "" {
		payload["explanation"] = truncateRunes(req.Explanation, pollExplanationMax)
	}
	if req.ReplyTo != 0 {
		payload["reply_to_message_id"] = req.ReplyTo
	}

	var msg struct {
		MessageID int64 `json:"message_id"`
		Poll      struct {
			ID string `json:"id"`
		} `json:"poll"`
	}
	if err := c.call(ctx, "sendPoll", payload, &msg); err != nil {
		return 0, "", err
	}
	return msg.MessageID, msg.Poll.ID, nil
}

// SendTextWithEffect sends text with an optional full-screen message effect.
// Telegram applies message_effect_id in private chats only; callers must not
// pass one for a group.
func (c *Client) SendTextWithEffect(ctx context.Context, chatID int64, text, effectID string, markup *InlineKeyboardMarkup) (int64, error) {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if effectID != "" {
		payload["message_effect_id"] = effectID
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	var msg Message
	if err := c.call(ctx, "sendMessage", payload, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// SetMessageReaction puts a single emoji reaction on a message — the one
// celebration primitive that works in groups.
func (c *Client) SetMessageReaction(ctx context.Context, chatID, messageID int64, emoji string) error {
	return c.call(ctx, "setMessageReaction", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction":   []map[string]string{{"type": "emoji", "emoji": emoji}},
	}, nil)
}

// SendSticker posts a sticker by file_id. Callers skip it when no file_id is
// configured — an unverified file_id is a runtime error, not a decoration.
func (c *Client) SendSticker(ctx context.Context, chatID int64, fileID string) error {
	if strings.TrimSpace(fileID) == "" {
		return nil
	}
	return c.call(ctx, "sendSticker", map[string]any{
		"chat_id": chatID,
		"sticker": fileID,
	}, nil)
}

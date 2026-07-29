package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type capturedPoll struct {
	mu      sync.Mutex
	path    string
	payload map[string]any
}

func newPollCapture(t *testing.T) (*capturedPoll, *Client) {
	t.Helper()
	c := &capturedPoll{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.mu.Lock()
		c.path = r.URL.Path
		c.payload = body
		c.mu.Unlock()
		_, _ = fmt.Fprint(w,
			`{"ok":true,"result":{"message_id":42,"poll":{"id":"poll-xyz"}}}`)
	}))
	t.Cleanup(srv.Close)
	return c, NewClient(srv.URL, "T", srv.Client())
}

func TestSendPollBuildsQuizPayload(t *testing.T) {
	cap, client := newPollCapture(t)
	msgID, pollID, err := client.SendPoll(context.Background(), -900, PollRequest{
		Question:    "Bu belgi nimani bildiradi?",
		Options:     []string{"Birinchi", "Ikkinchi", "Uchinchi"},
		CorrectIdx:  1,
		Explanation: "Izoh",
		OpenPeriod:  10,
		ReplyTo:     41,
	})
	if err != nil {
		t.Fatalf("SendPoll: %v", err)
	}
	if msgID != 42 || pollID != "poll-xyz" {
		t.Fatalf("msgID=%d pollID=%q", msgID, pollID)
	}
	if !strings.HasSuffix(cap.path, "/sendPoll") {
		t.Fatalf("path = %q", cap.path)
	}
	if cap.payload["type"] != "quiz" {
		t.Fatalf("type = %v, want quiz", cap.payload["type"])
	}
	// Without this the bot never learns who answered — poll_answer only
	// carries a user for non-anonymous polls.
	if cap.payload["is_anonymous"] != false {
		t.Fatalf("is_anonymous = %v, want false", cap.payload["is_anonymous"])
	}
	if cap.payload["correct_option_id"] != float64(1) {
		t.Fatalf("correct_option_id = %v", cap.payload["correct_option_id"])
	}
	if cap.payload["open_period"] != float64(10) {
		t.Fatalf("open_period = %v", cap.payload["open_period"])
	}
	if cap.payload["reply_to_message_id"] != float64(41) {
		t.Fatalf("reply_to_message_id = %v", cap.payload["reply_to_message_id"])
	}
	opts, _ := cap.payload["options"].([]any)
	if len(opts) != 3 {
		t.Fatalf("options = %v", cap.payload["options"])
	}
}

// Telegram rejects a poll whose question exceeds 300 chars or whose option
// exceeds 100. The client must not send one.
func TestSendPollRejectsOversizeFields(t *testing.T) {
	_, client := newPollCapture(t)
	ctx := context.Background()

	_, _, err := client.SendPoll(ctx, -900, PollRequest{
		Question: strings.Repeat("a", 301),
		Options:  []string{"bir", "ikki"}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a 301-char question")
	}

	_, _, err = client.SendPoll(ctx, -900, PollRequest{
		Question: "ok",
		Options:  []string{"bir", strings.Repeat("b", 101)}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a 101-char option")
	}

	_, _, err = client.SendPoll(ctx, -900, PollRequest{
		Question: "ok", Options: []string{"yolg'iz"}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a single option")
	}
}

func TestPollAnswerDecodesFromUpdate(t *testing.T) {
	raw := []byte(`{"update_id":7,"poll_answer":{"poll_id":"p1",
		"user":{"id":88,"first_name":"Aziz"},"option_ids":[2]}}`)
	var u Update
	if err := json.Unmarshal(raw, &u); err != nil {
		t.Fatal(err)
	}
	if u.PollAnswer == nil {
		t.Fatal("poll_answer not decoded — Update is missing the field")
	}
	if u.PollAnswer.PollID != "p1" || u.PollAnswer.User.ID != 88 {
		t.Fatalf("poll answer = %+v", u.PollAnswer)
	}
	if len(u.PollAnswer.OptionIDs) != 1 || u.PollAnswer.OptionIDs[0] != 2 {
		t.Fatalf("option_ids = %v", u.PollAnswer.OptionIDs)
	}
}

// poll_answer never arrives unless it is in allowed_updates.
func TestAllowedUpdatesIncludePollAnswer(t *testing.T) {
	cap, client := newPollCapture(t)
	if err := client.SetWebhook(context.Background(), "https://x.test/hook", "s"); err != nil {
		t.Fatal(err)
	}
	got, _ := cap.payload["allowed_updates"].([]any)
	found := false
	for _, v := range got {
		if v == "poll_answer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("allowed_updates = %v, want poll_answer", got)
	}
}

// Both SetWebhook and GetUpdates must have identical allowed_updates lists
// containing poll_answer, or poll votes silently vanish in one mode.
func TestAllowedUpdatesConsistentBothModes(t *testing.T) {
	c := &capturedPoll{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.mu.Lock()
		c.path = r.URL.Path
		c.payload = body
		c.mu.Unlock()

		// Return appropriate response per method; GetUpdates expects []Update
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			_, _ = fmt.Fprint(w, `{"ok":true,"result":[]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":42}}`)
		}
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, "T", srv.Client())

	// SetWebhook
	if err := client.SetWebhook(context.Background(), "https://x.test/hook", "s"); err != nil {
		t.Fatal(err)
	}
	webhookUpdates, _ := c.payload["allowed_updates"].([]any)

	// GetUpdates
	if _, err := client.GetUpdates(context.Background(), 0, 10); err != nil {
		t.Fatal(err)
	}
	pollUpdates, _ := c.payload["allowed_updates"].([]any)

	// Check poll_answer is in SetWebhook
	found := false
	for _, v := range webhookUpdates {
		if v == "poll_answer" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("SetWebhook: allowed_updates = %v, missing poll_answer", webhookUpdates)
	}

	// Check poll_answer is in GetUpdates
	found = false
	for _, v := range pollUpdates {
		if v == "poll_answer" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("GetUpdates: allowed_updates = %v, missing poll_answer", pollUpdates)
	}

	// Check lists are equal
	if len(webhookUpdates) != len(pollUpdates) {
		t.Fatalf("allowed_updates length mismatch: SetWebhook=%v, GetUpdates=%v", webhookUpdates, pollUpdates)
	}
	for _, wh := range webhookUpdates {
		found := false
		for _, poll := range pollUpdates {
			if wh == poll {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("SetWebhook and GetUpdates mismatch: SetWebhook=%v, GetUpdates=%v", webhookUpdates, pollUpdates)
		}
	}
}

func TestSetMessageReaction(t *testing.T) {
	cap, client := newPollCapture(t)
	if err := client.SetMessageReaction(context.Background(), -900, 42, "🎉"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cap.path, "/setMessageReaction") {
		t.Fatalf("path = %q", cap.path)
	}
	reactions, _ := cap.payload["reaction"].([]any)
	if len(reactions) != 1 {
		t.Fatalf("reaction = %v", cap.payload["reaction"])
	}
}

func TestSendTextWithEffectCarriesEffectID(t *testing.T) {
	cap, client := newPollCapture(t)
	if _, err := client.SendTextWithEffect(
		context.Background(), 900, "Tabriklaymiz", "5046509860389126442", nil,
	); err != nil {
		t.Fatal(err)
	}
	if cap.payload["message_effect_id"] != "5046509860389126442" {
		t.Fatalf("message_effect_id = %v", cap.payload["message_effect_id"])
	}
}

// An empty effect id must not put the key in the payload at all — Telegram
// rejects an empty string.
func TestSendTextWithEffectOmitsEmptyID(t *testing.T) {
	cap, client := newPollCapture(t)
	if _, err := client.SendTextWithEffect(
		context.Background(), 900, "Salom", "", nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, present := cap.payload["message_effect_id"]; present {
		t.Fatal("empty effect id must be omitted")
	}
}

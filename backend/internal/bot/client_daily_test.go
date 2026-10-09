package bot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Flood control tells the caller how long to back off; losing that number
// would turn a 429 into guesswork.
func TestClientAPIErrorCarriesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`))
	}))
	t.Cleanup(srv.Close)
	err := NewClient(srv.URL, "T", srv.Client()).SendMessage(context.Background(), 1, "x")
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if api.Code != 429 || api.RetryAfter != 7 {
		t.Fatalf("api = %+v, want 429 with retry_after 7", api)
	}
}

// The daily poll stays open all evening: no open_period at all.
func TestSendPollWithoutOpenPeriodStaysOpen(t *testing.T) {
	cap, client := newPollCapture(t)
	if _, _, err := client.SendPoll(context.Background(), 5, PollRequest{
		Question: "Savol?", Options: []string{"bir", "ikki"}, CorrectIdx: 0,
	}); err != nil {
		t.Fatalf("SendPoll: %v", err)
	}
	if _, ok := cap.payload["open_period"]; ok {
		t.Fatalf("open_period = %v, want omitted", cap.payload["open_period"])
	}
	if _, _, err := client.SendPoll(context.Background(), 5, PollRequest{
		Question: "Savol?", Options: []string{"bir", "ikki"}, CorrectIdx: 0, OpenPeriod: 3,
	}); err == nil {
		t.Fatal("a positive open_period under 5 must still be rejected")
	}
}

func TestClientEditMessageCaption(t *testing.T) {
	cap, client := newPollCapture(t)
	if err := client.EditMessageCaption(context.Background(), 9, 77, "yangi", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cap.path, "/editMessageCaption") {
		t.Fatalf("path = %q", cap.path)
	}
	if cap.payload["caption"] != "yangi" || cap.payload["message_id"] != float64(77) {
		t.Fatalf("payload = %v", cap.payload)
	}
}

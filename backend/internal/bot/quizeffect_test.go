package bot

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// Telegram applies message_effect_id in private chats only. Sending one to a
// group is an API error, so the group path must never carry it.
func TestGroupFinalMessageCarriesNoEffect(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8300, 51, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 901, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8300)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	for _, p := range rec.methodCalls("sendMessage") {
		if _, present := p["message_effect_id"]; present {
			t.Fatal("a group message must not carry message_effect_id")
		}
	}
	// The guaranteed group celebration is a reaction.
	if len(rec.methodCalls("setMessageReaction")) == 0 {
		t.Fatal("group finish did not react to the result message")
	}
}

func TestPrivateFinalMessageCarriesEffect(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, 9500, 52, "private"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 902, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, 9500)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	var effectSeen bool
	for _, p := range rec.methodCalls("sendMessage") {
		if p["message_effect_id"] == effectCelebrate {
			effectSeen = true
		}
	}
	if !effectSeen {
		t.Fatal("private finish did not carry the celebration effect")
	}
}

// An unset sticker id must not produce a sendSticker call at all.
func TestNoStickerWhenNotConfigured(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8301, 53, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 903, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8301)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}
	if len(rec.methodCalls("sendSticker")) != 0 {
		t.Fatal("sent a sticker with no file_id configured")
	}
}

// Nobody played means nothing to celebrate.
func TestNoCelebrationWhenNobodyPlayed(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8302, 54, "supergroup"); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8302)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}
	if len(rec.methodCalls("setMessageReaction")) != 0 {
		t.Fatal("celebrated an empty game")
	}
}

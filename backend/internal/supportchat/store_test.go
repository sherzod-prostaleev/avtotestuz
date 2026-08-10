package supportchat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func TestSingleThreadReopenAndUnread(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := redisx.NewTest(t)
	svc := NewService(pool, rdb, nil, "http://localhost:3000")

	var profileID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO profile (phone, name, password_hash)
		VALUES ('+998909990001', 'Test', 'h')
		RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}

	msg1, conv1, err := svc.PostUserMessage(context.Background(), profileID, "Hello admin", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if conv1.Status != "open" || conv1.UnreadAdmin != 1 || msg1.Body != "Hello admin" {
		t.Fatalf("after user msg: %+v", conv1)
	}

	adminID := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO admin_user (id, email, display_name, password_hash, status)
		VALUES ($1, 'chat@example.uz', 'Chat', 'x', 'active')`, adminID); err != nil {
		t.Fatal(err)
	}
	_, conv2, err := svc.PostAdminMessage(context.Background(), adminID, conv1.ID, "Here is help", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if conv2.UnreadUser != 1 {
		t.Fatalf("want unread_user=1, got %+v", conv2)
	}

	closed, err := svc.Store.SetStatus(context.Background(), conv1.ID, "closed")
	if err != nil || closed.Status != "closed" {
		t.Fatalf("close: %+v %v", closed, err)
	}

	_, conv3, err := svc.PostUserMessage(context.Background(), profileID, "Still need help", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if conv3.Status != "open" {
		t.Fatalf("user message should reopen closed thread, got %s", conv3.Status)
	}
	if conv3.ID != conv1.ID {
		t.Fatalf("must stay single thread: %s vs %s", conv3.ID, conv1.ID)
	}

	learner, err := svc.Store.GetLearnerSummary(context.Background(), profileID)
	if err != nil {
		t.Fatal(err)
	}
	if !learner.HasPassword || learner.Phone == "" {
		t.Fatalf("learner summary incomplete: %+v", learner)
	}
}

func TestAttachmentKeyBoundToConversation(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := redisx.NewTest(t)
	svc := NewService(pool, rdb, nil, "http://localhost:3000")

	var profileID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO profile (phone, name, password_hash)
		VALUES ('+998909990002', 'Attach', 'h')
		RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	conv, err := svc.Store.GetOrCreateConversation(context.Background(), profileID)
	if err != nil {
		t.Fatal(err)
	}
	foreign := "support/" + uuid.NewString() + "/deadbeef.png"
	if _, _, err := svc.PostUserMessage(context.Background(), profileID, "", foreign, "x.png", "image/png", 12); !errors.Is(err, ErrBadAttachment) {
		t.Fatalf("foreign key: got %v want ErrBadAttachment", err)
	}
	okKey := "support/" + conv.ID.String() + "/ok.png"
	if _, _, err := svc.PostUserMessage(context.Background(), profileID, "with file", okKey, "ok.png", "image/png", 12); err != nil {
		t.Fatalf("owned key rejected: %v", err)
	}
}

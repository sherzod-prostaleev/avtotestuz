package supportchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
)

const TicketTTL = 30 * time.Second

var (
	ErrTicketInvalid   = errors.New("ticket_invalid")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not_found")
	ErrBadAttachment   = errors.New("invalid attachment_key")
)

// Service owns persistence, tickets, uploads, and hub fanout.
type Service struct {
	Store     Store
	Pool      *pgxpool.Pool
	R         *redis.Client
	Lim       auth.Limiter
	Hub       *Hub
	Blobs     blob.Store
	PublicURL string
	Now       func() time.Time
}

func NewService(pool *pgxpool.Pool, r *redis.Client, blobs blob.Store, publicURL string) *Service {
	return &Service{
		Store:     Store{Pool: pool},
		Pool:      pool,
		R:         r,
		Lim:       auth.Limiter{R: r},
		Hub:       NewHub(),
		Blobs:     blobs,
		PublicURL: publicURL,
		Now:       time.Now,
	}
}

func (s *Service) ticketKey(kind, tok string) string {
	return "supportchat:ticket:" + kind + ":" + tok
}

// MintUserTicket issues a single-use WS ticket for a learner.
func (s *Service) MintUserTicket(ctx context.Context, profileID uuid.UUID) (string, int, error) {
	ok, err := s.Lim.Allow(ctx, "supportchat:rl:ticket:user:"+profileID.String(), 30, time.Minute)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, fmt.Errorf("rate_limited")
	}
	tok, err := auth.NewRefreshToken()
	if err != nil {
		return "", 0, err
	}
	if err := s.R.Set(ctx, s.ticketKey("user", tok), profileID.String(), TicketTTL).Err(); err != nil {
		return "", 0, err
	}
	return tok, int(TicketTTL.Seconds()), nil
}

// MintAdminTicket issues a single-use WS ticket for an admin.
func (s *Service) MintAdminTicket(ctx context.Context, adminID uuid.UUID) (string, int, error) {
	ok, err := s.Lim.Allow(ctx, "supportchat:rl:ticket:admin:"+adminID.String(), 30, time.Minute)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, fmt.Errorf("rate_limited")
	}
	tok, err := auth.NewRefreshToken()
	if err != nil {
		return "", 0, err
	}
	if err := s.R.Set(ctx, s.ticketKey("admin", tok), adminID.String(), TicketTTL).Err(); err != nil {
		return "", 0, err
	}
	return tok, int(TicketTTL.Seconds()), nil
}

func (s *Service) RedeemTicket(ctx context.Context, kind, tok string) (uuid.UUID, error) {
	val, err := s.R.GetDel(ctx, s.ticketKey(kind, tok)).Result()
	if err == redis.Nil || val == "" {
		return uuid.Nil, ErrTicketInvalid
	}
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, ErrTicketInvalid
	}
	return id, nil
}

// PostUserMessage inserts as the learner and broadcasts.
func (s *Service) PostUserMessage(ctx context.Context, profileID uuid.UUID, body, attachKey, attachName, attachMime string, attachSize int64) (Message, Conversation, error) {
	conv, err := s.Store.GetOrCreateConversation(ctx, profileID)
	if err != nil {
		return Message{}, Conversation{}, err
	}
	if err := validateAttachmentKey(conv.ID, attachKey); err != nil {
		return Message{}, Conversation{}, err
	}
	pid := profileID
	msg, conv, err := s.Store.InsertMessage(ctx, InsertMessageInput{
		ConversationID:  conv.ID,
		SenderKind:      "user",
		SenderProfileID: &pid,
		Body:            body,
		AttachmentKey:   attachKey,
		AttachmentName:  attachName,
		AttachmentMime:  attachMime,
		AttachmentSize:  attachSize,
		ReopenWhenUser:  true,
	})
	if err != nil {
		return Message{}, Conversation{}, err
	}
	s.broadcastMessage(msg, conv)
	return msg, conv, nil
}

// PostAdminMessage inserts as admin (IDOR-safe: conversation must exist).
func (s *Service) PostAdminMessage(ctx context.Context, adminID, conversationID uuid.UUID, body, attachKey, attachName, attachMime string, attachSize int64) (Message, Conversation, error) {
	conv, err := s.Store.GetByID(ctx, conversationID)
	if err != nil {
		return Message{}, Conversation{}, ErrNotFound
	}
	if err := validateAttachmentKey(conv.ID, attachKey); err != nil {
		return Message{}, Conversation{}, err
	}
	aid := adminID
	msg, conv, err := s.Store.InsertMessage(ctx, InsertMessageInput{
		ConversationID: conv.ID,
		SenderKind:     "admin",
		SenderAdminID:  &aid,
		Body:           body,
		AttachmentKey:  attachKey,
		AttachmentName: attachName,
		AttachmentMime: attachMime,
		AttachmentSize: attachSize,
	})
	if err != nil {
		return Message{}, Conversation{}, err
	}
	s.broadcastMessage(msg, conv)
	return msg, conv, nil
}

// validateAttachmentKey rejects foreign/forged blob keys (IDOR).
// Empty key is allowed (text-only messages). Uploaded keys must live under
// support/{conversationID}/…
func validateAttachmentKey(conversationID uuid.UUID, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	prefix := "support/" + conversationID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		return ErrBadAttachment
	}
	rest := strings.TrimPrefix(key, prefix)
	if rest == "" || strings.Contains(rest, "..") || strings.Contains(rest, "/") {
		return ErrBadAttachment
	}
	return nil
}

func (s *Service) broadcastMessage(msg Message, conv Conversation) {
	// Strip storage key from fanout; clients resolve downloads via message id.
	safe := msg
	safe.AttachmentKey = ""
	safe.AttachmentURL = ""
	payload, _ := json.Marshal(map[string]any{
		"t": "message.new",
		"d": map[string]any{
			"message":      safe,
			"conversation": conv,
		},
	})
	s.Hub.BroadcastConversation(conv.ID, payload)
}

func (s *Service) broadcastConversation(conv Conversation) {
	payload, _ := json.Marshal(map[string]any{
		"t": "conversation.updated",
		"d": map[string]any{"conversation": conv},
	})
	s.Hub.BroadcastConversation(conv.ID, payload)
}

// Package supportchat implements one durable learner↔admin support thread
// with realtime fanout and optional file attachments.
package supportchat

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxBody       = 4000
	maxAttachName = 200
	defaultLimit  = 50
	maxLimit      = 100
)

// Conversation is one support_conversation row (unique per profile).
type Conversation struct {
	ID            uuid.UUID  `json:"id"`
	ProfileID     uuid.UUID  `json:"profile_id"`
	Status        string     `json:"status"`
	UnreadAdmin   int        `json:"unread_admin"`
	UnreadUser    int        `json:"unread_user"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// Admin list enrichment (optional).
	ProfileName  string `json:"profile_name,omitempty"`
	ProfilePhone string `json:"profile_phone,omitempty"`
	Preview      string `json:"preview,omitempty"`
}

// Message is one support_message row.
type Message struct {
	ID              uuid.UUID  `json:"id"`
	ConversationID  uuid.UUID  `json:"conversation_id"`
	SenderKind      string     `json:"sender_kind"`
	SenderProfileID *uuid.UUID `json:"sender_profile_id,omitempty"`
	SenderAdminID   *uuid.UUID `json:"sender_admin_id,omitempty"`
	Body            string     `json:"body"`
	AttachmentKey   string     `json:"attachment_key,omitempty"`
	AttachmentName  string     `json:"attachment_name,omitempty"`
	AttachmentMime  string     `json:"attachment_mime,omitempty"`
	AttachmentSize  int64      `json:"attachment_size,omitempty"`
	// AttachmentURL is an authenticated API path (not a public MinIO URL).
	AttachmentURL   string     `json:"attachment_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// LearnerSummary is embedded in admin conversation detail.
type LearnerSummary struct {
	ID          uuid.UUID  `json:"id"`
	Phone       string     `json:"phone"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	VIPActive   bool       `json:"vip_active"`
	VIPEndsAt   *time.Time `json:"vip_ends_at,omitempty"`
	HasPassword bool       `json:"has_password"`
	Streak      int        `json:"streak"`
	LocalePref  string     `json:"locale_pref"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Store persists support chat rows via pgx.
type Store struct {
	Pool *pgxpool.Pool
}

// attachmentURL returns a private download path for the given audience.
// Callers must serve these behind JWT/admin auth — never public /media links.
func attachmentURL(messageID uuid.UUID, admin bool) string {
	if messageID == uuid.Nil {
		return ""
	}
	if admin {
		return "/support/messages/" + messageID.String() + "/attachment"
	}
	return "/me/support/messages/" + messageID.String() + "/attachment"
}

func scanConversation(row pgx.Row) (Conversation, error) {
	var c Conversation
	var last *time.Time
	err := row.Scan(
		&c.ID, &c.ProfileID, &c.Status, &c.UnreadAdmin, &c.UnreadUser,
		&last, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Conversation{}, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	c.UpdatedAt = c.UpdatedAt.UTC()
	if last != nil {
		t := last.UTC()
		c.LastMessageAt = &t
	}
	return c, nil
}

func (s Store) scanMessage(row pgx.Row) (Message, error) {
	var m Message
	err := row.Scan(
		&m.ID, &m.ConversationID, &m.SenderKind,
		&m.SenderProfileID, &m.SenderAdminID,
		&m.Body, &m.AttachmentKey, &m.AttachmentName, &m.AttachmentMime, &m.AttachmentSize,
		&m.CreatedAt,
	)
	if err != nil {
		return Message{}, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	return m, nil
}

// withAttachmentURL sets the private download path for API responses and
// strips the storage key so clients cannot hit anonymous object storage.
func withAttachmentURL(m Message, admin bool) Message {
	if strings.TrimSpace(m.AttachmentKey) != "" {
		m.AttachmentURL = attachmentURL(m.ID, admin)
	}
	m.AttachmentKey = ""
	return m
}

func withAttachmentURLs(items []Message, admin bool) []Message {
	out := make([]Message, len(items))
	for i, m := range items {
		out[i] = withAttachmentURL(m, admin)
	}
	return out
}

// GetOrCreateConversation returns the single durable thread for a profile.
func (s Store) GetOrCreateConversation(ctx context.Context, profileID uuid.UUID) (Conversation, error) {
	c, err := s.GetByProfile(ctx, profileID)
	if err == nil {
		return c, nil
	}
	if err != pgx.ErrNoRows {
		return Conversation{}, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO support_conversation (profile_id)
		VALUES ($1)
		ON CONFLICT (profile_id) DO UPDATE SET profile_id = EXCLUDED.profile_id
		RETURNING id, profile_id, status, unread_admin, unread_user,
		          last_message_at, created_at, updated_at`, profileID)
	return scanConversation(row)
}

// GetByProfile loads conversation by learner id.
func (s Store) GetByProfile(ctx context.Context, profileID uuid.UUID) (Conversation, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, profile_id, status, unread_admin, unread_user,
		       last_message_at, created_at, updated_at
		FROM support_conversation WHERE profile_id = $1`, profileID)
	c, err := scanConversation(row)
	if err != nil {
		return Conversation{}, err
	}
	return c, nil
}

// GetByID loads conversation by id.
func (s Store) GetByID(ctx context.Context, id uuid.UUID) (Conversation, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, profile_id, status, unread_admin, unread_user,
		       last_message_at, created_at, updated_at
		FROM support_conversation WHERE id = $1`, id)
	return scanConversation(row)
}

// ListConversations is the admin inbox (newest activity first).
func (s Store) ListConversations(ctx context.Context, status, q string, page, limit int) ([]Conversation, int, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	status = strings.TrimSpace(status)
	q = strings.TrimSpace(q)
	offset := (page - 1) * limit

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM support_conversation c
		JOIN profile p ON p.id = c.profile_id
		WHERE ($1 = '' OR c.status = $1)
		  AND ($2 = '' OR
		       p.name ILIKE '%' || $2 || '%' OR
		       p.phone ILIKE '%' || $2 || '%' OR
		       c.id::text ILIKE $2 || '%' OR
		       p.id::text ILIKE $2 || '%')`, status, q).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.profile_id, c.status, c.unread_admin, c.unread_user,
		       c.last_message_at, c.created_at, c.updated_at,
		       COALESCE(p.name, ''), p.phone,
		       COALESCE((
		         SELECT CASE
		           WHEN m.attachment_key <> '' AND m.body = '' THEN '[file] ' || m.attachment_name
		           ELSE m.body
		         END
		         FROM support_message m
		         WHERE m.conversation_id = c.id
		         ORDER BY m.created_at DESC LIMIT 1
		       ), '')
		FROM support_conversation c
		JOIN profile p ON p.id = c.profile_id
		WHERE ($1 = '' OR c.status = $1)
		  AND ($2 = '' OR
		       p.name ILIKE '%' || $2 || '%' OR
		       p.phone ILIKE '%' || $2 || '%' OR
		       c.id::text ILIKE $2 || '%' OR
		       p.id::text ILIKE $2 || '%')
		ORDER BY c.last_message_at DESC NULLS LAST, c.created_at DESC
		LIMIT $3 OFFSET $4`, status, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	items := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		var last *time.Time
		if err := rows.Scan(
			&c.ID, &c.ProfileID, &c.Status, &c.UnreadAdmin, &c.UnreadUser,
			&last, &c.CreatedAt, &c.UpdatedAt,
			&c.ProfileName, &c.ProfilePhone, &c.Preview,
		); err != nil {
			return nil, 0, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		c.UpdatedAt = c.UpdatedAt.UTC()
		if last != nil {
			t := last.UTC()
			c.LastMessageAt = &t
		}
		if utf8.RuneCountInString(c.Preview) > 120 {
			c.Preview = string([]rune(c.Preview)[:120]) + "…"
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}

// ListMessages returns newest-first page; pass before to paginate older.
func (s Store) ListMessages(ctx context.Context, conversationID uuid.UUID, before *time.Time, limit int) ([]Message, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	var rows pgx.Rows
	var err error
	if before != nil {
		rows, err = s.Pool.Query(ctx, `
			SELECT id, conversation_id, sender_kind, sender_profile_id, sender_admin_id,
			       body, attachment_key, attachment_name, attachment_mime, attachment_size, created_at
			FROM support_message
			WHERE conversation_id = $1 AND created_at < $2
			ORDER BY created_at DESC
			LIMIT $3`, conversationID, *before, limit)
	} else {
		rows, err = s.Pool.Query(ctx, `
			SELECT id, conversation_id, sender_kind, sender_profile_id, sender_admin_id,
			       body, attachment_key, attachment_name, attachment_mime, attachment_size, created_at
			FROM support_message
			WHERE conversation_id = $1
			ORDER BY created_at DESC
			LIMIT $2`, conversationID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	items := make([]Message, 0)
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// InsertMessageInput is validated before insert.
type InsertMessageInput struct {
	ConversationID  uuid.UUID
	SenderKind      string // user|admin
	SenderProfileID *uuid.UUID
	SenderAdminID   *uuid.UUID
	Body            string
	AttachmentKey   string
	AttachmentName  string
	AttachmentMime  string
	AttachmentSize  int64
	// ReopenWhenUser: if sender is user and conversation closed → open.
	ReopenWhenUser bool
}

// InsertMessage writes a message, bumps unread for the other party, updates last_message_at.
func (s Store) InsertMessage(ctx context.Context, in InsertMessageInput) (Message, Conversation, error) {
	body := strings.TrimSpace(in.Body)
	key := strings.TrimSpace(in.AttachmentKey)
	name := strings.TrimSpace(in.AttachmentName)
	mime := strings.TrimSpace(in.AttachmentMime)
	if body == "" && key == "" {
		return Message{}, Conversation{}, fmt.Errorf("body or attachment required")
	}
	if utf8.RuneCountInString(body) > maxBody {
		return Message{}, Conversation{}, fmt.Errorf("body too long")
	}
	if utf8.RuneCountInString(name) > maxAttachName {
		return Message{}, Conversation{}, fmt.Errorf("attachment_name too long")
	}
	if in.SenderKind != "user" && in.SenderKind != "admin" {
		return Message{}, Conversation{}, fmt.Errorf("invalid sender_kind")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Message{}, Conversation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var m Message
	err = tx.QueryRow(ctx, `
		INSERT INTO support_message (
		  conversation_id, sender_kind, sender_profile_id, sender_admin_id,
		  body, attachment_key, attachment_name, attachment_mime, attachment_size
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, conversation_id, sender_kind, sender_profile_id, sender_admin_id,
		          body, attachment_key, attachment_name, attachment_mime, attachment_size, created_at`,
		in.ConversationID, in.SenderKind, in.SenderProfileID, in.SenderAdminID,
		body, key, name, mime, in.AttachmentSize,
	).Scan(
		&m.ID, &m.ConversationID, &m.SenderKind, &m.SenderProfileID, &m.SenderAdminID,
		&m.Body, &m.AttachmentKey, &m.AttachmentName, &m.AttachmentMime, &m.AttachmentSize, &m.CreatedAt,
	)
	if err != nil {
		return Message{}, Conversation{}, fmt.Errorf("insert message: %w", err)
	}
	m.CreatedAt = m.CreatedAt.UTC()

	// User message → admin unread++; admin message → user unread++.
	// Closed thread reopens when the learner writes again (Telegram-style).
	var c Conversation
	var last *time.Time
	err = tx.QueryRow(ctx, `
		UPDATE support_conversation
		SET last_message_at = $2,
		    updated_at = now(),
		    status = CASE
		      WHEN $3::text = 'user' AND status = 'closed' THEN 'open'
		      ELSE status
		    END,
		    unread_admin = CASE WHEN $3::text = 'user' THEN unread_admin + 1 ELSE unread_admin END,
		    unread_user  = CASE WHEN $3::text = 'admin' THEN unread_user + 1 ELSE unread_user END
		WHERE id = $1
		RETURNING id, profile_id, status, unread_admin, unread_user,
		          last_message_at, created_at, updated_at`,
		in.ConversationID, m.CreatedAt, in.SenderKind,
	).Scan(
		&c.ID, &c.ProfileID, &c.Status, &c.UnreadAdmin, &c.UnreadUser,
		&last, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Message{}, Conversation{}, fmt.Errorf("touch conversation: %w", err)
	}
	c.CreatedAt = c.CreatedAt.UTC()
	c.UpdatedAt = c.UpdatedAt.UTC()
	if last != nil {
		t := last.UTC()
		c.LastMessageAt = &t
	}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, Conversation{}, err
	}
	return m, c, nil
}

// SetStatus patches open|closed.
func (s Store) SetStatus(ctx context.Context, id uuid.UUID, status string) (Conversation, error) {
	status = strings.TrimSpace(status)
	if status != "open" && status != "closed" {
		return Conversation{}, fmt.Errorf("invalid status")
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE support_conversation
		SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, profile_id, status, unread_admin, unread_user,
		          last_message_at, created_at, updated_at`, id, status)
	return scanConversation(row)
}

// ClearUnreadAdmin zeros admin unread (admin opened the thread).
func (s Store) ClearUnreadAdmin(ctx context.Context, id uuid.UUID) (Conversation, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE support_conversation
		SET unread_admin = 0, updated_at = now()
		WHERE id = $1
		RETURNING id, profile_id, status, unread_admin, unread_user,
		          last_message_at, created_at, updated_at`, id)
	return scanConversation(row)
}

// ClearUnreadUser zeros learner unread.
func (s Store) ClearUnreadUser(ctx context.Context, id uuid.UUID) (Conversation, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE support_conversation
		SET unread_user = 0, updated_at = now()
		WHERE id = $1
		RETURNING id, profile_id, status, unread_admin, unread_user,
		          last_message_at, created_at, updated_at`, id)
	return scanConversation(row)
}

// TotalUnreadUser returns unread_user for the learner's conversation (0 if none).
func (s Store) TotalUnreadUser(ctx context.Context, profileID uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(unread_user, 0) FROM support_conversation WHERE profile_id = $1`, profileID,
	).Scan(&n)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return n, err
}

// GetMessage loads one message by id.
func (s Store) GetMessage(ctx context.Context, id uuid.UUID) (Message, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, conversation_id, sender_kind, sender_profile_id, sender_admin_id,
		       body, attachment_key, attachment_name, attachment_mime, attachment_size, created_at
		FROM support_message WHERE id = $1`, id)
	return s.scanMessage(row)
}

// GetLearnerSummary loads admin-facing profile fields (no secrets).
func (s Store) GetLearnerSummary(ctx context.Context, profileID uuid.UUID) (LearnerSummary, error) {
	var d LearnerSummary
	var vipEnds *time.Time
	var status string
	err := s.Pool.QueryRow(ctx, `
		SELECT p.id, p.phone, COALESCE(p.name, ''), p.status,
		       EXISTS (SELECT 1 FROM entitlement e WHERE e.profile_id = p.id AND e.ends_at > now()),
		       (SELECT e.ends_at FROM entitlement e WHERE e.profile_id = p.id AND e.ends_at > now()
		        ORDER BY e.ends_at DESC LIMIT 1),
		       (p.password_hash IS NOT NULL AND length(p.password_hash) > 0),
		       COALESCE(st.current, 0),
		       p.locale_pref::text,
		       p.created_at
		FROM profile p
		LEFT JOIN streak st ON st.profile_id = p.id
		WHERE p.id = $1`, profileID).Scan(
		&d.ID, &d.Phone, &d.Name, &status,
		&d.VIPActive, &vipEnds, &d.HasPassword,
		&d.Streak, &d.LocalePref, &d.CreatedAt,
	)
	if err != nil {
		return LearnerSummary{}, err
	}
	d.Status = status
	if d.Status == "banned" {
		d.Status = "blocked"
	}
	d.CreatedAt = d.CreatedAt.UTC()
	if vipEnds != nil {
		t := vipEnds.UTC()
		d.VIPEndsAt = &t
	}
	return d, nil
}

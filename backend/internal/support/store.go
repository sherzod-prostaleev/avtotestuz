// Package support owns the honest support-inbox stub (tickets, not Zendesk).
package support

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/i18n"
)

const (
	maxSubject = 120
	maxBody    = 2000
	maxContact = 120
	maxNote    = 2000
)

// Ticket is one support_ticket row.
type Ticket struct {
	ID           uuid.UUID  `json:"id"`
	ProfileID    *uuid.UUID `json:"profile_id,omitempty"`
	ContactEmail string     `json:"contact_email"`
	ContactPhone string     `json:"contact_phone"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Status       string     `json:"status"`
	Locale       string     `json:"locale"`
	Source       string     `json:"source"`
	AdminNote    string     `json:"admin_note"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CreateInput is the learner/public create payload.
type CreateInput struct {
	ProfileID    *uuid.UUID
	ContactEmail string
	ContactPhone string
	Subject      string
	Body         string
	Locale       string
	Source       string
}

// Store persists support tickets.
type Store struct {
	Pool *pgxpool.Pool
}

// Create validates and inserts a ticket.
func (s Store) Create(ctx context.Context, in CreateInput) (Ticket, error) {
	subject := strings.TrimSpace(in.Subject)
	body := strings.TrimSpace(in.Body)
	email := strings.TrimSpace(in.ContactEmail)
	phone := strings.TrimSpace(in.ContactPhone)
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "public"
	}
	if source != "profile" && source != "public" {
		return Ticket{}, fmt.Errorf("invalid source")
	}
	if subject == "" {
		return Ticket{}, fmt.Errorf("subject required")
	}
	if body == "" {
		return Ticket{}, fmt.Errorf("body required")
	}
	if utf8.RuneCountInString(subject) > maxSubject {
		return Ticket{}, fmt.Errorf("subject too long")
	}
	if utf8.RuneCountInString(body) > maxBody {
		return Ticket{}, fmt.Errorf("body too long")
	}
	if utf8.RuneCountInString(email) > maxContact {
		return Ticket{}, fmt.Errorf("contact_email too long")
	}
	if utf8.RuneCountInString(phone) > maxContact {
		return Ticket{}, fmt.Errorf("contact_phone too long")
	}
	if in.ProfileID == nil && email == "" && phone == "" {
		return Ticket{}, fmt.Errorf("contact required")
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" || !i18n.Supported[locale] {
		locale = i18n.Default
	}

	var t Ticket
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO support_ticket
		  (profile_id, contact_email, contact_phone, subject, body, locale, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, profile_id, contact_email, contact_phone, subject, body,
		          status, locale, source, admin_note, created_at, updated_at`,
		in.ProfileID, email, phone, subject, body, locale, source,
	).Scan(
		&t.ID, &t.ProfileID, &t.ContactEmail, &t.ContactPhone, &t.Subject, &t.Body,
		&t.Status, &t.Locale, &t.Source, &t.AdminNote, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Ticket{}, fmt.Errorf("insert support_ticket: %w", err)
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

// ListResult is a paginated ticket list.
type ListResult struct {
	Items []Ticket `json:"items"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
	Total int      `json:"total"`
}

// List returns newest-first tickets with optional status/q filters.
func (s Store) List(ctx context.Context, status, q string, page, limit int) (ListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	status = strings.TrimSpace(status)
	q = strings.TrimSpace(q)
	offset := (page - 1) * limit

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM support_ticket
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR
		       subject ILIKE '%' || $2 || '%' OR
		       body ILIKE '%' || $2 || '%' OR
		       contact_email ILIKE '%' || $2 || '%' OR
		       contact_phone ILIKE '%' || $2 || '%' OR
		       id::text ILIKE '%' || $2 || '%')`,
		status, q,
	).Scan(&total)
	if err != nil {
		return ListResult{}, fmt.Errorf("count support_ticket: %w", err)
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT id, profile_id, contact_email, contact_phone, subject, body,
		       status, locale, source, admin_note, created_at, updated_at
		FROM support_ticket
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR
		       subject ILIKE '%' || $2 || '%' OR
		       body ILIKE '%' || $2 || '%' OR
		       contact_email ILIKE '%' || $2 || '%' OR
		       contact_phone ILIKE '%' || $2 || '%' OR
		       id::text ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`,
		status, q, limit, offset,
	)
	if err != nil {
		return ListResult{}, fmt.Errorf("list support_ticket: %w", err)
	}
	defer rows.Close()

	items := make([]Ticket, 0)
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(
			&t.ID, &t.ProfileID, &t.ContactEmail, &t.ContactPhone, &t.Subject, &t.Body,
			&t.Status, &t.Locale, &t.Source, &t.AdminNote, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return ListResult{}, err
		}
		t.CreatedAt = t.CreatedAt.UTC()
		t.UpdatedAt = t.UpdatedAt.UTC()
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, Page: page, Limit: limit, Total: total}, nil
}

// Get returns one ticket by id.
func (s Store) Get(ctx context.Context, id uuid.UUID) (Ticket, error) {
	var t Ticket
	err := s.Pool.QueryRow(ctx, `
		SELECT id, profile_id, contact_email, contact_phone, subject, body,
		       status, locale, source, admin_note, created_at, updated_at
		FROM support_ticket WHERE id = $1`, id,
	).Scan(
		&t.ID, &t.ProfileID, &t.ContactEmail, &t.ContactPhone, &t.Subject, &t.Body,
		&t.Status, &t.Locale, &t.Source, &t.AdminNote, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Ticket{}, err
		}
		return Ticket{}, fmt.Errorf("get support_ticket: %w", err)
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

// UpdateStatus patches status and optional admin_note.
func (s Store) UpdateStatus(ctx context.Context, id uuid.UUID, status string, adminNote *string) (Ticket, error) {
	status = strings.TrimSpace(status)
	switch status {
	case "open", "in_progress", "resolved", "closed":
	default:
		return Ticket{}, fmt.Errorf("invalid status")
	}
	if adminNote != nil {
		n := strings.TrimSpace(*adminNote)
		if utf8.RuneCountInString(n) > maxNote {
			return Ticket{}, fmt.Errorf("admin_note too long")
		}
		*adminNote = n
	}

	var t Ticket
	var err error
	if adminNote != nil {
		err = s.Pool.QueryRow(ctx, `
			UPDATE support_ticket
			SET status = $2, admin_note = $3, updated_at = now()
			WHERE id = $1
			RETURNING id, profile_id, contact_email, contact_phone, subject, body,
			          status, locale, source, admin_note, created_at, updated_at`,
			id, status, *adminNote,
		).Scan(
			&t.ID, &t.ProfileID, &t.ContactEmail, &t.ContactPhone, &t.Subject, &t.Body,
			&t.Status, &t.Locale, &t.Source, &t.AdminNote, &t.CreatedAt, &t.UpdatedAt,
		)
	} else {
		err = s.Pool.QueryRow(ctx, `
			UPDATE support_ticket
			SET status = $2, updated_at = now()
			WHERE id = $1
			RETURNING id, profile_id, contact_email, contact_phone, subject, body,
			          status, locale, source, admin_note, created_at, updated_at`,
			id, status,
		).Scan(
			&t.ID, &t.ProfileID, &t.ContactEmail, &t.ContactPhone, &t.Subject, &t.Body,
			&t.Status, &t.Locale, &t.Source, &t.AdminNote, &t.CreatedAt, &t.UpdatedAt,
		)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return Ticket{}, err
		}
		return Ticket{}, fmt.Errorf("update support_ticket: %w", err)
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

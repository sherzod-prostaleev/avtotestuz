package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// QuestionDirectoryRow is a list row for GET /admin/v1/content/questions.
type QuestionDirectoryRow struct {
	ID                 uuid.UUID `json:"id"`
	SourceExtID        string    `json:"source_ext_id"`
	CategoryCode       string    `json:"category_code"`
	CategoryName       string    `json:"category_name"`
	ValidationStatus   string    `json:"validation_status"`
	TextPreview        string    `json:"text_preview"`
	VariantNumbers     []int     `json:"variant_numbers"`
	ExplanationStatus  string    `json:"explanation_status"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ListQuestionsResult is a paginated questions page.
type ListQuestionsResult struct {
	Items []QuestionDirectoryRow `json:"items"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
	Total int                    `json:"total"`
}

// TranslationSummary is locale + status (+ optional text) for admin detail.
type TranslationSummary struct {
	Locale string `json:"locale"`
	Text   string `json:"text,omitempty"`
	Status string `json:"status"`
}

// AnswerSummary is one answer option with translations.
type AnswerSummary struct {
	ID           uuid.UUID             `json:"id"`
	Position     int                   `json:"position"`
	IsCorrect    bool                  `json:"is_correct"`
	Translations []TranslationSummary  `json:"translations"`
}

// VariantRef links a question into a numbered bilet.
type VariantRef struct {
	Number   int `json:"number"`
	Position int `json:"position"`
}

// ExplanationSummary is the explanation row for admin (uz-Latn primary + all locales).
type ExplanationSummary struct {
	ID            uuid.UUID            `json:"id"`
	Locales       []TranslationSummary `json:"locales"`
	BlocksPreview string               `json:"blocks_preview,omitempty"`
	VerifiedAt    *time.Time           `json:"verified_at,omitempty"`
}

// QuestionDetail is GET /admin/v1/content/questions/{id}.
type QuestionDetail struct {
	ID               uuid.UUID            `json:"id"`
	SourceExtID      string               `json:"source_ext_id"`
	CategoryID       uuid.UUID            `json:"category_id"`
	CategoryCode     string               `json:"category_code"`
	CategoryName     string               `json:"category_name"`
	ValidationStatus string               `json:"validation_status"`
	Source           string               `json:"source"`
	ContentHash      string               `json:"content_hash"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	Translations     []TranslationSummary `json:"translations"`
	Answers          []AnswerSummary      `json:"answers"`
	Variants         []VariantRef         `json:"variants"`
	Explanation      *ExplanationSummary  `json:"explanation,omitempty"`
}

// ExplanationQueueRow is GET /admin/v1/content/explanations.
type ExplanationQueueRow struct {
	QuestionID       uuid.UUID  `json:"question_id"`
	SourceExtID      string     `json:"source_ext_id"`
	TextPreview      string     `json:"text_preview"`
	Locale           string     `json:"locale"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	CategoryCode     string     `json:"category_code"`
	ExplanationID    uuid.UUID  `json:"explanation_id"`
}

// ListExplanationsResult is a paginated explanations queue.
type ListExplanationsResult struct {
	Items []ExplanationQueueRow `json:"items"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Total int                   `json:"total"`
}

// ListQuestions returns questions matching search/filters, newest updated first.
func (s Store) ListQuestions(ctx context.Context, q, category, validation, explanation string, page, limit int) (ListQuestionsResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	category = strings.TrimSpace(category)
	validation = strings.TrimSpace(validation)
	explanation = strings.TrimSpace(explanation)
	offset := (page - 1) * limit

	where := `
		WHERE ($1 = '' OR
		  q.source_ext_id ILIKE '%' || $1 || '%' OR
		  q.id::text ILIKE $1 || '%' OR
		  COALESCE(qt.text, '') ILIKE '%' || $1 || '%')
		AND ($2 = '' OR c.code = $2)
		AND ($3 = '' OR q.validation_status = $3)
		AND (
		  $4 = '' OR
		  ($4 = 'none' AND et.status IS NULL) OR
		  ($4 <> 'none' AND et.status = $4)
		)`

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM question q
		JOIN category c ON c.id = q.category_id
		LEFT JOIN question_translation qt
		  ON qt.question_id = q.id AND qt.locale = 'uz-Latn'
		LEFT JOIN explanation e ON e.question_id = q.id
		LEFT JOIN explanation_translation et
		  ON et.explanation_id = e.id AND et.locale = 'uz-Latn'
		`+where, q, category, validation, explanation).Scan(&total)
	if err != nil {
		return ListQuestionsResult{}, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT q.id, q.source_ext_id, c.code,
		       COALESCE(ct.name, c.code) AS category_name,
		       q.validation_status,
		       COALESCE(qt.text, '') AS text_preview,
		       COALESCE(et.status, 'none') AS explanation_status,
		       q.updated_at,
		       COALESCE((
		         SELECT array_agg(v.number ORDER BY v.number)
		         FROM variant_question vq
		         JOIN variant v ON v.id = vq.variant_id
		         WHERE vq.question_id = q.id
		       ), '{}') AS variant_numbers
		FROM question q
		JOIN category c ON c.id = q.category_id
		LEFT JOIN category_translation ct
		  ON ct.category_id = c.id AND ct.locale = 'uz-Latn'
		LEFT JOIN question_translation qt
		  ON qt.question_id = q.id AND qt.locale = 'uz-Latn'
		LEFT JOIN explanation e ON e.question_id = q.id
		LEFT JOIN explanation_translation et
		  ON et.explanation_id = e.id AND et.locale = 'uz-Latn'
		`+where+`
		ORDER BY q.updated_at DESC
		LIMIT $5 OFFSET $6`, q, category, validation, explanation, limit, offset)
	if err != nil {
		return ListQuestionsResult{}, err
	}
	defer rows.Close()

	items := make([]QuestionDirectoryRow, 0)
	for rows.Next() {
		var row QuestionDirectoryRow
		var variants []int32
		if err := rows.Scan(
			&row.ID, &row.SourceExtID, &row.CategoryCode, &row.CategoryName,
			&row.ValidationStatus, &row.TextPreview, &row.ExplanationStatus, &row.UpdatedAt,
			&variants,
		); err != nil {
			return ListQuestionsResult{}, err
		}
		row.UpdatedAt = row.UpdatedAt.UTC()
		row.VariantNumbers = make([]int, 0, len(variants))
		for _, n := range variants {
			row.VariantNumbers = append(row.VariantNumbers, int(n))
		}
		if len(row.TextPreview) > 160 {
			row.TextPreview = row.TextPreview[:160] + "…"
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return ListQuestionsResult{}, err
	}
	return ListQuestionsResult{Items: items, Page: page, Limit: limit, Total: total}, nil
}

// GetQuestion returns one question with answers, translations, variants, explanation.
func (s Store) GetQuestion(ctx context.Context, id uuid.UUID) (QuestionDetail, error) {
	var d QuestionDetail
	err := s.Pool.QueryRow(ctx, `
		SELECT q.id, q.source_ext_id, q.category_id, c.code,
		       COALESCE(ct.name, c.code), q.validation_status, q.source, q.content_hash,
		       q.created_at, q.updated_at
		FROM question q
		JOIN category c ON c.id = q.category_id
		LEFT JOIN category_translation ct
		  ON ct.category_id = c.id AND ct.locale = 'uz-Latn'
		WHERE q.id = $1`, id).Scan(
		&d.ID, &d.SourceExtID, &d.CategoryID, &d.CategoryCode, &d.CategoryName,
		&d.ValidationStatus, &d.Source, &d.ContentHash, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return QuestionDetail{}, err
	}
	d.CreatedAt = d.CreatedAt.UTC()
	d.UpdatedAt = d.UpdatedAt.UTC()

	tRows, err := s.Pool.Query(ctx, `
		SELECT locale::text, text, status
		FROM question_translation WHERE question_id = $1 ORDER BY locale`, id)
	if err != nil {
		return QuestionDetail{}, err
	}
	d.Translations = make([]TranslationSummary, 0)
	for tRows.Next() {
		var t TranslationSummary
		if err := tRows.Scan(&t.Locale, &t.Text, &t.Status); err != nil {
			tRows.Close()
			return QuestionDetail{}, err
		}
		d.Translations = append(d.Translations, t)
	}
	tRows.Close()
	if err := tRows.Err(); err != nil {
		return QuestionDetail{}, err
	}

	aRows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.position, a.is_correct
		FROM answer a WHERE a.question_id = $1 ORDER BY a.position`, id)
	if err != nil {
		return QuestionDetail{}, err
	}
	d.Answers = make([]AnswerSummary, 0)
	for aRows.Next() {
		var a AnswerSummary
		if err := aRows.Scan(&a.ID, &a.Position, &a.IsCorrect); err != nil {
			aRows.Close()
			return QuestionDetail{}, err
		}
		d.Answers = append(d.Answers, a)
	}
	aRows.Close()
	if err := aRows.Err(); err != nil {
		return QuestionDetail{}, err
	}
	for i := range d.Answers {
		tr, err := s.Pool.Query(ctx, `
			SELECT locale::text, text, status
			FROM answer_translation WHERE answer_id = $1 ORDER BY locale`, d.Answers[i].ID)
		if err != nil {
			return QuestionDetail{}, err
		}
		d.Answers[i].Translations = make([]TranslationSummary, 0)
		for tr.Next() {
			var t TranslationSummary
			if err := tr.Scan(&t.Locale, &t.Text, &t.Status); err != nil {
				tr.Close()
				return QuestionDetail{}, err
			}
			d.Answers[i].Translations = append(d.Answers[i].Translations, t)
		}
		tr.Close()
		if err := tr.Err(); err != nil {
			return QuestionDetail{}, err
		}
	}

	vRows, err := s.Pool.Query(ctx, `
		SELECT v.number, vq.position
		FROM variant_question vq
		JOIN variant v ON v.id = vq.variant_id
		WHERE vq.question_id = $1
		ORDER BY v.number`, id)
	if err != nil {
		return QuestionDetail{}, err
	}
	d.Variants = make([]VariantRef, 0)
	for vRows.Next() {
		var v VariantRef
		if err := vRows.Scan(&v.Number, &v.Position); err != nil {
			vRows.Close()
			return QuestionDetail{}, err
		}
		d.Variants = append(d.Variants, v)
	}
	vRows.Close()
	if err := vRows.Err(); err != nil {
		return QuestionDetail{}, err
	}

	var explID uuid.UUID
	err = s.Pool.QueryRow(ctx, `SELECT id FROM explanation WHERE question_id = $1`, id).Scan(&explID)
	if err == nil {
		sum := &ExplanationSummary{ID: explID, Locales: make([]TranslationSummary, 0)}
		eRows, err := s.Pool.Query(ctx, `
			SELECT locale::text, status, blocks, verified_at
			FROM explanation_translation
			WHERE explanation_id = $1 ORDER BY locale`, explID)
		if err != nil {
			return QuestionDetail{}, err
		}
		for eRows.Next() {
			var loc, status string
			var blocks []byte
			var verifiedAt *time.Time
			if err := eRows.Scan(&loc, &status, &blocks, &verifiedAt); err != nil {
				eRows.Close()
				return QuestionDetail{}, err
			}
			sum.Locales = append(sum.Locales, TranslationSummary{Locale: loc, Status: status})
			if loc == "uz-Latn" {
				sum.BlocksPreview = blocksPreview(blocks)
				if verifiedAt != nil {
					t := verifiedAt.UTC()
					sum.VerifiedAt = &t
				}
			}
		}
		eRows.Close()
		if err := eRows.Err(); err != nil {
			return QuestionDetail{}, err
		}
		d.Explanation = sum
	} else if !IsNoRows(err) {
		return QuestionDetail{}, err
	}

	return d, nil
}

// SetQuestionValidationStatus soft-edits validation_status (valid|quarantined).
func (s Store) SetQuestionValidationStatus(ctx context.Context, id uuid.UUID, status string) (before, after string, err error) {
	status = strings.TrimSpace(status)
	if status != "valid" && status != "quarantined" {
		return "", "", fmt.Errorf("invalid status")
	}
	err = s.Pool.QueryRow(ctx, `SELECT validation_status FROM question WHERE id = $1`, id).Scan(&before)
	if err != nil {
		return "", "", err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE question SET validation_status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return "", "", err
	}
	if tag.RowsAffected() == 0 {
		return "", "", pgx.ErrNoRows
	}
	return before, status, nil
}

// ListExplanations returns explanation translations for the queue (default uz-Latn).
func (s Store) ListExplanations(ctx context.Context, status string, page, limit int) (ListExplanationsResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "draft"
	}
	offset := (page - 1) * limit

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM explanation_translation et
		JOIN explanation e ON e.id = et.explanation_id
		WHERE et.locale = 'uz-Latn' AND et.status = $1`, status).Scan(&total)
	if err != nil {
		return ListExplanationsResult{}, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT e.question_id, q.source_ext_id,
		       COALESCE(qt.text, '') AS text_preview,
		       et.locale::text, et.status, et.source, et.verified_at,
		       c.code, e.id
		FROM explanation_translation et
		JOIN explanation e ON e.id = et.explanation_id
		JOIN question q ON q.id = e.question_id
		JOIN category c ON c.id = q.category_id
		LEFT JOIN question_translation qt
		  ON qt.question_id = q.id AND qt.locale = 'uz-Latn'
		WHERE et.locale = 'uz-Latn' AND et.status = $1
		ORDER BY COALESCE(et.verified_at, e.created_at) DESC, e.created_at DESC
		LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return ListExplanationsResult{}, err
	}
	defer rows.Close()

	items := make([]ExplanationQueueRow, 0)
	for rows.Next() {
		var row ExplanationQueueRow
		var verifiedAt *time.Time
		if err := rows.Scan(
			&row.QuestionID, &row.SourceExtID, &row.TextPreview,
			&row.Locale, &row.Status, &row.Source, &verifiedAt,
			&row.CategoryCode, &row.ExplanationID,
		); err != nil {
			return ListExplanationsResult{}, err
		}
		if verifiedAt != nil {
			t := verifiedAt.UTC()
			row.VerifiedAt = &t
		}
		if len(row.TextPreview) > 160 {
			row.TextPreview = row.TextPreview[:160] + "…"
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return ListExplanationsResult{}, err
	}
	return ListExplanationsResult{Items: items, Page: page, Limit: limit, Total: total}, nil
}

// VerifyQuestionExplanation marks uz-Latn explanation translation verified (same as CLI).
func (s Store) VerifyQuestionExplanation(ctx context.Context, questionID, verifiedBy uuid.UUID) (before, after string, err error) {
	var explID uuid.UUID
	err = s.Pool.QueryRow(ctx, `SELECT id FROM explanation WHERE question_id = $1`, questionID).Scan(&explID)
	if err != nil {
		return "", "", err
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT status FROM explanation_translation
		WHERE explanation_id = $1 AND locale = 'uz-Latn'`, explID).Scan(&before)
	if err != nil {
		return "", "", err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE explanation_translation
		SET status = 'verified', verified_by = $2, verified_at = now()
		WHERE explanation_id = $1 AND locale = 'uz-Latn'`, explID, verifiedBy)
	if err != nil {
		return "", "", err
	}
	if tag.RowsAffected() == 0 {
		return "", "", pgx.ErrNoRows
	}
	return before, "verified", nil
}

func blocksPreview(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil || len(blocks) == 0 {
		s := string(raw)
		if len(s) > 200 {
			return s[:200] + "…"
		}
		return s
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t, ok := b["text"].(string); ok && t != "" {
			parts = append(parts, t)
		} else if t, ok := b["body"].(string); ok && t != "" {
			parts = append(parts, t)
		}
	}
	joined := strings.Join(parts, " ")
	if len(joined) > 200 {
		return joined[:200] + "…"
	}
	return joined
}

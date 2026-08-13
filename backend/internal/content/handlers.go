// Package content serves read-only learning content. It never exposes
// correctness fields — scoring is session-scoped (Plan 03).
package content

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

type Handler struct {
	Q         *sqlc.Queries
	MediaBase string // e.g. http://localhost:9000/media
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/categories", h.listCategories)
	r.Get("/variants", h.listVariants)
	r.Get("/variants/{number}", h.getVariant)
	r.Get("/signs", h.listSigns)
	r.Get("/signs/{code}", h.getSign)
	r.Get("/questions/{id}", h.getQuestion)
}

func cacheable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=300")
}

func (h *Handler) media(key pgtype.Text) *string {
	if !key.Valid || key.String == "" {
		return nil
	}
	u := strings.TrimRight(h.MediaBase, "/") + "/" + key.String
	return &u
}

func locale(w http.ResponseWriter, r *http.Request) (string, bool) {
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
	}
	return loc, ok
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	loc, ok := locale(w, r)
	if !ok {
		return
	}
	rows, err := h.Q.ListCategories(r.Context(), loc)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "categories query failed")
		return
	}
	out := make([]CategoryDTO, 0, len(rows))
	fallback := false
	for _, c := range rows {
		fallback = fallback || c.FallbackUsed
		out = append(out, CategoryDTO{
			Code: c.Code, Name: c.Name, SortOrder: c.SortOrder, QuestionCount: c.QuestionCount,
		})
	}
	cacheable(w)
	httpx.DataMeta(w, http.StatusOK, out, LocaleMeta{Locale: loc, Fallback: fallback})
}

func (h *Handler) listVariants(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Q.ListVariants(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "variants query failed")
		return
	}
	out := make([]VariantListItemDTO, 0, len(rows))
	for _, v := range rows {
		out = append(out, VariantListItemDTO{Number: v.Number, QuestionCount: v.QuestionCount})
	}
	cacheable(w)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getVariant(w http.ResponseWriter, r *http.Request) {
	loc, ok := locale(w, r)
	if !ok {
		return
	}
	num, err := strconv.Atoi(chi.URLParam(r, "number"))
	if err != nil || num < 1 {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "variant number must be a positive integer")
		return
	}
	v, err := h.Q.GetVariantByNumber(r.Context(), int32(num))
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "variant not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "variant query failed")
		return
	}
	qs, err := h.Q.ListVariantQuestions(r.Context(),
		sqlc.ListVariantQuestionsParams{VariantID: v.ID, Locale: loc})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "variant questions query failed")
		return
	}
	ids := make([]uuid.UUID, len(qs))
	for i, q := range qs {
		ids[i] = q.ID
	}
	ans, err := h.Q.ListAnswersByQuestionIDs(r.Context(),
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: ids, Locale: loc})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "answers query failed")
		return
	}
	byQ := map[uuid.UUID][]AnswerDTO{}
	fallback := false
	for _, a := range ans {
		fallback = fallback || a.FallbackUsed
		byQ[a.QuestionID] = append(byQ[a.QuestionID], AnswerDTO{
			ID: a.ID.String(), Position: a.Position, Text: a.Text, ImageURL: h.media(a.ImageKey),
		})
	}
	detail := VariantDetailDTO{Number: v.Number, Questions: make([]QuestionDTO, 0, len(qs))}
	for _, q := range qs {
		fallback = fallback || q.FallbackUsed
		detail.Questions = append(detail.Questions, QuestionDTO{
			ID: q.ID.String(), Position: q.Position, CategoryCode: q.CategoryCode,
			Text: q.Text, ImageURL: h.media(q.ImageKey), Answers: byQ[q.ID],
		})
	}
	cacheable(w)
	httpx.DataMeta(w, http.StatusOK, detail, LocaleMeta{Locale: loc, Fallback: fallback})
}

func (h *Handler) listSigns(w http.ResponseWriter, r *http.Request) {
	loc, ok := locale(w, r)
	if !ok {
		return
	}
	rows, err := h.Q.ListSigns(r.Context(), sqlc.ListSignsParams{
		Locale:    loc,
		GroupCode: r.URL.Query().Get("group"),
		Search:    r.URL.Query().Get("q"),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "signs query failed")
		return
	}
	out := make([]SignListItemDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, SignListItemDTO{
			Code: s.Code, GroupCode: s.GroupCode, Name: s.Name,
			ImageURL: h.media(s.ImageKey), QuestionCount: s.QuestionCount,
		})
	}
	cacheable(w)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getSign(w http.ResponseWriter, r *http.Request) {
	loc, ok := locale(w, r)
	if !ok {
		return
	}
	s, err := h.Q.GetSignByCode(r.Context(),
		sqlc.GetSignByCodeParams{Code: chi.URLParam(r, "code"), Locale: loc})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "sign not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "sign query failed")
		return
	}
	qids, err := h.Q.ListSignQuestionIDs(r.Context(), s.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "sign questions query failed")
		return
	}
	idStrs := make([]string, len(qids))
	for i, id := range qids {
		idStrs[i] = id.String()
	}
	cacheable(w)
	httpx.Data(w, http.StatusOK, SignDetailDTO{
		Code: s.Code, GroupCode: s.GroupCode, Name: s.Name, Description: s.Description,
		ImageURL: h.media(s.ImageKey), QuestionIDs: idStrs,
	})
}

func (h *Handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	loc, ok := locale(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "question id must be a UUID")
		return
	}
	detail, fallback, err := h.LoadQuestionDetail(r.Context(), id, loc)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		return
	}
	// A verified explanation can state the correct option in prose. Public
	// question content must therefore stay grading-neutral; session-scoped
	// feedback surfaces reveal explanations only after the server permits it.
	detail.Explanation = nil
	cacheable(w)
	httpx.DataMeta(w, http.StatusOK, detail, LocaleMeta{Locale: loc, Fallback: fallback})
}

// LoadQuestionDetail loads a single question's full public detail (answers,
// linked signs, verified explanation) in the given locale, applying the same
// locale-fallback rule as every other content query (exact-locale row wins,
// otherwise uz-Latn). It never returns correctness fields — the anti-cheat
// rule for this whole package. This is GetQuestion's (GET /questions/{id})
// sole rendering path, extracted so the public demo endpoints
// (internal/demo) can reuse the exact same shape rather than inventing a
// second one. A missing/invalid question surfaces as pgx.ErrNoRows, exactly
// as h.Q.GetQuestion itself would.
func (h *Handler) LoadQuestionDetail(ctx context.Context, id uuid.UUID, loc string) (QuestionDetailDTO, bool, error) {
	q, err := h.Q.GetQuestion(ctx, sqlc.GetQuestionParams{ID: id, Locale: loc})
	if err != nil {
		return QuestionDetailDTO{}, false, err
	}
	ans, err := h.Q.ListAnswersByQuestionIDs(ctx,
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{id}, Locale: loc})
	if err != nil {
		return QuestionDetailDTO{}, false, err
	}
	answers := make([]AnswerDTO, 0, len(ans))
	for _, a := range ans {
		answers = append(answers, AnswerDTO{
			ID: a.ID.String(), Position: a.Position, Text: a.Text, ImageURL: h.media(a.ImageKey),
		})
	}
	signs, err := h.Q.ListQuestionSigns(ctx,
		sqlc.ListQuestionSignsParams{QuestionID: id, Locale: loc})
	if err != nil {
		return QuestionDetailDTO{}, false, err
	}
	chips := make([]SignChipDTO, 0, len(signs))
	for _, s := range signs {
		chips = append(chips, SignChipDTO{Code: s.Code, Name: s.Name, ImageURL: h.media(s.ImageKey)})
	}
	detail := QuestionDetailDTO{
		QuestionDTO: QuestionDTO{
			ID: q.ID.String(), CategoryCode: q.CategoryCode, Text: q.Text,
			ImageURL: h.media(q.ImageKey), Answers: answers,
		},
		Signs: chips,
	}
	expl, err := h.Q.GetVerifiedExplanation(ctx,
		sqlc.GetVerifiedExplanationParams{QuestionID: id, Locale: loc})
	if err == nil {
		detail.Explanation = &ExplanationDTO{LegalRefs: expl.LegalRefs, Blocks: expl.Blocks}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return QuestionDetailDTO{}, false, err
	}
	return detail, q.FallbackUsed, nil
}

// LoadQuestionDetails loads the same public question payload as
// LoadQuestionDetail, but in a handful of queries for the whole ID set
// instead of four round-trips per question. Missing/invalid IDs are omitted
// from the map (callers treat a hole as not found). An empty ID list is a
// no-op. includeExplanations is false when every question is still sealed
// so the batch path does not pull expert prose the handler would immediately
// discard.
func (h *Handler) LoadQuestionDetails(ctx context.Context, ids []uuid.UUID, loc string, includeExplanations bool) (map[uuid.UUID]QuestionDetailDTO, bool, error) {
	out := make(map[uuid.UUID]QuestionDetailDTO, len(ids))
	if len(ids) == 0 {
		return out, false, nil
	}

	questions, err := h.Q.ListQuestionsByIDs(ctx, sqlc.ListQuestionsByIDsParams{Ids: ids, Locale: loc})
	if err != nil {
		return nil, false, err
	}
	answers, err := h.Q.ListAnswersByQuestionIDs(ctx, sqlc.ListAnswersByQuestionIDsParams{QuestionIds: ids, Locale: loc})
	if err != nil {
		return nil, false, err
	}
	signs, err := h.Q.ListQuestionSignsByQuestionIDs(ctx, sqlc.ListQuestionSignsByQuestionIDsParams{QuestionIds: ids, Locale: loc})
	if err != nil {
		return nil, false, err
	}

	var explanations []sqlc.ListVerifiedExplanationsByQuestionIDsRow
	if includeExplanations {
		explanations, err = h.Q.ListVerifiedExplanationsByQuestionIDs(ctx, sqlc.ListVerifiedExplanationsByQuestionIDsParams{QuestionIds: ids, Locale: loc})
		if err != nil {
			return nil, false, err
		}
	}

	answersByQ := make(map[uuid.UUID][]AnswerDTO, len(questions))
	for _, a := range answers {
		answersByQ[a.QuestionID] = append(answersByQ[a.QuestionID], AnswerDTO{
			ID: a.ID.String(), Position: a.Position, Text: a.Text, ImageURL: h.media(a.ImageKey),
		})
	}
	signsByQ := make(map[uuid.UUID][]SignChipDTO, len(questions))
	for _, s := range signs {
		signsByQ[s.QuestionID] = append(signsByQ[s.QuestionID], SignChipDTO{
			Code: s.Code, Name: s.Name, ImageURL: h.media(s.ImageKey),
		})
	}
	explByQ := make(map[uuid.UUID]ExplanationDTO, len(explanations))
	for _, e := range explanations {
		explByQ[e.QuestionID] = ExplanationDTO{LegalRefs: e.LegalRefs, Blocks: e.Blocks}
	}

	fallback := false
	for _, q := range questions {
		fallback = fallback || q.FallbackUsed
		ans := answersByQ[q.ID]
		if ans == nil {
			ans = []AnswerDTO{}
		}
		chips := signsByQ[q.ID]
		if chips == nil {
			chips = []SignChipDTO{}
		}
		detail := QuestionDetailDTO{
			QuestionDTO: QuestionDTO{
				ID: q.ID.String(), CategoryCode: q.CategoryCode, Text: q.Text,
				ImageURL: h.media(q.ImageKey), Answers: ans,
			},
			Signs: chips,
		}
		if expl, ok := explByQ[q.ID]; ok {
			copied := expl
			detail.Explanation = &copied
		}
		out[q.ID] = detail
	}
	return out, fallback, nil
}

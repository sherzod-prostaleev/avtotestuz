// Package explanation owns the AI-draft → expert-verify → learner-feedback
// pipeline for per-question legal explanations. draft.go is pure (no DB);
// service.go integrates it with the explanation/explanation_translation/
// explanation_feedback tables.
package explanation

import "fmt"

const aiMarker = "[AI-QORALAMA]"

type AnswerAnalysisItem struct {
	Position int    `json:"position"`
	Correct  bool   `json:"correct"`
	Text     string `json:"text"`
}

type Block struct {
	Type  string               `json:"type"`
	Text  string               `json:"text,omitempty"`
	Items []AnswerAnalysisItem `json:"items,omitempty"`
}

type DraftInput struct {
	QuestionText      string
	CategoryCode      string
	CorrectAnswerText string
}

// AIDraftGenerator produces a first-pass explanation for expert review.
// TemplateDraftGenerator is the M1 stub — clearly marked, not real legal
// analysis; a real LLM-backed implementation can satisfy this same
// interface later without touching any caller.
type AIDraftGenerator interface {
	Generate(in DraftInput) []Block
}

type TemplateDraftGenerator struct{}

func (TemplateDraftGenerator) Generate(in DraftInput) []Block {
	qText := in.QuestionText
	if qText == "" {
		qText = "(savol matni mavjud emas)"
	}
	correct := in.CorrectAnswerText
	if correct == "" {
		correct = "(to'g'ri javob matni mavjud emas)"
	}
	cat := in.CategoryCode
	if cat == "" {
		cat = "umumiy"
	}

	return []Block{
		{Type: "intro", Text: fmt.Sprintf("%s Ushbu savol \"%s\" kategoriyasiga tegishli: %s", aiMarker, cat, qText)},
		{Type: "muhim", Text: fmt.Sprintf("%s MUHIM: to'g'ri javob — %s", aiMarker, correct)},
		{Type: "answer_analysis", Items: []AnswerAnalysisItem{
			{Position: 1, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 2, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 3, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 4, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
		}},
	}
}

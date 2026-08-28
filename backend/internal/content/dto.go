package content

import "encoding/json"

type CategoryDTO struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
	// QuestionCount lets a picker state how much material a topic holds
	// rather than implying they are all the same size; they are not.
	QuestionCount int32 `json:"question_count"`
}

type VariantListItemDTO struct {
	Number        int32 `json:"number"`
	QuestionCount int32 `json:"question_count"`
}

type AnswerDTO struct {
	ID       string  `json:"id"`
	Position int16   `json:"position"`
	Text     string  `json:"text"`
	ImageURL *string `json:"image_url"`

	// TextUzLatn is the same option in uz-Latn regardless of the reader's
	// locale, and never leaves the server. Answer shuffling decides on it so a
	// question that must keep its option order ("F1 va F2", "Barcha javoblar
	// to'g'ri") keeps it in every language: the three translations are worded
	// independently, so a per-locale decision disagreed with itself.
	TextUzLatn string `json:"-"`
}

type QuestionDTO struct {
	ID           string      `json:"id"`
	Position     int16       `json:"position,omitempty"`
	CategoryCode string      `json:"category_code"`
	Text         string      `json:"text"`
	ImageURL     *string     `json:"image_url"`
	Answers      []AnswerDTO `json:"answers"`
}

type VariantDetailDTO struct {
	Number    int32         `json:"number"`
	Questions []QuestionDTO `json:"questions"`
}

type SignListItemDTO struct {
	Code          string  `json:"code"`
	GroupCode     string  `json:"group_code"`
	Name          string  `json:"name"`
	ImageURL      *string `json:"image_url"`
	QuestionCount int32   `json:"question_count"`
}

type SignDetailDTO struct {
	Code        string   `json:"code"`
	GroupCode   string   `json:"group_code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ImageURL    *string  `json:"image_url"`
	QuestionIDs []string `json:"question_ids"`
}

type SignChipDTO struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ImageURL *string `json:"image_url"`
}

type ExplanationDTO struct {
	LegalRefs json.RawMessage `json:"legal_refs"`
	Blocks    json.RawMessage `json:"blocks"`
}

type QuestionDetailDTO struct {
	QuestionDTO
	Signs       []SignChipDTO   `json:"signs"`
	Explanation *ExplanationDTO `json:"explanation"`
}

type LocaleMeta struct {
	Locale   string `json:"locale"`
	Fallback bool   `json:"fallback"`
}

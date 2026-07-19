// Package importer ingests the canonical dataset with strict validation.
// Nothing is ever guessed or auto-filled: invalid entries are quarantined.
package importer

// RequiredLocales must be present (and will be stored) for every question/answer.
var RequiredLocales = []string{"uz-Latn", "uz-Cyrl", "ru"}

type Dataset struct {
	Categories   []CanonCategory    `json:"categories"`
	SignGroups   []CanonSignGroup   `json:"sign_groups"`
	Signs        []CanonSign        `json:"signs"`
	Questions    []CanonQuestion    `json:"questions"`
	Variants     []CanonVariant     `json:"variants"`
	Explanations []CanonExplanation `json:"explanations"`
}

type CanonCategory struct {
	Code  string            `json:"code"`
	Sort  int               `json:"sort"`
	Names map[string]string `json:"names"` // locale → name
}

type CanonSignGroup struct {
	Code  string            `json:"code"`
	Sort  int               `json:"sort"`
	Names map[string]string `json:"names"`
}

type CanonSign struct {
	Code         string            `json:"code"`
	Group        string            `json:"group"`
	Image        string            `json:"image"` // relative path inside dataset dir
	Sort         int               `json:"sort"`
	Names        map[string]string `json:"names"`
	Descriptions map[string]string `json:"descriptions"`
}

type CanonAnswer struct {
	Position int               `json:"position"` // 1..5
	Correct  bool              `json:"correct"`
	Texts    map[string]string `json:"texts"`
	Image    string            `json:"image,omitempty"`
}

type CanonQuestion struct {
	ExtID    string            `json:"ext_id"`
	Category string            `json:"category"`
	Image    string            `json:"image,omitempty"`
	Texts    map[string]string `json:"texts"`
	Answers  []CanonAnswer     `json:"answers"`
	Signs    []string          `json:"signs,omitempty"`
	Source   string            `json:"source,omitempty"`
}

type CanonVariant struct {
	Number    int      `json:"number"`
	Questions []string `json:"questions"` // ext_ids, ordered, exactly 20
}

type CanonExplanation struct {
	Question  string                      `json:"question"` // ext_id
	LegalRefs []map[string]string         `json:"legal_refs"`
	Blocks    map[string][]map[string]any `json:"blocks"` // locale → ordered blocks
}

type Issue struct {
	Entity string `json:"entity"` // question | variant | sign | category | dataset
	ID     string `json:"id"`     // ext_id / code / number
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

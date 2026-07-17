package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ImageSource interface {
	Open(path string) ([]byte, error)
}

type MapSource map[string][]byte

func (m MapSource) Open(path string) ([]byte, error) {
	b, ok := m[path]
	if !ok {
		return nil, fmt.Errorf("image not found: %s", path)
	}
	return b, nil
}

type DirSource string

func (d DirSource) Open(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(d), filepath.FromSlash(path)))
}

type Report struct {
	Categories           int
	Signs                int
	Images               int
	QuestionsValid       int
	QuestionsQuarantined int
	VariantsStored       int
	VariantsSkipped      int
	Issues               []Issue
}

func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "categories=%d signs=%d images=%d\n", r.Categories, r.Signs, r.Images)
	fmt.Fprintf(&b, "questions: valid=%d quarantined=%d\n", r.QuestionsValid, r.QuestionsQuarantined)
	fmt.Fprintf(&b, "variants: stored=%d skipped=%d\n", r.VariantsStored, r.VariantsSkipped)
	for _, i := range r.Issues {
		fmt.Fprintf(&b, "ISSUE %s[%s] %s %s\n", i.Entity, i.ID, i.Code, i.Detail)
	}
	return b.String()
}

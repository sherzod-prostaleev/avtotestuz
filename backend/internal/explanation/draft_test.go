package explanation

import (
	"strings"
	"testing"
)

func TestTemplateDraftGeneratorProducesMarkedPlaceholder(t *testing.T) {
	g := TemplateDraftGenerator{}
	blocks := g.Generate(DraftInput{
		QuestionText:      "Svetofor sariq rangda yonganda nima qilish kerak?",
		CategoryCode:      "rules",
		CorrectAnswerText: "To'xtash chizig'i oldida to'xtash",
	})
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	var hasIntro, hasMuhim, hasAnalysis bool
	for _, b := range blocks {
		switch b.Type {
		case "intro":
			hasIntro = true
			if !containsMarker(b.Text) {
				t.Errorf("intro block must carry the [AI-QORALAMA] marker, got %q", b.Text)
			}
		case "muhim":
			hasMuhim = true
			if !containsMarker(b.Text) {
				t.Errorf("muhim block must carry the [AI-QORALAMA] marker, got %q", b.Text)
			}
		case "answer_analysis":
			hasAnalysis = true
		}
	}
	if !hasIntro || !hasMuhim || !hasAnalysis {
		t.Fatalf("expected intro+muhim+answer_analysis blocks, got %+v", blocks)
	}
}

func TestTemplateDraftGeneratorNeverPanicsOnEmptyInput(t *testing.T) {
	g := TemplateDraftGenerator{}
	blocks := g.Generate(DraftInput{})
	if len(blocks) == 0 {
		t.Fatal("even empty input should produce a non-empty placeholder structure")
	}
}

func containsMarker(s string) bool {
	return strings.Contains(s, aiMarker)
}

package session_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

func TestCategoryMemorizeReturnsWholeTopicInSourceOrder(t *testing.T) {
	q, svc, _ := seed(t)
	ctx := context.Background()

	catID, err := q.GetCategoryIDByCode(ctx, "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	total, err := q.CountValidQuestionsInCategory(ctx, catID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total == 0 {
		t.Fatal("fixture category 'signs' has no questions to test against")
	}

	items, err := svc.CategoryMemorize(ctx, catID)
	if err != nil {
		t.Fatalf("CategoryMemorize: %v", err)
	}
	if len(items) != int(total) {
		t.Fatalf("len(items)=%d want %d", len(items), total)
	}

	wantIDs, err := q.OrderedQuestionIDsByCategory(ctx, sqlc.OrderedQuestionIDsByCategoryParams{
		CategoryID: catID, Skip: 0, LimitCount: total,
	})
	if err != nil {
		t.Fatalf("ordered ids: %v", err)
	}

	for i, item := range items {
		if item.QuestionID != wantIDs[i] {
			t.Fatalf("items[%d].QuestionID=%s want %s", i, item.QuestionID, wantIDs[i])
		}
		if item.Position != i+1 {
			t.Fatalf("items[%d].Position=%d want %d", i, item.Position, i+1)
		}
		wantCorrect, err := q.GetCorrectAnswerID(ctx, item.QuestionID)
		if err != nil {
			t.Fatalf("correct answer for %s: %v", item.QuestionID, err)
		}
		if item.CorrectAnswerID != wantCorrect {
			t.Fatalf("items[%d].CorrectAnswerID=%s want %s", i, item.CorrectAnswerID, wantCorrect)
		}
	}
}

func TestCategoryMemorizeEmptyCategoryReturnsEmptyNotError(t *testing.T) {
	_, svc, _ := seed(t)
	items, err := svc.CategoryMemorize(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("CategoryMemorize: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items)=%d want 0 for a category with no questions", len(items))
	}
}

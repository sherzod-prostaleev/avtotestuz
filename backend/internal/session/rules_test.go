package session

import (
	"testing"

	"avtotest.uz/backend/internal/learning"
)

func TestIsVariantUnlocked(t *testing.T) {
	if !IsVariantUnlocked(1, false, false) {
		t.Fatal("first variant must always be unlocked for free users")
	}
	if !IsVariantUnlocked(1, true, false) {
		t.Fatal("first variant must always be unlocked for VIP")
	}
	if IsVariantUnlocked(2, false, true) {
		t.Fatal("non-first free variant must stay locked even if previous completed")
	}
	if IsVariantUnlocked(2, true, false) {
		t.Fatal("VIP must not unlock #2 before previous is completed")
	}
	if !IsVariantUnlocked(2, true, true) {
		t.Fatal("VIP must unlock #2 after previous completed")
	}
}

func TestVariantLockReason(t *testing.T) {
	if got := VariantLockReason(1, false, true); got != "" {
		t.Fatalf("unlocked #1 reason=%q", got)
	}
	if got := VariantLockReason(2, false, false); got != LockReasonVIPRequired {
		t.Fatalf("free #2 reason=%q want vip_required", got)
	}
	if got := VariantLockReason(2, true, false); got != LockReasonPrevRequired {
		t.Fatalf("VIP locked #2 reason=%q want prev_required", got)
	}
}

func TestIsExamLike(t *testing.T) {
	for _, mode := range []string{"exam", "grand_mock", "placement"} {
		if !IsExamLike(mode) {
			t.Fatalf("%q must be exam-like", mode)
		}
	}
	for _, mode := range []string{"variant", "practice", "mistakes", "review", ""} {
		if IsExamLike(mode) {
			t.Fatalf("%q must not be exam-like", mode)
		}
	}
}

func TestShouldStopExam(t *testing.T) {
	if ShouldStopExam(1) || ShouldStopExam(2) {
		t.Fatal("1 or 2 wrong answers must not stop the exam")
	}
	if !ShouldStopExam(3) {
		t.Fatal("3rd wrong answer must stop the exam")
	}
}

func TestShouldStopForErrorsPlacement(t *testing.T) {
	if ShouldStopForErrors(1, PlacementErrorsAllowed) {
		t.Fatal("1 wrong must not stop placement")
	}
	if !ShouldStopForErrors(2, PlacementErrorsAllowed) {
		t.Fatal("2nd wrong must stop placement")
	}
}

func TestEvaluateExamCompletedPass(t *testing.T) {
	out := EvaluateExam(18, 2, 20, ExamErrorsAllowed, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("18/20 with 2 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamCompletedFail(t *testing.T) {
	out := EvaluateExam(17, 3, 20, ExamErrorsAllowed, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("17/20 with 3 wrong should fail: %+v", out)
	}
}

func TestEvaluateExamTooManyErrors(t *testing.T) {
	out := EvaluateExam(5, 3, 20, ExamErrorsAllowed, false, true)
	if out.Status != "failed" || out.StoppedReason != "too_many_errors" {
		t.Fatalf("3rd wrong must fail immediately: %+v", out)
	}
}

func TestEvaluateExamTimeUpPass(t *testing.T) {
	out := EvaluateExam(19, 1, 20, ExamErrorsAllowed, true, false)
	if out.Status != "passed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up but already 19/20 with 1 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamTimeUpFail(t *testing.T) {
	out := EvaluateExam(10, 1, 20, ExamErrorsAllowed, true, false)
	if out.Status != "failed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up with only 10 answered should fail: %+v", out)
	}
}

func TestEvaluatePlacementPass(t *testing.T) {
	out := EvaluatePlacement(9, 1, 10, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("9/10 with 1 wrong should pass: %+v", out)
	}
}

func TestEvaluatePlacementPerfectPass(t *testing.T) {
	out := EvaluatePlacement(10, 0, 10, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("10/10 should pass: %+v", out)
	}
}

func TestEvaluatePlacementFailTooManyWrong(t *testing.T) {
	out := EvaluatePlacement(8, 2, 10, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("8/10 with 2 wrong should fail: %+v", out)
	}
}

func TestEvaluatePlacementIncompleteFail(t *testing.T) {
	out := EvaluatePlacement(9, 0, 10, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("incomplete placement must fail even with 0 wrong: %+v", out)
	}
}

func TestEvaluatePlacementTooManyErrors(t *testing.T) {
	out := EvaluatePlacement(5, 2, 10, false, true)
	if out.Status != "failed" || out.StoppedReason != "too_many_errors" {
		t.Fatalf("2nd wrong must fail immediately: %+v", out)
	}
}

func TestEvaluatePlacementTimeUpFail(t *testing.T) {
	out := EvaluatePlacement(9, 1, 10, true, false)
	if out.Status != "failed" || out.StoppedReason != "time_up" {
		t.Fatalf("timed out placement must fail: %+v", out)
	}
}

func TestFSRSRatingForCorrect(t *testing.T) {
	again, hard, good, easy, invalid := 1, 2, 3, 4, 99

	cases := []struct {
		name      string
		explicit  *int
		latencyMs int
		want      learning.Rating
	}{
		{"nil defaults to Good", nil, 0, learning.Good},
		{"explicit Hard", &hard, 0, learning.Hard},
		{"explicit Good", &good, 99999, learning.Good},
		{"explicit Easy", &easy, 0, learning.Easy},
		{"Again coerced to Hard", &again, 0, learning.Hard},
		{"invalid explicit falls back to Good", &invalid, 0, learning.Good},
		{"fast latency Easy", nil, 3999, learning.Easy},
		{"medium latency Good", nil, 4000, learning.Good},
		{"slow latency Hard", nil, 25000, learning.Hard},
		{"zero latency ignored", nil, 0, learning.Good},
		{"negative latency ignored", nil, -1, learning.Good},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FSRSRatingForCorrect(tc.explicit, tc.latencyMs)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestExamConfigForStandard(t *testing.T) {
	cfg, ok := ExamConfigFor(ExamQuestionCount)
	if !ok {
		t.Fatal("20 must be a valid exam size")
	}
	if cfg.QuestionCount != 20 || cfg.TimeLimitSec != 25*60 || cfg.ErrorsAllowed != 2 {
		t.Fatalf("standard exam config = %+v", cfg)
	}
}

func TestExamConfigForRestore(t *testing.T) {
	cfg, ok := ExamConfigFor(ExamRestoreQuestionCount)
	if !ok {
		t.Fatal("50 must be a valid exam size")
	}
	if cfg.QuestionCount != 50 || cfg.TimeLimitSec != 50*60 || cfg.ErrorsAllowed != 4 {
		t.Fatalf("restore exam config = %+v", cfg)
	}
}

// An absent count is how every pre-existing ?mode=exam link and saved bookmark
// arrives; it must keep meaning the standard 20-question exam.
func TestExamConfigForZeroDefaultsToStandard(t *testing.T) {
	cfg, ok := ExamConfigFor(0)
	if !ok {
		t.Fatal("an unspecified count must fall back to the standard exam")
	}
	if cfg.QuestionCount != ExamQuestionCount {
		t.Fatalf("zero count = %d questions, want %d", cfg.QuestionCount, ExamQuestionCount)
	}
}

// Only 20 and 50 are real exams. Anything else must be refused rather than
// silently honoured, or a client could ask for a 3-question "exam" and pass it.
func TestExamConfigForRejectsUnknownSizes(t *testing.T) {
	for _, count := range []int{-1, 1, 3, 19, 21, 30, 49, 51, 100, 1000} {
		if _, ok := ExamConfigFor(count); ok {
			t.Fatalf("count=%d must not be a valid exam size", count)
		}
	}
}

// The 46/50 pass bar is the whole point of the restore exam: 4 mistakes still
// pass, the 5th does not. Before errorsAllowed was a parameter this evaluated
// against the constant 2 and failed a 47/50 run that really passes.
func TestEvaluateExamRestorePassBar(t *testing.T) {
	cases := []struct {
		name    string
		correct int
		wrong   int
		want    string
	}{
		{"50/50 passes", 50, 0, "passed"},
		{"47/50 passes", 47, 3, "passed"},
		{"46/50 passes", 46, 4, "passed"},
		{"45/50 fails", 45, 5, "failed"},
		{"40/50 fails", 40, 10, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := EvaluateExam(tc.correct, tc.wrong, 50, ExamRestoreErrorsAllowed, false, false)
			if out.Status != tc.want {
				t.Fatalf("%d correct / %d wrong = %q, want %q", tc.correct, tc.wrong, out.Status, tc.want)
			}
		})
	}
}

func TestShouldStopForErrorsRestore(t *testing.T) {
	for _, wrong := range []int{1, 2, 3, 4} {
		if ShouldStopForErrors(wrong, ExamRestoreErrorsAllowed) {
			t.Fatalf("%d wrong must not stop the restore exam", wrong)
		}
	}
	if !ShouldStopForErrors(5, ExamRestoreErrorsAllowed) {
		t.Fatal("5th wrong must stop the restore exam")
	}
}

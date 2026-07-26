package session

import (
	"testing"

	"avtotest.uz/backend/internal/learning"
)

func TestIsVariantUnlocked(t *testing.T) {
	if !IsVariantUnlocked(true, false) {
		t.Fatal("first variant must always be unlocked")
	}
	if !IsVariantUnlocked(false, true) {
		t.Fatal("VIP variant must always be unlocked")
	}
	if IsVariantUnlocked(false, false) {
		t.Fatal("non-first free variant must stay locked (matches StartSession VIP gate)")
	}
	if !IsVariantUnlocked(true, true) {
		t.Fatal("first VIP variant must be unlocked")
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
	out := EvaluateExam(18, 2, 20, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("18/20 with 2 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamCompletedFail(t *testing.T) {
	out := EvaluateExam(17, 3, 20, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("17/20 with 3 wrong should fail: %+v", out)
	}
}

func TestEvaluateExamTooManyErrors(t *testing.T) {
	out := EvaluateExam(5, 3, 20, false, true)
	if out.Status != "failed" || out.StoppedReason != "too_many_errors" {
		t.Fatalf("3rd wrong must fail immediately: %+v", out)
	}
}

func TestEvaluateExamTimeUpPass(t *testing.T) {
	out := EvaluateExam(19, 1, 20, true, false)
	if out.Status != "passed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up but already 19/20 with 1 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamTimeUpFail(t *testing.T) {
	out := EvaluateExam(10, 1, 20, true, false)
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

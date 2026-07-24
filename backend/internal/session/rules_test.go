package session

import "testing"

func TestIsVariantUnlocked(t *testing.T) {
	if !IsVariantUnlocked(true, false, 0, 10) {
		t.Fatal("first variant must always be unlocked")
	}
	if !IsVariantUnlocked(false, true, 0, 10) {
		t.Fatal("VIP variant must always be unlocked")
	}
	if IsVariantUnlocked(false, false, 9, 10) {
		t.Fatal("9 < threshold 10 must stay locked")
	}
	if !IsVariantUnlocked(false, false, 10, 10) {
		t.Fatal("10 >= threshold 10 must unlock")
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

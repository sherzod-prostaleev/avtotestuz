// Package session owns exam-session lifecycle: starting a session (question
// selection per mode), recording answers with server-side scoring, finishing
// with a computed result, resuming an interrupted session, and the bilet
// unlock / mistake-bank side effects that finishing triggers.
package session

const (
	ExamQuestionCount = 20
	ExamTimeLimitSec  = 25 * 60
	ExamErrorsAllowed = 2
)

// IsVariantUnlocked reports whether a variant is unlocked. The first variant
// in the sequence (isFirst) and any VIP/Premium profile (isVIP) are always
// unlocked; otherwise it requires the previous variant's best_correct to meet
// the configured threshold.
func IsVariantUnlocked(isFirst, isVIP bool, prevBestCorrect, threshold int) bool {
	if isFirst || isVIP {
		return true
	}
	return prevBestCorrect >= threshold
}

type ExamOutcome struct {
	Status        string // "passed" | "failed"
	StoppedReason string // "completed" | "time_up" | "too_many_errors"
}

// EvaluateExam computes the final status of an exam-mode session. Passing
// requires correct >= total-ExamErrorsAllowed (i.e. >=18/20) AND
// wrong <= ExamErrorsAllowed — matching the real exam's "≤2 xato" rule.
func EvaluateExam(correct, wrong, total int, timedOut, tooManyErrors bool) ExamOutcome {
	if tooManyErrors {
		return ExamOutcome{Status: "failed", StoppedReason: "too_many_errors"}
	}
	reason := "completed"
	if timedOut {
		reason = "time_up"
	}
	if correct >= total-ExamErrorsAllowed && wrong <= ExamErrorsAllowed {
		return ExamOutcome{Status: "passed", StoppedReason: reason}
	}
	return ExamOutcome{Status: "failed", StoppedReason: reason}
}

// ShouldStopExam reports whether the exam must stop immediately after this
// wrong answer — the real exam ends on the 3rd mistake.
func ShouldStopExam(wrongSoFar int) bool {
	return wrongSoFar > ExamErrorsAllowed
}

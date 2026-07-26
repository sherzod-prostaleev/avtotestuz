// Package session owns exam-session lifecycle: starting a session (question
// selection per mode), recording answers with server-side scoring, finishing
// with a computed result, resuming an interrupted session, and the bilet
// unlock / mistake-bank side effects that finishing triggers.
package session

import "avtotest.uz/backend/internal/learning"

const (
	ExamQuestionCount = 20
	ExamTimeLimitSec  = 25 * 60
	ExamErrorsAllowed = 2

	PlacementQuestionCount = 10
	PlacementTimeLimitSec  = 12 * 60
	PlacementErrorsAllowed = 1

	// MockMasteryThreshold and MockMinStudiedPct document the values SEEDED
	// into limit_config for the two Grand Mock gates: the minimum overall
	// readiness percentage (learning.Stats.ReadinessPct) and the minimum share
	// of the valid question bank that must actually have been studied.
	//
	// These constants are documentation only — limit_config is authoritative
	// and Service.MockEligibility reads it, so an operator (and later the M3
	// admin panel) can retune the gate. That deliberately replaces the earlier
	// arrangement where grand_mock_threshold_pct was seeded but read by
	// nothing while a constant silently won, so an admin who changed the row
	// would have watched it save and do absolutely nothing.
	MockMasteryThreshold = 85
	MockMinStudiedPct    = 25
)

// IsExamLike reports whether mode uses the strict timed/anti-cheat exam
// pipeline (time limit, answer redaction until finish, EvaluateExam /
// EvaluatePlacement scoring) — "exam", "grand_mock", and "placement".
func IsExamLike(mode string) bool {
	return mode == "exam" || mode == "grand_mock" || mode == "placement"
}

// Lock-reason codes returned on VariantStatus when Unlocked is false.
const (
	LockReasonVIPRequired  = "vip_required"
	LockReasonPrevRequired = "prev_required"
)

// IsVariantUnlocked reports whether variant number is playable.
//
//	#1 — always open (free demo bilet).
//	#N (N>1) — VIP only, and only after the previous bilet has completed_at
//	(≥ unlock_threshold_correct correct answers on finish).
//
// Must match StartSession's gates so unlocked=true never leads to vip_required
// / previous_ticket_required on start.
func IsVariantUnlocked(number int, isVIP, prevCompleted bool) bool {
	if number <= 1 {
		return true
	}
	return isVIP && prevCompleted
}

// VariantLockReason explains why a locked variant is locked. Empty when unlocked.
func VariantLockReason(number int, isVIP, unlocked bool) string {
	if unlocked || number <= 1 {
		return ""
	}
	if !isVIP {
		return LockReasonVIPRequired
	}
	return LockReasonPrevRequired
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

// EvaluatePlacement computes the final status of a placement diagnostic.
// Passing requires a completed run (all questions answered) with
// wrong <= PlacementErrorsAllowed. Timed out / too many errors always fail.
func EvaluatePlacement(correct, wrong, total int, timedOut, tooManyErrors bool) ExamOutcome {
	if tooManyErrors {
		return ExamOutcome{Status: "failed", StoppedReason: "too_many_errors"}
	}
	if timedOut {
		return ExamOutcome{Status: "failed", StoppedReason: "time_up"}
	}
	completed := correct+wrong >= total && total > 0
	if completed && wrong <= PlacementErrorsAllowed {
		return ExamOutcome{Status: "passed", StoppedReason: "completed"}
	}
	return ExamOutcome{Status: "failed", StoppedReason: "completed"}
}

// ShouldStopExam reports whether the exam must stop immediately after this
// wrong answer — the real exam ends on the 3rd mistake.
func ShouldStopExam(wrongSoFar int) bool {
	return ShouldStopForErrors(wrongSoFar, ExamErrorsAllowed)
}

// ShouldStopForErrors reports whether an exam-like session must stop after
// wrongSoFar mistakes given that mode's errors_allowed budget.
func ShouldStopForErrors(wrongSoFar, errorsAllowed int) bool {
	return wrongSoFar > errorsAllowed
}

// FSRSRatingForCorrect maps an optional explicit FSRS rating and/or answer
// latency into Hard/Good/Easy for a correct practice-style answer.
//
// Priority: explicit rating (1–4) → latency thresholds → Good.
// Again (1) on a correct answer is coerced to Hard; invalid values fall back
// to Good. latencyMs is only consulted when > 0.
func FSRSRatingForCorrect(explicit *int, latencyMs int) learning.Rating {
	if explicit != nil {
		switch learning.Rating(*explicit) {
		case learning.Hard, learning.Good, learning.Easy:
			return learning.Rating(*explicit)
		case learning.Again:
			return learning.Hard
		default:
			return learning.Good
		}
	}
	if latencyMs > 0 {
		switch {
		case latencyMs < 4000:
			return learning.Easy
		case latencyMs >= 25000:
			return learning.Hard
		}
	}
	return learning.Good
}

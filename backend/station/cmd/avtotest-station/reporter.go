package main

import (
	"context"
	"log"
	"time"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/status"
)

// The agent tells the school's admin panel what it is doing, so nobody has to
// walk to thirty machines and open a log file.
//
// Two routes, because the failures worth seeing most are the ones that leave a
// PC without a token: an enrolled station posts with its JWT, while a machine
// that could not enrol at all posts with the school's installer key. The
// second is the whole reason this exists -- a PC already registered to another
// school, or one whose school has no seats left, used to fail in total silence.

// reportSchedule bounds how often a PC files.
type reportSchedule struct {
	// settle is how long to wait after start before the first report, so it
	// describes a settled state rather than "starting".
	settle time.Duration
	// heartbeat is the interval for an unchanged, healthy PC.
	heartbeat time.Duration
	// minGap is the debounce. A PC flapping between waiting and blocked must
	// not turn into a report every few seconds.
	minGap time.Duration
	// poll is how often the status store is checked for a change.
	poll time.Duration
}

var defaultReportSchedule = reportSchedule{
	settle:    90 * time.Second,
	heartbeat: 6 * time.Hour,
	minGap:    5 * time.Minute,
	poll:      20 * time.Second,
}

// reportState is what a report was about, so an unchanged PC stays quiet.
type reportState struct {
	phase status.Phase
	code  string
}

// runReporter files reports for the life of the process: once the state has
// settled, again whenever phase or cause changes, and a heartbeat in between.
//
// Every failure is swallowed. A PC that cannot file a report is usually a PC
// that cannot reach the backend at all — which is the thing being reported —
// and retrying hard would add load to a network that is already failing. The
// kiosk screen still shows the same information locally.
func runReporter(ctx context.Context, rt *agentRuntime, st *status.Store, cfg resolved, logPath string, sched reportSchedule) {
	timer := time.NewTimer(sched.settle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	var last reportState
	var lastSent time.Time
	first := true

	for {
		snap := st.Get()
		now := time.Now()
		changed := first || snap.Phase != last.phase || snap.Code != last.code
		due := now.Sub(lastSent) >= sched.heartbeat
		if (changed || due) && now.Sub(lastSent) >= sched.minGap {
			if send(ctx, rt, snap, cfg, logPath) {
				last = reportState{phase: snap.Phase, code: snap.Code}
				lastSent = now
				first = false
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(sched.poll):
		}
	}
}

// send files one report and returns whether it landed.
//
// Which route depends on what this PC is: an enrolled station has a token and
// posts as itself; one that never enrolled has only the school's installer key
// and posts against the school instead. A PC with neither — a development
// build with no embedded key — has nothing to file under and stays silent.
func send(ctx context.Context, rt *agentRuntime, snap status.Snapshot, cfg resolved, logPath string) bool {
	a := rt.agent()
	if a == nil {
		return false
	}
	rep := agent.DiagReport{
		Phase:   string(snap.Phase),
		Code:    snap.Code,
		Problem: snap.Problem,
		Detail:  snap.Detail,
		Label:   snap.Label,
		LogTail: agent.ReadLogTail(logPath),
	}

	if snap.Enrolled {
		if err := a.ReportDiagnostics(ctx, rep); err == nil {
			return true
		} else {
			log.Printf("diagnostics report failed: %v", err)
			return false
		}
	}
	if cfg.Code == "" {
		return false
	}
	if err := a.ReportEnrollFailure(ctx, cfg.Code, rep); err != nil {
		log.Printf("enrollment diagnostics report failed: %v", err)
		return false
	}
	return true
}

package main

import (
	"context"
	"errors"
	"log"
	"os"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/diagnose"
	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/status"
)

// connectConfig is everything the background worker needs. It is a struct
// rather than six positional parameters, which at this length is a bug waiting
// for a refactor to swap two strings.
type connectConfig struct {
	rt       *agentRuntime
	st       *status.Store
	cfg      resolved
	stateDir string
	label    string
	reenroll bool
}

// connect owns everything that talks to the backend: the machine identity, the
// sealed key, enrollment and the token loop.
//
// It runs in its own goroutine, after the listener is already serving, and it
// never ends the process. Every failure it meets is written to the status
// store instead, where the kiosk page shows it in Uzbek. That is the whole
// difference from the old startup path: a PC that cannot enrol now displays a
// sentence naming the problem and the fix, rather than a console that hangs
// for two minutes and then disappears.
func connect(ctx context.Context, c connectConfig) {
	id, err := hwid.Collect()
	if err != nil {
		c.st.SetProblem(status.PhaseBlocked, "hwid",
			"Bu kompyuterning qurilma raqamini o'qib bo'lmadi.",
			"Dasturni administrator huquqi bilan bir marta ishga tushirib ko'ring. Muammo davom etsa, station.log faylini yuboring.",
			err.Error())
		log.Printf("hardware id: %v", err)
		return
	}
	keys, err := keystore.Open(c.stateDir)
	if err != nil {
		c.st.SetProblem(status.PhaseBlocked, "keystore",
			"Bu kompyuterda xavfsiz kalit saqlanmadi.",
			"C:\\ProgramData\\AvtoTest\\station papkasiga yozish huquqi borligini tekshiring.",
			err.Error())
		log.Printf("keystore: %v", err)
		return
	}

	name := c.label
	if name == "" {
		name, _ = os.Hostname()
	}

	a := &agent.Agent{APIBase: c.cfg.API, StateDir: c.stateDir, Keys: keys, HWID: id, Version: version}
	c.rt.setAgent(a)

	if c.reenroll {
		if c.cfg.Code == "" {
			c.st.SetProblem(status.PhaseBlocked, "no_code",
				"-reenroll uchun maktabning o'rnatish kaliti kerak.",
				"Admin paneldan shu maktab uchun yuklab olingan .exe ni ishga tushiring.",
				"reenroll without an installer key")
			return
		}
		if err := a.ResetEnrollment(); err != nil {
			c.st.SetProblem(status.PhaseBlocked, "reenroll",
				"Eski ro'yxatdan o'tish ma'lumotini o'chirib bo'lmadi.",
				"C:\\ProgramData\\AvtoTest\\station papkasidan station.json va station.key ni qo'lda o'chiring.",
				err.Error())
			return
		}
		log.Print("discarded the previous station identity; enrolling as a new station")
	}

	state := a.State()
	c.st.SetIdentity(displayOrg(state.Org, c.cfg.Org), state.StationID, state.Label)

	// A PC already enrolled somewhere, now holding a different school's
	// installer. The old agent ignored this completely -- it looked only at
	// whether station.json existed -- so the classroom silently kept running
	// as the previous school while the new school's panel showed nothing.
	if state.StationID != "" && differentSchool(state, c.cfg) {
		c.st.SetProblem(status.PhaseBlocked, "other_school",
			"Bu kompyuter boshqa avtomaktabga ("+orgLabel(state.Org)+") ro'yxatdan o'tgan, "+
				"lekin ishga tushirilgan fayl \""+orgLabel(c.cfg.Org)+"\" uchun.",
			"Yangi maktabga o'tkazish uchun: admin panelda ESKI maktabdan shu PC ulanishini bekor qiling, "+
				"so'ng shu faylni -reenroll bilan ishga tushiring. Aks holda kompyuter eski maktabda ishlashda davom etadi.",
			"enrolled org "+state.Org+" differs from installer org "+c.cfg.Org)
		log.Printf("this PC is enrolled to %q but the installer is for %q; keeping the existing enrollment",
			state.Org, c.cfg.Org)
		// Deliberately not returning: the existing enrollment may well be the
		// one the school actually wants running today, and cutting the
		// classroom off would help nobody. The token loop below keeps it
		// working while the warning stays on screen.
	}

	if _, err := a.Token(ctx); err != nil {
		if errors.Is(err, agent.ErrNotEnrolled) {
			if c.cfg.Code == "" {
				c.st.SetProblem(status.PhaseBlocked, "not_enrolled",
					"Bu kompyuter hech qaysi avtomaktabga ro'yxatdan o'tmagan.",
					"Admin paneldan shu maktab uchun yuklab olingan .exe faylni ishga tushiring.",
					"no installer key in this build and no saved enrollment")
				return
			}
			c.st.SetProblem(status.PhaseWaiting, "enrolling",
				"Kompyuter maktabga ro'yxatdan o'tkazilmoqda…",
				"Hech narsa qilish shart emas.",
				"")
			if err := enrollWithRetry(ctx, a, c.st, c.cfg.Code, name, c.cfg.Org, defaultEnrollRetry); err != nil {
				log.Printf("enrollment failed: %v", err)
				return
			}
			log.Printf("enrolled as %q", name)
			state = a.State()
			c.st.SetIdentity(displayOrg(state.Org, c.cfg.Org), state.StationID, state.Label)
		} else {
			off, known := a.ClockOffset()
			if !known {
				off = 0
			}
			d := diagnose.Token(err, off)
			c.st.SetProblem(d.Phase, d.Code, d.Problem, d.Action, d.Detail)
			log.Printf("station token unavailable at startup: %s | %s", d.Problem, d.Detail)
		}
	}

	keepTokenWarm(ctx, a, c.st, defaultTokenRetry)
}

// differentSchool reports whether the installer this binary carries belongs to
// a school other than the one this PC enrolled with.
//
// Both signals are optional, so the comparison is deliberately conservative:
// it fires only when there is positive evidence of a difference. State written
// by an agent older than this field carries neither, and a plain -code run
// carries no org name, and neither of those is a reason to alarm a working
// classroom.
func differentSchool(state agent.State, cfg resolved) bool {
	if state.CodeHash != "" && cfg.Code != "" {
		return agent.HashCode(cfg.Code) != state.CodeHash
	}
	if state.Org != "" && cfg.Org != "" {
		return state.Org != cfg.Org
	}
	return false
}

func displayOrg(saved, embedded string) string {
	if saved != "" {
		return saved
	}
	return embedded
}

func orgLabel(org string) string {
	if org == "" {
		return "nomi saqlanmagan maktab"
	}
	return org
}

// Command avtotest-station runs one classroom PC: it holds the station key,
// keeps an access token live, serves the browser from localhost and opens the
// kiosk.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/kiosk"
	"avtotest.uz/station/internal/proxy"
)

// version is stamped at build time: go build -ldflags "-X main.version=1.0.0"
var version = "dev"

// stationLocales mirrors frontend/src/i18n/config.ts's `locales` export.
// There is no "uz" locale and no rewrite for it — next-intl's
// localePrefix:"always" would just prefix a real locale onto an unknown one
// (e.g. "/uz-Latn/uz/station") and 404. Keep this list in sync with that
// file by hand; the two live in separate module trees with no shared build
// step to enforce it automatically.
var stationLocales = []string{"uz-Latn", "uz-Cyrl", "ru"}

// validLocale reports whether locale is one the frontend actually serves.
func validLocale(locale string) bool {
	for _, l := range stationLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// stationURL builds the kiosk landing page URL served at addr for locale.
func stationURL(addr, locale string) string {
	return fmt.Sprintf("http://%s/%s/station", addr, locale)
}

func main() {
	var (
		code     = flag.String("code", "", "one-time org enrollment code (first run only)")
		label    = flag.String("label", "", "PC name shown to the school (default: hostname)")
		apiBase  = flag.String("api", "https://api.avtotest.uz", "backend base URL")
		frontend = flag.String("frontend", "https://avtotest.uz", "frontend base URL")
		addr     = flag.String("addr", "127.0.0.1:17817", "local listen address")
		stateDir = flag.String("state", defaultStateDir(), "directory for the station key and state")
		noKiosk  = flag.Bool("no-kiosk", false, "serve only; do not launch a browser")
		locale   = flag.String("locale", "uz-Latn", "frontend locale the kiosk page opens in (uz-Latn, uz-Cyrl, ru)")
	)
	flag.Parse()

	if !validLocale(*locale) {
		log.Fatalf("invalid -locale %q: must be one of %s", *locale, strings.Join(stationLocales, ", "))
	}

	id, err := hwid.Collect()
	if err != nil {
		log.Fatalf("hardware id: %v", err)
	}
	keys, err := keystore.Open(*stateDir)
	if err != nil {
		log.Fatalf("keystore: %v", err)
	}
	name := *label
	if name == "" {
		name, _ = os.Hostname()
	}

	a := &agent.Agent{APIBase: *apiBase, StateDir: *stateDir, Keys: keys, HWID: id, Version: version}
	ctx := context.Background()

	if _, err := a.Token(ctx); err != nil {
		if errors.Is(err, agent.ErrNotEnrolled) {
			if *code == "" {
				log.Fatal("this PC is not enrolled yet: run again with -code AVTO-XXXX-XXXX")
			}
			// A first-boot GPO rollout runs this exe in the same cold-network
			// window as every later boot, so one failed attempt must not be
			// read as "the code is wrong". A one-time code doesn't deserve an
			// unbounded retry either, so this gives up loudly after a few
			// tries — there is nothing left to do automatically at that point.
			if err := enrollWithRetry(ctx, a, *code, name, defaultEnrollRetry); err != nil {
				log.Fatalf("enrollment failed: %v", err)
			}
			log.Printf("enrolled as %q", name)
			if _, err := a.Token(ctx); err != nil {
				log.Printf("first token fetch failed, will keep retrying in the background: %v", err)
			}
		} else {
			// Already enrolled — this machine has proved itself before, so an
			// unreachable backend right now (the classic cold-boot case: the
			// network adapter is not up yet when a startup program runs) is
			// not a reason to die. Serving is started below regardless; the
			// proxy already fails closed until keepTokenWarm lands a token,
			// and API calls start working the moment it does, with no
			// restart. This is deliberately not narrowed to "unreachable"
			// specifically: the backend's error envelope gives no reliable,
			// stable signal here to tell a network blip apart from a
			// server-side rejection (e.g. a revoked station), and exiting on
			// either one leaves the PC silently idle with no operator present
			// to notice. Serving on and retrying in the background is safe in
			// both cases — the kiosk visibly shows "station offline" instead
			// of nothing running at all.
			log.Printf("station token unavailable at startup, will keep retrying in the background: %v", err)
		}
	}
	go keepTokenWarm(ctx, a, defaultTokenRetry)

	handler := proxy.New(*frontend, *apiBase, a.Token)
	url := stationURL(*addr, *locale)

	if !*noKiosk {
		if _, err := kiosk.Launch(url); err != nil {
			log.Printf("kiosk launch: %v (open %s manually)", err, url)
		}
	}
	log.Printf("avtotest-station %s serving %s", version, url)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// defaultStateDir keeps the key beside the program data, not in a user
// profile, because the agent runs as a machine service.
func defaultStateDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return filepath.Join(dir, "AvtoTest", "station")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".avtotest-station"
	}
	return filepath.Join(dir, "avtotest-station")
}

// enrollSchedule bounds how hard enrollWithRetry tries before giving up.
type enrollSchedule struct {
	attempts int
	initial  time.Duration
	max      time.Duration
}

// defaultEnrollRetry rides out a cold-boot network window without blocking a
// headless install for long. Four attempts doubling from 3s (3+6+12s between
// tries, capped at 30s) cover roughly the first minute after the process
// starts — generous compared to how quickly a NIC normally comes up via
// DHCP, but still short enough that an operator watching a first install
// isn't left staring at a hung terminal.
var defaultEnrollRetry = enrollSchedule{attempts: 4, initial: 3 * time.Second, max: 30 * time.Second}

// enrollWithRetry calls Agent.Enroll up to sched.attempts times with
// exponential backoff, so a transient network failure during the first-boot
// enrollment window is not mistaken for a bad one-time code. It gives up and
// returns the last error once the schedule is exhausted — there is no
// automatic recovery from a genuinely wrong code, and main treats that as
// fatal.
func enrollWithRetry(ctx context.Context, a *agent.Agent, code, label string, sched enrollSchedule) error {
	backoff := sched.initial
	var err error
	for attempt := 1; attempt <= sched.attempts; attempt++ {
		if err = a.Enroll(ctx, code, label); err == nil {
			return nil
		}
		if attempt == sched.attempts {
			break
		}
		log.Printf("enrollment attempt %d/%d failed, retrying in %s: %v", attempt, sched.attempts, backoff, err)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		if backoff *= 2; backoff > sched.max {
			backoff = sched.max
		}
	}
	return err
}

// tokenSchedule bounds keepTokenWarm's backoff while the backend cannot be
// reached, and its poll interval once a token is live.
type tokenSchedule struct {
	initial time.Duration
	max     time.Duration
	steady  time.Duration
}

// defaultTokenRetry starts retrying quickly (5s) so a station that comes up
// mid-morning is only briefly stuck, and doubles up to a 2-minute ceiling so
// a station that stays unreachable all day — network down, or genuinely
// rejected server-side — polls at a harmless, bounded rate instead of
// hammering the backend forever. 30s once healthy is well under
// tokenRenewMargin (2 minutes), so renewal never gets a chance to lapse.
var defaultTokenRetry = tokenSchedule{initial: 5 * time.Second, max: 2 * time.Minute, steady: 30 * time.Second}

// keepTokenWarm runs for the life of the process, keeping a's token fresh.
// Agent.Token already no-ops when the cached token is not close to expiry,
// so the steady-state poll costs nothing extra; while the backend cannot be
// reached it backs off instead of spinning. It never returns on error and
// never calls the process fatal: the proxy fails closed on its own until a
// token lands, which is what makes leaving this to retry, rather than
// exiting, safe on an unattended machine.
func keepTokenWarm(ctx context.Context, a *agent.Agent, sched tokenSchedule) {
	backoff := sched.initial
	for {
		if _, err := a.Token(ctx); err != nil {
			log.Printf("station token unavailable, retrying in %s: %v", backoff, err)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff *= 2; backoff > sched.max {
				backoff = sched.max
			}
			continue
		}
		backoff = sched.initial
		select {
		case <-time.After(sched.steady):
		case <-ctx.Done():
			return
		}
	}
}

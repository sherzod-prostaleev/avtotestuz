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
	"avtotest.uz/station/internal/embedcfg"
	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/kiosk"
	"avtotest.uz/station/internal/proxy"
	"avtotest.uz/station/internal/selfinstall"
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
// The empty string passes: it means "no locale forced", which is the default
// and the case that lets the student's own choice stick (see stationURL).
func validLocale(locale string) bool {
	if locale == "" {
		return true
	}
	for _, l := range stationLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// stationURL builds the kiosk landing page URL served at addr.
//
// With locale empty -- the default -- the path carries no locale prefix and
// next-intl redirects it to whatever NEXT_LOCALE the student last picked in
// the kiosk's own language switcher, falling back to uz-Latn on a PC nobody
// has chosen on yet. That is what makes the choice survive a reboot: baking a
// prefix in would reset the classroom every morning to whatever was selected
// at download time, which is exactly the admin-decides-for-the-student
// behaviour the switcher replaced. A non-empty locale (only ever set by an
// explicit -locale flag) still forces one, for a school that wants a specific
// language on first run.
func stationURL(addr, locale string) string {
	if locale == "" {
		return fmt.Sprintf("http://%s/station", addr)
	}
	return fmt.Sprintf("http://%s/%s/station", addr, locale)
}

// resolved is the agent's effective configuration after merging what was baked
// into this copy with what the operator passed on the command line.
type resolved struct {
	Code     string
	API      string
	Frontend string
	Locale   string
	Org      string
}

// resolveConfig merges an embedded config with flag values. An explicitly-set
// flag always wins, so one PC can be pointed at staging without a rebuild;
// otherwise the embedded value wins over the compiled-in default, which is what
// makes a downloaded installer need no arguments at all.
func resolveConfig(embedded embedcfg.Config, flagCode, flagAPI, flagFrontend, flagLocale string, apiSet, frontendSet, localeSet bool) resolved {
	out := resolved{
		Code: flagCode, API: flagAPI, Frontend: flagFrontend,
		Locale: flagLocale, Org: embedded.Org,
	}
	if out.Code == "" {
		out.Code = embedded.Code
	}
	if !apiSet && embedded.API != "" {
		out.API = embedded.API
	}
	if !frontendSet && embedded.Frontend != "" {
		out.Frontend = embedded.Frontend
	}
	if !localeSet && embedded.Locale != "" {
		out.Locale = embedded.Locale
	}
	return out
}

func main() {
	var (
		code     = flag.String("code", "", "org installer key (reusable for the life of the licence, not a one-time code)")
		label    = flag.String("label", "", "PC name shown to the school (default: hostname)")
		apiBase  = flag.String("api", "https://drivergo.uz", "backend base URL")
		frontend = flag.String("frontend", "https://drivergo.uz", "frontend base URL")
		addr     = flag.String("addr", "127.0.0.1:17817", "local listen address")
		stateDir = flag.String("state", defaultStateDir(), "directory for the station key and state")
		noKiosk  = flag.Bool("no-kiosk", false, "serve only; do not launch a browser")
		locale   = flag.String("locale", "", "force the locale the kiosk opens in (uz-Latn, uz-Cyrl, ru); empty means the student's own choice in the kiosk decides")

		selfTest       = flag.Bool("selftest", false, "run hwid/keystore checks in a scratch directory and print a pass/fail verdict, then exit; does not touch the real enrollment")
		selfTestImport = flag.String("selftest-import", "", "path to a station.key copied from another machine; try to unseal it here and report whether the machine binding held, then exit")
		uninstall      = flag.Bool("uninstall", false, "remove the installed copy and autostart entry, then exit (does not free this station's seat -- revoke it in the admin panel too)")
		reenroll       = flag.Bool("reenroll", false, "discard this PC's station identity and enrol again as a new station; use when the backend no longer recognises it (the school was deleted and recreated). Spends a seat, so revoke the dead station in the admin panel too")
	)
	flag.Parse()

	if *selfTestImport != "" {
		os.Exit(runSelfTestImport(*selfTestImport))
	}
	if *selfTest {
		os.Exit(runSelfTest())
	}

	if *uninstall {
		if err := selfinstall.Remove(*stateDir); err != nil {
			log.Fatalf("uninstall: %v", err)
		}
		fmt.Printf("Uninstalled: removed the autostart entry and deleted %s plus this station's saved key and state\n", selfinstall.Target(*stateDir))
		fmt.Println("This only removes local files -- it does not free this station's seat.")
		fmt.Println("Revoke this station in the admin panel too, or the licence stays held.")
		return
	}

	embedded := embedcfg.Config{}
	if exe, err := os.Executable(); err == nil {
		if cfg, err := embedcfg.Read(exe); err == nil {
			embedded = cfg
		} else if !errors.Is(err, embedcfg.ErrNoConfig) {
			log.Printf("embedded config: %v (falling back to flags)", err)
		}
	}

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	cfg := resolveConfig(embedded, *code, *apiBase, *frontend, *locale,
		set["api"], set["frontend"], set["locale"])

	if !validLocale(cfg.Locale) {
		log.Fatalf("invalid -locale %q: must be one of %s", cfg.Locale, strings.Join(stationLocales, ", "))
	}

	if embedded.Code != "" {
		installed, didInstall, err := selfinstall.Ensure(*stateDir)
		if err != nil {
			log.Printf("self-install: %v (continuing from the current location)", err)
		} else if didInstall {
			log.Printf("installed to %s and registered autostart", installed)
		}
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

	a := &agent.Agent{APIBase: cfg.API, StateDir: *stateDir, Keys: keys, HWID: id, Version: version}
	ctx := context.Background()

	if *reenroll {
		if cfg.Code == "" {
			log.Fatal("-reenroll needs an installer key: run the .exe downloaded for this school, or pass -code AVTO-XXXX-XXXX")
		}
		if err := a.ResetEnrollment(); err != nil {
			log.Fatalf("reenroll: %v", err)
		}
		log.Print("discarded the previous station identity; enrolling as a new station")
	}

	if _, err := a.Token(ctx); err != nil {
		if errors.Is(err, agent.ErrNotEnrolled) {
			if cfg.Code == "" {
				log.Fatal("this PC is not enrolled yet: run again with -code AVTO-XXXX-XXXX")
			}
			// A first-boot GPO rollout runs this exe in the same cold-network
			// window as every later boot, so one failed attempt must not be
			// read as "the code is wrong". A one-time code doesn't deserve an
			// unbounded retry either, so this gives up loudly after a few
			// tries — there is nothing left to do automatically at that point.
			if err := enrollWithRetry(ctx, a, cfg.Code, name, defaultEnrollRetry); err != nil {
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
			if errors.Is(err, agent.ErrStationUnauthorized) {
				// The backend answers "unknown station", "revoked station"
				// and "bad signature" with one opaque code, so the agent
				// cannot tell a school that was deleted and recreated from a
				// PC an admin deliberately switched off. Enrolling again
				// would spend a seat and quietly undo a revoke, so it stays a
				// human decision -- but the console has to say which decision
				// it is, instead of repeating "authentication failed" until
				// someone gives up.
				log.Print("the backend does not recognise this PC's enrollment.")
				log.Print("  - if this school was deleted and recreated, or the station was removed: re-run this .exe with -reenroll")
				log.Print("  - if an admin revoked this PC on purpose: leave it; re-enrolling would undo that and spend a seat")
				log.Printf("  (retrying in the background in case this is temporary: %v)", err)
			} else {
				log.Printf("station token unavailable at startup, will keep retrying in the background: %v", err)
			}
		}
	}
	go keepTokenWarm(ctx, a, defaultTokenRetry)

	handler := proxy.New(cfg.Frontend, cfg.API, a.Token)
	url := stationURL(*addr, cfg.Locale)

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
			if errors.Is(err, agent.ErrStationUnauthorized) {
				// Repeating the full diagnosis every few seconds buries it.
				// The startup path already printed what to do about this one.
				log.Printf("station rejected by the backend, retrying in %s (see -reenroll above)", backoff)
			} else {
				log.Printf("station token unavailable, retrying in %s: %v", backoff, err)
			}
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

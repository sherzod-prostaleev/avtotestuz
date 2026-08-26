// Command avtotest-station runs one classroom PC: it holds the station key,
// keeps an access token live, serves the browser from localhost and opens the
// kiosk.
//
// Startup order matters more than anything else in this file. The listener is
// bound and serving BEFORE the browser is launched and before the first
// network call, and nothing after that point is allowed to end the process.
// The previous order -- enrol, then launch the browser, then bind -- meant a
// school whose PC could not enrol saw four retries, then a console that sat
// for two more minutes on "Press Enter to close", and never got a page at all.
// Now the kiosk always opens, and whatever is wrong is on the screen in Uzbek
// instead of in a window nobody was looking at.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/diagnose"
	"avtotest.uz/station/internal/embedcfg"
	"avtotest.uz/station/internal/kiosk"
	"avtotest.uz/station/internal/proxy"
	"avtotest.uz/station/internal/selfinstall"
	"avtotest.uz/station/internal/status"
	"avtotest.uz/station/internal/updater"
)

// version is stamped at build time from backend/station/VERSION; see
// backend/Dockerfile. "dev" means someone built this with a plain go build.
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
		showVersion    = flag.Bool("version", false, "print the agent version and exit")
		noUpdate       = flag.Bool("no-update", false, "do not check for or install new agent builds")
	)
	flag.Parse()

	// Every mode below that talks to a human needs a console, and this binary
	// is linked as a GUI application so that a classroom PC shows no black
	// window. attachConsole gets one back when the operator started us from
	// cmd; it is a no-op off Windows.
	if *showVersion || *selfTest || *selfTestImport != "" || *uninstall {
		attachConsole()
	}

	if *showVersion {
		fmt.Printf("avtotest-station %s\n", version)
		return
	}

	logPath := startLogging(*stateDir)

	if *selfTestImport != "" {
		os.Exit(runSelfTestImport(*selfTestImport))
	}
	if *selfTest {
		os.Exit(runSelfTest())
	}

	if *uninstall {
		if err := selfinstall.Remove(*stateDir); err != nil {
			fatal("uninstall: %v", err)
		}
		fmt.Printf("Uninstalled: removed the autostart entry and deleted %s plus this station's saved key and state\n", selfinstall.Target(*stateDir))
		fmt.Println("This only removes local files -- it does not free this station's seat.")
		fmt.Println("Revoke this station in the admin panel too, or the licence stays held.")
		return
	}

	embedded := embedcfg.Config{}
	exePath, _ := os.Executable()
	if exePath != "" {
		if c, err := embedcfg.Read(exePath); err == nil {
			embedded = c
		} else if !errors.Is(err, embedcfg.ErrNoConfig) {
			log.Printf("embedded config: %v (falling back to flags)", err)
		}
	}

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	cfg := resolveConfig(embedded, *code, *apiBase, *frontend, *locale,
		set["api"], set["frontend"], set["locale"])

	if !validLocale(cfg.Locale) {
		fatal("invalid -locale %q: must be one of %s", cfg.Locale, strings.Join(stationLocales, ", "))
	}

	// Bind before anything else can fail. From here on the kiosk has
	// somewhere to connect to no matter what the network, the backend or the
	// school's licence turn out to be doing.
	ln, listenAddr, err := listen(*addr)
	if err != nil {
		// Only reachable when every candidate port is taken by something
		// that is not one of us -- there is genuinely nothing left to serve
		// from, so this is the one startup failure that still ends the
		// process.
		fatal("cannot serve on %s: %v", *addr, err)
	}
	if ln == nil {
		// Another copy of the agent already holds the port and is answering.
		// Opening the browser at it is exactly what the operator wanted when
		// they double-clicked, so do that instead of dying with a port
		// conflict the way this used to.
		url := stationURL(listenAddr, cfg.Locale)
		log.Printf("an agent is already running on %s; opening the kiosk there", listenAddr)
		if !*noKiosk {
			openKiosk(url)
		}
		return
	}
	st := status.New(version, logPath, listenAddr)
	rt := &agentRuntime{}

	handler := proxy.New(cfg.Frontend, cfg.API, rt.token, st)
	srv := &http.Server{Handler: rt.trackIdle(handler)}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("serving stopped: %v", err)
		}
	}()
	log.Printf("avtotest-station %s serving %s (log: %s)", version, listenAddr, logPath)

	url := stationURL(listenAddr, cfg.Locale)
	if !*noKiosk {
		openKiosk(url)
	}

	// Cosmetic and known to block: createShortcut shells out to PowerShell,
	// which on a fresh school PC can sit for a long time behind an antivirus
	// inspecting a freshly-downloaded unsigned binary that spawned
	// `powershell -ExecutionPolicy Bypass`. It used to run inline, ahead of
	// everything, so that stall took the whole agent with it.
	if embedded.Code != "" {
		go install(*stateDir)
	}

	ctx := context.Background()
	go connect(ctx, connectConfig{
		rt:       rt,
		st:       st,
		cfg:      cfg,
		stateDir: *stateDir,
		label:    *label,
		reenroll: *reenroll,
	})

	// Reports what this PC is doing to the school's admin panel. Gated on an
	// embedded key for the same reason self-install is: a plain development
	// build belongs to no school and has nothing to file under.
	if embedded.Code != "" {
		go runReporter(ctx, rt, st, cfg, logPath, defaultReportSchedule)
	}

	if !*noUpdate && embedded.Code != "" {
		go updater.Run(ctx, updater.Config{
			APIBase:  cfg.API,
			Version:  version,
			Target:   selfinstall.Target(*stateDir),
			StateDir: *stateDir,
			Report:   st.SetUpdateState,
			Idle:     rt.idleFor,
			Restart:  func() { restart(selfinstall.Target(*stateDir), srv) },
		})
	}

	select {}
}

// agentRuntime is the mutable state the HTTP handler shares with the
// background worker that owns the network side.
type agentRuntime struct {
	mu sync.RWMutex
	ag *agent.Agent

	// lastCall is the Unix nano of the most recent proxied API request. The
	// updater reads it to decide whether restarting could interrupt a student
	// mid-exam.
	//
	// atomic.Int64, not a plain int64 read through atomic.StoreInt64. On 386 --
	// which is what every classroom PC runs, because a 386 binary is the one
	// build that works on both 32- and 64-bit Windows -- the compiler aligns an
	// int64 struct field to 4 bytes, while a 64-bit atomic operation requires 8.
	// Behind a sync.RWMutex and a pointer this field landed at offset 28, and
	// the very first atomic.StoreInt64 panicked with "unaligned 64-bit atomic
	// operation". On amd64 the same field sits at offset 32 and every test
	// passed. atomic.Int64 carries its own alignment guarantee, so no future
	// reordering of the fields above can put this back.
	lastCall atomic.Int64
}

// errNotReady is what the proxy sees before enrollment has produced an agent.
// The proxy already fails closed on any token error, so this only changes the
// wording in the log, never the behaviour.
var errNotReady = errors.New("station is still starting up")

func (r *agentRuntime) token(ctx context.Context) (string, error) {
	r.mu.RLock()
	a := r.ag
	r.mu.RUnlock()
	if a == nil {
		return "", errNotReady
	}
	return a.Token(ctx)
}

// agent returns the live agent, or nil before enrolment has produced one.
func (r *agentRuntime) agent() *agent.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ag
}

func (r *agentRuntime) setAgent(a *agent.Agent) {
	r.mu.Lock()
	r.ag = a
	r.mu.Unlock()
}

// trackIdle stamps every proxied API call so the updater can tell an empty
// classroom from one in the middle of an exam. Only /api/proxy/ counts: the
// browser fetches static assets on its own schedule and would keep the PC
// looking busy forever.
func (r *agentRuntime) trackIdle(next http.Handler) http.Handler {
	r.lastCall.Store(time.Now().UnixNano())
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/proxy/") {
			r.lastCall.Store(time.Now().UnixNano())
		}
		next.ServeHTTP(w, req)
	})
}

func (r *agentRuntime) idleFor() time.Duration {
	return time.Since(time.Unix(0, r.lastCall.Load()))
}

// listen binds want, falling back to the next few ports when it is taken.
//
// Three outcomes. A bound listener means we are the agent for this PC. A nil
// listener with no error means another agent already answers on want and the
// caller should just open the browser at it. An error means every candidate
// port is held by something else entirely.
func listen(want string) (net.Listener, string, error) {
	ln, err := net.Listen("tcp", want)
	if err == nil {
		return ln, want, nil
	}
	if agentAnswersOn(want) {
		return nil, want, nil
	}

	host, portStr, splitErr := net.SplitHostPort(want)
	if splitErr != nil {
		return nil, "", err
	}
	var port int
	if _, sErr := fmt.Sscanf(portStr, "%d", &port); sErr != nil {
		return nil, "", err
	}
	for next := port + 1; next <= port+9; next++ {
		cand := net.JoinHostPort(host, fmt.Sprintf("%d", next))
		if ln, lErr := net.Listen("tcp", cand); lErr == nil {
			log.Printf("%s was taken; serving on %s instead", want, cand)
			return ln, cand, nil
		}
	}
	return nil, "", err
}

// agentAnswersOn reports whether the process holding addr is one of ours,
// identified by the status route only this program serves.
func agentAnswersOn(addr string) bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + proxy.StatusPath)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK &&
		strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json")
}

func openKiosk(url string) {
	if _, err := kiosk.Launch(url); err != nil {
		log.Printf("kiosk launch: %v", err)
		log.Printf("open this address in a browser by hand: %s", url)
	}
}

// install copies this binary into the state directory and registers autostart,
// then places a desktop shortcut. Runs in its own goroutine: none of it is
// required for the kiosk to work, and all of it can block.
func install(stateDir string) {
	installed, didInstall, err := selfinstall.Ensure(stateDir)
	if err != nil {
		log.Printf("self-install: %v (continuing from the current location)", err)
		return
	}
	if didInstall {
		log.Printf("installed %s to %s and registered autostart", version, installed)
	}
	// A locked-down classroom profile may refuse to write to the desktop, and
	// the kiosk works perfectly well without an icon.
	if path, created, sErr := selfinstall.EnsureShortcut(installed); sErr != nil {
		log.Printf("desktop shortcut: %v (continuing without one)", sErr)
	} else if created {
		log.Printf("placed a DriverGo shortcut at %s", path)
	}
}

// restart hands over to the binary at target and exits.
//
// Order matters and is not the obvious one. The listener must be released
// BEFORE the replacement starts: a new agent that finds the port still held
// probes it, recognises the answer as one of ours, decides an agent is already
// running and exits after opening the browser (see listen). Starting the child
// first would therefore end with the child gone, this process exiting a moment
// later, and the classroom left with no agent at all.
//
// The kiosk page in the browser retries on its own, so a student sees at most a
// moment of the "Ulanmoqda…" screen -- and maybeRestart only calls this when
// nobody has touched the API for half an hour anyway.
func restart(target string, srv *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	cmd := exec.Command(target)
	cmd.Dir = filepath.Dir(target)
	if err := cmd.Start(); err != nil {
		// The port is already released and this process is about to be
		// useless, so staying alive would just hold the state directory. The
		// autostart entry brings the new binary up at the next logon.
		log.Printf("restart into %s failed: %v (the update lands at the next boot)", target, err)
		os.Exit(1)
	}
	log.Printf("handed over to the updated agent, exiting")
	os.Exit(0)
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

// defaultEnrollRetry rides out a cold-boot network window. A classroom PC
// starts the agent from autostart while the network adapter is still coming
// up, so the first attempts routinely fail on a machine that is perfectly
// healthy. Twelve attempts doubling from 3s to a 60s ceiling covers roughly
// the first ten minutes; unlike the four attempts this used to make, that
// outlasts a slow DHCP lease, a school router rebooting and a teacher
// switching the Wi-Fi on after the PCs.
//
// Nothing here rides out a permanent error, though: diagnose.Enroll marks
// those non-retryable and the loop stops on the first one, which is what
// turned "conflict, four times, then a dead console" into one clear sentence.
var defaultEnrollRetry = enrollSchedule{attempts: 12, initial: 3 * time.Second, max: 60 * time.Second}

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

// enrollWithRetry calls Agent.Enroll until it succeeds, the schedule is
// exhausted, or the failure is one that retrying cannot fix. It reports every
// state it passes through to st, because with no console that store is the
// only way the school finds out what is happening.
func enrollWithRetry(ctx context.Context, a *agent.Agent, st *status.Store, code, label, org string, sched enrollSchedule) error {
	backoff := sched.initial
	var err error
	for attempt := 1; attempt <= sched.attempts; attempt++ {
		if err = a.Enroll(ctx, code, label, org); err == nil {
			return nil
		}
		d := diagnose.Enroll(err)
		st.SetProblem(d.Phase, d.Code, d.Problem, d.Action, d.Detail)
		if !d.Retryable {
			log.Printf("enrollment refused: %s | %s", d.Problem, d.Detail)
			return err
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

// keepTokenWarm runs for the life of the process, keeping a's token fresh.
// Agent.Token already no-ops when the cached token is not close to expiry,
// so the steady-state poll costs nothing extra; while the backend cannot be
// reached it backs off instead of spinning. It never returns on error and
// never calls the process fatal: the proxy fails closed on its own until a
// token lands, which is what makes leaving this to retry, rather than
// exiting, safe on an unattended machine.
func keepTokenWarm(ctx context.Context, a *agent.Agent, st *status.Store, sched tokenSchedule) {
	backoff := sched.initial
	var lastCode string
	for {
		// Renew, not Token: this loop is the paced caller the failure cache is
		// meant to protect, not one of the callers it is meant to restrain.
		if _, err := a.Renew(ctx); err != nil {
			off, known := a.ClockOffset()
			if !known {
				off = 0
			}
			d := diagnose.Token(err, off)
			st.SetProblem(d.Phase, d.Code, d.Problem, d.Action, d.Detail)
			// Repeating the full diagnosis every few seconds buries it; the
			// state is on the kiosk screen either way.
			if d.Code != lastCode {
				log.Printf("station offline: %s | %s | %s", d.Problem, d.Action, d.Detail)
				lastCode = d.Code
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
		if lastCode != "" {
			log.Print("station is online again")
			lastCode = ""
		}
		st.SetReady()
		backoff = sched.initial
		select {
		case <-time.After(sched.steady):
		case <-ctx.Done():
			return
		}
	}
}

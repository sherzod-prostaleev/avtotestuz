// Package kiosk starts the local browser on the classroom station page.
package kiosk

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrNoBrowser means no browser could be started at all.
var ErrNoBrowser = errors.New("no browser could be started")

// candidates lists Chromium-family browsers in preference order.
//
// The per-user paths matter as much as the Program Files ones: Chrome's own
// installer puts the browser under %LOCALAPPDATA% whenever it is run by
// someone without administrator rights, which is the normal case on a
// locked-down classroom PC. Yandex Browser is here because it is genuinely
// common in Uzbekistan, and Opera because it ships on a lot of prebuilt
// machines. Anything Chromium-based accepts --app, which is what strips the
// tabs, the address bar and the bookmarks.
func candidates() []string {
	if runtime.GOOS != "windows" {
		return []string{"google-chrome", "chromium", "chromium-browser"}
	}
	local := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	if programFilesX86 == "" {
		programFilesX86 = `C:\Program Files (x86)`
	}

	var out []string
	add := func(parts ...string) {
		if parts[0] == "" {
			return
		}
		out = append(out, filepath.Join(parts...))
	}
	add(programFiles, `Google\Chrome\Application\chrome.exe`)
	add(programFilesX86, `Google\Chrome\Application\chrome.exe`)
	add(local, `Google\Chrome\Application\chrome.exe`)
	add(programFilesX86, `Microsoft\Edge\Application\msedge.exe`)
	add(programFiles, `Microsoft\Edge\Application\msedge.exe`)
	add(local, `Yandex\YandexBrowser\Application\browser.exe`)
	add(programFilesX86, `Yandex\YandexBrowser\Application\browser.exe`)
	add(programFiles, `Opera\opera.exe`)
	add(local, `Programs\Opera\opera.exe`)
	return out
}

// Launch opens url in an app window and returns the running process.
//
// --app without --kiosk on purpose. Full-screen kiosk mode covers the whole
// screen with no title bar, so a student cannot minimise, resize or close the
// window -- on a shared classroom PC that reads as the machine being stuck
// rather than as a focused exam, and there is no way back to Windows without
// knowing a keyboard shortcut. --app still drops the tabs, the address bar and
// the bookmarks, so there is nowhere else to browse, but keeps the ordinary
// minimise / maximise / close controls.
//
// The window fills the screen but stops at the taskbar, and keeps its title
// bar and its minimise/maximise/close buttons -- the same reasoning that
// rejected --kiosk above: a teacher has to be able to get back to Windows
// without knowing F11. It replaces a hard-coded --window-size=1280,860 that
// opened a small window in the middle of every classroom screen, which
// somebody was maximising by hand on every PC, every morning.
//
// It asks for that in two ways on purpose, and the second one is the one that
// matters. --start-maximized alone shipped in 1.3.2 and was not enough: on
// Windows the first launch came up maximised and every launch after it was
// small again, because Chrome restores the placement it saved for this app and
// that flag does not reliably win against it. --window-size does win -- the
// 1280x860 above proved it, every launch, for weeks -- so the size is also
// stated explicitly, taken from the monitor's work area (see workArea).
//
// Do not "simplify" this back to one flag. The Linux desktop this is developed
// on honours --start-maximized on every launch, so a local test cannot tell the
// two apart; only a Windows classroom PC can.
//
// When no Chromium-family browser is found it falls back to the machine's
// default browser, which loses the app window but is enormously better than
// the previous behaviour: a single English line in a log nobody reads and a PC
// where visibly nothing happened.
func Launch(url string) (*exec.Cmd, error) {
	for _, bin := range candidates() {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, browserArgs(url)...)
		if err := cmd.Start(); err != nil {
			continue
		}
		return cmd, nil
	}
	if cmd, ok := launchDefault(url); ok {
		return cmd, nil
	}
	return nil, ErrNoBrowser
}

// browserArgs is the command line Launch hands the browser, split out so the
// window flags can be asserted without starting one.
func browserArgs(url string) []string {
	args := []string{
		"--app=" + url,
		"--start-maximized",
		"--no-first-run",
		"--disable-session-crashed-bubble",
		"--disable-features=TranslateUI",
	}
	if x, y, w, h, ok := workArea(); ok {
		args = append(args,
			fmt.Sprintf("--window-position=%d,%d", x, y),
			fmt.Sprintf("--window-size=%d,%d", w, h),
		)
	}
	return args
}

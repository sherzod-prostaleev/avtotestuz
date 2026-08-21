// Package kiosk starts the local browser on the classroom station page.
package kiosk

import (
	"errors"
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
		cmd := exec.Command(path,
			"--app="+url,
			"--window-size=1280,860",
			"--no-first-run",
			"--disable-session-crashed-bubble",
			"--disable-features=TranslateUI",
		)
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

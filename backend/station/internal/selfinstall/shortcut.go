package selfinstall

import "fmt"

// shortcutName is what a student sees on the classroom desktop.
const shortcutName = "DriverGo"

// createShortcutFn is createShortcut by default. Tests substitute it, for the
// same reason registerAutostartFn exists: createShortcut is a no-op off
// Windows, and a no-op leaves no side effect a test could check.
var createShortcutFn = createShortcut

// EnsureShortcut puts a DriverGo shortcut on the shared desktop, pointing at
// the installed agent.
//
// Only when one is not already there. Autostart is re-registered on every
// boot because losing it silently breaks the product; a desktop shortcut is a
// convenience, and a school that deleted it on purpose should not find it
// back every morning.
//
// A failure here is never fatal to the caller: the kiosk works without an
// icon, and a locked-down classroom profile may simply refuse to write to the
// desktop.
func EnsureShortcut(target string) (path string, created bool, err error) {
	path, created, err = createShortcutFn(target)
	if err != nil {
		return "", false, fmt.Errorf("selfinstall: desktop shortcut: %w", err)
	}
	return path, created, nil
}

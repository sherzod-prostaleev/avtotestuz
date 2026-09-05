//go:build !windows

package kiosk

// workArea is answered only on Windows, the one platform a classroom PC runs.
// Everywhere else -- a developer's desktop -- Launch falls back to
// --start-maximized alone, which those window managers honour.
func workArea() (x, y, w, h int, ok bool) { return 0, 0, 0, 0, false }

//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The agent is linked with -H windowsgui so a classroom PC never shows a black
// console window (see backend/Dockerfile). That costs two things back, and
// this file buys both of them:
//
//   - the diagnostic modes (-selftest, -selftest-import, -uninstall, -version)
//     print to a terminal, and a GUI process starts with none. attachConsole
//     borrows the cmd window the operator launched it from, so those modes
//     behave exactly as they did before.
//   - a fatal error printed to a stream nobody is reading is a fatal error
//     nobody sees. showFatal puts it in a message box instead.
//
// Both are done by hand through the system DLLs rather than by adding a
// dependency: the module is pinned to Go 1.20 for Windows 7 and every extra
// package is another thing that can quietly stop supporting it.

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	user32            = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
)

// attachParentProcess is ATTACH_PARENT_PROCESS: use the console of whatever
// started us, if it has one.
const attachParentProcess = ^uintptr(0) // (DWORD)-1

// messageBoxIconError is MB_OK | MB_ICONERROR | MB_SETFOREGROUND.
const messageBoxIconError = 0x00000000 | 0x00000010 | 0x00010000

// attachConsole reconnects stdout and stderr to the console this process was
// launched from. It is a no-op when there is none -- a double-clicked icon, a
// scheduled task -- in which case the diagnostic modes still write their
// findings to station.log.
func attachConsole() {
	r, _, _ := procAttachConsole.Call(attachParentProcess)
	if r == 0 {
		return
	}
	// The Go runtime captured the (invalid) handles at startup, so reopen the
	// standard streams against the console we just acquired.
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}

// showFatal puts msg in front of whoever is standing at the PC.
func showFatal(msg string) {
	title, err := syscall.UTF16PtrFromString("DriverGo — sinfxona dasturi ishga tushmadi")
	if err != nil {
		return
	}
	body, err := syscall.UTF16PtrFromString(msg + "\n\nBatafsil ma'lumot: " + logHint())
	if err != nil {
		return
	}
	_, _, _ = procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), messageBoxIconError)
}

func logHint() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return dir + `\AvtoTest\station\station.log`
	}
	return "station.log"
}

//go:build !windows

package main

// Off Windows the process always has whatever streams it was started with, and
// there is no message box to put an error in -- stderr and station.log are
// already the whole story. Both hooks exist so main.go does not need a
// runtime.GOOS branch on the startup path.

func attachConsole() {}

func showFatal(string) {}

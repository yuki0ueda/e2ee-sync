//go:build windows

package main

import "syscall"

// detachConsole detaches from the parent console on Windows.
// Called in daemon mode so no console window remains visible.
func detachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	freeConsole := kernel32.NewProc("FreeConsole")
	freeConsole.Call()
}

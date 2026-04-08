//go:build !windows

package main

func detachConsole() {
	// No-op on non-Windows platforms
}

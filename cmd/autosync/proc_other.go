//go:build !windows

package main

import "os/exec"

func hideChildWindow(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}

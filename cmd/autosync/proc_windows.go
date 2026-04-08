//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideChildWindow prevents console windows from appearing when
// launching child processes (e.g., rclone) from a GUI application.
func hideChildWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

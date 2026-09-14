//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process with this PID exists. Signal 0 checks
// existence without delivering anything; EPERM means it exists but belongs to
// another user.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

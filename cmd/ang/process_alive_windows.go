//go:build windows

package main

// Without a probe that is reliable for exited processes, every owner counts as
// alive: an abandoned scratch directory is left behind rather than a running
// build's deleted.
func processAlive(pid int) bool {
	return true
}

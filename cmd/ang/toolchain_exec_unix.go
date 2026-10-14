//go:build unix

package main

import "syscall"

// execToolchain replaces this process with the toolchain binary: same pid,
// stdio, signals and exit status. It returns only when the exec failed.
func execToolchain(bin string, args, env []string) error {
	return syscall.Exec(bin, append([]string{bin}, args...), env)
}

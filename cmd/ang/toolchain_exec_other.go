//go:build !unix

package main

import (
	"errors"
	"os"
	"os/exec"
)

// execToolchain runs the toolchain binary with this process's stdio and exits
// with its status. It returns only when the binary could not be started.
func execToolchain(bin string, args, env []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
